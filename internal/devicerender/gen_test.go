package devicerender

import (
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/webfraggle/zza-generate-images/internal/devexport"
	"github.com/webfraggle/zza-generate-images/internal/renderer"
)

// Random template generator (spec §9 "zufällig erzeugte Templates"). It builds
// templates from the building blocks of docs/yaml-template-spec.md: all layer
// types, layer- and block-level if/elif/else, nesting up to three containers,
// value pipelines (filters incl. match, format, maths), conditions, coordinate
// expressions (i, loop.index, <id>.width/.end, anim.frame), StringOrCond
// fields and optionally split-flap or frame animation. Constants are Latin-1
// only and the canvas is 240 × 240 (anything else is rejected by design).
//
// Patterns the generator deliberately never produces:
//   - In split-flap mode no copy layer stands between the first and the last
//     splitflap layer (also not inside loops or blocks there). A copy there can
//     move an animated module onto the area of a later module, which the spec
//     forbids ("ein splitflap darf nicht auf etwas liegen, das sich pro Frame
//     ändert"); the device takes the leaves from the still canvas (ruling R13).
//     Copies before the first or after the last splitflap are generated.
//   - Train data stays Latin-1: the device draws dynamic runes > U+00FF as '?'
//     by design (spec §7); FuzzEvalData covers other runes for robustness.
//   - stripBetween with an empty delimiter (Go loops forever, ruling R5).
//   - Identical overdraw in loops: the y of every layer inside a loop depends
//     on the index. The device draws glyphs at whole pixels, Go at subpixel
//     positions (accepted, spec §12 "Glyphenkanten"); drawing the same text
//     up to 20 times on the same spot multiplies that edge difference (seed 41
//     of the first generator version: block Δ 46). Overdraw by separate layers
//     (fake bold, like oebb-105-v1) is still generated.
//
// Occasional invalid constructs (undefined i outside a loop, colours from the
// data, a zero-size copy, rotate "abc", elif without if, too many layers via
// loops) are generated on purpose: then Go's error must be matched by the
// exporter or the C++ core.

const (
	genStatic = iota
	genFlap
	genFrames
)

// genScope is what the expressions of a layer may refer to.
type genScope struct {
	loopVar string    // "" outside loops
	flapVar string    // "" outside splitflap children
	ids     *[]string // text ids visible here; layers append the ids they define
	depth   int       // container nesting (blocks, loop, splitflap)
	noCopy  bool      // split-flap section: no copy layers
	inLoop  bool
}

type genLayer struct {
	kv   [][2]string // key, YAML value
	kids []genLayer  // "layers:" (nil = none)
}

type gen struct {
	rng     *rand.Rand
	mode    int
	combos  [][3]float64 // size, scale_x, scale_y used by this template
	idSeq   int
	modules int
	flaps   []image.Rectangle
	badElif bool // emit one elif without if (error parity)
}

var (
	genFields  = []string{"zug", "ziel", "via", "zeit", "hinweis", "gleis", "abw", "linien", "halte", "n", "status", "zug1.zeit", "zug1.nr", "zug1.vonnach", "leer"}
	genNumeric = []string{"abw", "gleis", "n", "zug1.zeit | minutes", "zeit | minutes", "now.minute", "now.second", "now.hour12"}
	genWords   = []string{"Gleis", "nach", "Zürich", "Genève", "Ab", "Über", "»", "°", "Süd", "Straße", "ÆØÅ", "·", "¡Olé!", "13:30", "-", " ", "", "Fußweg ½ km", "©"}
	genRegexes = []string{`[0-9]+`, `^([A-Z]+) ?([0-9]*)`, `(\w+)$`, `[äöü]`, `H(a|ä)lt`, `^\*?(.*)$`, `(\d+):(\d+)`, `[^ ]+`}
	genReels   = []string{" ABCDEFGHIJKLMNOPQRSTUVWXYZ-.", "0123456789", " ÄÖÜABC", "ab", "-*+"}
	genSizes   = []float64{9, 11, 14, 18, 24, 13.5}
	genScales  = [][2]float64{{1, 1}, {0.5, 1}, {1.4, 1}, {1, 1.6}, {0.8, 1.3}}
)

func (g *gen) intn(n int) int              { return g.rng.Intn(n) }
func (g *gen) chance(p float64) bool       { return g.rng.Float64() < p }
func (g *gen) pick(s []string) string      { return s[g.intn(len(s))] }
func (g *gen) between(lo, hi int) int      { return lo + g.intn(hi-lo+1) }
func yq(s string) string                   { return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s) + `"` }
func (l *genLayer) set(k, v string)        { l.kv = append(l.kv, [2]string{k, v}) }
func (l *genLayer) setInt(k string, v int) { l.set(k, fmt.Sprint(v)) }

