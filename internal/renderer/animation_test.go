package renderer

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/draw"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestFlipSequence_TargetInReel(t *testing.T) {
	reel := []string{" ", "A", "B", "C", "D"}
	if got, want := flipSequence(reel, "D", 3), []string{"A", "B", "C", "D"}; !reflect.DeepEqual(got, want) {
		t.Errorf("k=3: got %q, want %q", got, want)
	}
	if got, want := flipSequence(reel, "D", 7), []string{"B", "C", "D", " ", "A", "B", "C", "D"}; !reflect.DeepEqual(got, want) {
		t.Errorf("k=7: got %q, want %q", got, want)
	}
}

func TestFlipSequence_TargetNotInReel(t *testing.T) {
	reel := []string{"x", "y"}
	got := flipSequence(reel, "Neuhof", 3)
	if len(got) != 4 || got[3] != "Neuhof" {
		t.Fatalf("got %q, want 4 entries ending in Neuhof", got)
	}
	start := -1
	for i, v := range reel {
		if v == got[0] {
			start = i
		}
	}
	if start < 0 {
		t.Fatalf("first entry %q not from reel", got[0])
	}
	for i := 0; i < 3; i++ {
		if want := reel[(start+i)%len(reel)]; got[i] != want {
			t.Errorf("entry %d = %q, want %q (cyclic reel order)", i, got[i], want)
		}
	}
	for i := 1; i < len(got); i++ {
		if got[i] == got[i-1] {
			t.Errorf("entries %d and %d are equal: %q", i-1, i, got)
		}
	}
}

func TestFlipSequence_NoReel(t *testing.T) {
	if got, want := flipSequence(nil, "A", 2), []string{"", "", "A"}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestFlipSequence_SingleEntryReelEqualsTarget(t *testing.T) {
	got := flipSequence([]string{"A"}, "A", 2)
	if len(got) != 3 || got[2] != "A" {
		t.Errorf("got %q, want 3 entries ending in A", got)
	}
}

func TestFlipCount_DeterministicAndBounded(t *testing.T) {
	seed := []byte("seed-1")
	other := []byte("seed-2")
	differ := 0
	for i := 0; i < 1000; i++ {
		layerIdx, cellIdx := i/50, i%50
		n := flipCount(seed, layerIdx, cellIdx, 4, 20)
		if n < 4 || n > 20 {
			t.Fatalf("flipCount(%d,%d) = %d, out of [4,20]", layerIdx, cellIdx, n)
		}
		if again := flipCount(seed, layerIdx, cellIdx, 4, 20); again != n {
			t.Fatalf("flipCount(%d,%d) not deterministic: %d vs %d", layerIdx, cellIdx, n, again)
		}
		if flipCount(other, layerIdx, cellIdx, 4, 20) != n {
			differ++
		}
	}
	if differ < 500 {
		t.Errorf("other seed changed only %d of 1000 values, want at least 500", differ)
	}
	distinct := map[int]bool{}
	for c := 0; c < 13; c++ {
		distinct[flipCount(seed, 0, c, 4, 20)] = true
	}
	if len(distinct) < 5 {
		t.Errorf("13 cells gave only %d distinct flip counts, want at least 5", len(distinct))
	}
	if got := flipCount(seed, 0, 0, 7, 7); got != 7 {
		t.Errorf("flipCount with min == max = %d, want 7", got)
	}
}

func solid(w, h int, c image.Image) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	draw.Draw(img, img.Bounds(), c, image.Point{}, draw.Src)
	return img
}

