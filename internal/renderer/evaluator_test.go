package renderer

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

// fixedEval returns an Evaluator with known data and a fixed time (2026-03-24 15:54:07, Tuesday).
func fixedEval(data map[string]interface{}) *Evaluator {
	e := NewEvaluator(data)
	e.now = time.Date(2026, 3, 24, 15, 54, 7, 0, time.UTC) // Tuesday
	return e
}

func testData() map[string]interface{} {
	return map[string]interface{}{
		"zug1": map[string]interface{}{
			"zeit":    "15:54",
			"vonnach": "Sennhof-Kyburg",
			"nr":      "IC23",
			"hinweis": "*Abweichende Wagenreihung",
			"abw":     10,
		},
		"gleis": "3",
	}
}

// --- Interpolate / filter pipeline ---

func TestInterpolate_BasicVariable(t *testing.T) {
	e := fixedEval(testData())
	got := e.Interpolate("{{zug1.zeit}}")
	if got != "15:54" {
		t.Errorf("got %q, want %q", got, "15:54")
	}
}

func TestInterpolate_MissingVariable(t *testing.T) {
	e := fixedEval(testData())
	got := e.Interpolate("{{zug1.via}}")
	if got != "" {
		t.Errorf("missing variable should be empty, got %q", got)
	}
}

func TestInterpolate_Upper(t *testing.T) {
	e := fixedEval(testData())
	got := e.Interpolate("{{zug1.nr | upper}}")
	if got != "IC23" {
		t.Errorf("got %q, want %q", got, "IC23")
	}
}

