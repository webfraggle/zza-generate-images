package renderer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Tests of the real faltblatt-105-v1 template against the legacy generator's
// output (docs/superpowers/specs/assets/2026-11-13-faltblatt-referenz.png,
// rendered with the template's default.json).

const (
	faltblattTemplatesDir = "../../templates"
	faltblattName         = "faltblatt-105-v1"
	faltblattReference    = "../../docs/superpowers/specs/assets/2026-11-13-faltblatt-referenz.png"
	faltblattDarkLuma     = 110 // darkBounds threshold: text and logos, not the board
)

// Regions of the upper half (canvas coordinates, measured in bg.png).
var (
	faltblattGleisRect  = image.Rect(3, 10, 52, 118)
	faltblattHourRect   = image.Rect(53, 15, 122, 43)
	faltblattMinuteRect = image.Rect(123, 15, 143, 43)
	faltblattZugartRect = image.Rect(53, 48, 143, 76)
	faltblattViaRect    = image.Rect(144, 48, 234, 76)
)

// faltblattCellRect returns the rectangle of destination cell i (0-based).
func faltblattCellRect(i int) image.Rectangle {
	return image.Rect(54+14*i, 82, 65+14*i, 112)
}

func loadFaltblatt(t testing.TB) (*Renderer, *Template) {
	t.Helper()
	tmpl, err := LoadTemplate(faltblattTemplatesDir, faltblattName)
	if err != nil {
		t.Fatalf("LoadTemplate: %v", err)
	}
	return New(faltblattTemplatesDir), tmpl
}

// faltblattDefault returns default.json as raw bytes and decoded data.
func faltblattDefault(t testing.TB) ([]byte, map[string]interface{}) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(faltblattTemplatesDir, faltblattName, "default.json"))
	if err != nil {
		t.Fatalf("read default.json: %v", err)
	}
	var data map[string]interface{}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatalf("decode default.json: %v", err)
	}
	return raw, data
}

// faltblattDataWith returns the default data with zug1.<key> replaced.
func faltblattDataWith(t *testing.T, key string, value interface{}) map[string]interface{} {
	t.Helper()
	_, data := faltblattDefault(t)
	data["zug1"].(map[string]interface{})[key] = value
	return data
}

func loadPNG(t *testing.T, path string) *image.NRGBA {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
	out := image.NewNRGBA(img.Bounds())
	draw.Draw(out, out.Bounds(), img, img.Bounds().Min, draw.Src)
	return out
}

func mustRenderFaltblatt(t *testing.T, r *Renderer, tmpl *Template, data map[string]interface{}) *image.NRGBA {
	t.Helper()
	img, err := r.Render(tmpl, data)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	return img
}

func luma(c color.NRGBA) int {
	return (299*int(c.R) + 587*int(c.G) + 114*int(c.B)) / 1000
}

// darkBounds returns the bounding box of the pixels with luminance below
// faltblattDarkLuma inside rect (empty if there are none).
func darkBounds(img *image.NRGBA, rect image.Rectangle) image.Rectangle {
	var b image.Rectangle
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			if luma(img.NRGBAAt(x, y)) < faltblattDarkLuma {
				b = b.Union(image.Rect(x, y, x+1, y+1))
			}
		}
	}
	return b
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// boundsClose reports whether all four edges of a and b differ by at most tol;
// two empty rectangles are close, an empty and a non-empty one are not.
func boundsClose(a, b image.Rectangle, tol int) bool {
	if a.Empty() || b.Empty() {
		return a.Empty() && b.Empty()
	}
	return absInt(a.Min.X-b.Min.X) <= tol && absInt(a.Min.Y-b.Min.Y) <= tol &&
		absInt(a.Max.X-b.Max.X) <= tol && absInt(a.Max.Y-b.Max.Y) <= tol
}

// meanAbsDiff returns the mean absolute difference of the RGB channels of a
// and b inside rect.
func meanAbsDiff(a, b *image.NRGBA, rect image.Rectangle) float64 {
	var sum, n int
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			ca, cb := a.NRGBAAt(x, y), b.NRGBAAt(x, y)
			sum += absInt(int(ca.R)-int(cb.R)) + absInt(int(ca.G)-int(cb.G)) + absInt(int(ca.B)-int(cb.B))
			n += 3
		}
	}
	return float64(sum) / float64(n)
}

func sameRegion(a, b *image.NRGBA, rect image.Rectangle) bool {
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			if a.NRGBAAt(x, y) != b.NRGBAAt(x, y) {
				return false
			}
		}
	}
	return true
}

