package renderer

import "regexp"

var (
	nowTokenRe     = regexp.MustCompile(`(^|[^A-Za-z0-9_.])now([^A-Za-z0-9_]|$)`)
	formatFilterRe = regexp.MustCompile(`\bformat\s*\(`)
)

// UsesNow reports whether any evaluated field of the template reads the
// clock ({{now.*}} or the format filter), also inside nested layers of
// blocks, loops and splitflap. Detection is conservative: a false positive
// only costs a cache miss, a false negative would freeze a clock.
func (t *Template) UsesNow() bool {
	return layersUseNow(t.Layers)
}

func layersUseNow(layers []Layer) bool {
	for i := range layers {
		l := &layers[i]
		if condUsesNow(l.Value) || condUsesNow(l.Color) || condUsesNow(l.Rotate) ||
			exprUsesNow(l.If) || exprUsesNow(l.Elif) || layersUseNow(l.Layers) {
			return true
		}
	}
	return false
}

// condUsesNow checks the raw value and all branches of a StringOrCond.
func condUsesNow(s StringOrCond) bool {
	if interpUsesNow(s.raw) {
		return true
	}
	if s.cond == nil {
		return false
	}
	for _, b := range s.cond.branches {
		if exprUsesNow(b.ifExpr) || interpUsesNow(b.then) {
			return true
		}
	}
	return interpUsesNow(s.cond.els)
}

// exprUsesNow reports whether a bare expression reads the clock.
func exprUsesNow(s string) bool {
	return nowTokenRe.MatchString(s) || formatFilterRe.MatchString(s)
}

// interpUsesNow reports whether any {{…}} in s reads the clock. Text outside
// the braces does not count.
func interpUsesNow(s string) bool {
	for _, m := range interpolateRe.FindAllStringSubmatch(s, -1) {
		if exprUsesNow(m[1]) {
			return true
		}
	}
	return false
}