// operand returns a variable for a pipeline.
func (g *gen) operand(sc genScope) string {
	switch {
	case sc.flapVar != "" && g.chance(0.5):
		return sc.flapVar
	case sc.loopVar != "" && g.chance(0.4):
		return g.pick([]string{sc.loopVar, "i", "loop.index"})
	case g.mode == genFrames && g.chance(0.25):
		return g.pick([]string{"anim.frame", "anim.frames", "anim.time"})
	case g.chance(0.12):
		return g.pick([]string{"now", "now.hour", "now.minute", "now.weekday", "now.day", "now.year", "anim.frame"})
	}
	return g.pick(genFields)
}

// filter returns one pipeline filter (operand is the pipeline's operand).
func (g *gen) filter(sc genScope, operand string) string {
	if operand == "now" && g.chance(0.6) {
		f := g.pick([]string{"HH:mm", "dd.MM.yyyy", "EE HH:mm:ss", "EEEE", "hh:mm", "yyyy-MM-dd"})
		if g.mode == genFrames && g.chance(0.5) {
			return fmt.Sprintf("format('%s', anim.time)", f)
		}
		return fmt.Sprintf("format('%s')", f)
	}
	switch g.intn(22) {
	case 0:
		return "strip('*')"
	case 1:
		return "stripAll('" + g.pick([]string{" ", "|", "-", "ü"}) + "')"
	case 2:
		return "stripBetween('{', '}')"
	case 3:
		return "upper"
	case 4:
		return "lower"
	case 5:
		return "prefix('" + g.pick([]string{"+", "Gl. ", "» "}) + "')"
	case 6:
		return "suffix('" + g.pick([]string{" min", " Uhr", "°"}) + "')"
	case 7:
		return "trim"
	case 8:
		return "clock"
	case 9:
		return "replace('" + g.pick([]string{"a", " ", "ü", "IC"}) + "', '" + g.pick([]string{"ä", "_", "", "EC"}) + "')"
	case 10, 11:
		return fmt.Sprintf("match('%s', %d)", g.pick(genRegexes), g.intn(3))
	case 12:
		return "before('" + g.pick([]string{":", " ", "-"}) + "')"
	case 13:
		return "after('" + g.pick([]string{":", " ", "-"}) + "')"
	case 14:
		return "minutes"
	case 15:
		return "round"
	case 16:
		return fmt.Sprintf("mul(%s)", g.pick([]string{"2", "6", "0.5", "-1", "'3'"}))
	case 17:
		return fmt.Sprintf("div(%s)", g.pick([]string{"2", "3", "0", "0.25"}))
	case 18:
		return fmt.Sprintf("add(%s)", g.pick([]string{"5", "-0.5", "now.minute", "abw", "zeit | minutes"}))
	case 19:
		return fmt.Sprintf("sub(%s)", g.pick([]string{"1", "zug1.zeit | minutes", "n"}))
	case 20:
		return fmt.Sprintf("mod(%s)", g.pick([]string{"60", "1440", "0", "7"}))
	}
	if sc.loopVar != "" {
		return "add(i)"
	}
	return "upper"
}

// pipeline returns "operand | filter …" (no braces).
func (g *gen) pipeline(sc genScope) string {
	op := g.operand(sc)
	s := op
	for n := g.intn(4); n > 0; n-- {
		s += " | " + g.filter(sc, op)
	}
	return s
}

// text returns an interpolated string: literal words and {{pipelines}}.
func (g *gen) text(sc genScope) string {
	var b strings.Builder
	for n := g.between(1, 3); n > 0; n-- {
		if g.chance(0.6) {
			b.WriteString("{{" + g.pipeline(sc) + "}}")
		} else {
			b.WriteString(g.pick(genWords))
		}
		if g.chance(0.5) {
			b.WriteString(" ")
		}
	}
	return b.String()
}

// cond returns a condition expression.
func (g *gen) cond(sc genScope, depth int) string {
	if depth < 2 && g.chance(0.25) {
		switch g.intn(3) {
		case 0:
			return "not(" + g.cond(sc, depth+1) + ")"
		case 1:
			return "and(" + g.cond(sc, depth+1) + ", " + g.cond(sc, depth+1) + ")"
		default:
			return "or(" + g.cond(sc, depth+1) + ", " + g.cond(sc, depth+1) + ")"
		}
	}
	first := g.operand(sc)
	if g.chance(0.3) {
		first = g.pipeline(sc)
	}
	// Literals that the generated data often matches, so both branches occur.
	lit := g.pick([]string{"I", "IC", "S", "*", "Z", "B", "1", "13", "", "ü", "ok", "Basel SBB", "5", " ", "{", "full", "0"})
	switch g.intn(7) {
	case 0:
		return fmt.Sprintf("startsWith(%s, '%s')", first, lit)
	case 1:
		return fmt.Sprintf("endsWith(%s, '%s')", first, lit)
	case 2:
		return fmt.Sprintf("contains(%s, '%s')", first, lit)
	case 3:
		return fmt.Sprintf("isEmpty(%s)", first)
	case 4:
		return fmt.Sprintf("equals(%s, '%s')", first, lit)
	case 5:
		return fmt.Sprintf("greaterThan(%s, %s)", g.pick(genNumeric), g.pick([]string{"0", "5", "600", "-1", "2.5"}))
	}
	if g.mode == genFrames {
		return fmt.Sprintf("greaterThan(anim.frame, %d)", g.intn(6))
	}
	return g.pick([]string{"true", "false"})
}

