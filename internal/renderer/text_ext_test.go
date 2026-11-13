package renderer

import (
	"image"
	"image/color"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
)

const textExtFontDir = "../../templates/faltblatt-105-v1"

// textExtTemplate builds an in-memory template (transparent canvas) with one text layer.
func textExtTemplate(l Layer) *Template {
	l.Type = "text"
	l.Font = "f"
	if l.Color.raw == "" {
		l.Color = StringOrCond{raw: "#000000"}
	}
	return &Template{
		Meta:   Meta{Canvas: Canvas{Width: 200, Height: 80}},
		Fonts:  []FontDef{{ID: "f", File: "NimbusSanL-Bol.otf"}},
		Layers: []Layer{l},
		Dir:    textExtFontDir,
	}
}

func renderTextLayer(t *testing.T, l Layer) *image.NRGBA {
	t.Helper()
	r := New(textExtFontDir)
	return mustRender(t, r, textExtTemplate(l), nil)
}

// inkAlphaMin ignores the faint (alpha < 16) tails the Catmull-Rom kernel leaves
// next to resampled glyphs; real glyph edges are far above it.
const inkAlphaMin = 16

// inkBounds returns the bounding box of all pixels with alpha >= inkAlphaMin.
func inkBounds(img *image.NRGBA) image.Rectangle {
	var b image.Rectangle
	first := true
	for y := img.Rect.Min.Y; y < img.Rect.Max.Y; y++ {
		for x := img.Rect.Min.X; x < img.Rect.Max.X; x++ {
			if img.NRGBAAt(x, y).A < inkAlphaMin {
				continue
			}
			p := image.Rect(x, y, x+1, y+1)
			if first {
				b, first = p, false
			} else {
				b = b.Union(p)
			}
		}
	}
	return b
}

func TestText_ValignBaseline(t *testing.T) {
	l := Layer{Value: StringOrCond{raw: "H"}, Size: 20, Y: IntOrExpr{val: 30}, X: IntOrExpr{val: 10}}
	plain := inkBounds(renderTextLayer(t, l))
	if plain.Min.Y < 30 {
		t.Errorf("without valign: Min.Y = %d, want >= 30", plain.Min.Y)
	}
	l.Valign = "baseline"
	b := inkBounds(renderTextLayer(t, l))
	if b.Max.Y != 30 && b.Max.Y != 31 {
		t.Errorf("baseline: Max.Y = %d, want 30 or 31", b.Max.Y)
	}
	if b.Min.Y >= 20 {
		t.Errorf("baseline: Min.Y = %d, want < 20", b.Min.Y)
	}
}

func TestText_ValignBaseline_ScaleXMultiLine(t *testing.T) {
	// Baseline of the first line = y, even with scale_x and several lines.
	l := Layer{Value: StringOrCond{raw: "H H"}, Size: 20, Y: IntOrExpr{val: 30}, X: IntOrExpr{val: 10},
		Width: IntOrExpr{val: 12}, ScaleX: 0.5, Valign: "baseline"} // 12px box = 24px natural: "H H" wraps into two lines
	img := renderTextLayer(t, l)
	// First line occupies rows above y=30; second line starts below it.
	var first image.Rectangle
	for y := 0; y <= 31; y++ {
		for x := 0; x < 200; x++ {
			if img.NRGBAAt(x, y).A >= inkAlphaMin {
				first = first.Union(image.Rect(x, y, x+1, y+1))
			}
		}
	}
	if first.Max.Y != 30 && first.Max.Y != 31 {
		t.Errorf("first line bottom = %d, want 30 or 31", first.Max.Y)
	}
	if b := inkBounds(img); b.Max.Y <= 31 {
		t.Errorf("expected a second line below the baseline, ink Max.Y = %d", b.Max.Y)
	}
}

