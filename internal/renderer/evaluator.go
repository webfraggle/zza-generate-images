package renderer

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"sync"
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
	anim    animVars       // {{anim.*}} values
}

// animVars holds the animation variables: current frame, frame count and
// frame delay in hundredths of a second (0 outside frame mode).
type animVars struct{ frame, frames, delayCS int }

// NewEvaluator creates a new Evaluator with the given data map. It uses
// time.Now in the local zone; the renderer uses newEvaluator (template time
// zone, injectable clock).
func NewEvaluator(data map[string]interface{}) *Evaluator {
	return newEvaluatorAt(data, time.Now())
}

// newEvaluatorAt creates an Evaluator with a fixed `now`.
func newEvaluatorAt(data map[string]interface{}, now time.Time) *Evaluator {
	return &Evaluator{data: data, now: now, anim: animVars{frames: 1}}
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
	return &Evaluator{data: newData, intVars: newIntVars, now: e.now, anim: e.anim}
}

// withOwnIntVars returns a copy of e with its own integer-variable map (data
// and now are shared), so that variables set during one render stay private to it.
func (e *Evaluator) withOwnIntVars() *Evaluator {
	iv := make(map[string]int, len(e.intVars)+4)
	for k, v := range e.intVars {
		iv[k] = v
	}
	c := *e
	c.intVars = iv
	return &c
}

// withAnim returns a copy of e (data and now are shared) with the animation
// variables set: {{anim.*}} in strings and anim.frame / anim.frames in
// coordinate expressions.
func (e *Evaluator) withAnim(frame, frames, delayCS int) *Evaluator {
	iv := make(map[string]int, len(e.intVars)+2)
	for k, v := range e.intVars {
		iv[k] = v
	}
	iv["anim.frame"], iv["anim.frames"] = frame, frames
	return &Evaluator{data: e.data, intVars: iv, now: e.now, anim: animVars{frame, frames, delayCS}}
}

// Interpolate replaces all {{expr}} placeholders in s.
// Expressions are pipelines: operand ( "|" filter )*, where
// filter := name [ "(" arg ( "," arg )* ")" ] and
// arg := 'text' | "text" | number | pipeline (without {{ }}).
// "|" and "," only separate at top level, i.e. outside quotes and parentheses.
// Numeric filters (add, sub, mul, div, mod) evaluate non-literal arguments as
// nested pipelines; text filters take literal arguments only.
func (e *Evaluator) Interpolate(s string) string {
	return interpolateRe.ReplaceAllStringFunc(s, func(match string) string {
		inner := strings.TrimSpace(match[2 : len(match)-2])
		return e.evalPipeline(inner)
	})
}

// maxPipelineDepth limits nested argument pipelines; deeper ones yield "".
const maxPipelineDepth = 4

// minutesRe matches "H:MM"/"HH:MM" at the start of a value.
var minutesRe = regexp.MustCompile(`^(\d{1,2}):(\d{1,2})`)

// splitTopLevel splits s at sep outside of '…', "…" and ( … ). The parenthesis
// depth never drops below 0: a surplus ")" counts as a normal character.
// The parts are not trimmed.
func splitTopLevel(s string, sep byte) []string {
	var parts []string
	depth := 0
	var quote byte
	start := 0
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if quote != 0 {
			if ch == quote {
				quote = 0
			}
			continue
		}
		switch {
		case ch == '\'' || ch == '"':
			quote = ch
		case ch == '(':
			depth++
		case ch == ')':
			if depth > 0 {
				depth--
			}
		case ch == sep && depth == 0:
			parts = append(parts, s[start:i])
			start = i + 1
		}
	}
	return append(parts, s[start:])
}

// evalPipeline evaluates a pipeline expression like "var | filter1 | filter2(arg)".
func (e *Evaluator) evalPipeline(expr string) string {
	return e.evalPipelineDepth(expr, 0)
}

