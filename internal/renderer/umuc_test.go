package renderer

import (
	"encoding/json"
	"image"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func loadUmuc(t *testing.T) (*Renderer, *Template, map[string]interface{}) {
	t.Helper()
	r := New("../../templates")
	r.nowFunc = func() time.Time { return fixedNow }
	tmpl, err := LoadTemplate("../../templates", "umuc-105-v1")
	if err != nil {
		t.Fatalf("LoadTemplate: %v", err)
	}
	raw, err := os.ReadFile("../../templates/umuc-105-v1/default.json")
	if err != nil {
		t.Fatal(err)
	}
	var data map[string]interface{}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	return r, tmpl, data
}

func setZug(data map[string]interface{}, zug, key, val string) {
	data[zug].(map[string]interface{})[key] = val
}

// reddish reports whether any pixel in the 3×3 window around (x, y) is red.
func reddish(img *image.NRGBA, x, y int) bool {
	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			c := img.NRGBAAt(x+dx, y+dy)
			if c.R > 150 && c.G < 120 && int(c.R)-int(c.G) > 80 {
				return true
			}
		}
	}
	return false
}

func umucMinuteTexts(t *testing.T, r *Renderer, tmpl *Template, data map[string]interface{}) []string {
	t.Helper()
	e := r.newEvaluator(tmpl, data)
	var out []string
	for _, l := range tmpl.Layers {
		if l.Type == "text" && l.X.Resolve(nil) == 238 {
			out = append(out, e.Interpolate(l.Value.Resolve(e)))
		}
	}
	return out
}

func TestUmuc105_Meta(t *testing.T) {
	_, tmpl, _ := loadUmuc(t)
	want := AnimationMeta{Frames: 60, FrameDelay: 1, Loop: true}
	if tmpl.Meta.Animation != want {
		t.Errorf("animation = %+v, want %+v", tmpl.Meta.Animation, want)
	}
	if !tmpl.IsAnimated() || !tmpl.UsesNow() {
		t.Errorf("IsAnimated=%v UsesNow=%v, want both true", tmpl.IsAnimated(), tmpl.UsesNow())
	}
	if got := tmpl.location().String(); got != "Europe/Berlin" {
		t.Errorf("location = %q", got)
	}
}

func TestUmuc105_Expressions(t *testing.T) {
	r, tmpl, data := loadUmuc(t)
	if got, want := umucMinuteTexts(t, r, tmpl, data), []string{"5", "8", "16"}; !reflect.DeepEqual(got, want) {
		t.Errorf("default minutes = %v, want %v", got, want)
	}
	setZug(data, "zug1", "zeit", "23:58")
	setZug(data, "zug2", "zeit", "00:04")
	setZug(data, "zug3", "zeit", "")
	if got, want := umucMinuteTexts(t, r, tmpl, data), []string{"5", "11", ""}; !reflect.DeepEqual(got, want) {
		t.Errorf("midnight minutes = %v, want %v", got, want)
	}

	e := r.newEvaluator(tmpl, data)
	want := map[string]string{"clock-hour.png": "307.5", "clock-minutes.png": "90", "clock-seconds.png": "180"}
	for _, l := range tmpl.Layers {
		w, ok := want[l.File]
		if !ok {
			continue
		}
		if got := e.Interpolate(l.Rotate.Resolve(e)); got != w {
			t.Errorf("rotate %s = %q, want %q", l.File, got, w)
		}
		if l.File == "clock-seconds.png" {
			if got := e.withAnim(59, 60, 100).Interpolate(l.Rotate.Resolve(e)); got != "534" {
				t.Errorf("rotate seconds at frame 59 = %q, want 534", got)
			}
		}
	}
}

func decodePNG(t *testing.T, path string) image.Image {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	return img
}

