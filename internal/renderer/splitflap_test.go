package renderer

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

var (
	flapRed   = color.NRGBA{R: 0xff, A: 0xff}
	flapGreen = color.NRGBA{G: 0xff, A: 0xff}
	flapBlue  = color.NRGBA{B: 0xff, A: 0xff}
)

// flapLayer returns a splitflap Layer with the given geometry.
func flapLayer(x, y, w, h, cells, pitch int, split, value, varName string, children []Layer) Layer {
	return Layer{
		Type:   "splitflap",
		X:      IntOrExpr{val: x},
		Y:      IntOrExpr{val: y},
		Width:  IntOrExpr{val: w},
		Height: IntOrExpr{val: h},
		Cells:  cells,
		Pitch:  pitch,
		Split:  split,
		Value:  StringOrCond{raw: value},
		Var:    varName,
		Layers: children,
	}
}

func assertPixel(t *testing.T, img *image.NRGBA, x, y int, want color.NRGBA) {
	t.Helper()
	if got := img.NRGBAAt(x, y); got != want {
		t.Errorf("pixel (%d,%d) = %v, want %v", x, y, got, want)
	}
}

func TestSplitflap_Static_Chars(t *testing.T) {
	r := New("/tmp")
	tmpl := makeTemplate(60, 10, []Layer{
		rectLayer(0, 0, 60, 10, "#0000ff"),
		flapLayer(0, 0, 8, 10, 4, 15, "chars", "AB", "ch", []Layer{
			blockIf("eq(ch, 'A')", []Layer{rectLayer(0, 0, 8, 10, "#ff0000")}),
			blockElif("eq(ch, 'B')", []Layer{rectLayer(0, 0, 8, 10, "#00ff00")}),
		}),
	})
	img := mustRender(t, r, tmpl, nil)

	assertPixel(t, img, 4, 5, flapRed)
	assertPixel(t, img, 19, 5, flapGreen)
	assertPixel(t, img, 34, 5, flapBlue)
	assertPixel(t, img, 49, 5, flapBlue)
	assertPixel(t, img, 10, 5, flapBlue) // gap between modules
}

func TestSplitflap_Static_RelativeAndClipped(t *testing.T) {
	r := New("/tmp")
	tmpl := makeTemplate(60, 10, []Layer{
		rectLayer(0, 0, 60, 10, "#0000ff"),
		flapLayer(20, 0, 8, 10, 0, 0, "", "x", "v", []Layer{
			rectLayer(-5, 2, 100, 3, "#ff0000"),
		}),
	})
	img := mustRender(t, r, tmpl, nil)

	assertPixel(t, img, 20, 3, flapRed)
	assertPixel(t, img, 27, 3, flapRed)
	assertPixel(t, img, 19, 3, flapBlue)
	assertPixel(t, img, 28, 3, flapBlue)
	assertPixel(t, img, 24, 1, flapBlue) // y is relative to the module: rows 2..4 only
	assertPixel(t, img, 24, 5, flapBlue)
}

func TestSplitflap_Chars_Runes(t *testing.T) {
	cases := []struct {
		value string
		cells int
		want  []string
	}{
		{"ÄÖÜß€", 3, []string{"Ä", "Ö", "Ü"}},
		{"A B", 4, []string{"A", " ", "B", ""}},
	}
	for _, c := range cases {
		l := flapLayer(0, 0, 8, 10, c.cells, 10, "chars", c.value, "ch", nil)
		mods, err := planModules(&l, 0, NewEvaluator(nil), image.Rect(0, 0, 100, 10))
		if err != nil {
			t.Fatalf("planModules(%q): %v", c.value, err)
		}
		var got []string
		for i, m := range mods {
			got = append(got, m.target)
			if m.cellIdx != i {
				t.Errorf("module %d: cellIdx = %d", i, m.cellIdx)
			}
			wantRect := image.Rect(i*10, 0, i*10+8, 10)
			if m.rect != wantRect {
				t.Errorf("module %d: rect = %v, want %v", i, m.rect, wantRect)
			}
			if m.splitY != 5 {
				t.Errorf("module %d: splitY = %d, want default 5", i, m.splitY)
			}
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("targets for %q = %q, want %q", c.value, got, c.want)
		}
	}
}