// ycoord is coord for y: inside a loop always index-dependent (see above).
func (g *gen) ycoord(sc genScope, lo, hi int) string {
	if sc.loopVar == "" {
		return g.coord(sc, lo, hi)
	}
	if g.chance(0.5) {
		return yq(fmt.Sprintf("{{i * %d + %d}}", g.between(6, 20), g.between(lo, hi/2)))
	}
	return yq(fmt.Sprintf("{{%d - loop.index * %d}}", g.between(hi/2, hi), g.between(6, 20)))
}

// coord returns a coordinate: an integer in [lo, hi] or an expression around it.
func (g *gen) coord(sc genScope, lo, hi int) string {
	v := g.between(lo, hi)
	if !g.chance(0.3) {
		return fmt.Sprint(v)
	}
	var terms []string
	if sc.loopVar != "" {
		terms = append(terms, fmt.Sprintf("i * %d + %d", g.between(4, 20), v/2), fmt.Sprintf("%d - loop.index * %d", v, g.between(2, 9)))
	}
	for _, id := range sc.idList() {
		terms = append(terms, id+".end", fmt.Sprintf("%s.width / 2 + %d", id, v/2), fmt.Sprintf("%d - %s.width", v, id))
	}
	if g.mode == genFrames {
		terms = append(terms, fmt.Sprintf("anim.frame * %d + %d", g.between(1, 9), v/2), fmt.Sprintf("%d - anim.frames", v))
	}
	terms = append(terms, fmt.Sprintf("(%d + %d) * 2", v/4, v/4), fmt.Sprintf("%d - %d / 3", v+v/3, v), fmt.Sprintf("%d", v))
	if g.chance(0.02) {
		terms = append(terms, "i * 2") // undefined outside loops: Go's verdict decides
	}
	return yq("{{" + terms[g.intn(len(terms))] + "}}")
}

// color returns a colour value: constant, conditional or (rarely) from data.
func (g *gen) color(sc genScope, alpha bool) string {
	c := func() string {
		s := fmt.Sprintf("#%02x%02x%02x", g.intn(256), g.intn(256), g.intn(256))
		if alpha && g.chance(0.4) {
			s += fmt.Sprintf("%02x", g.between(30, 255))
		}
		return s
	}
	switch {
	case g.chance(0.15):
		return fmt.Sprintf("{if: %s, then: %s, else: %s}", yq(g.cond(sc, 0)), yq(c()), yq(c()))
	case g.chance(0.05):
		return fmt.Sprintf("{if: %s, then: %s, elif: %s, then: %s, else: %s}", yq(g.cond(sc, 0)), yq(c()), yq(g.cond(sc, 0)), yq(c()), yq(c()))
	case g.chance(0.02):
		return yq("{{farbe}}")
	}
	return yq(c())
}

func (g *gen) combo() [3]float64 {
	if len(g.combos) == 0 || (len(g.combos) < 4 && g.chance(0.3)) {
		s := genScales[g.intn(len(genScales))]
		g.combos = append(g.combos, [3]float64{genSizes[g.intn(len(genSizes))], s[0], s[1]})
	}
	return g.combos[g.intn(len(g.combos))]
}

func (sc genScope) idList() []string {
	if sc.ids == nil {
		return nil
	}
	return *sc.ids
}

func (g *gen) textLayer(sc genScope) genLayer {
	l := genLayer{}
	l.set("type", "text")
	if g.chance(0.15) {
		l.set("value", fmt.Sprintf("{if: %s, then: %s, else: %s}", yq(g.cond(sc, 0)), yq(g.text(sc)), yq(g.text(sc))))
	} else {
		l.set("value", yq(g.text(sc)))
	}
	c := g.combo()
	l.set("font", "reg")
	l.set("size", fmt.Sprint(c[0]))
	if c[1] != 1 {
		l.set("scale_x", fmt.Sprint(c[1]))
	}
	if c[2] != 1 {
		l.set("scale_y", fmt.Sprint(c[2]))
	}
	l.set("color", g.color(sc, false))
	maxX, maxY := 236, 236
	if sc.flapVar != "" {
		maxX, maxY = 40, 30
	}
	l.set("x", g.coord(sc, -10, maxX))
	l.set("y", g.ycoord(sc, -6, maxY))
	align := "left"
	fit := sc.flapVar == "" && g.chance(0.1)
	if !fit && g.chance(0.35) {
		align = g.pick([]string{"center", "right"})
		l.set("align", align)
		if g.chance(0.6) {
			l.set("width", g.coord(sc, 20, 200))
		}
	}
	switch g.intn(5) {
	case 0:
		l.set("valign", "baseline")
	case 1:
		l.set("valign", g.pick([]string{"middle", "bottom", "top"}))
		l.set("height", g.coord(sc, 10, 60))
	}
	if fit {
		l.set("max_width", g.coord(sc, 0, 200))
		l.set("fit_items", fmt.Sprintf("{split_by: %s, gap: %d}", yq(g.pick([]string{"|", " ", "-", ","})), g.intn(6)))
	} else if g.chance(0.3) {
		l.set("max_width", g.coord(sc, 30, 220))
		if g.chance(0.4) {
			l.set("line_height", g.pick([]string{"0.8", "1", "1.3", "2"}))
		}
	}
	if align == "left" && !fit && sc.ids != nil && g.chance(0.3) {
		g.idSeq++
		id := fmt.Sprintf("t%d", g.idSeq)
		l.set("id", id)
		if g.chance(0.5) {
			l.setInt("advance", g.intn(8))
		}
		*sc.ids = append(*sc.ids, id)
	}
	return l
}

