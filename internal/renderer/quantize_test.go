package renderer

import (
	"image"
	"image/color"
	"testing"
)

// makeNRGBA creates a w×h NRGBA image filled with the given color.
func makeNRGBA(w, h int, c color.NRGBA) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetNRGBA(x, y, c)
		}
	}
	return img
}

// paletteContains reports whether p contains c (by RGBA equality).
func paletteContains(p color.Palette, c color.Color) bool {
	r1, g1, b1, a1 := c.RGBA()
	for _, pc := range p {
		r2, g2, b2, a2 := pc.RGBA()
		if r1 == r2 && g1 == g2 && b1 == b2 && a1 == a2 {
			return true
		}
	}
	return false
}

func TestQuantize_OutputDimensions(t *testing.T) {
	src := makeNRGBA(10, 5, color.NRGBA{R: 100, G: 150, B: 200, A: 255})
	out := Quantize(src, 4)
	if out.Bounds() != src.Bounds() {
		t.Errorf("bounds: got %v, want %v", out.Bounds(), src.Bounds())
	}
}

func TestQuantize_PaletteSizeDoesNotExceedN(t *testing.T) {
	src := makeNRGBA(20, 20, color.NRGBA{R: 255, G: 0, B: 0, A: 255})
	// Two-color image: half red, half blue
	for y := 0; y < 20; y++ {
		for x := 10; x < 20; x++ {
			src.SetNRGBA(x, y, color.NRGBA{R: 0, G: 0, B: 255, A: 255})
		}
	}
	out := Quantize(src, 8)
	if len(out.Palette) > 8 {
		t.Errorf("palette size %d exceeds n=8", len(out.Palette))
	}
}

func TestQuantize_SingleColorImage(t *testing.T) {
	red := color.NRGBA{R: 200, G: 50, B: 50, A: 255}
	src := makeNRGBA(8, 8, red)
	out := Quantize(src, 4)
	// All output pixels must map to the same palette index
	idx0 := out.ColorIndexAt(0, 0)
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			if out.ColorIndexAt(x, y) != idx0 {
				t.Errorf("pixel (%d,%d) has different palette index", x, y)
			}
		}
	}
}

func TestQuantize_ClampN_TooSmall(t *testing.T) {
	src := makeNRGBA(4, 4, color.NRGBA{R: 100, G: 100, B: 100, A: 255})
	out := Quantize(src, 0) // clamped to 2
	if len(out.Palette) < 2 {
		t.Errorf("palette size %d: expected at least 2 (n=0 clamped to 2)", len(out.Palette))
	}
}

func TestQuantize_ClampN_TooLarge(t *testing.T) {
	src := makeNRGBA(4, 4, color.NRGBA{R: 100, G: 100, B: 100, A: 255})
	out := Quantize(src, 300) // clamped to 256
	if len(out.Palette) > 256 {
		t.Errorf("palette size %d exceeds maximum 256", len(out.Palette))
	}
}

func TestQuantize_AllPixelsValidPaletteIndex(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 16, 16))
	// Fill with gradient
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			src.SetNRGBA(x, y, color.NRGBA{R: uint8(x * 16), G: uint8(y * 16), B: 128, A: 255})
		}
	}
	out := Quantize(src, 16)
	maxIdx := uint8(len(out.Palette) - 1)
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			idx := out.ColorIndexAt(x, y)
			if idx > maxIdx {
				t.Errorf("pixel (%d,%d) has index %d >= palette size %d", x, y, idx, len(out.Palette))
			}
		}
	}
}