func TestSplitflap_Whole(t *testing.T) {
	r := New("/tmp")
	tmpl := makeTemplate(20, 10, []Layer{
		flapLayer(0, 0, 20, 10, 0, 0, "", "{{zug}}", "nr", []Layer{
			blockIf("startsWith(nr, 'ic')", []Layer{rectLayer(0, 0, 20, 10, "#ff0000")}),
		}),
	})
	img := mustRender(t, r, tmpl, map[string]interface{}{"zug": "ic23"})
	assertPixel(t, img, 10, 5, flapRed)

	l := tmpl.Layers[0]
	mods, err := planModules(&l, 0, NewEvaluator(map[string]interface{}{"zug": "ic23"}), image.Rect(0, 0, 20, 10))
	if err != nil {
		t.Fatal(err)
	}
	if len(mods) != 1 || mods[0].target != "ic23" {
		t.Errorf("modules = %+v, want one module with target ic23", mods)
	}
}

func TestSplitflap_EmptyValue(t *testing.T) {
	r := New("/tmp")
	tmpl := makeTemplate(30, 10, []Layer{
		flapLayer(0, 0, 10, 10, 3, 10, "chars", "", "ch", []Layer{
			blockIf("isEmpty(ch)", []Layer{rectLayer(0, 0, 10, 10, "#ff0000")}),
		}),
	})
	img := mustRender(t, r, tmpl, nil)
	for _, x := range []int{5, 15, 25} {
		assertPixel(t, img, x, 5, flapRed)
	}
}

func TestSplitflap_SplitYOverride(t *testing.T) {
	l := flapLayer(0, 0, 8, 10, 1, 0, "", "x", "v", nil)
	l.SplitY = 3
	mods, err := planModules(&l, 2, NewEvaluator(nil), image.Rect(0, 0, 8, 10))
	if err != nil {
		t.Fatal(err)
	}
	if mods[0].splitY != 3 || mods[0].layerIdx != 2 || mods[0].layer != &l {
		t.Errorf("module = %+v, want splitY 3, layerIdx 2, layer pointer kept", mods[0])
	}
}

func TestSplitflap_Validation(t *testing.T) {
	child := []Layer{rectLayer(0, 0, 1, 1, "#ff0000")}
	withSplitY := func(l Layer, s int) Layer { l.SplitY = s; return l }
	withReel := func(l Layer, n int) Layer {
		l.Reel = make(Reel, n)
		return l
	}
	withChild := func(typ string) Layer {
		c := Layer{Type: typ, Value: StringOrCond{raw: "a"}, Layers: child}
		return flapLayer(0, 0, 8, 10, 1, 0, "", "x", "v", []Layer{c})
	}
	var nineLayers []Layer
	for i := 0; i < 9; i++ {
		nineLayers = append(nineLayers, flapLayer(0, i*10, 4, 10, 8, 4, "chars", "ABCDEFGH", "ch", child))
	}

	cases := []struct {
		name   string
		layers []Layer
		want   string
	}{
		{"width 0", []Layer{flapLayer(0, 0, 0, 10, 1, 0, "", "x", "v", child)}, "width and height must be positive"},
		{"outside canvas", []Layer{flapLayer(0, 0, 8, 10, 8, 10, "chars", "x", "v", child)}, "outside the canvas"},
		{"cells 65", []Layer{flapLayer(0, 0, 1, 1, 65, 1, "chars", "x", "v", child)}, "cells 65 exceeds maximum 64"},
		{"reel 65", []Layer{withReel(flapLayer(0, 0, 8, 10, 1, 0, "", "x", "v", child), 65)}, "reel has 65 entries, exceeds maximum 64"},
		{"overlap", []Layer{
			flapLayer(0, 0, 8, 10, 1, 0, "", "x", "v", child),
			flapLayer(4, 0, 8, 10, 1, 0, "", "x", "v", child),
		}, "overlaps another splitflap module"},
		{"overlap within layer", []Layer{flapLayer(0, 0, 8, 10, 2, 4, "chars", "x", "v", child)}, "overlaps another splitflap module"},
		{"in loop", []Layer{{
			Type:   "loop",
			Value:  StringOrCond{raw: "a"},
			Layers: []Layer{flapLayer(0, 0, 8, 10, 1, 0, "", "x", "v", child)},
		}}, "splitflap inside loop is not supported"},
		{"loop child", []Layer{withChild("loop")}, `layer type "loop" is not allowed inside splitflap`},
		{"copy child", []Layer{withChild("copy")}, `layer type "copy" is not allowed inside splitflap`},
		{"splitflap child", []Layer{withChild("splitflap")}, `layer type "splitflap" is not allowed inside splitflap`},
		{"forbidden child in untaken branch", []Layer{flapLayer(0, 0, 8, 10, 1, 0, "", "x", "v", []Layer{
			blockIf("eq(v, 'nope')", []Layer{{Type: "loop", Value: StringOrCond{raw: "a"}}}),
		})}, `layer type "loop" is not allowed inside splitflap`},
		{"split_y = height", []Layer{withSplitY(flapLayer(0, 0, 8, 10, 1, 0, "", "x", "v", child), 10)}, "split_y 10 must be between 1 and 9"},
		{"split_y negative", []Layer{withSplitY(flapLayer(0, 0, 8, 10, 1, 0, "", "x", "v", child), -1)}, "split_y -1 must be between 1 and 9"},
		{"unknown split", []Layer{flapLayer(0, 0, 8, 10, 1, 0, "words", "x", "v", child)}, `unknown split "words"`},
		{"cells without split", []Layer{flapLayer(0, 0, 8, 10, 2, 10, "", "x", "v", child)}, "cells > 1 requires split: chars"},
		{"too many modules", nineLayers, "total splitflap modules exceed maximum 64"},
	}
	r := New("/tmp")
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := r.Render(makeTemplate(60, 90, c.layers), nil)
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", c.want)
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Errorf("error = %q, want it to contain %q", err, c.want)
			}
		})
	}
}