func (g *gen) imageLayer(sc genScope) genLayer {
	l := genLayer{}
	l.set("type", "image")
	l.set("file", g.pick([]string{"opaque.png", "alpha.png", "hand.png"}))
	maxX, maxY := 230, 230
	if sc.flapVar != "" {
		maxX, maxY = 30, 20
	}
	l.set("x", g.coord(sc, -20, maxX))
	l.set("y", g.ycoord(sc, -20, maxY))
	if g.chance(0.3) {
		l.set("width", g.coord(sc, 1, 120))
	}
	if g.chance(0.3) {
		l.set("height", g.coord(sc, 1, 120))
	}
	if g.chance(0.3) {
		switch g.intn(5) {
		case 0:
			l.set("rotate", yq(g.pick([]string{"0", "37.5", "90", "-15", "200", "720"})))
		case 1:
			l.set("rotate", yq("{{now.second | mul(6)}}"))
		case 2:
			if g.mode == genFrames {
				l.set("rotate", yq(fmt.Sprintf("{{anim.frame | mul(%d)}}", g.between(3, 45))))
			} else {
				l.set("rotate", yq("{{"+g.pick(genNumeric)+" | mul(6)}}"))
			}
		case 3:
			l.set("rotate", fmt.Sprintf("{if: %s, then: %s, else: %s}", yq(g.cond(sc, 0)), yq("10"), yq("{{now.minute | mul(6) | add(5)}}")))
		default:
			if g.chance(0.1) {
				l.set("rotate", yq("abc"))
			} else {
				l.set("rotate", yq("{{"+g.pick(genNumeric)+" | "+g.filter(sc, "")+"}}"))
			}
		}
	}
	return l
}

func (g *gen) rectLayer(sc genScope) genLayer {
	l := genLayer{}
	l.set("type", "rect")
	l.set("color", g.color(sc, true))
	maxX, maxY := 230, 230
	if sc.flapVar != "" {
		maxX, maxY = 40, 30
	}
	l.set("x", g.coord(sc, -10, maxX))
	l.set("y", g.ycoord(sc, -10, maxY))
	l.set("width", g.coord(sc, -10, 120))
	l.set("height", g.coord(sc, -5, 80))
	return l
}

func (g *gen) copyLayer() genLayer {
	l := genLayer{}
	l.set("type", "copy")
	l.setInt("src_x", g.between(-20, 230))
	l.setInt("src_y", g.between(-20, 230))
	w, h := g.between(1, 240), g.between(1, 240)
	if g.chance(0.03) {
		w = 0
	}
	l.setInt("src_width", w)
	l.setInt("src_height", h)
	l.setInt("x", g.between(-30, 235))
	l.setInt("y", g.between(-30, 235))
	return l
}

func (g *gen) loopLayer(sc genScope) genLayer {
	l := genLayer{}
	l.set("type", "loop")
	src := [][2]string{{"linien", "|"}, {"via", "|"}, {"halte", ","}, {"ziel", " "}, {"hinweis", " "}}[g.intn(5)]
	l.set("value", yq("{{"+src[0]+"}}"))
	if src[1] != "|" || g.chance(0.5) {
		l.set("split_by", yq(src[1]))
	}
	v := "item"
	if g.chance(0.5) {
		v = g.pick([]string{"halt", "x", "via_item"})
		l.set("var", v)
	}
	if g.chance(0.6) {
		l.setInt("max_items", g.pickInt([]int{1, 4, 6, 20, 30, 200}))
	}
	kid := sc
	kid.loopVar, kid.inLoop, kid.depth = v, true, sc.depth+1
	ids := append([]string(nil), sc.idList()...) // ids of an iteration stay in it
	kid.ids = &ids
	l.kids = g.layers(kid, g.between(1, 4), false)
	return l
}

func (g *gen) pickInt(s []int) int { return s[g.intn(len(s))] }

