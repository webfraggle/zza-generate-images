package renderer

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// interpolateRe matches {{expr}} where expr may contain quoted strings (including '}').
// Groups: single-quoted strings, double-quoted strings, or any non-'}' character.
var interpolateRe = regexp.MustCompile(`\{\{((?:'[^']*'|"[^"]*"|[^}])+)\}\}`)

// Evaluator resolves template variables, applies filter pipelines,
// and evaluates boolean conditions.
type Evaluator struct {
	data    map[string]interface{}
	intVars map[string]int // integer variables for coordinate expressions (i, loop.index, …)
	now     time.Time      // captured once at creation for consistent {{now.*}} values
}

// NewEvaluator creates a new Evaluator with the given data map.
func NewEvaluator(data map[string]interface{}) *Evaluator {
	return &Evaluator{data: data, now: time.Now()}
}

// withLoopVars returns a new Evaluator with additional loop-scope variables.
// strVars are merged into the data map (accessible via {{varName}} in text expressions).
// loopIntVars are set as integer expression variables (accessible in {{...}} coord expressions).
// The parent evaluator is not modified.
func (e *Evaluator) withLoopVars(strVars map[string]string, loopIntVars map[string]int) *Evaluator {
	newData := make(map[string]interface{}, len(e.data)+len(strVars))
	for k, v := range e.data {
		newData[k] = v
	}
	for k, v := range strVars {
		newData[k] = v
	}
	newIntVars := make(map[string]int, len(e.intVars)+len(loopIntVars))
	for k, v := range e.intVars {
		newIntVars[k] = v
	}
	for k, v := range loopIntVars {
		newIntVars[k] = v
	}
	return &Evaluator{data: newData, intVars: newIntVars, now: e.now}
}

// Interpolate replaces all {{expr}} placeholders in s.
// Expressions support filter pipelines: {{var | filter1 | filter2(arg)}}.
func (e *Evaluator) Interpolate(s string) string {
	return interpolateRe.ReplaceAllStringFunc(s, func(match string) string {
		inner := strings.TrimSpace(match[2 : len(match)-2])
		return e.evalPipeline(inner)
	})
}

var pipelineSplitRe = regexp.MustCompile(`\s*\|\s*`)

// evalPipeline evaluates a pipeline expression like "var | filter1 | filter2(arg)".
// Pipeline stages are separated by | with optional surrounding spaces.
func (e *Evaluator) evalPipeline(expr string) string {
	stages := pipelineSplitRe.Split(expr, -1)
	if len(stages) == 0 {
		return ""
	}
	value := e.resolveVar(strings.TrimSpace(stages[0]))
	for _, stage := range stages[1:] {
		value = e.applyFilter(value, strings.TrimSpace(stage))
	}
	return value
}

// resolveVar resolves a variable name to a string value.
// Handles "now" time variables and dot-notation data paths.
func (e *Evaluator) resolveVar(name string) string {
	if name == "now" || strings.HasPrefix(name, "now.") {
		return e.resolveNow(name)
	}
	return e.resolvePath(name)
}

// resolveNow returns time-related variable values.
func (e *Evaluator) resolveNow(name string) string {
	weekdays := []string{"Sonntag", "Montag", "Dienstag", "Mittwoch", "Donnerstag", "Freitag", "Samstag"}
	switch name {
	case "now":
		return e.now.Format("15:04")
	case "now.hour":
		return strconv.Itoa(e.now.Hour())
	case "now.hour12":
		h := e.now.Hour() % 12
		if h == 0 {
			h = 12
		}
		return strconv.Itoa(h)
	case "now.minute":
		return strconv.Itoa(e.now.Minute())
	case "now.second":
		return strconv.Itoa(e.now.Second())
	case "now.day":
		return strconv.Itoa(e.now.Day())
	case "now.month":
		return strconv.Itoa(int(e.now.Month()))
	case "now.year":
		return strconv.Itoa(e.now.Year())
	case "now.weekday":
		return weekdays[e.now.Weekday()]
	default:
		return ""
	}
}

// resolvePath resolves a dot-separated path like "zug1.zeit" against the data map.
func (e *Evaluator) resolvePath(path string) string {
	parts := strings.Split(path, ".")
	var current interface{} = map[string]interface{}(e.data)

	for _, part := range parts {
		part = strings.TrimSpace(part)
		switch m := current.(type) {
		case map[string]interface{}:
			val, ok := m[part]
			if !ok {
				return ""
			}
			current = val
		case map[interface{}]interface{}:
			val, ok := m[part]
			if !ok {
				return ""
			}
			current = val
		default:
			return ""
		}
	}
	if current == nil {
		return ""
	}
	return fmt.Sprintf("%v", current)
}