func TestText_ScaleX(t *testing.T) {
	l := Layer{Value: StringOrCond{raw: "HHHH"}, Size: 20, X: IntOrExpr{val: 10}, Y: IntOrExpr{val: 10}}
	n := inkBounds(renderTextLayer(t, l))
	l.ScaleX = 0.5
	s := inkBounds(renderTextLayer(t, l))
	if d := s.Dx() - n.Dx()/2; d < -1 || d > 1 {
		t.Errorf("scaled width = %d, want %d ±1", s.Dx(), n.Dx()/2)
	}
	if d := s.Dy() - n.Dy(); d < -1 || d > 1 {
		t.Errorf("scaled height = %d, want %d ±1", s.Dy(), n.Dy())
	}
}

func TestText_ScaleX_AlignRight(t *testing.T) {
	l := Layer{Value: StringOrCond{raw: "HHHH"}, Size: 20, X: IntOrExpr{val: 100}, Y: IntOrExpr{val: 10},
		Align: "right", ScaleX: 0.5}
	b := inkBounds(renderTextLayer(t, l))
	if b.Max.X > 100 || b.Max.X < 97 {
		t.Errorf("Max.X = %d, want within [97,100]", b.Max.X)
	}
}

// TestText_NoNewFields_Unchanged compares renderText against a frozen copy of the
// pre-change drawing algorithm (font.Drawer directly). The duplication is intended.
func TestText_NoNewFields_Unchanged(t *testing.T) {
	cases := []Layer{
		{Value: StringOrCond{raw: "Hanau Hbf"}, Size: 12.67, X: IntOrExpr{val: 7}, Y: IntOrExpr{val: 9}},
		{Value: StringOrCond{raw: "Hanau"}, Size: 14, X: IntOrExpr{val: 100}, Y: IntOrExpr{val: 9}, Align: "right"},
		{Value: StringOrCond{raw: "Hanau"}, Size: 14, X: IntOrExpr{val: 100}, Y: IntOrExpr{val: 9}, Align: "center"},
	}
	for _, l := range cases {
		tmpl := textExtTemplate(l)
		l = tmpl.Layers[0]
		r := New(textExtFontDir)
		got := mustRender(t, r, tmpl, nil)

		// Frozen old algorithm (single line, no height).
		want := image.NewNRGBA(image.Rect(0, 0, 200, 80))
		face, err := r.getFace(tmpl, l)
		if err != nil {
			t.Fatal(err)
		}
		defer face.Close()
		text := l.Value.raw
		x := l.X.val
		switch l.Align {
		case "center":
			x -= measureText(face, text) / 2
		case "right":
			x -= measureText(face, text)
		}
		d := &font.Drawer{Dst: want, Src: &image.Uniform{C: color.NRGBA{A: 255}}, Face: face,
			Dot: fixed.Point26_6{X: fixed.I(x), Y: fixed.I(l.Y.val + face.Metrics().Ascent.Round())}}
		d.DrawString(text)

		if !reflect.DeepEqual(got.Pix, want.Pix) {
			t.Errorf("align %q: Pix differs from frozen algorithm", l.Align)
		}
	}
}

func TestFitItems_Pure(t *testing.T) {
	// strict '<' and xpos advancing even for omitted items
	got := fitItems([]int{30, 50, 10}, 2, 87)
	want := []bool{true, true, false} // xpos: 0 -> 32 -> 84; 84+10 >= 87
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	got = fitItems([]int{30, 80, 5}, 2, 87) // middle too wide; xpos still advances: 32+80+2=114
	want = []bool{true, false, false}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	if got := fitItems([]int{87}, 2, 87); got[0] { // xpos+width == maxWidth is not drawn
		t.Error("width == maxWidth must not be drawn")
	}
}

