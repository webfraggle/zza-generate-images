// Package devexport converts templates into packages for the ZZA controller.
package devexport

import (
	"encoding/json"
	"regexp/syntax"
	"slices"
	"unicode"
)

// Prog is a compiled RE2 program for the device's Pike VM (device/zzarender/regex.cpp).
// Op values are those of regexp/syntax.InstOp. Rune instructions carry no
// flags: case folding is expanded into explicit ranges, InstRune holds sorted
// [lo,hi] pairs and InstRune1 exactly one rune.
//
// JSON form: [start, numCap, [[op,out,arg,[r0,r1,…]], …]].
type Prog struct {
	Start, NumCap int
	Inst          []Inst
}

// Inst is one program instruction; JSON form [op,out,arg,[r0,r1,…]].
type Inst struct {
	Op       uint8
	Out, Arg uint32
	Runes    []rune
}

// MarshalJSON writes the compact array form.
func (p Prog) MarshalJSON() ([]byte, error) {
	inst := p.Inst
	if inst == nil {
		inst = []Inst{}
	}
	return json.Marshal([]any{p.Start, p.NumCap, inst})
}

// MarshalJSON writes the compact array form.
func (in Inst) MarshalJSON() ([]byte, error) {
	r := in.Runes
	if r == nil {
		r = []rune{}
	}
	return json.Marshal([]any{in.Op, in.Out, in.Arg, r})
}

// CompileRegex compiles pattern exactly like regexp.Compile (same parser
// flags, same errors) and returns the program in device form.
func CompileRegex(pattern string) (Prog, error) {
	re, err := syntax.Parse(pattern, syntax.Perl)
	if err != nil {
		return Prog{}, err
	}
	sp, err := syntax.Compile(re.Simplify())
	if err != nil {
		return Prog{}, err
	}
	p := Prog{Start: sp.Start, NumCap: sp.NumCap, Inst: make([]Inst, len(sp.Inst))}
	for i, si := range sp.Inst {
		in := Inst{Op: uint8(si.Op), Out: si.Out, Arg: si.Arg}
		switch si.Op {
		case syntax.InstRune1:
			in.Runes = []rune{si.Rune[0]}
		case syntax.InstRune:
			in.Arg = 0
			if len(si.Rune) == 1 {
				// Single rune; with FoldCase it matches its whole SimpleFold orbit.
				in.Runes = foldRanges(si.Rune[0], syntax.Flags(si.Arg)&syntax.FoldCase != 0)
			} else {
				in.Runes = slices.Clone(si.Rune)
			}
		case syntax.InstRuneAny, syntax.InstRuneAnyNotNL:
			in.Arg = 0
		}
		p.Inst[i] = in
	}
	return p, nil
}

// foldRanges returns r (and, if fold, every rune of its SimpleFold orbit) as
// sorted, merged [lo,hi] pairs.
func foldRanges(r rune, fold bool) []rune {
	set := []rune{r}
	if fold {
		for f := unicode.SimpleFold(r); f != r; f = unicode.SimpleFold(f) {
			set = append(set, f)
		}
	}
	slices.Sort(set)
	var out []rune
	for _, c := range set {
		if n := len(out); n > 0 && out[n-1]+1 >= c {
			out[n-1] = max(out[n-1], c)
			continue
		}
		out = append(out, c, c)
	}
	return out
}