// evalPipelineDepth evaluates a pipeline; depth counts nested argument pipelines.
func (e *Evaluator) evalPipelineDepth(expr string, depth int) string {
	if depth > maxPipelineDepth {
		return ""
	}
	stages := splitTopLevel(expr, '|')
	value := e.resolveVar(strings.TrimSpace(stages[0]))
	for _, stage := range stages[1:] {
		value = e.applyFilter(value, strings.TrimSpace(stage), depth)
	}
	return value
}

// resolveVar resolves a variable name to a string value.
// Handles "now" time variables and dot-notation data paths.
func (e *Evaluator) resolveVar(name string) string {
	if name == "now" || strings.HasPrefix(name, "now.") {
		return e.resolveNow(name)
	}
	if name == "anim" || strings.HasPrefix(name, "anim.") {
		return e.resolveAnim(name)
	}
	return e.resolvePath(name)
}

// resolveAnim returns the animation variable values; unknown names give "".
func (e *Evaluator) resolveAnim(name string) string {
	switch name {
	case "anim.frame":
		return strconv.Itoa(e.anim.frame)
	case "anim.frames":
		return strconv.Itoa(e.anim.frames)
	case "anim.time":
		return strconv.FormatFloat(float64(e.anim.frame*e.anim.delayCS)/100, 'f', -1, 64)
	default:
		return ""
	}
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
func (e *Evaluator) applyFilter(value, filter string, depth int) string {
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
	case "before":
		if len(args) == 1 {
			if sep := unquote(args[0]); sep != "" {
				if i := strings.Index(value, sep); i >= 0 {
					return value[:i]
				}
			}
			return value
		}
	case "after":
		if len(args) == 1 {
			if sep := unquote(args[0]); sep != "" {
				if i := strings.Index(value, sep); i >= 0 {
					return value[i+len(sep):]
				}
			}
			return ""
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
		// Optional second argument: offset in seconds (number, variable or
		// pipeline); a non-numeric offset is ignored.
		if len(args) == 1 {
			return formatTime(e.now, unquote(args[0]))
		}
		if len(args) == 2 {
			t := e.now
			arg, _ := e.numericArg(args[1], depth)
			if sec, err := strconv.ParseFloat(strings.TrimSpace(arg), 64); err == nil &&
				!math.IsNaN(sec) && math.Abs(sec) <= maxFormatOffsetSec {
				t = t.Add(time.Duration(sec * float64(time.Second)))
			}
			return formatTime(t, unquote(args[0]))
		}
	case "clock":
		// Minutes since midnight -> HH:MM, wrapping modulo 1440.
		v, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
		if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
			return ""
		}
		m := math.Mod(math.Floor(v), 1440)
		if m < 0 {
			m += 1440
		}
		return fmt.Sprintf("%02d:%02d", int(m)/60, int(m)%60)
	case "replace":
		if len(args) == 2 {
			if from := unquote(args[0]); from != "" {
				return strings.ReplaceAll(value, from, unquote(args[1]))
			}
			return value
		}
	case "match":
		if len(args) == 2 {
			return matchGroup(value, unquote(args[0]), e.numericArgInt(args[1], depth))
		}
	case "mul":
		if len(args) == 1 {
			arg, strict := e.numericArg(args[0], depth)
			return doMath(value, arg, strict, func(a, b float64) float64 { return a * b })
		}
	case "div":
		if len(args) == 1 {
			arg, strict := e.numericArg(args[0], depth)
			return doMath(value, arg, strict, func(a, b float64) float64 {
				if b == 0 {
					return 0
				}
				return a / b
			})
		}
	case "mod":
		if len(args) == 1 {
			arg, strict := e.numericArg(args[0], depth)
			return doMath(value, arg, strict, func(a, b float64) float64 {
				if b == 0 {
					return 0
				}
				r := math.Mod(a, b)
				if r != 0 && (r < 0) != (b < 0) {
					r += b
				}
				if r == b { // float rounding, e.g. -1e-20 mod 1440
					r = 0
				}
				return r
			})
		}
	case "add":
		if len(args) == 1 {
			arg, strict := e.numericArg(args[0], depth)
			return doMath(value, arg, strict, func(a, b float64) float64 { return a + b })
		}
	case "sub":
		if len(args) == 1 {
			arg, strict := e.numericArg(args[0], depth)
			return doMath(value, arg, strict, func(a, b float64) float64 { return a - b })
		}
	case "minutes":
		if m := minutesRe.FindStringSubmatch(strings.TrimSpace(value)); m != nil {
			h, _ := strconv.Atoi(m[1])
			mi, _ := strconv.Atoi(m[2])
			return strconv.Itoa(h*60 + mi)
		}
		return ""
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

	fieldVal := e.evalPipelineDepth(strings.TrimSpace(args[0]), 1)

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

// parseArgs splits an argument string at top-level commas (outside quotes and
// parentheses), trims the parts and drops empty ones.
// Example: "'*', '{'" → ["'*'", "'{'"]
func parseArgs(s string) []string {
	var args []string
	for _, part := range splitTopLevel(s, ',') {
		if t := strings.TrimSpace(part); t != "" {
			args = append(args, t)
		}
	}
	return args
}

// numLiteralRe matches plain decimal literals (no inf/nan/hex/exponent); same
// pattern as the JS node editor.
var numLiteralRe = regexp.MustCompile(`^[+-]?(\d+\.?\d*|\.\d+)$`)

// numericArg resolves the argument of a numeric filter. Quoted text and plain
// numbers are literals (fromExpr=false); anything else is evaluated as a
// nested pipeline (fromExpr=true).
func (e *Evaluator) numericArg(arg string, depth int) (val string, fromExpr bool) {
	t := strings.TrimSpace(arg)
	if len(t) >= 2 && ((t[0] == '\'' && t[len(t)-1] == '\'') || (t[0] == '"' && t[len(t)-1] == '"')) {
		return unquote(t), false
	}
	if numLiteralRe.MatchString(t) {
		return t, false
	}
	return e.evalPipelineDepth(t, depth+1), true
}

// numericArgInt resolves a numeric filter argument to an int; anything that is
// not a plain integer yields -1.
func (e *Evaluator) numericArgInt(arg string, depth int) int {
	v, _ := e.numericArg(arg, depth)
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil {
		return -1
	}
	return n
}

// maxFormatOffsetSec bounds the format() offset (about 100 years) so the
// Duration conversion cannot overflow.
const maxFormatOffsetSec = 3.2e9

const (
	maxMatchPatternLen = 100
	maxRegexCacheSize  = 256
)

var (
	regexCacheMu sync.Mutex
	regexCache   = map[string]*regexp.Regexp{} // nil value = invalid pattern
)

// compileCached compiles a RE2 pattern, caching results (including failures)
// in a map bounded to maxRegexCacheSize entries.
func compileCached(pattern string) *regexp.Regexp {
	regexCacheMu.Lock()
	defer regexCacheMu.Unlock()
	if re, ok := regexCache[pattern]; ok {
		return re
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		re = nil
	}
	if len(regexCache) >= maxRegexCacheSize {
		regexCache = make(map[string]*regexp.Regexp, maxRegexCacheSize)
	}
	regexCache[pattern] = re
	return re
}

// matchGroup returns capture group n (0 = whole match) of the first match of
// pattern in value, or "" on no match, invalid/overlong pattern or bad n.
func matchGroup(value, pattern string, n int) string {
	if n < 0 || len(pattern) > maxMatchPatternLen {
		return ""
	}
	re := compileCached(pattern)
	if re == nil {
		return ""
	}
	m := re.FindStringSubmatch(value)
	if m == nil || n >= len(m) {
		return ""
	}
	return m[n]
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

// doMath applies a binary float64 operation. A non-numeric value is returned
// unchanged. A non-numeric arg returns "" if strictArg (arg came from an
// expression), otherwise the value unchanged.
// Returns an integer string when the result has no fractional part and fits in int64.
func doMath(value, arg string, strictArg bool, op func(a, b float64) float64) string {
	v, err1 := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err1 != nil {
		return value
	}
	a, err2 := strconv.ParseFloat(strings.TrimSpace(arg), 64)
	if err2 != nil {
		if strictArg {
			return ""
		}
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