func TestText_FitItems(t *testing.T) {
	r := New(textExtFontDir)
	l := Layer{Size: 12.67, Font: "f"}
	tmpl := textExtTemplate(l)
	face, err := r.getFace(tmpl, tmpl.Layers[0])
	if err != nil {
		t.Fatal(err)
	}
	defer face.Close()
	fi := &FitItems{SplitBy: "-", Gap: 2}
	text := "Neuhof - Gelnhausen - Hanau"

	// Scaled widths: Neuhof 22, Gelnhausen 37, Hanau 20 -> Hanau starts at 63 and
	// ends at 83, so max_width 80 omits it (87 would still draw it).
	items, xs := layoutFitItems(face, text, fi, 0.5, 80)
	if want := []string{"Neuhof", "Gelnhausen"}; !reflect.DeepEqual(items, want) {
		t.Errorf("items = %q, want %q", items, want)
	}
	if len(xs) != 2 || xs[0] != 0 || xs[1] != scaledWidth(measureText(face, "Neuhof"), 0.5)+2 {
		t.Errorf("xs = %v", xs)
	}

	if items, _ := layoutFitItems(face, text, fi, 0.5, 5); len(items) != 0 {
		t.Errorf("max_width 5: items = %q, want none", items)
	}

	// Middle item too wide: it is skipped but xpos still advances, so "Ab" no longer fits.
	items, _ = layoutFitItems(face, "Ab-Gelnhausen Gelnhausen Gelnhausen-Ab", fi, 0, 100)
	if want := []string{"Ab"}; !reflect.DeepEqual(items, want) {
		t.Errorf("items = %q, want %q", items, want)
	}

	// End-to-end: layer renders without error and draws something.
	l = Layer{Value: StringOrCond{raw: text}, Size: 12.67, FitItems: fi, ScaleX: 0.5,
		MaxWidth: IntOrExpr{val: 80}, X: IntOrExpr{val: 0}, Y: IntOrExpr{val: 10}}
	if b := inkBounds(renderTextLayer(t, l)); b.Empty() || b.Max.X > 80 {
		t.Errorf("fit_items render bounds = %v", b)
	}
}

func TestFilter_BeforeAfter(t *testing.T) {
	cases := []struct{ tpl, val, want string }{
		{"{{t | before(':')}}", "13:30", "13"},
		{"{{t | after(':')}}", "13:30", "30"},
		{"{{t | before(':')}}", "1330", "1330"},
		{"{{t | after(':')}}", "1330", ""},
		{"{{t | before(':')}}", "", ""},
		{"{{t | after(':')}}", "", ""},
	}
	for _, c := range cases {
		e := fixedEval(map[string]interface{}{"t": c.val})
		if got := e.Interpolate(c.tpl); got != c.want {
			t.Errorf("%s with %q = %q, want %q", c.tpl, c.val, got, c.want)
		}
	}
}

// A very long request value on a scale_x layer must not allocate memory in
// proportion to its length (the offscreen line used to be as wide as the text).
func TestText_ScaleX_LongTextBoundedMemory(t *testing.T) {
	long := strings.Repeat("W", 20000)
	for _, align := range []string{"", "right", "center"} {
		l := Layer{Value: StringOrCond{raw: long}, Size: 20, X: IntOrExpr{val: 100}, Y: IntOrExpr{val: 10},
			Align: align, ScaleX: 0.5}
		r := New(textExtFontDir)
		tmpl := textExtTemplate(l)
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		img := mustRender(t, r, tmpl, nil)
		runtime.ReadMemStats(&after)
		if mb := (after.TotalAlloc - before.TotalAlloc) >> 20; mb > 32 {
			t.Errorf("align %q: render allocated %d MB, want ≤ 32", align, mb)
		}
		b := inkBounds(img)
		wantMin, wantMax := 0, 200 // right/center: text covers the whole canvas width
		switch align {
		case "":
			wantMin = 100
		case "right":
			wantMax = 100
		}
		if b.Min.X > wantMin+4 || b.Min.X < wantMin-1 || b.Max.X < wantMax-4 || b.Max.X > wantMax+1 {
			t.Errorf("align %q: ink x range [%d,%d), want about [%d,%d)", align, b.Min.X, b.Max.X, wantMin, wantMax)
		}
	}
}