// flapLayer returns a splitflap layer at a free place, or false if none fits.
func (g *gen) flapLayer(sc genScope) (genLayer, bool) {
	left := 64 - g.modules
	if left <= 0 {
		return genLayer{}, false
	}
	w, h := g.between(8, 60), g.between(12, 40)
	cells := 1
	if g.chance(0.5) {
		cells = g.between(1, min(10, left))
	}
	pitch := w + g.intn(5)
	total := image.Rect(0, 0, pitch*(cells-1)+w, h)
	if total.Dx() > 240 {
		cells, pitch = 1, w
		total = image.Rect(0, 0, w, h)
	}
	var at image.Rectangle
	found := false
	for try := 0; try < 30 && !found; try++ {
		x, y := g.intn(240-total.Dx()+1), g.intn(240-h+1)
		at = total.Add(image.Pt(x, y))
		found = true
		for _, r := range g.flaps {
			if r.Overlaps(at) {
				found = false
				break
			}
		}
	}
	if !found {
		return genLayer{}, false
	}
	g.flaps = append(g.flaps, at)
	g.modules += cells
	l := genLayer{}
	l.set("type", "splitflap")
	l.setInt("x", at.Min.X)
	l.setInt("y", at.Min.Y)
	l.setInt("width", w)
	l.setInt("height", h)
	if g.chance(0.4) {
		l.setInt("split_y", g.between(1, h-1))
	}
	if cells > 1 || g.chance(0.3) {
		l.set("split", "chars")
		l.setInt("cells", cells)
		if pitch != w || g.chance(0.5) {
			l.setInt("pitch", pitch)
		}
	}
	val := "{{" + g.pick([]string{"zug", "ziel", "gleis", "zeit", "zug1.nr", "status"}) + "}}"
	if g.chance(0.4) {
		val = "{{" + g.pipeline(sc) + "}}"
	}
	l.set("value", yq(val))
	switch g.intn(4) {
	case 0:
		l.set("reel", yq(g.pick(genReels)))
	case 1:
		l.set("reel", `["IC", "ICE", "RE", "S", ""]`)
	case 2:
		l.set("reel", "[]")
	}
	v := g.pick([]string{"v", "ch", "z"})
	l.set("var", v)
	if g.chance(0.6) {
		l.setInt("flip_frames", g.between(1, 6))
	}
	if g.chance(0.5) {
		mn := g.intn(6)
		l.setInt("min_flips", mn)
		if g.chance(0.5) {
			l.setInt("max_flips", mn+g.intn(8))
		}
	}
	kid := sc
	kid.flapVar, kid.depth, kid.noCopy = v, sc.depth+1, true
	kid.ids = nil // no ids in modules
	bg := genLayer{}
	bg.set("type", "rect")
	bg.set("color", g.color(kid, false))
	bg.set("x", "0")
	bg.set("y", "0")
	bg.setInt("width", w)
	bg.setInt("height", h)
	l.kids = append([]genLayer{bg}, g.layers(kid, g.between(1, 3), false)...)
	return l, true
}

// layers returns n sibling layers; flaps allows splitflap layers among them.
func (g *gen) layers(sc genScope, n int, flaps bool) []genLayer {
	var out []genLayer
	chain := false // the previous layer (or block) has if/elif
	for k := 0; k < n; k++ {
		var l genLayer
		ok, isBlock := true, false
		r := g.intn(100)
		container := sc.depth < 3
		switch {
		case flaps && sc.flapVar == "" && !sc.inLoop && r < 30:
			l, ok = g.flapLayer(sc)
		case container && r < 42:
			isBlock = true
			kid := sc
			kid.depth++
			l.kids = g.layers(kid, g.between(1, 3), flaps)
		case container && !sc.inLoop && sc.flapVar == "" && r < 50:
			l = g.loopLayer(sc)
		case r < 65:
			l = g.textLayer(sc)
		case r < 78:
			l = g.imageLayer(sc)
		case !sc.noCopy && sc.flapVar == "" && r < 84:
			l = g.copyLayer()
		default:
			l = g.rectLayer(sc)
		}
		if !ok {
			continue
		}
		// Layer- and block-level if/elif/else chains.
		var head [2]string
		switch {
		case chain && g.chance(0.2):
			head, chain = [2]string{"elif", yq(g.cond(sc, 0))}, true
		case chain && g.chance(0.2):
			head, chain = [2]string{"else", "true"}, false
		case g.badElif && !chain:
			g.badElif = false
			head, chain = [2]string{"elif", yq("true")}, true // without if: an error
		case isBlock || g.chance(0.2):
			head, chain = [2]string{"if", yq(g.cond(sc, 0))}, true
		default:
			chain = false
		}
		if head[0] != "" {
			l.kv = append([][2]string{head}, l.kv...)
		}
		out = append(out, l)
	}
	return out
}

