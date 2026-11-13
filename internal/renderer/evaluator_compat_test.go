package renderer

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

var legacyPipelineSplitRe = regexp.MustCompile(`\s*\|\s*`)

// legacyEvaluator is a frozen copy of the pre-parser pipeline implementation.
type legacyEvaluator struct {
	data map[string]interface{}
	now  time.Time
}

func (e *legacyEvaluator) resolveVar(name string) string {
	ev := NewEvaluator(e.data)
	ev.now = e.now
	return ev.resolveVar(name)
}

func (e *legacyEvaluator) evalPipeline(expr string) string {
	stages := legacyPipelineSplitRe.Split(expr, -1)
	if len(stages) == 0 {
		return ""
	}
	value := e.resolveVar(strings.TrimSpace(stages[0]))
	for _, stage := range stages[1:] {
		value = e.applyFilter(value, strings.TrimSpace(stage))
	}
	return value
}

// applyFilter applies a single filter expression to a value.
func (e *legacyEvaluator) applyFilter(value, filter string) string {
	name, args := legacyParseFilterCall(filter)
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
		if len(args) == 1 {
			return formatTime(e.now, unquote(args[0]))
		}
	case "mul":
		if len(args) == 1 {
			return legacyDoMath(value, unquote(args[0]), func(a, b float64) float64 { return a * b })
		}
	case "div":
		if len(args) == 1 {
			return legacyDoMath(value, unquote(args[0]), func(a, b float64) float64 {
				if b == 0 {
					return 0
				}
				return a / b
			})
		}
	case "add":
		if len(args) == 1 {
			return legacyDoMath(value, unquote(args[0]), func(a, b float64) float64 { return a + b })
		}
	case "sub":
		if len(args) == 1 {
			return legacyDoMath(value, unquote(args[0]), func(a, b float64) float64 { return a - b })
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

// legacyParseFilterCall parses "funcName(arg1, arg2)" into name and args.
// For bare words like "upper", returns (name, nil).
func legacyParseFilterCall(filter string) (name string, args []string) {
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
	args = legacyParseArgs(argsStr)
	return
}

// legacyParseArgs splits a comma-separated argument string, respecting quoted strings.
// Example: "'*', '{'" → ["'*'", "'{'"]
func legacyParseArgs(s string) []string {
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

// legacyDoMath applies a binary float64 operation. Returns the original string on parse error.
// Returns an integer string when the result has no fractional part and fits in int64.
func legacyDoMath(value, arg string, op func(a, b float64) float64) string {
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

// legacyCorpus holds every {{…}} expression found in templates/*/template.yaml
// before the pipeline parser rewrite (fixed list, not read dynamically).
var legacyCorpus = []string{
	"ch",
	"gleis",
	"hour",
	"i * 11 + 14",
	"i * 11 + 15",
	"i * 16 + 21",
	"i * 30 + 40",
	"i * 45 + 60",
	"minute",
	"now.hour",
	"now.hour | mul(30)",
	"now.hour12 | mul(30)",
	"now.minute",
	"now.minute | mul(6)",
	"now.second | mul(6)",
	"via",
	"via | trim",
	"zug1.abw",
	"zug1.hinweis",
	"zug1.hinweis | strip('*')",
	"zug1.nr",
	"zug1.nr | lower",
	"zug1.via",
	"zug1.vonnach",
	"zug1.vonnach | upper",
	"zug1.zeit",
	"zug1.zeit | after(':')",
	"zug1.zeit | before(':')",
	"zug2.nr",
	"zug2.vonnach",
	"zug2.zeit",
	"zug3.vonnach",
	"zug3.zeit",
	// additional fixed expressions
	"x | mul(6)",
	"x | mul('6')",
	"x | add( 5 )",
	"x|upper",
	"x | round",
	"x | div(0)",
	"x | strip('*') | upper",
	"x | stripBetween('(', ')')",
	"x | before(':')",
	"x | after(':')",
	"now | format('HH:mm')",
	"now.hour12 | mul(30)",
}

func TestPipeline_LegacyCompat(t *testing.T) {
	fixedNow := time.Date(2026, 11, 13, 9, 15, 30, 0, time.UTC)

	datasets := []map[string]interface{}{
		{"x": "12.5"}, {"x": "abc"}, {"x": ""},
		{"x": "foo (bar) baz:qux *star"},
	}
	files, _ := filepath.Glob("../../templates/*/default.json")
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		var d map[string]interface{}
		if err := json.Unmarshal(b, &d); err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		datasets = append(datasets, d)
	}

	for di, data := range datasets {
		for _, expr := range legacyCorpus {
			e := NewEvaluator(data)
			e.now = fixedNow
			l := &legacyEvaluator{data: data, now: fixedNow}
			got := e.Interpolate("{{" + expr + "}}")
			want := l.evalPipeline(strings.TrimSpace(expr))
			if got != want {
				t.Errorf("dataset %d expr %q: got %q, legacy %q", di, expr, got, want)
			}
		}
	}
}