// Clipping an over-long scaled line must not move the visible glyphs.
func TestText_ScaleX_ClippedMatchesUnclipped(t *testing.T) {
	// 60 glyphs: natural width far beyond the 200px canvas, so the clip path runs.
	text := strings.Repeat("HI", 30)
	l := Layer{Value: StringOrCond{raw: text}, Size: 20, X: IntOrExpr{val: 10}, Y: IntOrExpr{val: 10}, ScaleX: 0.9}
	got := renderTextLayer(t, l)

	// Reference: the same line drawn on a canvas wide enough to skip clipping.
	wide := textExtTemplate(l)
	wide.Meta.Canvas.Width = 2000
	ref := mustRender(t, New(textExtFontDir), wide, nil)

	// Compare the ink per column. The resampled width is rounded up to whole
	// pixels, so the shorter run drifts by a fraction of a pixel; a glyph that
	// moved by a pixel or more would change a column by far more than 5 rows.
	for x := 0; x < 190; x++ { // stay clear of the right canvas edge
		a, b := 0, 0
		for y := 0; y < 80; y++ {
			a += int(got.NRGBAAt(x, y).A)
			b += int(ref.NRGBAAt(x, y).A)
		}
		if d := (a - b) / 255; d > 5 || d < -5 {
			t.Errorf("column %d: ink %d vs %d rows (clipped vs unclipped)", x, a/255, b/255)
		}
	}
}

func TestLoadTemplate_ScaleXValidation(t *testing.T) {
	for _, tc := range []struct {
		val string
		ok  bool
	}{{"0.5", true}, {"1", true}, {"8", true}, {".inf", false}, {".nan", false}, {"-1", false}, {"0.001", false}, {"100", false}} {
		dir := t.TempDir()
		if err := os.MkdirAll(filepath.Join(dir, "t"), 0o755); err != nil {
			t.Fatal(err)
		}
		y := "meta:\n  canvas: {width: 20, height: 10}\nlayers:\n  - type: loop\n    layers:\n      - type: text\n        value: x\n        scale_x: " + tc.val + "\n"
		if err := os.WriteFile(filepath.Join(dir, "t", "template.yaml"), []byte(y), 0o644); err != nil {
			t.Fatal(err)
		}
		_, err := LoadTemplate(dir, "t")
		if (err == nil) != tc.ok {
			t.Errorf("scale_x %s: err = %v, want ok=%v", tc.val, err, tc.ok)
		}
	}
}

// --- text id / advance ---

func exprInt(s string) IntOrExpr { return IntOrExpr{expr: s, isExpr: true} }

// idTemplate builds a template with one text layer (id "a", x 10) followed by the given layers.
func idTemplate(first Layer, follow ...Layer) *Template {
	first.ID = "a"
	if first.Size == 0 {
		first.Size = 20
	}
	first.X = IntOrExpr{val: 10}
	first.Y = IntOrExpr{val: 5}
	tmpl := textExtTemplate(first)
	tmpl.Layers = append(tmpl.Layers, follow...)
	return tmpl
}

func redRect(x IntOrExpr, y int) Layer {
	return Layer{Type: "rect", X: x, Y: IntOrExpr{val: y}, Width: IntOrExpr{val: 4}, Height: IntOrExpr{val: 4},
		Color: StringOrCond{raw: "#ff0000"}}
}

// redMinX returns the smallest x of a red pixel, or -1.
func redMinX(img *image.NRGBA) int {
	for x := 0; x < img.Rect.Dx(); x++ {
		for y := 0; y < img.Rect.Dy(); y++ {
			if c := img.NRGBAAt(x, y); c.A > 0 && c.R == 255 && c.G == 0 {
				return x
			}
		}
	}
	return -1
}

func textWidthFor(t *testing.T, tmpl *Template, text string) int {
	t.Helper()
	face, err := New(textExtFontDir).getFace(tmpl, tmpl.Layers[0])
	if err != nil {
		t.Fatal(err)
	}
	defer face.Close()
	return measureText(face, text)
}