func TestSplitflap_InBlockNode(t *testing.T) {
	r := New("/tmp")
	tmpl := makeTemplate(10, 10, []Layer{
		blockIf("eq(on, '1')", []Layer{
			flapLayer(0, 0, 10, 10, 1, 0, "", "A", "ch", []Layer{rectLayer(0, 0, 10, 10, "#ff0000")}),
		}),
	})
	img := mustRender(t, r, tmpl, map[string]interface{}{"on": "1"})
	assertPixel(t, img, 5, 5, flapRed)
}

func TestReel_Unmarshal(t *testing.T) {
	cases := []struct {
		yaml string
		want Reel
	}{
		{`reel: " AB"`, Reel{" ", "A", "B"}},
		{`reel: "ÄÖ"`, Reel{"Ä", "Ö"}},
		{`reel: ["rb", "", "ice"]`, Reel{"rb", "", "ice"}},
	}
	for _, c := range cases {
		var l Layer
		if err := yaml.Unmarshal([]byte("type: splitflap\n"+c.yaml), &l); err != nil {
			t.Fatalf("%s: %v", c.yaml, err)
		}
		if !reflect.DeepEqual(l.Reel, c.want) {
			t.Errorf("%s: Reel = %q, want %q", c.yaml, l.Reel, c.want)
		}
	}

	var l Layer
	if err := yaml.Unmarshal([]byte("reel: {a: b}"), &l); err == nil {
		t.Error("reel as mapping: expected error")
	}
}

func TestSplitflap_FieldsUnmarshal(t *testing.T) {
	src := `
type: splitflap
cells: 13
pitch: 14
split: chars
split_y: 14
flip_frames: 3
min_flips: 2
max_flips: 9
var: ch
`
	var l Layer
	if err := yaml.Unmarshal([]byte(src), &l); err != nil {
		t.Fatal(err)
	}
	if l.Cells != 13 || l.Pitch != 14 || l.Split != "chars" || l.SplitY != 14 ||
		l.FlipFrames != 3 || l.MinFlips != 2 || l.MaxFlips != 9 || l.Var != "ch" {
		t.Errorf("decoded layer = %+v", l)
	}
}

func TestHasSplitflap(t *testing.T) {
	nested := makeTemplate(10, 10, []Layer{
		rectLayer(0, 0, 1, 1, "#000000"),
		blockIf("eq(a, 'b')", []Layer{
			{Else: false, Layers: []Layer{flapLayer(0, 0, 1, 1, 1, 0, "", "x", "v", nil)}},
		}),
	})
	if !nested.HasSplitflap() {
		t.Error("HasSplitflap() = false for splitflap inside block nodes, want true")
	}

	tmpl, err := LoadTemplate("../../templates", "sbb-096-v1")
	if err != nil {
		t.Fatal(err)
	}
	if tmpl.HasSplitflap() {
		t.Error("HasSplitflap() = true for sbb-096-v1, want false")
	}
}

