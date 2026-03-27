package renderer

import (
	"image"
	"image/color"
	"testing"
)

// --- test helpers ---

// rectLayer returns a rect Layer with the given geometry and hex color.
func rectLayer(x, y, w, h int, hexColor string) Layer {
	return Layer{
		Type:   "rect",
		X:      IntOrExpr{val: x},
		Y:      IntOrExpr{val: y},
		Width:  IntOrExpr{val: w},
		Height: IntOrExpr{val: h},
		Color:  StringOrCond{raw: hexColor},
	}
}

// blockIf returns a block-level if node (no type, has condition + sub-layers).
func blockIf(cond string, layers []Layer) Layer {
	return Layer{If: cond, Layers: layers}
}

// blockElif returns a block-level elif node.
func blockElif(cond string, layers []Layer) Layer {
	return Layer{Elif: cond, Layers: layers}
}

// blockElse returns a block-level else node.
func blockElse(layers []Layer) Layer {
	return Layer{Else: true, Layers: layers}
}

// makeTemplate creates a minimal in-memory Template (no fonts/files needed for rect layers).
func makeTemplate(w, h int, layers []Layer) *Template {
	return &Template{
		Meta:   Meta{Canvas: Canvas{Width: w, Height: h}},
		Layers: layers,
	}
}

// nrgbaAt returns the NRGBA color at pixel (x, y).
func nrgbaAt(img *image.NRGBA, x, y int) color.NRGBA {
	return img.NRGBAAt(x, y)
}

// mustRender renders tmpl and fails the test on error.
func mustRender(t *testing.T, r *Renderer, tmpl *Template, data map[string]interface{}) *image.NRGBA {
	t.Helper()
	img, err := r.Render(tmpl, data)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}
	return img
}

// --- block-level if tests ---

// TestRender_BlockIf_True: condition true → sub-layers render.
func TestRender_BlockIf_True(t *testing.T) {
	r := New("/tmp") // no file assets needed for rect-only templates
	tmpl := makeTemplate(10, 10, []Layer{
		blockIf("eq(flag, '1')", []Layer{
			rectLayer(0, 0, 4, 4, "#ff0000"),
		}),
	})

	img := mustRender(t, r, tmpl, map[string]interface{}{"flag": "1"})

	got := nrgbaAt(img, 0, 0)
	want := color.NRGBA{R: 0xff, A: 0xff}
	if got != want {
		t.Errorf("pixel (0,0) = %v, want %v", got, want)
	}
}

// TestRender_BlockIf_False: condition false → sub-layers do not render.
func TestRender_BlockIf_False(t *testing.T) {
	r := New("/tmp")
	tmpl := makeTemplate(10, 10, []Layer{
		blockIf("eq(flag, '1')", []Layer{
			rectLayer(0, 0, 4, 4, "#ff0000"),
		}),
	})

	img := mustRender(t, r, tmpl, map[string]interface{}{"flag": "0"})

	got := nrgbaAt(img, 0, 0)
	if got.A != 0 {
		t.Errorf("pixel should be transparent (not rendered), got %v", got)
	}
}