func TestText_IDEndAndWidth(t *testing.T) {
	tmpl := idTemplate(Layer{Value: StringOrCond{raw: "Hallo"}, Advance: 3},
		redRect(exprInt("a.end"), 50), redRect(exprInt("a.width + 100"), 60))
	w := textWidthFor(t, tmpl, "Hallo")
	img := mustRender(t, New(textExtFontDir), tmpl, nil)
	if got := redMinX(img); got != 10+w+3 {
		t.Errorf("a.end rect at x = %d, want %d", got, 10+w+3)
	}
	// second rect (y 60) at 100 + width
	for x := 0; x < 200; x++ {
		if img.NRGBAAt(x, 61).A > 0 {
			if x != w+100 {
				t.Errorf("a.width rect at x = %d, want %d", x, w+100)
			}
			break
		}
	}
}

func TestText_IDEmptyValue(t *testing.T) {
	tmpl := idTemplate(Layer{Value: StringOrCond{raw: ""}, Advance: 3},
		redRect(exprInt("a.end"), 50))
	if got := redMinX(mustRender(t, New(textExtFontDir), tmpl, nil)); got != 10 {
		t.Errorf("empty text: a.end rect at x = %d, want 10", got)
	}
	tmpl = idTemplate(Layer{Value: StringOrCond{raw: ""}}, redRect(exprInt("a.width + 20"), 50))
	if got := redMinX(mustRender(t, New(textExtFontDir), tmpl, nil)); got != 20 {
		t.Errorf("empty text: a.width = %d, want 0", got-20)
	}
}

func TestText_IDWithScaleX(t *testing.T) {
	tmpl := idTemplate(Layer{Value: StringOrCond{raw: "Hallo"}, ScaleX: 0.5},
		redRect(exprInt("a.end"), 50))
	w := scaledWidth(textWidthFor(t, tmpl, "Hallo"), 0.5)
	if got := redMinX(mustRender(t, New(textExtFontDir), tmpl, nil)); got != 10+w {
		t.Errorf("scaled a.end = %d, want %d", got, 10+w)
	}
}

func TestText_IDVisibleInLoop(t *testing.T) {
	loop := Layer{Type: "loop", Value: StringOrCond{raw: "x|y"}, Var: "v",
		Layers: []Layer{redRect(exprInt("a.end + loop.index * 30"), 50)}}
	tmpl := idTemplate(Layer{Value: StringOrCond{raw: "Hi"}}, loop)
	w := textWidthFor(t, tmpl, "Hi")
	img := mustRender(t, New(textExtFontDir), tmpl, nil)
	if got := redMinX(img); got != 10+w {
		t.Errorf("loop rect at x = %d, want %d", got, 10+w)
	}
	if img.NRGBAAt(10+w+30, 51).A == 0 {
		t.Errorf("second iteration rect missing at x = %d", 10+w+30)
	}
}

func TestText_IDInLoopDoesNotLeak(t *testing.T) {
	inner := Layer{Type: "text", Font: "f", Size: 20, ID: "b", Value: StringOrCond{raw: "WWWW"},
		X: IntOrExpr{val: 0}, Y: IntOrExpr{val: 5}, Color: StringOrCond{raw: "#000000"}}
	loop := Layer{Type: "loop", Value: StringOrCond{raw: "x"}, Var: "v", Layers: []Layer{inner}}
	tmpl := idTemplate(Layer{Value: StringOrCond{raw: "Hi"}}, loop, redRect(exprInt("b.end + 150"), 50))
	// An unknown variable makes the whole expression 0; a leaked b would give > 150.
	if got := redMinX(mustRender(t, New(textExtFontDir), tmpl, nil)); got != 0 {
		t.Errorf("rect after loop at x = %d, want 0 (b must not leak)", got)
	}
}