func writeGenLayers(b *strings.Builder, ls []genLayer, indent int) {
	pad := strings.Repeat(" ", indent)
	for _, l := range ls {
		for i, kv := range l.kv {
			lead := pad + "  "
			if i == 0 {
				lead = pad + "- "
			}
			fmt.Fprintf(b, "%s%s: %s\n", lead, kv[0], kv[1])
		}
		if l.kids != nil {
			lead := pad + "  "
			if len(l.kv) == 0 {
				lead = pad + "- "
			}
			b.WriteString(lead + "layers:\n")
			writeGenLayers(b, l.kids, indent+4)
		}
	}
}

// genTemplate returns a random template (YAML) using the assets in the
// directory assets (font NimbusSanL-Reg.otf as "reg", opaque.png, alpha.png,
// hand.png) and matching train data (JSON).
func genTemplate(rng *rand.Rand, assets string) (yaml string, data string) {
	_ = assets // the templates name these files; copyGenAssets copies them next to template.yaml
	g := &gen{rng: rng}
	g.badElif = g.chance(0.03)
	switch r := g.intn(10); {
	case r < 3:
		g.mode = genFlap
	case r < 5:
		g.mode = genFrames
	}
	var b strings.Builder
	b.WriteString("meta:\n  name: \"Generiert\"\n  canvas: {width: 240, height: 240}\n")
	if tz := g.pick([]string{"", "", "UTC", "America/New_York", "Asia/Kolkata"}); tz != "" {
		fmt.Fprintf(&b, "  timezone: %s\n", yq(tz))
	}
	switch g.mode {
	case genFlap:
		fmt.Fprintf(&b, "  animation: {fps: %d, duration: %s, loop: %v}\n",
			g.pickInt([]int{5, 10, 20, 25}), g.pick([]string{"0.5", "1", "2.5", "5"}), g.chance(0.2))
	case genFrames:
		frames := g.between(2, 16)
		if g.chance(0.1) {
			frames = 125
		}
		delay := g.pick([]string{"0.02", "0.05", "0.1", "0.5", "1"})
		fmt.Fprintf(&b, "  animation: {frames: %d, frame_delay: %s, loop: %v}\n", frames, delay, g.chance(0.5))
	}
	b.WriteString("fonts:\n  - id: reg\n    file: NimbusSanL-Reg.otf\nlayers:\n")
	var ids []string
	root := genScope{ids: &ids}
	var ls []genLayer
	switch g.intn(3) { // background
	case 0:
		bg := genLayer{}
		bg.set("type", "rect")
		bg.set("color", g.color(root, false))
		bg.kv = append(bg.kv, [][2]string{{"x", "0"}, {"y", "0"}, {"width", "240"}, {"height", "240"}}...)
		ls = append(ls, bg)
	case 1:
		bg := genLayer{}
		bg.set("type", "image")
		bg.set("file", g.pick([]string{"opaque.png", "alpha.png"}))
		bg.kv = append(bg.kv, [][2]string{{"x", "0"}, {"y", "0"}, {"width", "240"}, {"height", "240"}}...)
		ls = append(ls, bg)
	}
	if g.mode == genFlap {
		// prefix (copies allowed) | split-flap section (no copies) | suffix
		ls = append(ls, g.layers(root, g.between(1, 6), false)...)
		sec := root
		sec.noCopy = true
		ls = append(ls, g.layers(sec, g.between(3, 8), true)...)
		ls = append(ls, g.layers(root, g.between(0, 6), false)...)
	} else {
		ls = append(ls, g.layers(root, g.between(6, 18), false)...)
	}
	writeGenLayers(&b, ls, 2)
	return b.String(), genData(rng)
}

// genData returns random Latin-1 train data for the generated templates.
func genData(rng *rand.Rand) string {
	g := &gen{rng: rng}
	long := func(s string) string {
		if g.chance(0.1) {
			return strings.Repeat(s+" ", 20)
		}
		return s
	}
	num := func() any {
		return []any{0, 5, 12, -3, 2.5, "", "+3", "7", 1e6, "x"}[g.intn(10)]
	}
	m := map[string]any{
		"zug":     g.pick([]string{"IC 23", "ICE 77", "S1", "RE 4711", "", "EC", "TGV 9261", "ic 5"}),
		"ziel":    long(g.pick([]string{"Zürich HB", "Genève-Aéroport", "Basel SBB", "Bern", "", "Neulengbach über Tullnerbach-Pressbaum", "ÄÖÜ äöü ß «Test»", "BASEL SBB"})),
		"via":     long(g.pick([]string{"Othmarsingen|Mägenwil|Mellingen", "Neuhof - Gelnhausen - Hanau", "", "A|B|C|D|E|F|G|H|I|J|K|L|M|N|O|P|Q|R|S|T|U|V", "Olten"})),
		"zeit":    g.pick([]string{"13:30", "9:05", "", "ca. 10", "23:59 Uhr", "1330", " 7:45"}),
		"hinweis": g.pick([]string{"*Hält nicht in Olten", "{Info} Ersatzbus", "", "Zug fällt aus", "**Achtung** Gleis", "Ersatz"}),
		"gleis":   []any{6, "12", "3a", "", 0}[g.intn(5)],
		"abw":     num(),
		"n":       num(),
		"linien":  g.pick([]string{"S1|S12 | IR 36||RE|ICE 77", "", "S3", "a|b|c|d|e|f|g|h|i|j|k|l|m|n|o|p|q|r|s|t|u|v|w|x|y|z|1|2|3|4|5"}),
		"halte":   g.pick([]string{"Baden,Brugg,Aarau,Olten", "", "Wil", ",,Lenzburg,"}),
		"status":  g.pick([]string{"ok", "spaet", "", "ausfall"}),
		"farbe":   g.pick([]string{"#ff0000", "#00ff0080", "#12345", "rot"}),
		"mode":    g.pick([]string{"full", "none", ""}),
		"zug1": map[string]any{
			"zeit":    g.pick([]string{"13:30", "", "0:05", "10:5"}),
			"nr":      g.pick([]string{"IC23", "", "S 7"}),
			"vonnach": g.pick([]string{"Frankfurt", "", "Köln Messe/Deutz"}),
		},
	}
	b, err := json.Marshal(m)
	if err != nil {
		panic(err)
	}
	return string(b)
}

