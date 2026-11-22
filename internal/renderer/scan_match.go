package renderer

import "strings"

// MatchVisitor receives every two-argument match filter found by the scanners.
// pattern is the unquoted pattern when literal is true; otherwise the pattern
// argument is not a quoted literal (a variable or a nested pipeline).
type MatchVisitor func(pattern string, literal bool)

// ScanMatchInterpolated finds the match filters in all {{…}} pipelines of s,
// including pipelines nested in filter arguments. The device exporter uses it
// to collect the constant regex patterns of a template.
func ScanMatchInterpolated(s string, visit MatchVisitor) {
	for _, m := range interpolateRe.FindAllStringSubmatch(s, -1) {
		scanPipelineMatches(strings.TrimSpace(m[1]), 0, visit)
	}
}

// ScanMatchCondition finds the match filters in a condition expression
// (if/elif), which uses function-call syntax without {{ }}.
func ScanMatchCondition(expr string, visit MatchVisitor) {
	scanCondMatches(expr, 0, visit)
	ScanMatchInterpolated(expr, visit)
}

func scanCondMatches(expr string, depth int, visit MatchVisitor) {
	if depth > 2*maxCondDepth {
		return
	}
	expr = strings.TrimSpace(expr)
	if strings.HasPrefix(expr, "not(") && strings.HasSuffix(expr, ")") {
		scanCondMatches(expr[4:len(expr)-1], depth+1, visit)
		return
	}
	i := strings.Index(expr, "(")
	if i < 0 || !strings.HasSuffix(expr, ")") {
		return
	}
	name := strings.TrimSpace(expr[:i])
	args := parseArgs(expr[i+1 : len(expr)-1])
	for _, a := range args {
		if name == "and" || name == "or" {
			scanCondMatches(a, depth+1, visit)
		} else {
			scanPipelineMatches(a, 0, visit)
		}
	}
}

func scanPipelineMatches(expr string, depth int, visit MatchVisitor) {
	if depth > 2*maxPipelineDepth {
		return
	}
	stages := splitTopLevel(expr, '|')
	for _, st := range stages[1:] {
		name, args := parseFilterCall(strings.TrimSpace(st))
		if name == "match" && len(args) == 2 {
			p := strings.TrimSpace(args[0])
			if len(p) >= 2 && (p[0] == '\'' || p[0] == '"') && p[len(p)-1] == p[0] {
				visit(p[1:len(p)-1], true)
			} else {
				visit(p, false)
			}
		}
		for _, a := range args {
			a = strings.TrimSpace(a)
			if len(a) >= 2 && (a[0] == '\'' || a[0] == '"') && a[len(a)-1] == a[0] {
				continue
			}
			scanPipelineMatches(a, depth+1, visit)
		}
	}
}

// ScanDrawnLiterals finds the quoted filter arguments in the {{…}} pipelines
// of s whose text becomes part of the result: the argument of prefix and
// suffix, the replacement of replace and the pattern of format. Arguments
// that only select or remove text (match, strip, before, …) and numeric
// arguments are not visited. The device exporter checks these constants for
// Latin-1.
func ScanDrawnLiterals(s string, visit func(lit string)) {
	for _, m := range interpolateRe.FindAllStringSubmatch(s, -1) {
		stages := splitTopLevel(strings.TrimSpace(m[1]), '|')
		for _, st := range stages[1:] {
			name, args := parseFilterCall(strings.TrimSpace(st))
			var a string
			switch {
			case (name == "prefix" || name == "suffix") && len(args) == 1:
				a = args[0]
			case name == "replace" && len(args) == 2:
				a = args[1]
			case name == "format" && (len(args) == 1 || len(args) == 2):
				a = args[0]
			default:
				continue
			}
			visit(unquote(a))
		}
	}
}