// applyFilter applies a single filter expression to a value.
func (e *Evaluator) applyFilter(value, filter string) string {
	name, args := parseFilterCall(filter)
	switch name {
	case "strip":
		if len(args) == 1 {
			return strings.TrimPrefix(value, unquote(args[0]))
		}
	case "stripAll":
		if len(args) == 1 {
			return strings.ReplaceAll(value, unquote(args[0]), "")
		}
	case "stripBetween":
		if len(args) == 2 {
			return doStripBetween(value, unquote(args[0]), unquote(args[1]))
		}
	case "upper":
		return strings.ToUpper(value)
	case "lower":
		return strings.ToLower(value)
	case "trim":
		return strings.TrimSpace(value)
	case "prefix":
		if len(args) == 1 {
			return unquote(args[0]) + value
		}
	case "suffix":
		if len(args) == 1 {
			return value + unquote(args[0])
		}
	case "format":
		// Applies to e.now regardless of input value.
		// Intended use: {{now | format('HH:mm')}}.
		if len(args) == 1 {
			return formatTime(e.now, unquote(args[0]))
		}
	case "mul":
		if len(args) == 1 {
			return doMath(value, unquote(args[0]), func(a, b float64) float64 { return a * b })
		}
	case "div":
		if len(args) == 1 {
			return doMath(value, unquote(args[0]), func(a, b float64) float64 {
				if b == 0 {
					return 0
				}
				return a / b
			})
		}
	case "add":
		if len(args) == 1 {
			return doMath(value, unquote(args[0]), func(a, b float64) float64 { return a + b })
		}
	case "sub":
		if len(args) == 1 {
			return doMath(value, unquote(args[0]), func(a, b float64) float64 { return a - b })
		}
	case "round":
		v, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
		if err == nil {
			r := math.Round(v)
			if !math.IsInf(r, 0) && !math.IsNaN(r) && r >= math.MinInt64 && r <= math.MaxInt64 {
				return strconv.FormatInt(int64(r), 10)
			}
		}
	}
	return value
}

const maxCondDepth = 10

// EvalCondition evaluates a boolean condition expression.
// Supported functions: startsWith, endsWith, contains, isEmpty, equals, greaterThan, not.
// Returns true for empty expressions.
func (e *Evaluator) EvalCondition(expr string) bool {
	return e.evalCond(expr, 0)
}

func (e *Evaluator) evalCond(expr string, depth int) bool {
	if depth > maxCondDepth {
		return false // treat overly-nested expressions as false
	}
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return true
	}

	// not(inner)
	if strings.HasPrefix(expr, "not(") && strings.HasSuffix(expr, ")") {
		return !e.evalCond(expr[4:len(expr)-1], depth+1)
	}

	parenIdx := strings.Index(expr, "(")
	if parenIdx < 0 || !strings.HasSuffix(expr, ")") {
		return false
	}
	funcName := strings.TrimSpace(expr[:parenIdx])
	argsStr := expr[parenIdx+1 : len(expr)-1]
	args := parseArgs(argsStr)
	if len(args) < 1 {
		return false
	}

	fieldVal := e.resolveVar(strings.TrimSpace(args[0]))

	switch funcName {
	case "startsWith":
		if len(args) == 2 {
			return strings.HasPrefix(fieldVal, unquote(args[1]))
		}
	case "endsWith":
		if len(args) == 2 {
			return strings.HasSuffix(fieldVal, unquote(args[1]))
		}
	case "contains":
		if len(args) == 2 {
			return strings.Contains(fieldVal, unquote(args[1]))
		}
	case "isEmpty":
		return fieldVal == ""
	case "eq", "equals":
		if len(args) == 2 {
			return fieldVal == unquote(args[1])
		}
	case "greaterThan":
		if len(args) == 2 {
			fv, err1 := strconv.ParseFloat(fieldVal, 64)
			threshold, err2 := strconv.ParseFloat(strings.TrimSpace(unquote(args[1])), 64)
			return err1 == nil && err2 == nil && fv > threshold
		}
	}
	return false
}

