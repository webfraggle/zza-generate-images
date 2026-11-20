package renderer

import (
	"math"
	"regexp"
	"strconv"
	"strings"
)

var (
	nowTokenRe       = regexp.MustCompile(`(^|[^A-Za-z0-9_.])now([^A-Za-z0-9_]|$)`)
	nowSecondRe      = regexp.MustCompile(`(^|[^A-Za-z0-9_.])now\.second([^A-Za-z0-9_]|$)`)
	formatFilterRe   = regexp.MustCompile(`\bformat\s*\(`)
	formatLiteralArg = regexp.MustCompile(`^\s*(?:'([^']*)'|"([^"]*)")`)
	formatSecondArg  = regexp.MustCompile(`^\s*,`)
)

// Clock levels, ordered: a higher level includes the lower ones.
const (
	clockNone = iota
	clockMinute
	clockSecond
)

// UsesNow reports whether any evaluated field of the template reads the
// clock ({{now.*}} or the format filter), also inside nested layers of
// blocks, loops and splitflap. Detection is conservative: a false positive
// only costs a cache miss, a false negative would freeze a clock.
func (t *Template) UsesNow() bool {
	return t.clockLevel() > clockNone
}

// ClockGranularity reports how often the template changes with the clock:
// "second" if any evaluated expression reads now.second or formats with
// seconds (or with a pattern that is not a string literal, or with an offset
// that is not a numeric literal of whole minutes), "minute" if it reads the
// clock otherwise, else "none".
func (t *Template) ClockGranularity() string {
	switch t.clockLevel() {
	case clockSecond:
		return "second"
	case clockMinute:
		return "minute"
	}
	return "none"
}

func (t *Template) clockLevel() int {
	return layersClock(t.Layers)
}

func layersClock(layers []Layer) int {
	level := clockNone
	for i := range layers {
		l := &layers[i]
		level = max(level, condClock(l.Value), condClock(l.Color), condClock(l.Rotate),
			exprClock(l.If), exprClock(l.Elif), layersClock(l.Layers))
		if level == clockSecond {
			break
		}
	}
	return level
}

// condClock checks the raw value and all branches of a StringOrCond.
func condClock(s StringOrCond) int {
	level := interpClock(s.raw)
	if s.cond == nil {
		return level
	}
	for _, b := range s.cond.branches {
		level = max(level, exprClock(b.ifExpr), interpClock(b.then))
	}
	return max(level, interpClock(s.cond.els))
}

// exprClock returns the clock level of a bare expression.
func exprClock(s string) int {
	if nowSecondRe.MatchString(s) {
		return clockSecond
	}
	level := clockNone
	if nowTokenRe.MatchString(s) {
		level = clockMinute
	}
	for _, loc := range formatFilterRe.FindAllStringIndex(s, -1) {
		arg := s[loc[1]:]
		m := formatLiteralArg.FindStringSubmatch(arg)
		switch {
		case m == nil:
			return clockSecond // pattern not known statically
		case strings.Contains(m[1]+m[2], "ss"):
			return clockSecond
		case formatSecondArg.MatchString(arg[len(m[0]):]) && !wholeMinutes(offsetArg(arg[len(m[0]):])):
			return clockSecond // the offset moves the shown minute off the minute boundary
		}
		level = max(level, clockMinute)
	}
	return level
}

// offsetArg returns the second argument of a format call: rest starts at the comma after the
// pattern, the argument ends at the closing parenthesis of the call (nested calls and quoted
// strings skipped). Unbalanced input returns the rest after the comma.
func offsetArg(rest string) string {
	rest = rest[strings.IndexByte(rest, ',')+1:]
	depth := 0
	var quote byte
	for i := 0; i < len(rest); i++ {
		c := rest[i]
		switch {
		case quote != 0:
			if c == quote {
				quote = 0
			}
		case c == '\'' || c == '"':
			quote = c
		case c == '(':
			depth++
		case c == ')':
			if depth == 0 {
				return rest[:i]
			}
			depth--
		}
	}
	return rest
}

// wholeMinutes reports whether arg is a numeric literal that is a multiple of 60 seconds: only
// then does a format offset keep the shown minute changing on the minute boundary.
func wholeMinutes(arg string) bool {
	v, err := strconv.ParseFloat(strings.TrimSpace(arg), 64)
	return err == nil && !math.IsNaN(v) && !math.IsInf(v, 0) && math.Mod(v, 60) == 0
}

// interpClock returns the highest clock level of any {{…}} in s. Text
// outside the braces does not count.
func interpClock(s string) int {
	level := clockNone
	for _, m := range interpolateRe.FindAllStringSubmatch(s, -1) {
		level = max(level, exprClock(m[1]))
	}
	return level
}
