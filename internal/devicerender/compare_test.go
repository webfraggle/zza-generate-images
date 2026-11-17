package devicerender

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// "Near" measure (spec global constraints): both images composed over black;
// after 4×4 block averaging the largest per-channel difference is at most
// nearMaxBlockDelta, and at most nearMaxShare of the pixels differ by more
// than nearPixelDelta in some channel.
const (
	nearBlock         = 4
	nearMaxBlockDelta = 48
	nearPixelDelta    = 8
	nearMaxShare      = 0.12
)

// overBlack composes img over black and returns its RGB values (3 bytes per pixel).
func overBlack(img *image.NRGBA) []int {
	b := img.Bounds()
	out := make([]int, 0, b.Dx()*b.Dy()*3)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			c := img.NRGBAAt(x, y)
			a := int(c.A)
			out = append(out, (int(c.R)*a+127)/255, (int(c.G)*a+127)/255, (int(c.B)*a+127)/255)
		}
	}
	return out
}

// nearStats returns the largest block-averaged channel difference and the
// share of pixels with a channel difference above nearPixelDelta.
func nearStats(a, b []int, w, h int) (maxBlock int, share float64) {
	over := 0
	for i := 0; i < w*h; i++ {
		for c := 0; c < 3; c++ {
			if d := abs(a[3*i+c] - b[3*i+c]); d > nearPixelDelta {
				over++
				break
			}
		}
	}
	for by := 0; by < h; by += nearBlock {
		for bx := 0; bx < w; bx += nearBlock {
			for c := 0; c < 3; c++ {
				sa, sb, n := 0, 0, 0
				for y := by; y < by+nearBlock && y < h; y++ {
					for x := bx; x < bx+nearBlock && x < w; x++ {
						sa += a[3*(y*w+x)+c]
						sb += b[3*(y*w+x)+c]
						n++
					}
				}
				if d := abs(sa-sb) / n; d > maxBlock {
					maxBlock = d
				}
			}
		}
	}
	return maxBlock, float64(over) / float64(w*h)
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// assertNear fails the test if the C++ image is not near the Go image. On
// failure it writes "Go | C++ | |Δ|×4" as PNG (to $ZZA_COMPARE_DIR if set,
// else t.TempDir()) and logs the path.
func assertNear(t *testing.T, goImg, cImg *image.NRGBA) {
	t.Helper()
	if msg := nearCheck(t, goImg, cImg); msg != "" {
		t.Error(msg)
	}
}

// nearCheck returns "" if the images are near, else a description including
// the path of the written comparison image.
func nearCheck(t *testing.T, goImg, cImg *image.NRGBA) string {
	t.Helper()
	gb, cb := goImg.Bounds(), cImg.Bounds()
	if gb.Size() != cb.Size() {
		t.Fatalf("size: go %v, c++ %v", gb.Size(), cb.Size())
	}
	w, h := gb.Dx(), gb.Dy()
	a, b := overBlack(goImg), overBlack(cImg)
	maxBlock, share := nearStats(a, b, w, h)
	if maxBlock <= nearMaxBlockDelta && share <= nearMaxShare {
		return ""
	}
	path := writeComparison(t, a, b, w, h)
	return fmt.Sprintf("not near: max block Δ %d (limit %d), %.1f%% pixels Δ>%d (limit %.0f%%); comparison: %s",
		maxBlock, nearMaxBlockDelta, share*100, nearPixelDelta, nearMaxShare*100, path)
}

// writeComparison writes Go | C++ | |Δ|×4 side by side and returns the path.
func writeComparison(t *testing.T, a, b []int, w, h int) string {
	t.Helper()
	dir := os.Getenv("ZZA_COMPARE_DIR")
	if dir == "" {
		dir = t.TempDir()
	} else if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	out := image.NewNRGBA(image.Rect(0, 0, 3*w, h))
	clamp := func(v int) uint8 {
		if v > 255 {
			return 255
		}
		return uint8(v)
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i := 3 * (y*w + x)
			out.SetNRGBA(x, y, color.NRGBA{uint8(a[i]), uint8(a[i+1]), uint8(a[i+2]), 255})
			out.SetNRGBA(w+x, y, color.NRGBA{uint8(b[i]), uint8(b[i+1]), uint8(b[i+2]), 255})
			out.SetNRGBA(2*w+x, y, color.NRGBA{clamp(4 * abs(a[i]-b[i])), clamp(4 * abs(a[i+1]-b[i+1])), clamp(4 * abs(a[i+2]-b[i+2])), 255})
		}
	}
	name := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name()) + ".png"
	path := filepath.Join(dir, name)
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, out); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestNearStats checks the measure itself: identical images pass, a small
// shift in one block passes, a large difference fails.
func TestNearStats(t *testing.T) {
	const w, h = 8, 8
	a := make([]int, w*h*3)
	b := make([]int, w*h*3)
	if m, s := nearStats(a, b, w, h); m != 0 || s != 0 {
		t.Fatalf("identical: %d %v", m, s)
	}
	b[0] = 100 // one pixel, one channel: block average 100/16 = 6
	if m, s := nearStats(a, b, w, h); m != 6 || s != 1.0/64 {
		t.Fatalf("one pixel: %d %v", m, s)
	}
	for i := range b {
		b[i] = 60
	}
	if m, s := nearStats(a, b, w, h); m != 60 || s != 1 {
		t.Fatalf("all: %d %v", m, s)
	}
}