// copyGenAssets copies the generator's asset files from assets into td.
func copyGenAssets(t *testing.T, assets, td string) {
	t.Helper()
	for _, f := range []string{"NimbusSanL-Reg.otf", "opaque.png", "alpha.png", "hand.png"} {
		b, err := os.ReadFile(filepath.Join(assets, f))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(td, f), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// genOutcome counts how the generated cases ended.
type genOutcome struct{ near, goErr, anim, exprs int }

// genExprs returns the conditions (if/elif) and the interpolated strings
// ({{…}}) of a generated template.
func genExprs(t *testing.T, src string) (conds, strs []string) {
	t.Helper()
	var root yaml.Node
	if err := yaml.Unmarshal([]byte(src), &root); err != nil {
		t.Fatalf("generated YAML: %v", err)
	}
	var walk func(n *yaml.Node)
	walk = func(n *yaml.Node) {
		if n.Kind == yaml.MappingNode {
			for i := 0; i+1 < len(n.Content); i += 2 {
				k, v := n.Content[i].Value, n.Content[i+1]
				if v.Kind == yaml.ScalarNode && (k == "if" || k == "elif") {
					conds = append(conds, v.Value)
					continue
				}
				if v.Kind == yaml.ScalarNode && strings.Contains(v.Value, "{{") {
					strs = append(strs, v.Value)
				}
				walk(v)
			}
			return
		}
		for _, c := range n.Content {
			walk(c)
		}
	}
	walk(&root)
	return conds, strs
}

// checkGenExprs evaluates every condition and interpolated string of the
// template with the Go and the C++ evaluator (exact equality): the image
// comparison tolerates small text differences, the evaluator must not. Loop
// and module variables are unknown here, on both sides alike.
func checkGenExprs(t *testing.T, src, data string, now time.Time, anim [3]int, fail func(string, ...any)) int {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(data), &m); err != nil {
		t.Fatal(err)
	}
	conds, strs := genExprs(t, src)
	re := regexTableFor(t, append(append([]string(nil), conds...), strs...)...)
	n := 0
	for i, list := range [][]string{strs, conds} {
		cond := i == 1
		for _, e := range list {
			want := renderer.EvalForTest(m, now, anim[0], anim[1], anim[2], e, cond)
			got, rc := testEval(data, re, toZza(now), anim[0], anim[1], anim[2], e, cond)
			if rc != 0 || got != want {
				fail("evaluator (cond=%v, anim=%v) %q: go %q, c++ %q (code %d)", cond, anim, e, want, got, rc)
			}
			n++
		}
	}
	return n
}

func TestGeneratedNear(t *testing.T) {
	const n = 300 // about 25 s locally, so no -short variant
	assets := filepath.Join(covRoot, "cov-image")
	dir := t.TempDir()
	var out genOutcome
	start := time.Now()
	for seed := 1; seed <= n; seed++ {
		t.Run(fmt.Sprint(seed), func(t *testing.T) { checkGenerated(t, dir, assets, int64(seed), &out) })
	}
	t.Logf("%d templates in %v: %d near (%d animated), %d rejected by Go and the device; %d expressions equal; "+
		"largest work of one call %d (still %d, anim_begin %d, frame %d)",
		n, time.Since(start).Round(time.Millisecond), out.near, out.anim, out.goErr, out.exprs,
		genWork.max(), genWork.still, genWork.begin, genWork.frame)
	if out.near < n/2 {
		t.Errorf("only %d of %d generated templates render in Go: generator too often invalid", out.near, n)
	}
}

// saveGenCase writes the template and the data of a failing case and returns the directory.
func saveGenCase(t *testing.T, seed int64, src, data string) string {
	t.Helper()
	dir := os.Getenv("ZZA_COMPARE_DIR")
	if dir == "" {
		dir = t.TempDir()
	}
	d := filepath.Join(dir, fmt.Sprintf("gen-%d", seed))
	if err := os.MkdirAll(d, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(d, "template.yaml"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(d, "default.json"), []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	return d
}

func checkGenerated(t *testing.T, dir, assets string, seed int64, out *genOutcome) {
	src, data := genTemplate(rand.New(rand.NewSource(seed)), assets)
	name := fmt.Sprintf("gen-%d", seed)
	td := filepath.Join(dir, name)
	if err := os.MkdirAll(td, 0o755); err != nil {
		t.Fatal(err)
	}
	copyGenAssets(t, assets, td)
	if err := os.WriteFile(filepath.Join(td, "template.yaml"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	fail := func(format string, args ...any) {
		t.Helper()
		t.Errorf("seed %d: %s (template and data: %s)", seed, fmt.Sprintf(format, args...), saveGenCase(t, seed, src, data))
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(data), &m); err != nil {
		t.Fatal(err)
	}

	// Go reference: load, still image.
	var (
		now  time.Time
		gImg *image.NRGBA
	)
	tmpl, gerr := renderer.LoadTemplate(dir, name)
	if gerr == nil {
		now = fixedNow(t, tmpl)
		anim := [3]int{0, 1, 0}
		if a := tmpl.Meta.Animation; a.Frames > 0 {
			fd := a.FrameDelay
			if fd == 0 {
				fd = 1 // Go's default
			}
			anim = [3]int{1, a.Frames, int(math.Round(fd * 100))}
		}
		out.exprs += checkGenExprs(t, src, data, now, anim, fail)
		gImg, gerr = renderer.NewWithClock(dir, func() time.Time { return now }).Render(tmpl, m)
	}
	p, xerr := devexport.Export(dir, name)
	if gerr != nil {
		if xerr != nil {
			out.goErr++
			return
		}
		r := loadPkg(t, p)
		if now.IsZero() {
			now = time.Date(2026, 1, 1, 10, 8, 37, 0, time.UTC)
		}
		if _, cerr := r.Render([]byte(data), now); cerr == nil {
			fail("Go rejects (%v), export and C++ render", gerr)
		} else if !errors.Is(cerr, ErrTemplate) {
			fail("Go rejects (%v), C++: %v, want ErrTemplate", gerr, cerr)
		}
		out.goErr++
		return
	}
	if xerr != nil {
		fail("Go renders, export fails: %v", xerr)
		return
	}
	r := loadPkg(t, p)
	cImg, cerr := r.Render([]byte(data), now)
	if cerr != nil {
		fail("Go renders, C++: %v", cerr)
		return
	}
	used := r.MemUsed() // package + scaled-image cache
	if _, err := r.Render([]byte(data), now); err != nil || r.MemUsed() != used {
		fail("second render: %v, MemUsed %d, want %d", err, r.MemUsed(), used)
	}
	if msg := nearCheck(t, gImg, cImg); msg != "" {
		fail("still image: %s", msg)
		return
	}
	out.near++
	if w, err := measureWork(r, []byte(data), now); err == nil {
		recordGenWork(w)
		if w.max()*workMargin > defaultWorkLimit {
			fail("work %d, more than 1/%d of the default limit %d", w.max(), workMargin, defaultWorkLimit)
		}
	}

	// Animation (split-flap or frame mode).
	if tmpl.Meta.Animation.Frames == 0 && !strings.Contains(src, "type: splitflap") {
		return
	}
	out.anim++
	ga, gaerr := renderer.NewWithClock(dir, func() time.Time { return now }).
		RenderAnimation(tmpl, m, renderer.AnimationSeed(name, []byte(data)))
	ca, caerr := r.RenderAnimation([]byte(data), now)
	if gaerr != nil || caerr != nil {
		if gaerr == nil || !errors.Is(caerr, ErrTemplate) {
			fail("animation: Go %v; C++ %v", gaerr, caerr)
		}
		return
	}
	if len(ca.Frames) != len(ga.Frames) || ca.FPS != ga.FPS || ca.Delay != ga.Delay || ca.Loop != ga.Loop {
		fail("animation: C++ %d frames FPS %d Delay %d Loop %v; Go %d frames FPS %d Delay %d Loop %v",
			len(ca.Frames), ca.FPS, ca.Delay, ca.Loop, len(ga.Frames), ga.FPS, ga.Delay, ga.Loop)
		return
	}
	nf := len(ga.Frames)
	idx := []int{0, 1, 2, nf / 4, nf / 2, 3 * nf / 4, nf - 2, nf - 1}
	if nf <= 12 {
		idx = idx[:0]
		for i := 0; i < nf; i++ {
			idx = append(idx, i)
		}
	}
	for _, i := range idx {
		if i < 0 || i >= nf {
			continue
		}
		if msg := nearCheck(t, ga.Frames[i], ca.Frames[i]); msg != "" {
			fail("frame %d/%d: %s", i, nf, msg)
			return
		}
	}
}