func TestUmuc105_Badges(t *testing.T) {
	r, tmpl, data := loadUmuc(t)
	area := image.Rect(120, 62, 139, 75)
	badge := decodePNG(t, "../../templates/umuc-105-v1/U2.png")
	if badge.Bounds().Dx() != area.Dx() || badge.Bounds().Dy() != area.Dy() {
		t.Fatalf("U2.png is %v, want size of %v", badge.Bounds(), area)
	}

	setZug(data, "zug2", "nr", "U2")
	imgU2, err := r.Render(tmpl, data)
	if err != nil {
		t.Fatal(err)
	}
	for y := 0; y < area.Dy(); y++ {
		for x := 0; x < area.Dx(); x++ {
			br, bg, bb, _ := badge.At(badge.Bounds().Min.X+x, badge.Bounds().Min.Y+y).RGBA()
			c := imgU2.NRGBAAt(area.Min.X+x, area.Min.Y+y)
			if uint8(br>>8) != c.R || uint8(bg>>8) != c.G || uint8(bb>>8) != c.B {
				t.Fatalf("badge pixel (%d,%d) differs", x, y)
			}
		}
	}

	setZug(data, "zug2", "nr", "U9")
	imgU9, err := r.Render(tmpl, data)
	if err != nil {
		t.Fatal(err)
	}
	differs, white := false, false
	for y := area.Min.Y; y < area.Max.Y; y++ {
		for x := area.Min.X; x < area.Max.X; x++ {
			a, b := imgU2.NRGBAAt(x, y), imgU9.NRGBAAt(x, y)
			if a != b {
				differs = true
			}
			if b.R > 200 && b.G > 200 && b.B > 200 {
				white = true
			}
		}
	}
	if !differs || !white {
		t.Errorf("U9 fallback: differs=%v whiteText=%v, want both true", differs, white)
	}

	setZug(data, "zug2", "nr", "")
	if _, err := r.Render(tmpl, data); err != nil {
		t.Errorf("empty nr: %v", err)
	}
}

func TestUmuc105_SecondHand(t *testing.T) {
	r, tmpl, data := loadUmuc(t)
	a := mustAnimate(t, r, tmpl, data, nil)
	if len(a.Frames) != 60 || a.Delay != 100 || !a.Loop {
		t.Fatalf("frames=%d delay=%d loop=%v, want 60/100/true", len(a.Frames), a.Delay, a.Loop)
	}
	f0, f15 := a.Frames[0], a.Frames[15]
	if !reddish(f0, 39, 76) || !reddish(f0, 201, 196) {
		t.Error("frame 0: second hand (down) not found in both clocks")
	}
	if !reddish(f15, 19, 56) || !reddish(f15, 181, 176) {
		t.Error("frame 15: second hand (left) not found in both clocks")
	}
	if reddish(f15, 39, 76) {
		t.Error("frame 15: second hand still pointing down")
	}
	still, err := r.Render(tmpl, data)
	if err != nil {
		t.Fatal(err)
	}
	if !framesEqual([]*image.NRGBA{still}, []*image.NRGBA{f0}) {
		t.Error("frame 0 != Render")
	}
	if framesEqual([]*image.NRGBA{f0}, []*image.NRGBA{a.Frames[1]}) {
		t.Error("frame 0 == frame 1")
	}
	list := image.Rect(120, 18, 240, 114)
	for i, f := range a.Frames {
		for y := list.Min.Y; y < list.Max.Y; y++ {
			for x := list.Min.X; x < list.Max.X; x++ {
				if f.NRGBAAt(x, y) != f0.NRGBAAt(x, y) {
					t.Fatalf("frame %d: train list differs at (%d,%d)", i, x, y)
				}
			}
		}
	}
}

func TestUmuc105_HandAxis(t *testing.T) {
	for _, name := range []string{"clock-hour.png", "clock-minutes.png", "clock-seconds.png"} {
		img := decodePNG(t, filepath.Join("../../templates/umuc-105-v1", name))
		b := img.Bounds()
		if b.Dx() != 70 || b.Dy() != 70 {
			t.Errorf("%s is %dx%d, want 70x70", name, b.Dx(), b.Dy())
			continue
		}
		var sum, wsum float64
		for y := b.Min.Y; y < b.Max.Y; y++ {
			for x := b.Min.X; x < b.Max.X; x++ {
				_, _, _, a := img.At(x, y).RGBA()
				w := float64(a) / 65535
				sum += w * (float64(x-b.Min.X) + 0.5)
				wsum += w
			}
		}
		if wsum == 0 {
			t.Errorf("%s is empty", name)
			continue
		}
		if cx := sum / wsum; math.Abs(cx-35.0) > 0.3 {
			t.Errorf("%s: x centroid = %.2f, want 35.0 +- 0.3", name, cx)
		}
	}
}
