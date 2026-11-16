package devicerender

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/webfraggle/zza-generate-images/internal/devexport"
	"github.com/webfraggle/zza-generate-images/internal/renderer"
)

// regexCase is one corpus entry; inx (hex bytes) carries inputs JSON cannot
// hold (invalid UTF-8). The expected result always comes from Go.
type regexCase struct {
	P   string `json:"p"`
	In  string `json:"in"`
	Inx string `json:"inx"`
	G   int    `json:"g"`
}

func (c regexCase) input(t testing.TB) string {
	if c.Inx == "" {
		return c.In
	}
	b, err := hex.DecodeString(c.Inx)
	if err != nil {
		t.Fatalf("bad inx %q: %v", c.Inx, err)
	}
	return string(b)
}

func loadRegexCorpus(t testing.TB) []regexCase {
	t.Helper()
	b, err := os.ReadFile("testdata/regex_corpus.json")
	if err != nil {
		t.Fatal(err)
	}
	var c []regexCase
	if err := json.Unmarshal(b, &c); err != nil {
		t.Fatal(err)
	}
	return c
}

func progJSON(t testing.TB, pattern string) string {
	t.Helper()
	p, err := devexport.CompileRegex(pattern)
	if err != nil {
		t.Fatalf("CompileRegex(%q): %v", pattern, err)
	}
	b, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func checkRegex(t testing.TB, pattern, in string, g int) {
	t.Helper()
	want := renderer.MatchGroupForTest(in, pattern, g)
	got, rc := testRegex(progJSON(t, pattern), in, g)
	if rc != 0 {
		t.Errorf("%q on %q group %d: C++ error code %d", pattern, in, g, rc)
		return
	}
	if got != want {
		t.Errorf("%q on %q group %d: go %q, c++ %q", pattern, in, g, want, got)
	}
}

func TestRegexCorpus(t *testing.T) {
	corpus := loadRegexCorpus(t)
	if len(corpus) < 150 {
		t.Fatalf("corpus has %d cases, want >= 150", len(corpus))
	}
	for _, c := range corpus {
		checkRegex(t, c.P, c.input(t), c.G)
	}
}

// Go's input limit: values longer than 64 KiB never match.
func TestRegexInputLimit(t *testing.T) {
	for _, n := range []int{64*1024 - 1, 64 * 1024, 64*1024 + 1} {
		checkRegex(t, "a$", strings.Repeat("a", n), 0)
	}
}

// Review focus 3: pathological patterns stay linear and agree with Go.
func TestRegexLinear(t *testing.T) {
	cases := []struct{ p, in string }{
		{`(a*)*b`, strings.Repeat("a", 10000)},
		{`(a|aa)+$`, strings.Repeat("a", 10000)},
		{`(a|aa)+$`, strings.Repeat("a", 9999) + "b"},
		{`(x+x+)+y`, strings.Repeat("x", 10000)},
		{`(x+x+)+y`, strings.Repeat("x", 9999) + "y"},
	}
	for _, c := range cases {
		prog := progJSON(t, c.p)
		for g := 0; g <= 1; g++ {
			want := renderer.MatchGroupForTest(c.in, c.p, g)
			start := time.Now()
			got, rc := testRegex(prog, c.in, g)
			el := time.Since(start)
			if rc != 0 || got != want {
				t.Errorf("%q group %d: rc %d, len go %d, c++ %d", c.p, g, rc, len(want), len(got))
			}
			if el > 50*time.Millisecond {
				t.Errorf("%q group %d took %v, want < 50ms", c.p, g, el)
			}
		}
	}
}

// Malformed programs (a damaged or hostile package) are rejected, never run.
func TestRegexProgRejected(t *testing.T) {
	for _, p := range []string{
		`[]`,
		`[0,2,[]]`,                               // no instructions
		`[5,2,[[5,0,0,[]],[4,0,0,[]]]]`,          // start out of range
		`[1,2,[[5,0,0,[]],[6,9,0,[]]]]`,          // out out of range
		`[1,2,[[5,0,0,[]],[0,0,7,[]]]]`,          // alt arg out of range
		`[1,2,[[5,0,0,[]],[42,0,0,[]]]]`,         // unknown op
		`[1,2,[[5,0,0,[]],[7,0,0,[1]]]]`,         // odd range list
		`[1,2,[[5,0,0,[]],[7,0,0,[5,3]]]]`,       // lo > hi
		`[1,2,[[5,0,0,[]],[7,0,0,[1,5,4,8]]]]`,   // overlapping ranges
		`[1,2,[[5,0,0,[]],[7,0,0,[1,1114112]]]]`, // beyond U+10FFFF
		`[1,2,[[5,0,0,[]],[8,0,0,[1,2]]]]`,       // Rune1 with two runes
		`[1,3,[[5,0,0,[]],[4,0,0,[]]]]`,          // odd capture count
		`[1,2,[[5,0,0,[]],[4,0,0]]]`,             // short instruction
		`[1,2,[[5,0,0,[]],[4,0,0,[]]]`,           // broken JSON
	} {
		if _, rc := testRegex(p, "a", 0); rc == 0 {
			t.Errorf("program %s was accepted", p)
		}
	}
}

func FuzzRegex(f *testing.F) {
	for _, c := range loadRegexCorpus(f) {
		f.Add(c.P, c.input(f))
	}
	f.Fuzz(func(t *testing.T, pattern, in string) {
		if utf8.RuneCountInString(pattern) > 100 || len(in) > 4096 {
			return
		}
		if _, err := regexp.Compile(pattern); err != nil {
			return
		}
		for g := 0; g <= 1; g++ {
			checkRegex(t, pattern, in, g)
		}
	})
}
