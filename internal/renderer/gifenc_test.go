package renderer

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	"testing"
)

func gifSolid(w, h int, c color.NRGBA) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	draw.Draw(img, img.Bounds(), &image.Uniform{C: c}, image.Point{}, draw.Src)
	return img
}

func gifSquare(w, h, x, y, size int, bg, fg color.NRGBA) *image.NRGBA {
	img := gifSolid(w, h, bg)
	draw.Draw(img, image.Rect(x, y, x+size, y+size), &image.Uniform{C: fg}, image.Point{}, draw.Src)
	return img
}

func encodeAndDecode(t *testing.T, anim *Animation, colors int) *gif.GIF {
	t.Helper()
	var buf bytes.Buffer
	if err := EncodeGIF(&buf, anim, colors); err != nil {
		t.Fatalf("EncodeGIF: %v", err)
	}
	g, err := gif.DecodeAll(&buf)
	if err != nil {
		t.Fatalf("DecodeAll: %v", err)
	}
	return g
}

// reconstruct composes the decoded frames (DisposalNone) and returns the
// full canvas after each frame.
func reconstruct(g *gif.GIF) []*image.RGBA {
	canvas := image.NewRGBA(image.Rect(0, 0, g.Config.Width, g.Config.Height))
	var out []*image.RGBA
	for _, f := range g.Image {
		draw.Draw(canvas, f.Bounds(), f, f.Bounds().Min, draw.Over)
		snap := image.NewRGBA(canvas.Bounds())
		copy(snap.Pix, canvas.Pix)
		out = append(out, snap)
	}
	return out
}

func samePixels(a *image.RGBA, b *image.NRGBA) bool {
	for y := 0; y < b.Bounds().Dy(); y++ {
		for x := 0; x < b.Bounds().Dx(); x++ {
			if color.RGBAModel.Convert(b.NRGBAAt(x, y)) != a.RGBAAt(x, y) {
				return false
			}
		}
	}
	return true
}

var (
	gifBlack = color.NRGBA{0, 0, 0, 255}
	gifWhite = color.NRGBA{255, 255, 255, 255}
)

func TestEncodeGIF_Structure(t *testing.T) {
	anim := &Animation{FPS: 20, Frames: []*image.NRGBA{
		gifSolid(20, 20, gifBlack),
		gifSquare(20, 20, 5, 5, 4, gifBlack, gifWhite),
		gifSquare(20, 20, 10, 10, 4, gifBlack, gifWhite),
	}}
	g := encodeAndDecode(t, anim, 0)
	if len(g.Image) != 3 {
		t.Fatalf("frames = %d, want 3", len(g.Image))
	}
	if g.LoopCount != -1 {
		t.Errorf("LoopCount = %d, want -1", g.LoopCount)
	}
	if want := []int{5, 5, 100}; g.Delay[0] != want[0] || g.Delay[1] != want[1] || g.Delay[2] != want[2] {
		t.Errorf("Delay = %v, want %v", g.Delay, want)
	}
	if g.Config.Width != 20 || g.Config.Height != 20 {
		t.Errorf("Config size = %dx%d", g.Config.Width, g.Config.Height)
	}
	if got, want := g.Image[1].Bounds(), image.Rect(5, 5, 9, 9); got != want {
		t.Errorf("frame 1 bounds = %v, want %v", got, want)
	}
	if got, want := g.Image[2].Bounds(), image.Rect(5, 5, 14, 14); got != want {
		t.Errorf("frame 2 bounds = %v, want %v", got, want)
	}
	cm, _ := g.Config.ColorModel.(color.Palette)
	for i, f := range g.Image {
		if len(f.Palette) != len(cm) {
			t.Fatalf("frame %d palette len differs from global", i)
		}
		for j := range cm {
			if f.Palette[j] != cm[j] {
				t.Fatalf("frame %d palette differs at %d", i, j)
			}
		}
	}
}

func TestEncodeGIF_Reconstructs(t *testing.T) {
	cols := []color.NRGBA{{255, 0, 0, 255}, {0, 255, 0, 255}, {0, 0, 255, 255}, {255, 255, 0, 255}}
	var frames []*image.NRGBA
	for i := 0; i < 5; i++ {
		img := gifSolid(24, 16, cols[i%4])
		draw.Draw(img, image.Rect(i*3, 2, i*3+6, 9), &image.Uniform{C: cols[(i+1)%4]}, image.Point{}, draw.Src)
		draw.Draw(img, image.Rect(20-i, 10, 24, 16), &image.Uniform{C: gifWhite}, image.Point{}, draw.Src)
		frames = append(frames, img)
	}
	g := encodeAndDecode(t, &Animation{FPS: 10, Frames: frames}, 16)
	rec := reconstruct(g)
	if len(rec) != len(frames) {
		t.Fatalf("got %d frames, want %d", len(rec), len(frames))
	}
	for i := range frames {
		if !samePixels(rec[i], frames[i]) {
			t.Errorf("frame %d not reconstructed exactly", i)
		}
	}
}

func TestEncodeGIF_IdenticalFrames(t *testing.T) {
	f := gifSquare(10, 10, 2, 2, 4, gifBlack, gifWhite)
	anim := &Animation{FPS: 20, Frames: []*image.NRGBA{f, f, f}}
	g := encodeAndDecode(t, anim, 0)
	sum := 0
	for _, d := range g.Delay {
		sum += d
	}
	if sum != 110 {
		t.Errorf("delay sum = %d, want 110", sum)
	}
	for i, im := range g.Image {
		if im.Bounds().Empty() {
			t.Errorf("frame %d has empty bounds", i)
		}
	}
	rec := reconstruct(g)
	if !samePixels(rec[len(rec)-1], f) {
		t.Error("final image wrong")
	}
}