func TestComposeFlip_Phases(t *testing.T) {
	from := solid(10, 10, image.NewUniform(flapRed))
	to := solid(10, 10, image.NewUniform(flapGreen))

	// Phase 0: upper flap falling.
	dst := image.NewNRGBA(image.Rect(0, 0, 20, 20))
	at := image.Pt(5, 5)
	composeFlip(dst, at, from, to, 5, 0, 2)
	for x := 0; x < 10; x++ {
		assertPixel(t, dst, at.X+x, at.Y+0, flapGreen)
		if c := dst.NRGBAAt(at.X+x, at.Y+4); !(c.R > c.G) {
			t.Errorf("phase 0: pixel (%d,4) = %v, want reddish", x, c)
		}
		for y := 5; y < 10; y++ {
			assertPixel(t, dst, at.X+x, at.Y+y, flapRed)
		}
	}
	assertPixel(t, dst, 4, 4, color.NRGBA{}) // nothing drawn outside the module

	// Phase 1: lower flap falling.
	dst = image.NewNRGBA(image.Rect(0, 0, 20, 20))
	composeFlip(dst, at, from, to, 5, 1, 2)
	for x := 0; x < 10; x++ {
		for y := 0; y < 5; y++ {
			assertPixel(t, dst, at.X+x, at.Y+y, flapGreen)
		}
		if c := dst.NRGBAAt(at.X+x, at.Y+5); !(c.G > c.R) {
			t.Errorf("phase 1: pixel (%d,5) = %v, want greenish", x, c)
		}
		assertPixel(t, dst, at.X+x, at.Y+9, flapRed)
	}
}

// flapAnimTemplate is the TestSplitflap_Static_Chars template with a reel.
func flapAnimTemplate(cells int, value string) *Template {
	l := flapLayer(0, 0, 8, 10, cells, 10, "chars", value, "ch", []Layer{
		blockIf("eq(ch, 'A')", []Layer{rectLayer(0, 0, 8, 10, "#ff0000")}),
		blockElif("eq(ch, 'B')", []Layer{rectLayer(0, 0, 8, 10, "#00ff00")}),
	})
	l.Reel = Reel{" ", "A", "B"}
	return makeTemplate(cells*10, 10, []Layer{rectLayer(0, 0, cells*10, 10, "#0000ff"), l})
}

func mustAnimate(t *testing.T, r *Renderer, tmpl *Template, data map[string]interface{}, seed []byte) *Animation {
	t.Helper()
	a, err := r.RenderAnimation(tmpl, data, seed)
	if err != nil {
		t.Fatalf("RenderAnimation: %v", err)
	}
	return a
}

func framesEqual(a, b []*image.NRGBA) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !bytes.Equal(a[i].Pix, b[i].Pix) {
			return false
		}
	}
	return true
}

func regionEqual(a, b *image.NRGBA, rect image.Rectangle) bool {
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		i, j := a.PixOffset(rect.Min.X, y), b.PixOffset(rect.Min.X, y)
		if !bytes.Equal(a.Pix[i:i+4*rect.Dx()], b.Pix[j:j+4*rect.Dx()]) {
			return false
		}
	}
	return true
}

func TestRenderAnimation_Basics(t *testing.T) {
	r := New("/tmp")
	tmpl := flapAnimTemplate(4, "AB")
	seed := AnimationSeed("test", []byte(`{"a":1}`))

	a := mustAnimate(t, r, tmpl, nil, seed)
	if n := len(a.Frames); n < 2 || n > maxAnimFrames {
		t.Fatalf("frames = %d, want 2..%d", n, maxAnimFrames)
	}
	if a.FPS != defaultAnimFPS {
		t.Errorf("FPS = %d, want default %d", a.FPS, defaultAnimFPS)
	}
	static := mustRender(t, r, tmpl, nil)
	last := a.Frames[len(a.Frames)-1]
	if !bytes.Equal(last.Pix, static.Pix) {
		t.Error("last frame differs from Render output")
	}
	if bytes.Equal(a.Frames[0].Pix, last.Pix) {
		t.Error("frame 0 equals the last frame")
	}
	if b := mustAnimate(t, r, tmpl, nil, seed); !framesEqual(a.Frames, b.Frames) {
		t.Error("same seed produced different frames")
	}
	if c := mustAnimate(t, r, tmpl, nil, AnimationSeed("test", []byte(`{"a":2}`))); framesEqual(a.Frames, c.Frames) {
		t.Error("different seed produced identical frames")
	}
	if _, err := r.RenderAnimation(tmpl, nil, nil); err != nil {
		t.Errorf("nil seed: %v", err)
	}
}

func TestAnimationSeed(t *testing.T) {
	a := AnimationSeed("ab", []byte("c"))
	if b := AnimationSeed("a", []byte("bc")); bytes.Equal(a, b) {
		t.Error("name/body boundary is not separated")
	}
	if b := AnimationSeed("ab", []byte("c")); !bytes.Equal(a, b) {
		t.Error("not deterministic")
	}
	if len(a) != 32 {
		t.Errorf("len = %d, want 32 (sha256)", len(a))
	}
}

