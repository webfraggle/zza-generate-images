package devicerender

import (
	"encoding/json"
	"math"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/webfraggle/zza-generate-images/internal/devexport"
	"github.com/webfraggle/zza-generate-images/internal/renderer"
)

// corpusEntry holds only inputs; the expected output always comes from the Go
// renderer at test time.
type corpusEntry struct {
	Data json.RawMessage `json:"data"`
	Now  string          `json:"now"`
	Anim []int           `json:"anim"`
	Expr string          `json:"expr"`
	Cond bool            `json:"cond"`
	Int  bool            `json:"int"`
	Vars map[string]int  `json:"vars"`
}

// matchPatternRe finds the pattern argument of match filters in an expression.
var matchPatternRe = regexp.MustCompile(`match\(\s*('[^']*'|"[^"]*"|[^,)]*)`)

// regexTableFor compiles the constant match patterns of exprs into a `_re`
// table (as the exporter does); invalid patterns get no program.
func regexTableFor(t *testing.T, exprs ...string) string {
	t.Helper()
	table := map[string]devexport.Prog{}
	for _, expr := range exprs {
		for _, m := range matchPatternRe.FindAllStringSubmatch(expr, -1) {
			p := strings.TrimSpace(m[1])
			if len(p) >= 2 && (p[0] == '\'' || p[0] == '"') && p[len(p)-1] == p[0] {
				p = p[1 : len(p)-1]
			}
			if prog, err := devexport.CompileRegex(p); err == nil {
				table[p] = prog
			}
		}
	}
	b, err := json.Marshal(table)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func loadCorpus(t *testing.T) []corpusEntry {
	t.Helper()
	b, err := os.ReadFile("testdata/eval_corpus.json")
	if err != nil {
		t.Fatal(err)
	}
	var c []corpusEntry
	if err := json.Unmarshal(b, &c); err != nil {
		t.Fatal(err)
	}
	return c
}

func toZza(tm time.Time) zzaTime {
	return zzaTime{tm.Year(), int(tm.Month()), tm.Day(), tm.Hour(), tm.Minute(), tm.Second(), int(tm.Weekday())}
}

// compareEval runs one entry through Go and C++ and reports a mismatch.
func compareEval(t *testing.T, idx int, dataJSON, regexJSON string, now time.Time, anim []int, expr string, cond bool) {
	t.Helper()
	var data map[string]any
	if err := json.Unmarshal([]byte(dataJSON), &data); err != nil {
		t.Fatalf("entry %d: bad data: %v", idx, err)
	}
	want := renderer.EvalForTest(data, now, anim[0], anim[1], anim[2], expr, cond)
	got, rc := testEval(dataJSON, regexJSON, toZza(now), anim[0], anim[1], anim[2], expr, cond)
	if rc != 0 {
		t.Errorf("entry %d %q: C++ error code %d", idx, expr, rc)
		return
	}
	if got != want {
		t.Errorf("entry %d expr %q (cond=%v, now=%s, anim=%v)\n data: %s\n  go:  %q\n  c++: %q", idx, expr, cond, now.Format(time.RFC3339), anim, dataJSON, want, got)
	}
}

func TestEvalCorpus(t *testing.T) {
	corpus := loadCorpus(t)
	var exprs []string
	for _, e := range corpus {
		exprs = append(exprs, e.Expr)
	}
	regexJSON := regexTableFor(t, exprs...)
	n, nMatch := 0, 0
	for i, e := range corpus {
		if e.Int {
			continue
		}
		now, err := time.Parse(time.RFC3339, e.Now)
		if err != nil {
			t.Fatalf("entry %d: %v", i, err)
		}
		compareEval(t, i, string(e.Data), regexJSON, now, e.Anim, e.Expr, e.Cond)
		n++
		if strings.Contains(e.Expr, "match(") {
			nMatch++
		}
	}
	t.Logf("compared %d entries (%d with match)", n, nMatch)
	if nMatch == 0 {
		t.Fatal("corpus has no match entries")
	}
}

// match with Go's limits: overlong pattern (even with a program), overlong
// value, negative group; and a pattern without program behaves as invalid.
func TestEvalMatchLimits(t *testing.T) {
	now := time.Date(2024, 3, 5, 7, 8, 9, 0, time.UTC)
	long := strings.Repeat("a", 101)
	exprs := []string{
		"{{v | match('" + long + "', 0)}}",
		"{{v | match('" + long[:100] + "', 0)}}",
		"{{w | match('a+', 0)}}",
		"{{x | match('a+', 0)}}",
		"{{v | match('(a)', -1)}}",
		"{{v | match(\"(a+)\", 1)}}",
	}
	big := strings.Repeat("a", 64*1024)
	data, _ := json.Marshal(map[string]string{"v": strings.Repeat("a", 120), "w": big + "a", "x": big})
	regexJSON := regexTableFor(t, exprs...)
	if !strings.Contains(regexJSON, long) {
		t.Fatal("table must hold the overlong pattern so the C++ limit is exercised")
	}
	for i, expr := range exprs {
		compareEval(t, 200000+i, string(data), regexJSON, now, []int{0, 1, 0}, expr, false)
	}
	// No program for the pattern: "" (Go yields "aaa" here, but the exporter
	// only accepts templates whose patterns all have programs).
	got, rc := testEval(`{"v":"aaa"}`, "", toZza(now), 0, 1, 0, "{{v | match('a+', 0)}}", false)
	if rc != 0 || got != "" {
		t.Errorf("match without program = %q (rc %d), want \"\"", got, rc)
	}
	// A malformed table is a format error.
	if _, rc := testEval(`{"v":"a"}`, `{"a":[0,2,[]]}`, toZza(now), 0, 1, 0, "{{v}}", false); rc != 1 { // ZZA_ERR_FORMAT
		t.Errorf("malformed regex table: rc %d, want ZZA_ERR_FORMAT", rc)
	}
}

// Expressions with raw bytes JSON cannot carry (invalid UTF-8).
func TestEvalInvalidUTF8(t *testing.T) {
	now := time.Date(2024, 3, 5, 7, 8, 9, 0, time.UTC)
	data := `{"v":"ab"}`
	for i, expr := range []string{
		"{{v | prefix('\xff') | upper}}",
		"{{v | prefix('\xff') | lower}}",
		"{{v | prefix('a\xc3') | upper}}",
		"{{v | suffix('\xe2\x82') | upper}}",
		"{{v | prefix('\xed\xa0\x80') | upper}}",
		"{{v | prefix('\xc0\x80') | lower}}",
		"{{v | prefix('\xf4\x90\x80\x80') | upper}}",
		"{{v | prefix('\xff') | trim}}",
		"{{v | prefix('\xc2\xa0 ') | suffix(' \xe2\x80\x83') | trim}}",
		"{{v | prefix('\xe2\x80') | trim}}",
		"{{v | prefix('\xef\xbf\xbd') | upper}}",
		"{{v | prefix('ä\xffö') | upper | lower}}",
	} {
		compareEval(t, 100000+i, data, "", now, []int{0, 1, 0}, expr, false)
	}
}

func TestIntExprCorpus(t *testing.T) {
	corpus := loadCorpus(t)
	n := 0
	for i, e := range corpus {
		if !e.Int {
			continue
		}
		want, werr := renderer.EvalIntExprForTest(e.Expr, e.Vars)
		varsJSON, _ := json.Marshal(e.Vars)
		got, rc := testIntExpr(e.Expr, string(varsJSON))
		// The core returns int: results outside int32 count as errors there.
		wantErr := werr != nil || want < math.MinInt32 || want > math.MaxInt32
		if wantErr {
			if rc == 0 {
				t.Errorf("entry %d %q: Go error/overflow (%v, %d) but C++ returned %d", i, e.Expr, werr, want, got)
			}
		} else if rc != 0 || got != want {
			t.Errorf("entry %d %q: go %d, C++ %d (rc %d)", i, e.Expr, want, got, rc)
		}
		n++
	}
	t.Logf("compared %d expressions", n)
}

// Review focus 1: a large value must not blow up time or memory.
func TestEvalLongInput(t *testing.T) {
	var sb strings.Builder
	for sb.Len() < 200*1024 {
		sb.WriteString("ab\U0001F600c")
	}
	if utf8.RuneCountInString(sb.String()) < 10000 {
		t.Fatal("test value too small")
	}
	dataBytes, _ := json.Marshal(map[string]any{"v": sb.String()})
	now := time.Date(2024, 3, 5, 7, 8, 9, 0, time.UTC)
	expr := "{{v | upper | replace('A','bb') | before('x')}}"
	var data map[string]any
	_ = json.Unmarshal(dataBytes, &data)
	want := renderer.EvalForTest(data, now, 0, 1, 0, expr, false)
	start := time.Now()
	got, rc := testEval(string(dataBytes), "", toZza(now), 0, 1, 0, expr, false)
	el := time.Since(start)
	if rc != 0 || got != want {
		t.Fatalf("long input mismatch (rc=%d, len go=%d c++=%d)", rc, len(want), len(got))
	}
	if el > 100*time.Millisecond {
		t.Errorf("evaluation took %v, want < 100ms", el)
	}
	// Same through a 60 KB value so replace actually runs (Go caps the result at 64 KB).
	small := sb.String()[:60*1024]
	dataBytes, _ = json.Marshal(map[string]any{"v": small})
	_ = json.Unmarshal(dataBytes, &data)
	expr = "{{v | replace('a','bb') | upper | before('x')}}"
	want = renderer.EvalForTest(data, now, 0, 1, 0, expr, false)
	start = time.Now()
	got, rc = testEval(string(dataBytes), "", toZza(now), 0, 1, 0, expr, false)
	if rc != 0 || got != want {
		t.Fatalf("60KB mismatch (rc=%d, len go=%d c++=%d)", rc, len(want), len(got))
	}
	if el = time.Since(start); el > 100*time.Millisecond {
		t.Errorf("60KB evaluation took %v, want < 100ms", el)
	}
	// stripBetween over many pairs must stay linear.
	var pairs strings.Builder
	for i := 0; i < 20000; i++ {
		pairs.WriteString("x[ab]y")
	}
	dataBytes, _ = json.Marshal(map[string]any{"v": pairs.String()})
	_ = json.Unmarshal(dataBytes, &data)
	expr = "{{v | stripBetween('[',']') | upper}}"
	want = renderer.EvalForTest(data, now, 0, 1, 0, expr, false)
	start = time.Now()
	got, rc = testEval(string(dataBytes), "", toZza(now), 0, 1, 0, expr, false)
	if rc != 0 || got != want {
		t.Fatalf("stripBetween mismatch (rc=%d)", rc)
	}
	if el = time.Since(start); el > 100*time.Millisecond {
		t.Errorf("stripBetween took %v, want < 100ms", el)
	}
}

func TestLatin1Helper(t *testing.T) {
	for _, s := range []string{"", "abc", "äöüß ÿ É", "€ ő 😀 x", "\xff\xfe", "a\xc3", "日本"} {
		var want []byte
		strictOK := true
		for _, r := range s { // invalid bytes iterate as U+FFFD
			if r > 0xFF {
				want = append(want, '?')
				strictOK = false
			} else {
				want = append(want, byte(r))
			}
		}
		got, ok := testLatin1(s, false)
		if !ok || string(got) != string(want) {
			t.Errorf("latin1(%q) = %q ok=%v, want %q", s, got, ok, want)
		}
		_, ok = testLatin1(s, true)
		if ok != strictOK {
			t.Errorf("strict latin1(%q) ok=%v, want %v", s, ok, strictOK)
		}
	}
}