func TestFaltblatt_MatchesReference(t *testing.T) {
	r, tmpl := loadFaltblatt(t)
	_, data := faltblattDefault(t)
	got := mustRenderFaltblatt(t, r, tmpl, data)
	ref := loadPNG(t, faltblattReference)

	type region struct {
		name string
		rect image.Rectangle
	}
	regions := []region{
		{"gleis", faltblattGleisRect},
		{"hour", faltblattHourRect},
		{"minute", faltblattMinuteRect},
		{"via", faltblattViaRect},
	}
	for i := 0; i < 13; i++ {
		regions = append(regions, region{fmt.Sprintf("cell%d", i+1), faltblattCellRect(i)})
	}
	for _, rg := range regions {
		gb, rb := darkBounds(got, rg.rect), darkBounds(ref, rg.rect)
		t.Logf("%-7s render %v reference %v", rg.name, gb, rb)
		if !boundsClose(gb, rb, 1) {
			t.Errorf("%s: dark bounds %v, reference %v", rg.name, gb, rb)
		}
	}
}

func TestFaltblatt_ZugartIcon(t *testing.T) {
	r, tmpl := loadFaltblatt(t)
	_, data := faltblattDefault(t)
	ic := mustRenderFaltblatt(t, r, tmpl, data)
	ref := loadPNG(t, faltblattReference)

	if d := meanAbsDiff(ic, ref, faltblattZugartRect); d >= 12 {
		t.Errorf("zugart region differs from reference: mean abs diff %.2f, want < 12", d)
	}

	ice := mustRenderFaltblatt(t, r, tmpl, faltblattDataWith(t, "nr", "ICE 71"))
	if sameRegion(ice, ic, faltblattZugartRect) {
		t.Error(`nr "ICE 71" shows the same zugart flap as "IC23"`)
	}

	lower := mustRenderFaltblatt(t, r, tmpl, faltblattDataWith(t, "nr", "ic 5"))
	if !sameRegion(lower, ic, faltblattZugartRect) {
		t.Error(`nr "ic 5" does not show the IC flap`)
	}
}

// applyLegacyCopies applies the three imagecopy calls of the legacy generator
// to img, in order.
func applyLegacyCopies(img *image.NRGBA) {
	copies := []struct{ sx, sy, w, h, dx, dy int }{
		{0, 0, 240, 120, 0, 120},
		{3, 10, 49, 120, 240 - 49 - 5, 120 + 10},
		{52, 10, 183, 106, 3, 120 + 10},
	}
	b := img.Bounds()
	for _, c := range copies {
		for y := 0; y < c.h; y++ {
			for x := 0; x < c.w; x++ {
				dst := image.Pt(c.dx+x, c.dy+y)
				if !dst.In(b) {
					continue
				}
				img.SetNRGBA(dst.X, dst.Y, img.NRGBAAt(c.sx+x, c.sy+y))
			}
		}
	}
}

func TestFaltblatt_BottomHalf(t *testing.T) {
	r, tmpl := loadFaltblatt(t)
	_, data := faltblattDefault(t)
	got := mustRenderFaltblatt(t, r, tmpl, data)

	want := image.NewNRGBA(got.Bounds())
	draw.Draw(want, image.Rect(0, 0, 240, 120), got, image.Point{}, draw.Src)
	applyLegacyCopies(want)

	bottom := image.Rect(0, 120, 240, 240)
	for y := bottom.Min.Y; y < bottom.Max.Y; y++ {
		for x := bottom.Min.X; x < bottom.Max.X; x++ {
			if g, w := got.NRGBAAt(x, y), want.NRGBAAt(x, y); g != w {
				t.Fatalf("bottom half pixel (%d,%d) = %v, want %v (legacy copies of the upper half)", x, y, g, w)
			}
		}
	}
}

// Hinge line of the destination row in fg.png, crossing the stem of the "F"
// in cell 1 with the default data.
const (
	faltblattHingeX = 58
	faltblattHingeY = 96
)