// stopFrames returns, per module rect, the first frame from which the region
// stays unchanged until the end.
func stopFrames(frames []*image.NRGBA, rects []image.Rectangle) []int {
	last := frames[len(frames)-1]
	stops := make([]int, len(rects))
	for c, rect := range rects {
		stop := len(frames) - 1
		for f := len(frames) - 2; f >= 0 && regionEqual(frames[f], last, rect); f-- {
			stop = f
		}
		stops[c] = stop
	}
	return stops
}

func cellRects(cells int) []image.Rectangle {
	rects := make([]image.Rectangle, cells)
	for c := range rects {
		rects[c] = image.Rect(c*10, 0, c*10+8, 10)
	}
	return rects
}

func TestRenderAnimation_StaggeredStops(t *testing.T) {
	r := New("/tmp")
	a := mustAnimate(t, r, flapAnimTemplate(13, "ABABABABABABA"), nil, []byte("staggered"))
	distinct := map[int]bool{}
	for _, s := range stopFrames(a.Frames, cellRects(13)) {
		distinct[s] = true
	}
	if len(distinct) < 4 {
		t.Errorf("only %d distinct stop frames, want at least 4", len(distinct))
	}
}

func TestRenderAnimation_FrameZeroInMotion(t *testing.T) {
	r := New("/tmp")
	a := mustAnimate(t, r, flapAnimTemplate(13, "AB A"), nil, []byte("motion"))
	last := a.Frames[len(a.Frames)-1]
	for c, rect := range cellRects(13) {
		if regionEqual(a.Frames[0], last, rect) {
			t.Errorf("cell %d: frame 0 equals the final state", c)
		}
	}
}

func TestRenderAnimation_CopyFollows(t *testing.T) {
	r := New("/tmp")
	tmpl := flapAnimTemplate(4, "AB")
	tmpl.Meta.Canvas.Height = 20
	tmpl.Layers = append(tmpl.Layers, Layer{
		Type: "copy", SrcX: 0, SrcY: 0, SrcWidth: 40, SrcHeight: 10,
		X: IntOrExpr{val: 0}, Y: IntOrExpr{val: 10},
	})
	a := mustAnimate(t, r, tmpl, nil, []byte("copy"))
	for f, img := range a.Frames {
		for y := 0; y < 10; y++ {
			i, j := img.PixOffset(0, y), img.PixOffset(0, y+10)
			if !bytes.Equal(img.Pix[i:i+4*40], img.Pix[j:j+4*40]) {
				t.Fatalf("frame %d: copy row %d differs from source", f, y)
			}
		}
	}
}

func TestRenderAnimation_NoSplitflap(t *testing.T) {
	templatesDir := filepath.Join("..", "..", "templates")
	tmpl, err := LoadTemplate(templatesDir, "sbb-105-v1")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(templatesDir, "sbb-105-v1", "default.json"))
	if err != nil {
		t.Fatal(err)
	}
	var data map[string]interface{}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	r := New(templatesDir)
	a := mustAnimate(t, r, tmpl, data, AnimationSeed("sbb-105-v1", raw))
	if len(a.Frames) != 1 {
		t.Fatalf("frames = %d, want 1", len(a.Frames))
	}
	static := mustRender(t, r, tmpl, data)
	if !bytes.Equal(a.Frames[0].Pix, static.Pix) {
		t.Error("single frame differs from Render output")
	}
}

const animLimitsYAML = `meta:
  canvas: {width: 20, height: 10}
  animation: {fps: %FPS%, duration: %DUR%}
layers:
  - type: splitflap
    x: 0
    y: 0
    width: 10
    height: 10
    cells: 2
    split: chars
    value: "AB"
    reel: " AB"
    var: ch
    flip_frames: %FF%
    max_flips: 500
    layers:
      - if: "eq(ch, 'A')"
        layers: [{type: rect, x: 0, y: 0, width: 10, height: 10, color: "#ff0000"}]
`