func TestEncodeGIF_SingleFrame(t *testing.T) {
	f := gifSquare(10, 10, 2, 2, 4, gifBlack, gifWhite)
	var buf bytes.Buffer
	if err := EncodeGIF(&buf, &Animation{FPS: 20, Frames: []*image.NRGBA{f}}, 0); err != nil {
		t.Fatal(err)
	}
	g, err := gif.DecodeAll(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if len(g.Image) != 1 || g.Delay[0] != 100 {
		t.Errorf("frames=%d delay=%v", len(g.Image), g.Delay)
	}
	if _, err := gif.Decode(bytes.NewReader(buf.Bytes())); err != nil {
		t.Errorf("gif.Decode: %v", err)
	}
}

func TestEncodeGIF_Colors(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 16, 16))
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			img.SetNRGBA(x, y, color.NRGBA{uint8(x * 16), uint8(y * 16), uint8(x * y), 255})
		}
	}
	g := encodeAndDecode(t, &Animation{FPS: 10, Frames: []*image.NRGBA{img}}, 4)
	if n := len(g.Image[0].Palette); n > 4 || n < 2 {
		t.Errorf("palette len = %d, want 2..4", n)
	}
}

func TestEncodeGIF_Deterministic(t *testing.T) {
	anim := &Animation{FPS: 20, Frames: []*image.NRGBA{
		gifSolid(20, 20, gifBlack),
		gifSquare(20, 20, 5, 5, 4, gifBlack, gifWhite),
		gifSquare(20, 20, 10, 10, 4, color.NRGBA{200, 10, 10, 255}, gifWhite),
	}}
	var a, b bytes.Buffer
	if err := EncodeGIF(&a, anim, 0); err != nil {
		t.Fatal(err)
	}
	if err := EncodeGIF(&b, anim, 0); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a.Bytes(), b.Bytes()) {
		t.Error("output differs between runs")
	}
}

func TestEncodeGIF_Errors(t *testing.T) {
	f := gifSolid(4, 4, gifBlack)
	cases := map[string]*Animation{
		"nil":          nil,
		"no frames":    {FPS: 10},
		"zero fps":     {Frames: []*image.NRGBA{f}},
		"nil frame":    {FPS: 10, Frames: []*image.NRGBA{f, nil}},
		"size differs": {FPS: 10, Frames: []*image.NRGBA{f, gifSolid(5, 4, gifBlack)}},
		"min not zero": {FPS: 10, Frames: []*image.NRGBA{image.NewNRGBA(image.Rect(1, 1, 5, 5))}},
	}
	for name, a := range cases {
		if err := EncodeGIF(&bytes.Buffer{}, a, 0); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
	for _, c := range []int{1, 257, -1} {
		if err := EncodeGIF(&bytes.Buffer{}, &Animation{FPS: 10, Frames: []*image.NRGBA{f}}, c); err == nil {
			t.Errorf("colors=%d: expected error", c)
		}
	}
}

// Semi-transparent input must still reconstruct consistently: the palette is
// always opaque, so no index can be mistaken for "transparent".
func TestEncodeGIF_TransparentInput(t *testing.T) {
	a := gifSolid(8, 8, color.NRGBA{0, 0, 0, 0})
	b := gifSquare(8, 8, 2, 2, 3, color.NRGBA{0, 0, 0, 0}, gifWhite)
	g := encodeAndDecode(t, &Animation{FPS: 10, Frames: []*image.NRGBA{a, b}}, 0)
	for _, f := range g.Image {
		for _, c := range f.Palette {
			if _, _, _, al := c.RGBA(); al != 0xffff {
				t.Fatalf("palette contains non-opaque color %v", c)
			}
		}
	}
	rec := reconstruct(g)
	want := gifSquare(8, 8, 2, 2, 3, gifBlack, gifWhite) // alpha is dropped, RGB kept
	if !samePixels(rec[len(rec)-1], want) {
		t.Error("last composited frame != palette-mapped last input frame")
	}
}

// More distinct colours than the palette budget forces the Quantize fallback.
func TestEncodeGIF_QuantizeFallback(t *testing.T) {
	mk := func(shift int) *image.NRGBA {
		img := image.NewNRGBA(image.Rect(0, 0, 16, 16))
		for y := 0; y < 16; y++ {
			for x := 0; x < 16; x++ {
				img.SetNRGBA(x, y, color.NRGBA{uint8(x*16 + shift), uint8(y * 16), uint8(x * y), 255})
			}
		}
		return img
	}
	anim := &Animation{FPS: 10, Frames: []*image.NRGBA{mk(0), mk(3), mk(7)}}
	var a, b bytes.Buffer
	if err := EncodeGIF(&a, anim, 8); err != nil {
		t.Fatal(err)
	}
	if err := EncodeGIF(&b, anim, 8); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a.Bytes(), b.Bytes()) {
		t.Error("not deterministic")
	}
	g, err := gif.DecodeAll(&a)
	if err != nil {
		t.Fatal(err)
	}
	if n := len(g.Image[0].Palette); n > 8 {
		t.Errorf("palette len %d > 8", n)
	}
}