func TestText_IDFreshPerAnimationFrame(t *testing.T) {
	// a is only set while the condition holds; a stale value from an earlier frame must not survive.
	first := Layer{Type: "text", Font: "f", Size: 20, ID: "a", If: "equals(anim.frame, 0)",
		Value: StringOrCond{raw: "WW"}, X: IntOrExpr{val: 0}, Y: IntOrExpr{val: 5}, Color: StringOrCond{raw: "#000000"}}
	tmpl := textExtTemplate(first)
	tmpl.Meta.Animation = AnimationMeta{Frames: 2, FrameDelay: 1}
	tmpl.Layers = append(tmpl.Layers, redRect(exprInt("a.end + 100"), 50))
	anim, err := New(textExtFontDir).RenderAnimation(tmpl, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := redMinX(anim.Frames[0]); got <= 100 {
		t.Errorf("frame 0: rect at x = %d, want > 100", got)
	}
	if got := redMinX(anim.Frames[1]); got != 0 {
		t.Errorf("frame 1: rect at x = %d, want 0 (stale id from frame 0)", got)
	}
}

func loadYAMLTemplate(t *testing.T, layers string) error {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "t"), 0o755); err != nil {
		t.Fatal(err)
	}
	y := "meta:\n  canvas: {width: 20, height: 10}\nlayers:\n" + layers
	if err := os.WriteFile(filepath.Join(dir, "t", "template.yaml"), []byte(y), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadTemplate(dir, "t")
	return err
}

func TestLoadTemplate_IDValidation(t *testing.T) {
	txt := func(extra string) string { return "  - type: text\n    value: x\n" + extra }
	for _, tc := range []struct {
		name, layers string
		ok           bool
	}{
		{"valid", txt("    id: zeit\n    advance: 3\n"), true},
		{"advance max", txt("    id: a\n    advance: 240\n"), true},
		{"align left", txt("    id: a\n    align: left\n"), true},
		{"duplicate", txt("    id: a\n") + txt("    id: a\n"), false},
		{"duplicate in loop", txt("    id: a\n") + "  - type: loop\n    layers:\n" + "      - type: text\n        value: x\n        id: a\n", false},
		{"digit start", txt("    id: \"9x\"\n"), false},
		{"uppercase", txt("    id: Abc\n"), false},
		{"too long", txt("    id: a" + strings.Repeat("b", 32) + "\n"), false},
		{"align right", txt("    id: a\n    align: right\n"), false},
		{"align center", txt("    id: a\n    align: center\n"), false},
		{"fit_items", txt("    id: a\n    fit_items: {split_by: ','}\n"), false},
		{"on rect", "  - type: rect\n    id: a\n", false},
		{"advance -1", txt("    id: a\n    advance: -1\n"), false},
		{"advance 241", txt("    id: a\n    advance: 241\n"), false},
	} {
		if err := loadYAMLTemplate(t, tc.layers); (err == nil) != tc.ok {
			t.Errorf("%s: err = %v, want ok=%v", tc.name, err, tc.ok)
		}
	}
}

// --- scale_y ---

func TestText_ScaleY(t *testing.T) {
	l := Layer{Value: StringOrCond{raw: "HHHH"}, Size: 20, X: IntOrExpr{val: 10}, Y: IntOrExpr{val: 10}}
	n := inkBounds(renderTextLayer(t, l))
	l.ScaleY = 2
	s := inkBounds(renderTextLayer(t, l))
	// Catmull-Rom blurs each stretched edge by up to a pixel, hence ±2.
	if d := s.Dy() - 2*n.Dy(); d < -2 || d > 2 {
		t.Errorf("scaled height = %d, want %d ±2", s.Dy(), 2*n.Dy())
	}
	// Top edge of the text box stays at y: glyph top moves from asc-capHeight to 2*(asc-capHeight).
	face, err := New(textExtFontDir).getFace(textExtTemplate(l), textExtTemplate(l).Layers[0])
	if err != nil {
		t.Fatal(err)
	}
	asc := face.Metrics().Ascent.Round()
	face.Close()
	capTop := n.Min.Y - 10 // distance of the glyph top below the box top
	if want := 10 + capTop*2; s.Min.Y < want-1 || s.Min.Y > want+1 {
		t.Errorf("scaled Min.Y = %d, want %d ±1 (ascent %d)", s.Min.Y, want, asc)
	}
	if d := s.Dx() - n.Dx(); d < -1 || d > 1 {
		t.Errorf("scaled width = %d, want %d ±1", s.Dx(), n.Dx())
	}
}

