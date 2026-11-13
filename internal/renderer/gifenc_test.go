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
			if _, _, _, a := f.Palette[j].RGBA(); a == 0 {
				continue // decoder turns the transparent entry into alpha 0
			}
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

// gifUsedEntries is the highest palette index used in any frame plus one. The
// decoded palette length is padded to a power of two, so it cannot be asserted.
func gifUsedEntries(g *gif.GIF) int {
	m := 0
	for _, f := range g.Image {
		for _, v := range f.Pix {
			if int(v)+1 > m {
				m = int(v) + 1
			}
		}
	}
	return m
}

func TestEncodeGIF_Colors(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 16, 16))
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			img.SetNRGBA(x, y, color.NRGBA{uint8(x * 16), uint8(y * 16), uint8(x * y), 255})
		}
	}
	g := encodeAndDecode(t, &Animation{FPS: 10, Frames: []*image.NRGBA{img}}, 4)
	if n := gifUsedEntries(g); n > 5 || n < 2 {
		t.Errorf("used entries = %d, want 2..5 (4 colours + transparent)", n)
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

// Semi-transparent input must still reconstruct consistently: input alpha is
// dropped, so only the reserved entry is transparent.
func TestEncodeGIF_TransparentInput(t *testing.T) {
	a := gifSolid(8, 8, color.NRGBA{0, 0, 0, 0})
	b := gifSquare(8, 8, 2, 2, 3, color.NRGBA{0, 0, 0, 0}, gifWhite)
	g := encodeAndDecode(t, &Animation{FPS: 10, Frames: []*image.NRGBA{a, b}}, 0)
	for _, f := range g.Image {
		transparent := 0
		for _, c := range f.Palette {
			switch _, _, _, al := c.RGBA(); al {
			case 0:
				transparent++
			case 0xffff:
			default:
				t.Fatalf("palette contains semi-transparent color %v", c)
			}
		}
		if transparent != 1 {
			t.Fatalf("palette has %d transparent entries, want exactly 1", transparent)
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
	if n := gifUsedEntries(g); n > 9 {
		t.Errorf("used entries %d > 9", n)
	}
}

func gifTransparentIndex(t *testing.T, p color.Palette) int {
	t.Helper()
	for i, c := range p {
		if _, _, _, a := c.RGBA(); a == 0 {
			return i
		}
	}
	t.Fatal("palette has no transparent entry")
	return -1
}

func TestEncodeGIF_UnchangedPixelsInDeltaRectAreTransparent(t *testing.T) {
	f0 := gifSolid(20, 20, gifBlack)
	f1 := gifSolid(20, 20, gifBlack)
	draw.Draw(f1, image.Rect(2, 2, 4, 4), &image.Uniform{C: gifWhite}, image.Point{}, draw.Src)
	draw.Draw(f1, image.Rect(15, 15, 18, 18), &image.Uniform{C: gifWhite}, image.Point{}, draw.Src)
	g := encodeAndDecode(t, &Animation{FPS: 10, Frames: []*image.NRGBA{f0, f1}}, 0)
	if len(g.Image) != 2 {
		t.Fatalf("frames = %d", len(g.Image))
	}
	d := g.Image[1]
	if d.Bounds() != image.Rect(2, 2, 18, 18) {
		t.Fatalf("delta bounds = %v", d.Bounds())
	}
	tr := uint8(gifTransparentIndex(t, d.Palette))
	if got := d.ColorIndexAt(10, 10); got != tr {
		t.Errorf("unchanged pixel index = %d, want transparent %d", got, tr)
	}
	if got := d.ColorIndexAt(2, 2); got == tr {
		t.Error("changed pixel is transparent")
	}
	if got := g.Image[0].ColorIndexAt(0, 0); got == tr {
		t.Error("frame 0 must not use the transparent index")
	}
}

// gifBlocks walks the GIF byte stream and returns whether a global colour
// table exists and how many image descriptors carry a local colour table.
func gifBlocks(t *testing.T, b []byte) (global bool, localTables, images int) {
	t.Helper()
	if len(b) < 13 {
		t.Fatal("too short")
	}
	global = b[10]&0x80 != 0
	pos := 13
	if global {
		pos += 3 << (int(b[10]&7) + 1)
	}
	skipSub := func() {
		for b[pos] != 0 {
			pos += int(b[pos]) + 1
		}
		pos++
	}
	for {
		switch b[pos] {
		case 0x3B:
			return
		case 0x21:
			pos += 2
			skipSub()
		case 0x2C:
			images++
			packed := b[pos+9]
			pos += 10
			if packed&0x80 != 0 {
				localTables++
				pos += 3 << (int(packed&7) + 1)
			}
			pos++ // LZW min code size
			skipSub()
		default:
			t.Fatalf("unexpected block 0x%x at %d", b[pos], pos)
		}
	}
}

func TestEncodeGIF_OneGlobalTableNoLocal(t *testing.T) {
	anim := &Animation{FPS: 10, Frames: []*image.NRGBA{
		gifSolid(20, 20, gifBlack),
		gifSquare(20, 20, 5, 5, 4, gifBlack, gifWhite),
		gifSquare(20, 20, 10, 10, 4, gifBlack, gifWhite),
	}}
	var buf bytes.Buffer
	if err := EncodeGIF(&buf, anim, 0); err != nil {
		t.Fatal(err)
	}
	global, local, images := gifBlocks(t, buf.Bytes())
	if !global || local != 0 || images != 3 {
		t.Errorf("global=%v local=%d images=%d, want true/0/3", global, local, images)
	}
	g, err := gif.DecodeAll(&buf)
	if err != nil {
		t.Fatal(err)
	}
	gifTransparentIndex(t, g.Image[1].Palette)
}

func gifNoisyBackground(w, h int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	seed := uint32(12345)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			seed = seed*1664525 + 1013904223
			v := uint8((seed >> 24) % 100)
			img.SetNRGBA(x, y, color.NRGBA{v, v * 2, 255 - v, 255})
		}
	}
	return img
}

func TestEncodeGIF_SizeWithStaticNoisyBackground(t *testing.T) {
	bg := gifNoisyBackground(240, 240)
	var frames []*image.NRGBA
	for i := 0; i < 10; i++ {
		f := &image.NRGBA{Pix: append([]uint8(nil), bg.Pix...), Stride: bg.Stride, Rect: bg.Rect}
		draw.Draw(f, image.Rect(5+i*3, 5, 15+i*3, 15), &image.Uniform{C: gifWhite}, image.Point{}, draw.Src)
		draw.Draw(f, image.Rect(225-i*3, 225, 235-i*3, 235), &image.Uniform{C: color.NRGBA{250, 0, 0, 255}}, image.Point{}, draw.Src)
		frames = append(frames, f)
	}
	var buf bytes.Buffer
	if err := EncodeGIF(&buf, &Animation{FPS: 20, Frames: frames}, 0); err != nil {
		t.Fatal(err)
	}
	t.Logf("size = %d bytes", buf.Len())
	if buf.Len() > 90000 {
		t.Errorf("GIF is %d bytes, want <= 90000", buf.Len())
	}
	g, err := gif.DecodeAll(&buf)
	if err != nil {
		t.Fatal(err)
	}
	rec := reconstruct(g)
	for i := range frames {
		if !samePixels(rec[i], frames[i]) {
			t.Fatalf("frame %d not reconstructed", i)
		}
	}
}

// colors: 2 means two real colours plus the transparent entry.
func TestEncodeGIF_TwoColors(t *testing.T) {
	anim := &Animation{FPS: 10, Frames: []*image.NRGBA{
		gifSolid(6, 6, gifBlack), gifSquare(6, 6, 1, 1, 3, gifBlack, gifWhite), gifSquare(6, 6, 2, 2, 3, gifBlack, gifWhite),
	}}
	g := encodeAndDecode(t, anim, 2)
	if n := gifUsedEntries(g); n != 3 {
		t.Errorf("used palette entries = %d, want 3", n)
	}
	rec := reconstruct(g)
	for i := range anim.Frames {
		if !samePixels(rec[i], anim.Frames[i]) {
			t.Errorf("frame %d not reconstructed exactly", i)
		}
	}
}

// 256 colours still fit: 255 real + transparent.
func TestEncodeGIF_FullPalette(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 32, 32))
	for i := 0; i < 32*32; i++ {
		img.SetNRGBA(i%32, i/32, color.NRGBA{uint8(i), uint8(i / 4), 7, 255})
	}
	g := encodeAndDecode(t, &Animation{FPS: 10, Frames: []*image.NRGBA{img, img}}, 256)
	if n := len(g.Image[0].Palette); n > 256 {
		t.Errorf("palette len = %d > 256", n)
	}
}
