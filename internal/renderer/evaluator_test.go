package renderer

import (
	"fmt"
	"math/rand"
	"reflect"
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

// --- pipeline parser, variable arguments, mod/minutes ---

func pipeData() map[string]interface{} {
	return map[string]interface{}{
		"a": "5", "b": "2.5", "n": "-3",
		"z1": "10:15", "z2": "10:18", "z3": "10:26",
		"t1": "23:58", "t2": "00:04",
		"empty": "", "txt": "abc", "v": "a|b",
		"seven": "7", "sevenhalf": "7.5", "mseven": "-7",
	}
}

func checkInterp(t *testing.T, e *Evaluator, expr, want string) {
	t.Helper()
	if got := e.Interpolate(expr); got != want {
		t.Errorf("%s = %q, want %q", expr, got, want)
	}
}

func TestPipeline_VariableArgs(t *testing.T) {
	e := fixedEval(pipeData())
	for expr, want := range map[string]string{
		"{{a | add(b)}}":         "7.5",
		"{{a|add(b)}}":           "7.5",
		"{{a | mul(b) | round}}": "13",
		"{{a | add('3')}}":       "8",
		"{{a | sub(missing)}}":   "",
		"{{txt | add(a)}}":       "abc",
		"{{a | add(txt)}}":       "",
	} {
		checkInterp(t, e, expr, want)
	}
}

func TestFilter_Minutes(t *testing.T) {
	for in, want := range map[string]string{
		"10:15": "615", "0:04": "4", "23:58": "1438", " 10:15 Uhr": "615", "10:5": "605",
		"": "", "ca. 10": "", "1015": "", ":15": "",
	} {
		e := fixedEval(map[string]interface{}{"t": in})
		if got := e.Interpolate("{{t | minutes}}"); got != want {
			t.Errorf("minutes(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFilter_Mod(t *testing.T) {
	e := fixedEval(pipeData())
	for expr, want := range map[string]string{
		"{{n | mod(1440)}}":      "1437",
		"{{seven | mod(3)}}":     "1",
		"{{sevenhalf | mod(2)}}": "1.5",
		"{{mseven | mod(-3)}}":   "-1",
		"{{a | mod(0)}}":         "0",
		"{{txt | mod(3)}}":       "abc",
	} {
		checkInterp(t, e, expr, want)
	}
}

func TestPipeline_MinutesColumn(t *testing.T) {
	const expr = "{{Z | minutes | sub(R | minutes) | mod(1440) | add(5)}}"
	cases := []struct{ z, r, want string }{
		{"z1", "z1", "5"}, {"z2", "z1", "8"}, {"z3", "z1", "16"}, {"t2", "t1", "11"},
		{"empty", "z1", ""}, {"z2", "empty", ""}, {"txt", "z1", ""},
	}
	e := fixedEval(pipeData())
	for _, c := range cases {
		x := strings.NewReplacer("Z", c.z, "R", c.r).Replace(expr)
		checkInterp(t, e, x, c.want)
	}
}

func TestPipeline_ClockAngles(t *testing.T) {
	const expr = "{{now.hour12 | mul(30) | add(now.minute | div(2))}}"
	cases := []struct {
		h, m int
		want string
	}{{10, 15, "307.5"}, {12, 0, "360"}, {0, 30, "375"}, {3, 0, "90"}}
	for _, c := range cases {
		e := NewEvaluator(map[string]interface{}{})
		e.now = time.Date(2026, 11, 13, c.h, c.m, 0, 0, time.UTC)
		checkInterp(t, e, expr, c.want)
	}
}

func TestPipeline_QuotedPipeAndParens(t *testing.T) {
	e := fixedEval(pipeData())
	checkInterp(t, e, "{{v | stripAll('|')}}", "ab")
	checkInterp(t, e, "{{v | before('|')}}", "a")
}

func TestPipeline_DepthLimit(t *testing.T) {
	e := fixedEval(pipeData())
	checkInterp(t, e, "{{a | add(a | add(a | add(a | add(a))))}}", "25")
	checkInterp(t, e, "{{a | add(a | add(a | add(a | add(a | add(a)))))}}", "")
}

func TestPipeline_Malformed(t *testing.T) {
	e := fixedEval(pipeData())
	for expr, want := range map[string]string{
		"{{a | add(b}}":   "5",
		"{{a | add()}}":   "5",
		"{{a | add(b))}}": "",
		"{{a | add('x)}}": "",
		"{{ | }}":         "",
	} {
		checkInterp(t, e, expr, want)
	}
	rng := rand.New(rand.NewSource(42))
	const alphabet = "a|(),'\"0 ."
	for i := 0; i < 2000; i++ {
		b := make([]byte, rng.Intn(24))
		for j := range b {
			b[j] = alphabet[rng.Intn(len(alphabet))]
		}
		e.Interpolate("{{" + string(b) + "}}")
	}
}

func TestCondition_PipelineArg(t *testing.T) {
	e := fixedEval(pipeData())
	for expr, want := range map[string]bool{
		"greaterThan(z2 | minutes, 600)": true,
		"greaterThan(z1 | minutes, 615)": false,
		"equals(z1 | minutes, '615')":    true,
		"isEmpty(empty | minutes)":       true,
		"startsWith(txt, 'a')":           true,
	} {
		if got := e.EvalCondition(expr); got != want {
			t.Errorf("%s = %v, want %v", expr, got, want)
		}
	}
}

func TestSplitTopLevel(t *testing.T) {
	cases := []struct {
		in   string
		sep  byte
		want []string
	}{
		{"a | b(c | d) | e", '|', []string{"a ", " b(c | d) ", " e"}},
		{"'x|y' | z", '|', []string{"'x|y' ", " z"}},
		{"a)|b", '|', []string{"a)", "b"}},
		{"x, f(a, b), 'c,d'", ',', []string{"x", " f(a, b)", " 'c,d'"}},
	}
	for _, c := range cases {
		if got := splitTopLevel(c.in, c.sep); !reflect.DeepEqual(got, c.want) {
			t.Errorf("splitTopLevel(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// --- clock / replace / match / format offset ---

func TestFilter_ClockReplaceMatchFormatOffset(t *testing.T) {
	cases := []struct {
		name string
		data map[string]interface{}
		expr string
		want string
	}{
		{"clock 963", map[string]interface{}{"m": 963}, "{{m | clock}}", "16:03"},
		{"clock 0", map[string]interface{}{"m": 0}, "{{m | clock}}", "00:00"},
		{"clock wrap", map[string]interface{}{"m": 1445}, "{{m | clock}}", "00:05"},
		{"clock negative", map[string]interface{}{"m": -5}, "{{m | clock}}", "23:55"},
		{"clock text", map[string]interface{}{"m": "abc"}, "{{m | clock}}", ""},
		{"clock empty", map[string]interface{}{"m": ""}, "{{m | clock}}", ""},
		{"clock chain", map[string]interface{}{"zug1": map[string]interface{}{"zeit": "15:53", "abw": 10}},
			"{{zug1.zeit | minutes | add(zug1.abw) | clock}}", "16:03"},
		{"replace", map[string]interface{}{"v": "A - B - C"}, "{{v | replace(' - ', '~')}}", "A~B~C"},
		{"replace empty from", map[string]interface{}{"v": "A - B"}, "{{v | replace('', 'x')}}", "A - B"},
		{"match g1", map[string]interface{}{"nr": "S1"}, "{{nr | match('([a-zA-Z]+)([0-9]+)', 1)}}", "S"},
		{"match g2", map[string]interface{}{"nr": "S1"}, "{{nr | match('([a-zA-Z]+)([0-9]+)', 2)}}", "1"},
		{"match g0", map[string]interface{}{"nr": "S1"}, "{{nr | match('([a-zA-Z]+)([0-9]+)', 0)}}", "S1"},
		{"match none", map[string]interface{}{"nr": "REX 1234"}, "{{nr | match('^([a-zA-Z]+)([0-9]+)$', 1)}}", ""},
		{"match group out of range", map[string]interface{}{"nr": "S1"}, "{{nr | match('([a-zA-Z]+)([0-9]+)', 5)}}", ""},
		{"match invalid", map[string]interface{}{"nr": "S1"}, "{{nr | match('(', 0)}}", ""},
		{"match too long", map[string]interface{}{"nr": "S1"}, "{{nr | match('" + strings.Repeat("a", 101) + "', 0)}}", ""},
		{"match 100 chars ok", map[string]interface{}{"nr": strings.Repeat("a", 100)},
			"{{nr | match('" + strings.Repeat("a", 100) + "', 0)}}", strings.Repeat("a", 100)},
		{"format offset", nil, "{{now | format('HH:mm:ss', 5)}}", "12:01:03"},
		{"format offset negative", nil, "{{now | format('HH:mm:ss', 0 | sub(58))}}", "12:00:58"},
		{"format offset text", nil, "{{now | format('HH:mm', 'x')}}", "12:00"},
		{"format offset huge", nil, "{{now | format('HH:mm', 1e30)}}", "12:00"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := newEvaluatorAt(c.data, time.Date(2026, 3, 24, 12, 0, 58, 0, time.UTC))
			if got := e.Interpolate(c.expr); got != c.want {
				t.Errorf("%s = %q, want %q", c.expr, got, c.want)
			}
		})
	}
}

func TestFilter_FormatOffsetAnimTime(t *testing.T) {
	e := newEvaluatorAt(nil, time.Date(2026, 3, 24, 12, 0, 0, 0, time.UTC)).withAnim(3, 60, 100)
	if got := e.Interpolate("{{now | format('HH:mm:ss', anim.time)}}"); got != "12:00:03" {
		t.Errorf("got %q, want 12:00:03", got)
	}
}

func TestFilter_MatchAsCondition(t *testing.T) {
	e := NewEvaluator(map[string]interface{}{"nr": "S1", "x": "REX 12"})
	if e.EvalCondition("isEmpty(nr | match('([a-zA-Z]+)([0-9]+)', 2))") {
		t.Error("S1 should match")
	}
	if !e.EvalCondition("isEmpty(x | match('^([a-zA-Z]+)([0-9]+)$', 2))") {
		t.Error("REX 12 should not match")
	}
}
