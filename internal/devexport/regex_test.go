package devexport

import (
	"encoding/json"
	"regexp"
	"regexp/syntax"
	"strings"
	"testing"
)

func TestCompileRegexRejects(t *testing.T) {
	for _, p := range []string{"(", "a**", `\8`, "[z-a]", "x{1001}", "(?P<>a)", `\pX`, "a)", "(?z)", "\xff"} {
		_, want := regexp.Compile(p)
		if want == nil {
			t.Fatalf("%q: test pattern must be invalid in Go", p)
		}
		_, got := CompileRegex(p)
		if got == nil {
			t.Errorf("%q: CompileRegex accepted an invalid pattern", p)
			continue
		}
		if got.Error() != want.Error() {
			t.Errorf("%q: error %q, want %q", p, got, want)
		}
	}
}

func TestCompileRegexFoldCaseExpanded(t *testing.T) {
	p, err := CompileRegex("(?i)k")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, in := range p.Inst {
		if in.Op == uint8(syntax.InstRune) {
			found = true
			// K, k and KELVIN SIGN as sorted ranges.
			want := []rune{'K', 'K', 'k', 'k', 0x212A, 0x212A}
			if in.Arg != 0 || len(in.Runes) != len(want) {
				t.Fatalf("rune inst = %+v, want runes %v and arg 0", in, want)
			}
			for i := range want {
				if in.Runes[i] != want[i] {
					t.Fatalf("runes %v, want %v", in.Runes, want)
				}
			}
		}
	}
	if !found {
		t.Fatal("no InstRune in program")
	}
}

func TestCompileRegexJSON(t *testing.T) {
	p, err := CompileRegex("a")
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	var raw []json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil || len(raw) != 3 {
		t.Fatalf("prog JSON %s: want [start,ncap,[insts]]", b)
	}
	var insts [][]json.RawMessage
	if err := json.Unmarshal(raw[2], &insts); err != nil {
		t.Fatal(err)
	}
	for _, in := range insts {
		if len(in) != 4 {
			t.Fatalf("inst JSON %s: want [op,out,arg,[runes]]", b)
		}
	}
	// Rune1 'a' must be present as [8,out,0,[97]].
	if !strings.Contains(string(b), `[8,`) || !strings.Contains(string(b), `[97]`) {
		t.Errorf("prog JSON %s lacks Rune1 'a'", b)
	}
}
