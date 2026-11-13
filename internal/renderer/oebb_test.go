package renderer

import (
	"encoding/json"
	"image"
	"image/color"
	"os"
	"testing"
	"time"
)

var oebbYellow = color.NRGBA{R: 0xfb, G: 0xef, B: 0x52, A: 0xff}

func loadOebb(t *testing.T) (*Renderer, *Template, map[string]interface{}) {
	t.Helper()
	r := New("../../templates")
	r.nowFunc = func() time.Time { return fixedNow }
	tmpl, err := LoadTemplate("../../templates", "oebb-105-v1")
	if err != nil {
		t.Fatalf("LoadTemplate: %v", err)
	}
	raw, err := os.ReadFile("../../templates/oebb-105-v1/default.json")
	if err != nil {
		t.Fatal(err)
	}
	var data map[string]interface{}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	return r, tmpl, data
}

// oebbYellowish counts yellow pixels (antialiased text edges included) in rows y0..y1-1.
func oebbYellowish(img *image.NRGBA, y0, y1 int) int {
	n := 0
	for y := y0; y < y1; y++ {
		for x := 0; x < img.Bounds().Dx(); x++ {
			c := img.NRGBAAt(x, y)
			if c.R > 200 && c.G > 190 && c.B < 150 {
				n++
			}
		}
	}
	return n
}

func TestOebb105_Meta(t *testing.T) {
	_, tmpl, _ := loadOebb(t)
	want := AnimationMeta{Frames: 60, FrameDelay: 1, Loop: true}
	if tmpl.Meta.Animation != want {
		t.Errorf("animation = %+v, want %+v", tmpl.Meta.Animation, want)
	}
	if !tmpl.IsAnimated() || !tmpl.UsesNow() {
		t.Errorf("IsAnimated=%v UsesNow=%v, want both true", tmpl.IsAnimated(), tmpl.UsesNow())
	}
	if got := tmpl.location().String(); got != "Europe/Vienna" {
		t.Errorf("location = %q", got)
	}
}

func TestOebb105_Animation(t *testing.T) {
	r, tmpl, data := loadOebb(t)
	a := mustAnimate(t, r, tmpl, data, nil)
	if len(a.Frames) != 60 || a.Delay != 100 || !a.Loop {
		t.Fatalf("frames=%d delay=%d loop=%v, want 60/100/true", len(a.Frames), a.Delay, a.Loop)
	}
	if framesEqual([]*image.NRGBA{a.Frames[0]}, []*image.NRGBA{a.Frames[1]}) {
		t.Error("frame 0 == frame 1: clock does not run")
	}
	// obere Hälfte wird nach y 121 kopiert
	f := a.Frames[5]
	for y := 0; y < 119; y++ {
		for x := 0; x < 240; x++ {
			if f.NRGBAAt(x, y) != f.NRGBAAt(x, y+121) {
				t.Fatalf("copy differs at (%d,%d)", x, y)
			}
		}
	}
}

func TestOebb105_Cases(t *testing.T) {
	r, tmpl, data := loadOebb(t)
	cases := []struct {
		name string
		set  map[string]string
	}{
		{"default", nil},
		{"abw0", map[string]string{"abw": "0"}},
		{"abw-empty", map[string]string{"abw": ""}},
		{"nostar", map[string]string{"hinweis": "Hält nicht in Unter Tullnerbach!"}},
		{"nohint", map[string]string{"hinweis": ""}},
		{"rex", map[string]string{"nr": "REX1234"}},
		{"rj", map[string]string{"nr": "RJ"}},
		{"s3", map[string]string{"nr": "s3"}},
		{"nonr", map[string]string{"nr": ""}},
		{"nozeit", map[string]string{"zeit": ""}},
		{"sofort", map[string]string{"zeit": "sofort"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			d := map[string]interface{}{}
			for k, v := range data {
				d[k] = v
			}
			z := map[string]interface{}{}
			for k, v := range data["zug1"].(map[string]interface{}) {
				z[k] = v
			}
			for k, v := range c.set {
				if k == "abw" && v == "0" {
					z[k] = float64(0)
				} else {
					z[k] = v
				}
			}
			d["zug1"] = z
			img, err := r.Render(tmpl, d)
			if err != nil {
				t.Fatal(err)
			}
			if img.Bounds().Dx() != 240 || img.Bounds().Dy() != 240 {
				t.Errorf("size = %v", img.Bounds())
			}
		})
	}
}

func TestOebb105_Yellow(t *testing.T) {
	r, tmpl, data := loadOebb(t)

	img, err := r.Render(tmpl, data)
	if err != nil {
		t.Fatal(err)
	}
	if n := oebbYellowish(img, 40, 66); n == 0 {
		t.Error("default: no yellow pixels (new time) in rows 40-65")
	}
	if img.NRGBAAt(1, 90) != oebbYellow {
		t.Errorf("default: bar pixel (1,90) = %v, want %v", img.NRGBAAt(1, 90), oebbYellow)
	}

	data["zug1"].(map[string]interface{})["abw"] = float64(0)
	img, err = r.Render(tmpl, data)
	if err != nil {
		t.Fatal(err)
	}
	if n := oebbYellowish(img, 0, 89); n != 0 {
		t.Errorf("abw 0: %d yellow pixels above y 89", n)
	}
	if img.NRGBAAt(1, 90) != oebbYellow {
		t.Error("abw 0 with *: bar missing")
	}

	setZug(data, "zug1", "hinweis", "Hält nicht in Unter Tullnerbach!")
	img, err = r.Render(tmpl, data)
	if err != nil {
		t.Fatal(err)
	}
	if img.NRGBAAt(1, 90) == oebbYellow || oebbYellowish(img, 85, 120) != 0 {
		t.Error("hinweis without *: bar must not be drawn")
	}
}

func TestOebb105_TrainType(t *testing.T) {
	r, tmpl, data := loadOebb(t)
	count := func(nr string) int {
		setZug(data, "zug1", "nr", nr)
		img, err := r.Render(tmpl, data)
		if err != nil {
			t.Fatal(err)
		}
		n := 0
		for y := 8; y < 32; y++ {
			for x := 56; x < 140; x++ {
				if c := img.NRGBAAt(x, y); c.R > 200 && c.G > 200 && c.B > 200 {
					n++
				}
			}
		}
		return n
	}
	rj, rex, empty := count("RJ"), count("REX1234"), count("")
	if rex <= rj {
		t.Errorf("REX1234 (%d px) should draw more than RJ (%d px)", rex, rj)
	}
	if empty != 0 {
		t.Errorf("empty nr draws %d px", empty)
	}
	// S/s: Logo (weiß) + Nummer, Groß-/Kleinschreibung egal
	if a, b := count("S3"), count("s3"); a != b || a == 0 {
		t.Errorf("S3=%d s3=%d, want equal and > 0", a, b)
	}
}