// parseFilterCall parses "funcName(arg1, arg2)" into name and args.
// For bare words like "upper", returns (name, nil).
func parseFilterCall(filter string) (name string, args []string) {
	filter = strings.TrimSpace(filter)
	parenIdx := strings.Index(filter, "(")
	if parenIdx < 0 {
		return filter, nil
	}
	if !strings.HasSuffix(filter, ")") {
		return filter, nil
	}
	name = strings.TrimSpace(filter[:parenIdx])
	argsStr := filter[parenIdx+1 : len(filter)-1]
	args = parseArgs(argsStr)
	return
}

// parseArgs splits a comma-separated argument string, respecting quoted strings.
// Example: "'*', '{'" → ["'*'", "'{'"]
func parseArgs(s string) []string {
	var args []string
	var current strings.Builder
	inQuote := false
	var quoteChar byte

	for i := 0; i < len(s); i++ {
		ch := s[i]
		if inQuote {
			current.WriteByte(ch)
			if ch == quoteChar {
				inQuote = false
			}
		} else {
			switch ch {
			case '\'', '"':
				inQuote = true
				quoteChar = ch
				current.WriteByte(ch)
			case ',':
				if t := strings.TrimSpace(current.String()); t != "" {
					args = append(args, t)
				}
				current.Reset()
			default:
				current.WriteByte(ch)
			}
		}
	}
	if t := strings.TrimSpace(current.String()); t != "" {
		args = append(args, t)
	}
	return args
}

// unquote removes surrounding single or double quotes from a string.
func unquote(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 &&
		((s[0] == '\'' && s[len(s)-1] == '\'') ||
			(s[0] == '"' && s[len(s)-1] == '"')) {
		return s[1 : len(s)-1]
	}
	return s
}

// doMath applies a binary float64 operation. Returns the original string on parse error.
// Returns an integer string when the result has no fractional part and fits in int64.
func doMath(value, arg string, op func(a, b float64) float64) string {
	v, err1 := strconv.ParseFloat(strings.TrimSpace(value), 64)
	a, err2 := strconv.ParseFloat(strings.TrimSpace(arg), 64)
	if err1 != nil || err2 != nil {
		return value
	}
	result := op(v, a)
	if !math.IsInf(result, 0) && !math.IsNaN(result) && result == math.Trunc(result) &&
		result >= math.MinInt64 && result <= math.MaxInt64 {
		return strconv.FormatInt(int64(result), 10)
	}
	return strconv.FormatFloat(result, 'f', -1, 64)
}

// doStripBetween removes all substrings between open and close delimiters (inclusive).
func doStripBetween(s, open, close string) string {
	for {
		start := strings.Index(s, open)
		if start < 0 {
			break
		}
		end := strings.Index(s[start+len(open):], close)
		if end < 0 {
			break
		}
		s = s[:start] + s[start+len(open)+end+len(close):]
	}
	return s
}

// formatTime formats a time value using the documented pattern tokens.
// Tokens: HH (hour 00-23), hh (hour 01-12), mm (minute), ss (second),
// dd (day), MM (month), yyyy (year), EE (weekday short), EEEE (weekday long).
func formatTime(t time.Time, pattern string) string {
	weekdaysShort := []string{"So", "Mo", "Di", "Mi", "Do", "Fr", "Sa"}
	weekdaysLong := []string{"Sonntag", "Montag", "Dienstag", "Mittwoch", "Donnerstag", "Freitag", "Samstag"}

	h12 := t.Hour() % 12
	if h12 == 0 {
		h12 = 12
	}
	r := pattern
	// Replace longer tokens first to avoid partial matches.
	r = strings.ReplaceAll(r, "EEEE", weekdaysLong[t.Weekday()])
	r = strings.ReplaceAll(r, "yyyy", fmt.Sprintf("%04d", t.Year()))
	r = strings.ReplaceAll(r, "HH", fmt.Sprintf("%02d", t.Hour()))
	r = strings.ReplaceAll(r, "hh", fmt.Sprintf("%02d", h12))
	r = strings.ReplaceAll(r, "MM", fmt.Sprintf("%02d", int(t.Month())))
	r = strings.ReplaceAll(r, "mm", fmt.Sprintf("%02d", t.Minute()))
	r = strings.ReplaceAll(r, "ss", fmt.Sprintf("%02d", t.Second()))
	r = strings.ReplaceAll(r, "dd", fmt.Sprintf("%02d", t.Day()))
	r = strings.ReplaceAll(r, "EE", weekdaysShort[t.Weekday()])
	return r
}