func TestFaltblatt_OverlayOnTop(t *testing.T) {
	fg := loadPNG(t, filepath.Join(faltblattTemplatesDir, faltblattName, "fg.png"))
	over := fg.NRGBAAt(faltblattHingeX, faltblattHingeY)
	if over.A <= 200 {
		t.Fatalf("fg.png at (%d,%d) has alpha %d, want an opaque hinge pixel", faltblattHingeX, faltblattHingeY, over.A)
	}

	r, tmpl := loadFaltblatt(t)
	_, data := faltblattDefault(t)
	got := mustRenderFaltblatt(t, r, tmpl, data)

	// The letter is there: dark just above the hinge line.
	if l := luma(got.NRGBAAt(faltblattHingeX, faltblattHingeY-1)); l >= faltblattDarkLuma {
		t.Fatalf("no letter stroke at (%d,%d): luminance %d", faltblattHingeX, faltblattHingeY-1, l)
	}
	// ... and the overlay is drawn over it.
	want := color.NRGBA{R: over.R, G: over.G, B: over.B, A: 0xff}
	if g := got.NRGBAAt(faltblattHingeX, faltblattHingeY); g != want {
		t.Errorf("hinge pixel (%d,%d) = %v, want overlay colour %v", faltblattHingeX, faltblattHingeY, g, want)
	}
}

// assertLastFrameIsStill renders data as still and animation and checks that
// the last frame equals the still.
func assertLastFrameIsStill(t *testing.T, r *Renderer, tmpl *Template, data map[string]interface{}, body []byte) {
	t.Helper()
	still, err := r.Render(tmpl, data)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	anim, err := r.RenderAnimation(tmpl, data, AnimationSeed(faltblattName, body))
	if err != nil {
		t.Fatalf("RenderAnimation: %v", err)
	}
	last := anim.Frames[len(anim.Frames)-1]
	if !bytes.Equal(last.Pix, still.Pix) {
		t.Error("last animation frame differs from the still image")
	}
}

func TestFaltblatt_EdgeData(t *testing.T) {
	r, tmpl := loadFaltblatt(t)
	cases := []struct {
		name  string
		key   string
		value interface{}
	}{
		{"via empty", "via", ""},
		{"zeit without colon", "zeit", "7"},
		{"zeit empty", "zeit", ""},
		{"nr empty", "nr", ""},
		{"vonnach empty", "vonnach", ""},
		{"vonnach too long", "vonnach", "München Hbf tief Gleis"},
		{"gleis empty", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var data map[string]interface{}
			if tc.key == "" {
				_, data = faltblattDefault(t)
				data["gleis"] = tc.value
			} else {
				data = faltblattDataWith(t, tc.key, tc.value)
			}
			body, err := json.Marshal(data)
			if err != nil {
				t.Fatal(err)
			}
			assertLastFrameIsStill(t, r, tmpl, data, body)
		})
	}
	t.Run("no data", func(t *testing.T) {
		assertLastFrameIsStill(t, r, tmpl, map[string]interface{}{}, []byte("{}"))
	})
}

func TestFaltblatt_Animation(t *testing.T) {
	r, tmpl := loadFaltblatt(t)
	body, data := faltblattDefault(t)

	still := mustRenderFaltblatt(t, r, tmpl, data)

	start := time.Now()
	anim, err := r.RenderAnimation(tmpl, data, AnimationSeed(faltblattName, body))
	if err != nil {
		t.Fatalf("RenderAnimation: %v", err)
	}
	var buf bytes.Buffer
	if err := EncodeGIF(&buf, anim, tmpl.Meta.Canvas.Colors); err != nil {
		t.Fatalf("EncodeGIF: %v", err)
	}
	elapsed := time.Since(start)

	if !bytes.Equal(anim.Frames[len(anim.Frames)-1].Pix, still.Pix) {
		t.Error("last animation frame differs from the still image")
	}
	if n := len(anim.Frames); n < 30 || n > 61 {
		t.Errorf("%d frames, want 30–61", n)
	}
	for i, f := range anim.Frames {
		if !sameRegion(f, anim.Frames[0], faltblattGleisRect) {
			t.Errorf("frame %d: Gleis block changes during the animation", i)
		}
	}
	t.Logf("frames %d, GIF %d bytes, RenderAnimation+EncodeGIF %v", len(anim.Frames), buf.Len(), elapsed)
	if buf.Len() > 400*1024 {
		t.Errorf("GIF is %d bytes, want ≤ 400 KB", buf.Len())
	}
}

// BenchmarkFaltblatt_GIF measures one GIF cache miss (target: well below 1 s).
// A wall-clock assertion in the test above would fail under -race and on a
// loaded machine.
func BenchmarkFaltblatt_GIF(b *testing.B) {
	r, tmpl := loadFaltblatt(b)
	body, data := faltblattDefault(b)
	seed := AnimationSeed(faltblattName, body)
	for i := 0; i < b.N; i++ {
		anim, err := r.RenderAnimation(tmpl, data, seed)
		if err != nil {
			b.Fatal(err)
		}
		if err := EncodeGIF(io.Discard, anim, tmpl.Meta.Canvas.Colors); err != nil {
			b.Fatal(err)
		}
	}
}
