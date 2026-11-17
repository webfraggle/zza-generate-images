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

// "Near" measure (rulings R11, R12), shift tolerant so that glyph edges drawn
// at whole pixels (device) instead of subpixel positions (Go) pass, while
// layout errors fail. Both images are composed over black.
//
//	(A) Both images are 3×3 box-blurred (evens out subpixel stem shapes).
//	    For each blurred C++ pixel, d = the smallest max-channel |Δ| to any
//	    blurred Go pixel in its 3×3 neighbourhood (clamped at the borders); at
//	    most nearMaxShare of the pixels may have d > nearPixelDelta.
//	(B) After 8×8 block averaging of the unblurred images, the largest
//	    per-channel |Δ| is at most nearMaxBlockDelta.
const (
	nearBlock         = 8
	nearMaxBlockDelta = 40
	nearPixelDelta    = 40
	nearMaxShare      = 0.005 // R12 variation: 1 % let a 3 px text shift (0.61 %) pass
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

// nearStats returns the largest block-averaged channel difference (B) and
// the share of pixels whose 3×3-neighbourhood difference exceeds nearPixelDelta (A).
func nearStats(goPix, cPix []int, w, h int) (maxBlock int, share float64) {
	gb, cb := boxBlur3(goPix, w, h), boxBlur3(cPix, w, h)
	over := 0
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			ci := 3 * (y*w + x)
			best := 1 << 30
			for ny := max(0, y-1); ny <= min(h-1, y+1); ny++ {
				for nx := max(0, x-1); nx <= min(w-1, x+1); nx++ {
					gi := 3 * (ny*w + nx)
					d := 0
					for c := 0; c < 3; c++ {
						d = max(d, abs(gb[gi+c]-cb[ci+c]))
					}
					best = min(best, d)
				}
			}
			if best > nearPixelDelta {
				over++
			}
		}
	}
	for by := 0; by < h; by += nearBlock {
		for bx := 0; bx < w; bx += nearBlock {
			for c := 0; c < 3; c++ {
				sa, sb, n := 0, 0, 0
				for y := by; y < by+nearBlock && y < h; y++ {
					for x := bx; x < bx+nearBlock && x < w; x++ {
						sa += goPix[3*(y*w+x)+c]
						sb += cPix[3*(y*w+x)+c]
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

// boxBlur3 returns the 3×3 box blur of an RGB image (average of the pixels
// inside the image, rounded), so subpixel stem shapes even out.
func boxBlur3(p []int, w, h int) []int {
	out := make([]int, len(p))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			for c := 0; c < 3; c++ {
				sum, n := 0, 0
				for ny := max(0, y-1); ny <= min(h-1, y+1); ny++ {
					for nx := max(0, x-1); nx <= min(w-1, x+1); nx++ {
						sum += p[3*(ny*w+nx)+c]
						n++
					}
				}
				out[3*(y*w+x)+c] = (sum + n/2) / n
			}
		}
	}
	return out
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
	return fmt.Sprintf("not near: max 8×8 block Δ %d (limit %d), %.2f%% pixels with 3×3 Δ>%d (limit %.1f%%); comparison: %s",
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

// TestNearStats checks the measure on synthetic data: identical passes, a
// one-pixel shift of a sharp edge passes (A), a large uniform difference fails.
func TestNearStats(t *testing.T) {
	const w, h = 16, 16
	a := make([]int, w*h*3)
	b := make([]int, w*h*3)
	if m, s := nearStats(a, b, w, h); m != 0 || s != 0 {
		t.Fatalf("identical: %d %v", m, s)
	}
	for y := 0; y < h; y++ { // vertical white edge at x 8 (Go) vs x 9 (C++)
		for x := 0; x < w; x++ {
			for c := 0; c < 3; c++ {
				if x >= 8 {
					a[3*(y*w+x)+c] = 255
				}
				if x >= 9 {
					b[3*(y*w+x)+c] = 255
				}
			}
		}
	}
	if _, s := nearStats(a, b, w, h); s != 0 {
		t.Fatalf("1 px shift: share %v, want 0", s)
	}
	for i := range b {
		b[i] = a[i] + 60
	}
	if m, s := nearStats(a, b, w, h); m != 60 || s <= nearMaxShare {
		t.Fatalf("uniform +60: %d %v", m, s)
	}
}

// TestAssertNearDetectsLayoutErrors proves that the shift-tolerant measure
// still fails for real layout errors: Go renders of slightly modified
// templates are compared against the original.
func TestAssertNearDetectsLayoutErrors(t *testing.T) {
	dir := t.TempDir()
	layers := func(textX, imgX int, withRect bool, rectColor string) string {
		s := fmt.Sprintf(`  - {type: rect, color: "#203040", x: 0, y: 0, width: 240, height: 240}
  - {type: text, value: "Zürich HB 15:53", font: reg, size: 18, color: "#ffffff", x: %d, y: 20}
  - {type: image, file: img.png, x: %d, y: 80}
`, textX, imgX)
		if withRect {
			s += fmt.Sprintf(`  - {type: rect, color: "%s", x: 20, y: 140, width: 120, height: 40}
`, rectColor)
		}
		return s
	}
	render := func(name, l string) *image.NRGBA {
		writeTemplate(t, dir, name, l)
		img, err := goRender(t, dir, name, []byte("{}"))
		if err != nil {
			t.Fatal(err)
		}
		return img
	}
	base := render("base", layers(10, 100, true, "#ffcc00"))

	stats := func(other *image.NRGBA) (int, float64) {
		return nearStats(overBlack(base), overBlack(other), 240, 240)
	}
	if m, s := stats(render("same", layers(10, 100, true, "#ffcc00"))); m > nearMaxBlockDelta || s > nearMaxShare {
		t.Fatalf("identical template not near: %d %v", m, s)
	}
	cases := map[string]string{
		"text line shifted 3 px": layers(13, 100, true, "#ffcc00"),
		"image shifted 3 px":     layers(10, 103, true, "#ffcc00"),
		"missing layer":          layers(10, 100, false, ""),
		"wrong fill colour":      layers(10, 100, true, "#ff8800"),
	}
	i := 0
	for name, l := range cases {
		i++
		m, s := stats(render(fmt.Sprintf("v%d", i), l))
		t.Logf("%s: block Δ %d, share %.2f%%", name, m, s*100)
		if m <= nearMaxBlockDelta && s <= nearMaxShare {
			t.Errorf("%s passes the measure (block Δ %d, share %.2f%%)", name, m, s*100)
		}
	}
}