// TestRender_BlockIfElifElse: first matching branch renders, others don't.
func TestRender_BlockIfElifElse(t *testing.T) {
	r := New("/tmp")
	tmpl := makeTemplate(10, 10, []Layer{
		blockIf("eq(x, 'a')", []Layer{
			rectLayer(0, 0, 4, 4, "#ff0000"), // red
		}),
		blockElif("eq(x, 'b')", []Layer{
			rectLayer(0, 0, 4, 4, "#00ff00"), // green
		}),
		blockElse([]Layer{
			rectLayer(0, 0, 4, 4, "#0000ff"), // blue
		}),
	})

	cases := []struct {
		x    string
		want color.NRGBA
		name string
	}{
		{"a", color.NRGBA{R: 0xff, A: 0xff}, "x=a → red"},
		{"b", color.NRGBA{G: 0xff, A: 0xff}, "x=b → green"},
		{"c", color.NRGBA{B: 0xff, A: 0xff}, "x=c → blue (else)"},
	}

	for _, tc := range cases {
		img := mustRender(t, r, tmpl, map[string]interface{}{"x": tc.x})
		got := nrgbaAt(img, 0, 0)
		if got != tc.want {
			t.Errorf("%s: pixel (0,0) = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// TestRender_BlockIf_MultipleSublayers: all sub-layers in a matching block render.
func TestRender_BlockIf_MultipleSublayers(t *testing.T) {
	r := New("/tmp")
	tmpl := makeTemplate(10, 10, []Layer{
		blockIf("eq(flag, '1')", []Layer{
			rectLayer(0, 0, 2, 2, "#ff0000"), // red at (0,0)
			rectLayer(6, 6, 2, 2, "#00ff00"), // green at (6,6)
		}),
	})

	img := mustRender(t, r, tmpl, map[string]interface{}{"flag": "1"})

	if got := nrgbaAt(img, 0, 0); got != (color.NRGBA{R: 0xff, A: 0xff}) {
		t.Errorf("pixel (0,0) = %v, want red", got)
	}
	if got := nrgbaAt(img, 6, 6); got != (color.NRGBA{G: 0xff, A: 0xff}) {
		t.Errorf("pixel (6,6) = %v, want green", got)
	}
}

// TestRender_BlockIf_Nested: block if inside another block if.
func TestRender_BlockIf_Nested(t *testing.T) {
	r := New("/tmp")
	tmpl := makeTemplate(10, 10, []Layer{
		blockIf("eq(outer, '1')", []Layer{
			blockIf("eq(inner, '1')", []Layer{
				rectLayer(0, 0, 4, 4, "#ff0000"), // red
			}),
			blockElse([]Layer{
				rectLayer(0, 0, 4, 4, "#00ff00"), // green
			}),
		}),
	})

	cases := []struct {
		outer, inner string
		want         color.NRGBA
		name         string
	}{
		{"1", "1", color.NRGBA{R: 0xff, A: 0xff}, "outer=1,inner=1 → red"},
		{"1", "0", color.NRGBA{G: 0xff, A: 0xff}, "outer=1,inner=0 → green"},
		{"0", "1", color.NRGBA{}, "outer=0 → nothing"},
	}

	for _, tc := range cases {
		img := mustRender(t, r, tmpl, map[string]interface{}{"outer": tc.outer, "inner": tc.inner})
		got := nrgbaAt(img, 0, 0)
		if tc.want.A == 0 {
			if got.A != 0 {
				t.Errorf("%s: pixel should be transparent, got %v", tc.name, got)
			}
		} else if got != tc.want {
			t.Errorf("%s: pixel (0,0) = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// TestRender_BlockElif_NoChain: elif without preceding if must return an error.
func TestRender_BlockElif_NoChain(t *testing.T) {
	r := New("/tmp")
	tmpl := makeTemplate(10, 10, []Layer{
		blockElif("eq(x, 'a')", []Layer{
			rectLayer(0, 0, 4, 4, "#ff0000"),
		}),
	})

	_, err := r.Render(tmpl, map[string]interface{}{"x": "a"})
	if err == nil {
		t.Error("expected error for elif without preceding if, got nil")
	}
}

// TestRender_BlockIf_MixedWithRegular: block nodes and regular layers coexist.
func TestRender_BlockIf_MixedWithRegular(t *testing.T) {
	r := New("/tmp")
	tmpl := makeTemplate(10, 10, []Layer{
		rectLayer(0, 0, 10, 10, "#0000ff"), // always: blue background
		blockIf("eq(flag, '1')", []Layer{
			rectLayer(2, 2, 4, 4, "#ff0000"), // conditional: red square
		}),
	})

	img := mustRender(t, r, tmpl, map[string]interface{}{"flag": "1"})

	// Corner (0,0) is blue (not covered by red square).
	if got := nrgbaAt(img, 0, 0); got != (color.NRGBA{B: 0xff, A: 0xff}) {
		t.Errorf("pixel (0,0) = %v, want blue", got)
	}
	// Center (2,2) is red (covered by red square).
	if got := nrgbaAt(img, 2, 2); got != (color.NRGBA{R: 0xff, A: 0xff}) {
		t.Errorf("pixel (2,2) = %v, want red", got)
	}
}