func TestText_ScaleY_Baseline(t *testing.T) {
	l := Layer{Value: StringOrCond{raw: "H"}, Size: 20, X: IntOrExpr{val: 10}, Y: IntOrExpr{val: 40}, Valign: "baseline"}
	n := inkBounds(renderTextLayer(t, l))
	l.ScaleY = 2
	s := inkBounds(renderTextLayer(t, l))
	if d := s.Max.Y - n.Max.Y; d < -1 || d > 1 {
		t.Errorf("baseline moved: Max.Y %d vs %d", s.Max.Y, n.Max.Y)
	}
	if d := s.Dy() - 2*n.Dy(); d < -2 || d > 2 {
		t.Errorf("height = %d, want %d ±2", s.Dy(), 2*n.Dy())
	}
}

func TestText_ScaleY_OneIsIdentical(t *testing.T) {
	l := Layer{Value: StringOrCond{raw: "Hallo Welt"}, Size: 20, X: IntOrExpr{val: 10}, Y: IntOrExpr{val: 10}}
	a := renderTextLayer(t, l)
	l.ScaleY = 1
	b := renderTextLayer(t, l)
	if !reflect.DeepEqual(a.Pix, b.Pix) {
		t.Error("scale_y 1 differs from scale_y omitted")
	}
}

func TestText_ScaleY_LineHeight(t *testing.T) {
	// Two lines: with scale_y 2 the second line starts about twice as far below the first.
	l := Layer{Value: StringOrCond{raw: "H H"}, Size: 10, X: IntOrExpr{val: 10}, Y: IntOrExpr{val: 0}, Width: IntOrExpr{val: 12}}
	one := inkBounds(renderTextLayer(t, l))
	l.ScaleY = 2
	two := inkBounds(renderTextLayer(t, l))
	if two.Dy() < one.Dy()*2-3 {
		t.Errorf("two-line block height %d, want about %d", two.Dy(), one.Dy()*2)
	}
}

func TestLoadTemplate_ScaleYValidation(t *testing.T) {
	for _, tc := range []struct {
		val string
		ok  bool
	}{{"0.5", true}, {"1", true}, {"8", true}, {".inf", false}, {".nan", false}, {"-1", false}, {"0.001", false}, {"9", false}} {
		err := loadYAMLTemplate(t, "  - type: text\n    value: x\n    scale_y: "+tc.val+"\n")
		if (err == nil) != tc.ok {
			t.Errorf("scale_y %s: err = %v, want ok=%v", tc.val, err, tc.ok)
		}
	}
}

func TestText_IDInIfElifElseChain(t *testing.T) {
	idText := Layer{Type: "text", Font: "f", Size: 20, ID: "a", Value: StringOrCond{raw: "WW"},
		X: IntOrExpr{val: 0}, Y: IntOrExpr{val: 5}, Color: StringOrCond{raw: "#000000"}}
	block := func(cond func(*Layer)) Layer {
		l := Layer{Layers: []Layer{idText}}
		cond(&l)
		return l
	}
	other := Layer{Layers: []Layer{redRect(IntOrExpr{val: 150}, 5)}}
	cases := []struct {
		name  string
		chain []Layer
	}{
		{"if", []Layer{block(func(l *Layer) { l.If = "equals(k, 1)" })}},
		{"elif", []Layer{
			{If: "equals(k, 9)", Layers: other.Layers},
			block(func(l *Layer) { l.Elif = "equals(k, 1)" })}},
		{"else", []Layer{
			{If: "equals(k, 9)", Layers: other.Layers},
			block(func(l *Layer) { l.Else = true })}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tmpl := textExtTemplate(idText)
			tmpl.Layers = append(c.chain, redRect(exprInt("a.end + 100"), 50))
			img := mustRender(t, New(textExtFontDir), tmpl, map[string]interface{}{"k": 1})
			// a.end of "WW" is > 0, so the rect lands right of x 100; unknown id would give 0.
			if got := redMinX(img); got <= 100 {
				t.Errorf("rect after chain at x = %d, want > 100 (id set inside block must be visible)", got)
			}
		})
	}
}
