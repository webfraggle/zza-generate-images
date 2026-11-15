package mcp

import (
	"image"
	"image/color"
	"math"
	"strings"
	"testing"
)

func TestHomography_Identity(t *testing.T) {
	src := [4]point{{0, 0}, {99, 0}, {99, 49}, {0, 49}}
	h, err := homography(src, 100, 50)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []point{{0, 0}, {10, 20}, {99, 49}, {50.5, 7}} {
		x, y := applyH(h, p.X, p.Y)
		if math.Abs(x-p.X) > 1e-6 || math.Abs(y-p.Y) > 1e-6 {
			t.Fatalf("(%v,%v) -> (%v,%v)", p.X, p.Y, x, y)
		}
	}
}

func TestHomography_Degenerate(t *testing.T) {
	src := [4]point{{0, 0}, {10, 10}, {20, 20}, {0, 30}}
	if _, err := homography(src, 100, 50); err == nil || !strings.Contains(err.Error(), "corners do not form a valid quadrilateral") {
		t.Fatalf("err = %v", err)
	}
	bow := [4]point{{0, 0}, {100, 100}, {100, 0}, {0, 100}}
	if _, err := homography(bow, 100, 50); err == nil {
		t.Fatal("expected error for bow tie")
	}
}

func TestWarpPerspective_Trapezoid(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 200, 100))
	for i := range src.Pix {
		src.Pix[i] = 255
	}
	red := color.NRGBA{255, 0, 0, 255}
	quad := [4]point{{40, 10}, {160, 20}, {150, 90}, {50, 80}}
	for y := 0; y < 100; y++ {
		for x := 0; x < 200; x++ {
			inside := true
			for i := 0; i < 4; i++ {
				a, b := quad[i], quad[(i+1)%4]
				if (b.X-a.X)*(float64(y)-a.Y)-(b.Y-a.Y)*(float64(x)-a.X) < 0 {
					inside = false
				}
			}
			if inside {
				src.SetNRGBA(x, y, red)
			}
		}
	}
	out, err := warpPerspective(src, quad, 60, 30)
	if err != nil {
		t.Fatal(err)
	}
	if out.Bounds().Dx() != 60 || out.Bounds().Dy() != 30 {
		t.Fatalf("size %v", out.Bounds())
	}
	if c := out.NRGBAAt(30, 15); c != red {
		t.Fatalf("centre = %v", c)
	}
	n := 0
	for y := 0; y < 30; y++ {
		for x := 0; x < 60; x++ {
			c := out.NRGBAAt(x, y)
			if c.R > 200 && c.G < 60 && c.B < 60 {
				n++
			}
		}
	}
	if n < 60*30*95/100 {
		t.Fatalf("only %d of %d pixels red", n, 60*30)
	}
}

func TestColorToAlpha(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	g := color.NRGBA{0, 255, 0, 255}
	img.SetNRGBA(0, 0, g)
	if got := colorToAlpha(img, g, 0).NRGBAAt(0, 0).A; got != 0 {
		t.Fatalf("alpha = %d", got)
	}
	near := color.NRGBA{0x10, 0xef, 0x10, 255}
	img.SetNRGBA(0, 0, near)
	if got := colorToAlpha(img, g, 16).NRGBAAt(0, 0).A; got != 0 {
		t.Fatalf("tol 16 alpha = %d", got)
	}
	if got := colorToAlpha(img, g, 15).NRGBAAt(0, 0); got != near {
		t.Fatalf("tol 15 = %v", got)
	}
}

func TestSampleAt(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	img.SetNRGBA(0, 0, color.NRGBA{0, 0, 0, 255})
	img.SetNRGBA(1, 1, color.NRGBA{0, 0, 0, 255})
	img.SetNRGBA(1, 0, color.NRGBA{255, 255, 255, 255})
	img.SetNRGBA(0, 1, color.NRGBA{255, 255, 255, 255})
	c := sampleAt(img, 0, 0, 1)
	if absInt(int(c.R)-128) > 1 || absInt(int(c.G)-128) > 1 || absInt(int(c.B)-128) > 1 {
		t.Fatalf("got %v", c)
	}
}

func TestParseHexColor(t *testing.T) {
	c, err := parseHexColor("#FF8000")
	if err != nil || c != (color.NRGBA{255, 128, 0, 255}) {
		t.Fatalf("%v %v", c, err)
	}
	for _, s := range []string{"ff8000", "#fff", "#gg0000", "", "#ff80000"} {
		if _, err := parseHexColor(s); err == nil || err.Error() != "color must be #rrggbb" {
			t.Fatalf("%q: %v", s, err)
		}
	}
}