func TestInterpolate_StripLeading(t *testing.T) {
	e := fixedEval(testData())
	got := e.Interpolate("{{zug1.hinweis | strip('*')}}")
	want := "Abweichende Wagenreihung"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestInterpolate_ChainedFilters(t *testing.T) {
	e := fixedEval(testData())
	got := e.Interpolate("{{zug1.hinweis | strip('*') | upper}}")
	want := "ABWEICHENDE WAGENREIHUNG"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestInterpolate_StripAll(t *testing.T) {
	e := fixedEval(map[string]interface{}{"x": "a-b-c"})
	got := e.Interpolate("{{x | stripAll('-')}}")
	if got != "abc" {
		t.Errorf("got %q, want %q", got, "abc")
	}
}

func TestInterpolate_StripBetween(t *testing.T) {
	e := fixedEval(map[string]interface{}{"x": "Halt {gesperrt} normal"})
	got := e.Interpolate("{{x | stripBetween('{', '}')}}")
	if got != "Halt  normal" {
		t.Errorf("got %q, want %q", got, "Halt  normal")
	}
}

func TestInterpolate_PrefixSuffix(t *testing.T) {
	e := fixedEval(map[string]interface{}{"v": "5"})
	if got := e.Interpolate("{{v | prefix('+')}}"); got != "+5" {
		t.Errorf("prefix: got %q, want %q", got, "+5")
	}
	if got := e.Interpolate("{{v | suffix(' min')}}"); got != "5 min" {
		t.Errorf("suffix: got %q, want %q", got, "5 min")
	}
}

func TestInterpolate_Trim(t *testing.T) {
	e := fixedEval(map[string]interface{}{"x": "  hallo  "})
	got := e.Interpolate("{{x | trim}}")
	if got != "hallo" {
		t.Errorf("got %q, want %q", got, "hallo")
	}
}

// --- Math filters ---

func TestInterpolate_Mul(t *testing.T) {
	e := fixedEval(testData())
	// now.minute = 54; 54 * 6 = 324
	got := e.Interpolate("{{now.minute | mul(6)}}")
	if got != "324" {
		t.Errorf("got %q, want %q", got, "324")
	}
}

func TestInterpolate_Div(t *testing.T) {
	e := fixedEval(map[string]interface{}{"x": "10"})
	got := e.Interpolate("{{x | div(4)}}")
	if got != "2.5" {
		t.Errorf("got %q, want %q", got, "2.5")
	}
}

func TestInterpolate_Add(t *testing.T) {
	e := fixedEval(map[string]interface{}{"x": "10"})
	got := e.Interpolate("{{x | add(5)}}")
	if got != "15" {
		t.Errorf("got %q, want %q", got, "15")
	}
}

func TestInterpolate_Sub(t *testing.T) {
	e := fixedEval(map[string]interface{}{"x": "10"})
	got := e.Interpolate("{{x | sub(3)}}")
	if got != "7" {
		t.Errorf("got %q, want %q", got, "7")
	}
}

func TestInterpolate_Round(t *testing.T) {
	e := fixedEval(map[string]interface{}{"x": "3.7"})
	got := e.Interpolate("{{x | round}}")
	if got != "4" {
		t.Errorf("got %q, want %q", got, "4")
	}
}

func TestInterpolate_DivByZero(t *testing.T) {
	e := fixedEval(map[string]interface{}{"x": "10"})
	// div by zero returns 0 (safe)
	got := e.Interpolate("{{x | div(0)}}")
	if got != "0" {
		t.Errorf("got %q, want %q", got, "0")
	}
}

// --- Now variables ---

func TestInterpolate_NowHour(t *testing.T) {
	e := fixedEval(nil)
	if got := e.Interpolate("{{now.hour}}"); got != "15" {
		t.Errorf("got %q, want %q", got, "15")
	}
}

func TestInterpolate_NowHour12(t *testing.T) {
	e := fixedEval(nil)
	if got := e.Interpolate("{{now.hour12}}"); got != "3" {
		t.Errorf("got %q, want %q", got, "3")
	}
}

func TestInterpolate_NowMinute(t *testing.T) {
	e := fixedEval(nil)
	if got := e.Interpolate("{{now.minute}}"); got != "54" {
		t.Errorf("got %q, want %q", got, "54")
	}
}

func TestInterpolate_Now(t *testing.T) {
	e := fixedEval(nil)
	if got := e.Interpolate("{{now}}"); got != "15:54" {
		t.Errorf("got %q, want %q", got, "15:54")
	}
}

func TestInterpolate_NowWeekday(t *testing.T) {
	e := fixedEval(nil)
	if got := e.Interpolate("{{now.weekday}}"); got != "Dienstag" {
		t.Errorf("got %q, want %q", got, "Dienstag")
	}
}

func TestInterpolate_Format(t *testing.T) {
	e := fixedEval(nil)
	if got := e.Interpolate("{{now | format('HH:mm:ss')}}"); got != "15:54:07" {
		t.Errorf("got %q, want %q", got, "15:54:07")
	}
}

func TestInterpolate_FormatDate(t *testing.T) {
	e := fixedEval(nil)
	if got := e.Interpolate("{{now | format('dd.MM.yyyy')}}"); got != "24.03.2026" {
		t.Errorf("got %q, want %q", got, "24.03.2026")
	}
}

// --- EvalCondition ---

func TestCondition_StartsWith(t *testing.T) {
	e := fixedEval(testData())
	if !e.EvalCondition("startsWith(zug1.hinweis, '*')") {
		t.Error("expected true")
	}
	if e.EvalCondition("startsWith(zug1.hinweis, 'X')") {
		t.Error("expected false")
	}
}

func TestCondition_IsEmpty(t *testing.T) {
	e := fixedEval(testData())
	if e.EvalCondition("isEmpty(zug1.hinweis)") {
		t.Error("hinweis is not empty")
	}
	if !e.EvalCondition("isEmpty(zug1.via)") {
		t.Error("via is not set, should be empty")
	}
}

func TestCondition_GreaterThan(t *testing.T) {
	e := fixedEval(testData())
	if !e.EvalCondition("greaterThan(zug1.abw, 0)") {
		t.Error("abw=10 > 0 should be true")
	}
	if e.EvalCondition("greaterThan(zug1.abw, 10)") {
		t.Error("abw=10 > 10 should be false")
	}
}

func TestCondition_Not(t *testing.T) {
	e := fixedEval(testData())
	if !e.EvalCondition("not(isEmpty(zug1.hinweis))") {
		t.Error("not(isEmpty) should be true when hinweis is set")
	}
}

func TestCondition_Equals(t *testing.T) {
	e := fixedEval(testData())
	if !e.EvalCondition("equals(gleis, '3')") {
		t.Error("gleis equals 3 should be true")
	}
}

func TestCondition_Empty_AlwaysTrue(t *testing.T) {
	e := fixedEval(testData())
	if !e.EvalCondition("") {
		t.Error("empty condition should always return true")
	}
}

// --- doStripBetween ---

func TestStripBetween_Multiple(t *testing.T) {
	got := doStripBetween("a {x} b {y} c", "{", "}")
	if got != "a  b  c" {
		t.Errorf("got %q, want %q", got, "a  b  c")
	}
}

func TestStripBetween_NoMatch(t *testing.T) {
	got := doStripBetween("hello", "{", "}")
	if got != "hello" {
		t.Errorf("got %q, want %q", got, "hello")
	}
}

// --- Edge cases ---

func TestInterpolate_PipelineNoSpaces(t *testing.T) {
	// Pipeline split now handles |upper| without spaces
	e := fixedEval(map[string]interface{}{"x": "hello"})
	got := e.Interpolate("{{x|upper}}")
	if got != "HELLO" {
		t.Errorf("got %q, want %q", got, "HELLO")
	}
}

func TestInterpolate_NonNumericMath(t *testing.T) {
	e := fixedEval(map[string]interface{}{"x": "notanumber"})
	got := e.Interpolate("{{x | mul(6)}}")
	// non-numeric input should be returned unchanged
	if got != "notanumber" {
		t.Errorf("got %q, want %q", got, "notanumber")
	}
}

func TestCondition_DoubleNot(t *testing.T) {
	e := fixedEval(testData())
	// not(not(isEmpty(zug1.via))) — via is empty → isEmpty=true → not=false → not=true
	if !e.EvalCondition("not(not(isEmpty(zug1.via)))") {
		t.Error("expected true")
	}
}

func TestCondition_GreaterThanNonNumeric(t *testing.T) {
	e := fixedEval(map[string]interface{}{"x": "abc"})
	if e.EvalCondition("greaterThan(x, 0)") {
		t.Error("non-numeric field should return false for greaterThan")
	}
}

func TestCondition_StartsWith_EmptyArg(t *testing.T) {
	e := fixedEval(testData())
	// empty prefix always matches
	if !e.EvalCondition("startsWith(zug1.hinweis, '')") {
		t.Error("empty prefix should always match")
	}
}

// --- StringOrCond / elif ---

// parseStringOrCond is a helper that parses a YAML fragment into a StringOrCond.
func parseStringOrCond(t *testing.T, yamlStr string) StringOrCond {
	t.Helper()
	type wrapper struct {
		V StringOrCond `yaml:"v"`
	}
	var w wrapper
	if err := yaml.Unmarshal([]byte("v:\n"+yamlStr), &w); err != nil {
		t.Fatalf("yaml.Unmarshal: %v", err)
	}
	return w.V
}

func TestStringOrCond_PlainString(t *testing.T) {
	s := parseStringOrCond(t, `  hello`)
	if got := s.Resolve(nil); got != "hello" {
		t.Errorf("got %q, want %q", got, "hello")
	}
}

func TestStringOrCond_IfThenElse_TrueBranch(t *testing.T) {
	yaml := `
  if:   equals(status, 'ok')
  then: green
  else: red`
	e := fixedEval(map[string]interface{}{"status": "ok"})
	s := parseStringOrCond(t, yaml)
	if got := s.Resolve(e); got != "green" {
		t.Errorf("got %q, want %q", got, "green")
	}
}

func TestStringOrCond_IfThenElse_FalseBranch(t *testing.T) {
	yaml := `
  if:   equals(status, 'ok')
  then: green
  else: red`
	e := fixedEval(map[string]interface{}{"status": "fail"})
	s := parseStringOrCond(t, yaml)
	if got := s.Resolve(e); got != "red" {
		t.Errorf("got %q, want %q", got, "red")
	}
}

func TestStringOrCond_Elif_FirstBranch(t *testing.T) {
	raw := `
  if:   equals(status, 'delayed')
  then: '#FF0000'
  elif: equals(status, 'cancelled')
  then: '#888888'
  else: '#FFFFFF'`
	e := fixedEval(map[string]interface{}{"status": "delayed"})
	s := parseStringOrCond(t, raw)
	if got := s.Resolve(e); got != "#FF0000" {
		t.Errorf("got %q, want %q", got, "#FF0000")
	}
}

func TestStringOrCond_Elif_SecondBranch(t *testing.T) {
	raw := `
  if:   equals(status, 'delayed')
  then: '#FF0000'
  elif: equals(status, 'cancelled')
  then: '#888888'
  else: '#FFFFFF'`
	e := fixedEval(map[string]interface{}{"status": "cancelled"})
	s := parseStringOrCond(t, raw)
	if got := s.Resolve(e); got != "#888888" {
		t.Errorf("got %q, want %q", got, "#888888")
	}
}

func TestStringOrCond_Elif_ElseBranch(t *testing.T) {
	raw := `
  if:   equals(status, 'delayed')
  then: '#FF0000'
  elif: equals(status, 'cancelled')
  then: '#888888'
  else: '#FFFFFF'`
	e := fixedEval(map[string]interface{}{"status": "on-time"})
	s := parseStringOrCond(t, raw)
	if got := s.Resolve(e); got != "#FFFFFF" {
		t.Errorf("got %q, want %q", got, "#FFFFFF")
	}
}

func TestStringOrCond_MultipleElif(t *testing.T) {
	raw := `
  if:   equals(x, '1')
  then: one
  elif: equals(x, '2')
  then: two
  elif: equals(x, '3')
  then: three
  else: other`
	for _, tc := range []struct{ val, want string }{
		{"1", "one"}, {"2", "two"}, {"3", "three"}, {"9", "other"},
	} {
		e := fixedEval(map[string]interface{}{"x": tc.val})
		s := parseStringOrCond(t, raw)
		if got := s.Resolve(e); got != tc.want {
			t.Errorf("x=%q: got %q, want %q", tc.val, got, tc.want)
		}
	}
}

func TestStringOrCond_NoElif_NilEval(t *testing.T) {
	raw := `
  if:   equals(x, '1')
  then: one
  else: fallback`
	s := parseStringOrCond(t, raw)
	// nil eval → always falls back to else
	if got := s.Resolve(nil); got != "fallback" {
		t.Errorf("got %q, want %q", got, "fallback")
	}
}

func TestStringOrCond_Error_ThenWithoutIf(t *testing.T) {
	type wrapper struct {
		V StringOrCond `yaml:"v"`
	}
	var w wrapper
	err := yaml.Unmarshal([]byte("v:\n  then: oops"), &w)
	if err == nil {
		t.Error("expected error for 'then' without 'if'")
	}
}

func TestStringOrCond_Error_IfWithoutThen(t *testing.T) {
	type wrapper struct {
		V StringOrCond `yaml:"v"`
	}
	var w wrapper
	err := yaml.Unmarshal([]byte("v:\n  if: equals(x, '1')"), &w)
	if err == nil {
		t.Error("expected error for 'if' without 'then'")
	}
}

func TestStringOrCond_Error_TooManyBranches(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("v:\n  if: equals(x, '0')\n  then: zero\n")
	for i := 1; i <= maxCondBranches; i++ {
		fmt.Fprintf(&sb, "  elif: equals(x, '%d')\n  then: val%d\n", i, i)
	}
	type wrapper struct {
		V StringOrCond `yaml:"v"`
	}
	var w wrapper
	err := yaml.Unmarshal([]byte(sb.String()), &w)
	if err == nil {
		t.Errorf("expected error when exceeding maxCondBranches (%d)", maxCondBranches)
	}
}

// --- formatTime ---

func TestFormatTime_EE(t *testing.T) {
	// 2026-03-24 is Tuesday = "Di"
	t1 := time.Date(2026, 3, 24, 0, 0, 0, 0, time.UTC)
	if got := formatTime(t1, "EE"); got != "Di" {
		t.Errorf("got %q, want %q", got, "Di")
	}
	if got := formatTime(t1, "EEEE"); got != "Dienstag" {
		t.Errorf("got %q, want %q", got, "Dienstag")
	}
}