func TestRenderImage_UsesImgCache(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.png")
	src := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	for i := range src.Pix {
		src.Pix[i] = 0xff
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, src); err != nil {
		t.Fatal(err)
	}
	f.Close()

	r := New(dir)
	rc := newRenderCtx(&Template{Dir: dir}, NewEvaluator(nil))
	layer := Layer{Type: "image", File: "a.png"}
	dst := image.NewNRGBA(image.Rect(0, 0, 4, 4))
	if err := r.renderImage(rc, dst, layer, NewEvaluator(nil)); err != nil {
		t.Fatal(err)
	}
	if _, ok := rc.imgCache["a.png"]; !ok {
		t.Fatal("decoded image not stored in imgCache")
	}

	// Second draw must come from the cache: the file is gone.
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	dst2 := image.NewNRGBA(image.Rect(0, 0, 4, 4))
	if err := r.renderImage(rc, dst2, layer, NewEvaluator(nil)); err != nil {
		t.Fatalf("second renderImage: %v", err)
	}
	assertPixel(t, dst2, 1, 1, color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff})
}

func TestSplitflap_TextChild(t *testing.T) {
	r := New(textExtFontDir)
	tmpl := &Template{
		Meta:  Meta{Canvas: Canvas{Width: 90, Height: 30}},
		Fonts: []FontDef{{ID: "f", File: "NimbusSanL-Bol.otf"}},
		Dir:   textExtFontDir,
		Layers: []Layer{flapLayer(0, 0, 20, 30, 3, 30, "chars", "W W", "ch", []Layer{{
			Type:   "text",
			Font:   "f",
			Size:   40, // wider than the module: must be clipped to it
			Value:  StringOrCond{raw: "{{ch}}"},
			Color:  StringOrCond{raw: "#000000"},
			X:      IntOrExpr{val: 0},
			Y:      IntOrExpr{val: 28},
			Valign: "baseline",
		}})},
	}
	img := mustRender(t, r, tmpl, nil)

	for _, c := range []struct {
		name    string
		rect    image.Rectangle
		wantInk bool
	}{
		{"module 0", image.Rect(0, 0, 20, 30), true},
		{"gap 0-1", image.Rect(20, 0, 30, 30), false},
		{"module 1 (space)", image.Rect(30, 0, 50, 30), false},
		{"gap 1-2", image.Rect(50, 0, 60, 30), false},
		{"module 2", image.Rect(60, 0, 80, 30), true},
		{"right of module 2", image.Rect(80, 0, 90, 30), false},
	} {
		sub := img.SubImage(c.rect).(*image.NRGBA)
		if got := !inkBounds(sub).Empty(); got != c.wantInk {
			t.Errorf("%s: ink = %v, want %v", c.name, got, c.wantInk)
		}
	}
}

func TestSplitflap_ChildrenCountPerModule(t *testing.T) {
	children := []Layer{
		rectLayer(0, 0, 1, 1, "#ff0000"),
		rectLayer(0, 0, 1, 1, "#ff0000"),
		rectLayer(0, 0, 1, 1, "#ff0000"),
		rectLayer(0, 0, 1, 1, "#ff0000"),
	}
	r := New("/tmp")
	// 1 splitflap layer + 64 modules × 4 children = 257 > maxLayers.
	over := makeTemplate(64, 10, []Layer{flapLayer(0, 0, 1, 10, 64, 1, "chars", "", "ch", children)})
	if _, err := r.Render(over, nil); err == nil || !strings.Contains(err.Error(), "total layer count exceeds maximum") {
		t.Errorf("64 modules × 4 children: err = %v, want layer limit error", err)
	}
	// 1 + 63 × 4 = 253 stays within the limit.
	under := makeTemplate(64, 10, []Layer{flapLayer(0, 0, 1, 10, 63, 1, "chars", "", "ch", children)})
	if _, err := r.Render(under, nil); err != nil {
		t.Errorf("63 modules × 4 children: unexpected error %v", err)
	}
}