func writeAnimTemplate(t *testing.T, fps, dur, ff string) (string, string) {
	t.Helper()
	dir := t.TempDir()
	y := strings.NewReplacer("%FPS%", fps, "%DUR%", dur, "%FF%", ff).Replace(animLimitsYAML)
	if err := os.WriteFile(filepath.Join(dir, "template.yaml"), []byte(y), 0644); err != nil {
		t.Fatal(err)
	}
	return filepath.Dir(dir), filepath.Base(dir)
}

func TestRenderAnimation_Limits(t *testing.T) {
	root, name := writeAnimTemplate(t, "25", "5", "1")
	tmpl, err := LoadTemplate(root, name)
	if err != nil {
		t.Fatalf("LoadTemplate: %v", err)
	}
	a := mustAnimate(t, New(root), tmpl, nil, []byte("limits"))
	if n := len(a.Frames); n < 2 || n > maxAnimFrames {
		t.Errorf("frames = %d, want 2..%d", n, maxAnimFrames)
	}
	if a.FPS != 25 {
		t.Errorf("FPS = %d, want 25", a.FPS)
	}

	for _, c := range []struct{ fps, dur string }{{"60", "2"}, {"4", "2"}, {"20", "0.4"}, {"20", "6"}, {"-1", "2"}, {"20", "-1"}} {
		root, name := writeAnimTemplate(t, c.fps, c.dur, "2")
		if _, err := LoadTemplate(root, name); err == nil {
			t.Errorf("fps %s, duration %s: expected LoadTemplate error", c.fps, c.dur)
		}
	}
	for _, c := range []struct{ fps, dur string }{{"0", "0"}, {"5", "0.5"}} {
		root, name := writeAnimTemplate(t, c.fps, c.dur, "2")
		if _, err := LoadTemplate(root, name); err != nil {
			t.Errorf("fps %s, duration %s: unexpected error %v", c.fps, c.dur, err)
		}
	}

	// flip_frames outside 1–6 is a template error.
	root, name = writeAnimTemplate(t, "20", "2", "7")
	tmpl, err = LoadTemplate(root, name)
	if err != nil {
		t.Fatalf("LoadTemplate: %v", err)
	}
	if _, err := New(root).RenderAnimation(tmpl, nil, nil); err == nil || !strings.Contains(err.Error(), "flip_frames") {
		t.Errorf("flip_frames 7: err = %v, want flip_frames error", err)
	}
}

func TestRenderAnimation_FlipFramesShapeFrameCount(t *testing.T) {
	// min_flips = max_flips pins k, so the frame count is k·f + 1.
	r := New("/tmp")
	tmpl := flapAnimTemplate(2, "AB")
	tmpl.Layers[1].FlipFrames = 3
	tmpl.Layers[1].MinFlips = 5
	tmpl.Layers[1].MaxFlips = 5
	a := mustAnimate(t, r, tmpl, nil, []byte("x"))
	if len(a.Frames) != 16 {
		t.Errorf("frames = %d, want 16", len(a.Frames))
	}
}

func TestRenderAnimation_PixelLimit(t *testing.T) {
	big := strings.Replace(animLimitsYAML, "canvas: {width: 20, height: 10}", "canvas: {width: 600, height: 600}", 1)
	write := func(y string) (string, string) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "template.yaml"), []byte(y), 0644); err != nil {
			t.Fatal(err)
		}
		return filepath.Dir(dir), filepath.Base(dir)
	}
	rep := strings.NewReplacer("%FPS%", "25", "%DUR%", "5", "%FF%", "1")
	root, name := write(rep.Replace(big))
	tmpl, err := LoadTemplate(root, name)
	if err != nil {
		t.Fatalf("LoadTemplate: %v", err)
	}
	if _, err := New(root).RenderAnimation(tmpl, nil, []byte("x")); err == nil || !strings.Contains(err.Error(), "too large") {
		t.Errorf("err = %v, want 'too large' error", err)
	}

	// Same canvas with few frames stays under the limit.
	root, name = write(strings.NewReplacer("%FPS%", "5", "%DUR%", "0.5", "%FF%", "1").Replace(big))
	tmpl, err = LoadTemplate(root, name)
	if err != nil {
		t.Fatalf("LoadTemplate: %v", err)
	}
	if _, err := New(root).RenderAnimation(tmpl, nil, []byte("x")); err != nil {
		t.Errorf("small animation: unexpected error %v", err)
	}
}
