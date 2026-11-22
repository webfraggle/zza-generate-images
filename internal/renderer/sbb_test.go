package renderer

import (
	"encoding/json"
	"image"
	"image/color"
	"os"
	"testing"
)

var (
	sbbRed     = color.NRGBA{R: 0xff, G: 0x00, B: 0x00, A: 0xff}
	sbbWhite   = color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
	sbbAbwBlue = color.NRGBA{R: 0x13, G: 0x2a, B: 0x9b, A: 0xff}
)

func loadSbb(t *testing.T) (*Renderer, *Template, map[string]interface{}) {
	t.Helper()
	tmpl, err := LoadTemplate("../../templates", "sbb-105-v1")
	if err != nil {
		t.Fatalf("LoadTemplate: %v", err)
	}
	raw, err := os.ReadFile("../../templates/sbb-105-v1/default.json")
	if err != nil {
		t.Fatal(err)
	}
	var data map[string]interface{}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	return New("../../templates"), tmpl, data
}

// sbbCount counts pixels in [x0,x1)×[y0,y1) for which match returns true.
func sbbCount(img *image.NRGBA, x0, y0, x1, y1 int, match func(color.NRGBA) bool) int {
	n := 0
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			if match(img.NRGBAAt(x, y)) {
				n++
			}
		}
	}
	return n
}

func sbbIsWhite(c color.NRGBA) bool  { return c.R > 200 && c.G > 200 && c.B > 200 }
func sbbIsYellow(c color.NRGBA) bool { return c.R > 200 && c.G > 190 && c.B < 150 }
func sbbIsRed(c color.NRGBA) bool    { return c.R > 200 && c.G < 80 && c.B < 80 }
func sbbIsDark(c color.NRGBA) bool   { return c.R < 100 && c.G < 100 && c.B < 100 }

func TestSbb105_Cases(t *testing.T) {
	r, tmpl, data := loadSbb(t)
	// Zuggattungs-Feld: x 64–121, y 16–31 (obere Hälfte)
	badge := func(img *image.NRGBA, m func(color.NRGBA) bool) int { return sbbCount(img, 64, 16, 122, 32, m) }

	cases := []struct {
		name  string
		set   map[string]interface{}
		check func(t *testing.T, img *image.NRGBA)
	}{
		{"default: abw+hinweis -> blue box, no train", nil, func(t *testing.T, img *image.NRGBA) {
			if img.NRGBAAt(117, 18) != sbbAbwBlue {
				t.Errorf("abw box pixel = %v, want %v", img.NRGBAAt(117, 18), sbbAbwBlue)
			}
			if n := badge(img, sbbIsRed); n != 0 {
				t.Errorf("%d red pixels in badge area, want 0 (train hidden)", n)
			}
			if sbbCount(img, 66, 15, 118, 32, sbbIsYellow) == 0 {
				t.Error("no yellow delay text in badge area")
			}
			if img.NRGBAAt(4, 106) != sbbRed {
				t.Error("red hinweis bar missing")
			}
		}},
		{"abw without hinweis -> delay under time, train shown", map[string]interface{}{"hinweis": "", "abw": float64(5)}, func(t *testing.T, img *image.NRGBA) {
			if sbbCount(img, 0, 36, 64, 58, sbbIsYellow) == 0 {
				t.Error("no yellow delay under the time")
			}
			if badge(img, sbbIsRed) == 0 {
				t.Error("IC logo missing")
			}
			if img.NRGBAAt(4, 106) == sbbRed {
				t.Error("red hinweis bar drawn without hinweis")
			}
		}},
		{"IC with number", map[string]interface{}{"abw": float64(0)}, func(t *testing.T, img *image.NRGBA) {
			if sbbCount(img, 71, 17, 95, 31, sbbIsRed) == 0 {
				t.Error("IC logo missing")
			}
			if sbbCount(img, 96, 17, 120, 31, sbbIsWhite) == 0 {
				t.Error("IC number missing")
			}
		}},
		{"RE red on white", map[string]interface{}{"abw": "", "hinweis": "", "nr": "RE12"}, func(t *testing.T, img *image.NRGBA) {
			if img.NRGBAAt(117, 17) != sbbWhite || badge(img, sbbIsRed) == 0 {
				t.Error("want white box with red text")
			}
		}},
		{"S black on white", map[string]interface{}{"abw": "", "hinweis": "", "nr": "s12"}, func(t *testing.T, img *image.NRGBA) {
			if img.NRGBAAt(119, 17) != sbbWhite || sbbCount(img, 72, 17, 120, 31, sbbIsDark) == 0 {
				t.Error("want white box with black text")
			}
		}},
		{"ICE white italic on red", map[string]interface{}{"abw": "", "hinweis": "", "nr": "ICE278"}, func(t *testing.T, img *image.NRGBA) {
			if img.NRGBAAt(65, 17) != sbbRed || sbbCount(img, 65, 17, 118, 31, sbbIsWhite) == 0 {
				t.Error("want red box with white text")
			}
		}},
		{"PE GEX on red", map[string]interface{}{"abw": "", "hinweis": "", "nr": "PE GEX"}, func(t *testing.T, img *image.NRGBA) {
			if img.NRGBAAt(65, 17) != sbbRed {
				t.Error("want red box")
			}
		}},
		{"unknown type plain", map[string]interface{}{"abw": "", "hinweis": "", "nr": "Bus"}, func(t *testing.T, img *image.NRGBA) {
			if badge(img, sbbIsRed) != 0 || sbbCount(img, 72, 15, 120, 31, sbbIsWhite) == 0 {
				t.Error("want plain white text without box")
			}
		}},
		{"vias capped at five", map[string]interface{}{"via": "A - B - C - D - E - F - G"}, func(t *testing.T, img *image.NRGBA) {
			if sbbCount(img, 140, 85, 238, 96, sbbIsWhite) == 0 {
				t.Error("fifth via missing")
			}
			if n := sbbCount(img, 140, 101, 238, 120, sbbIsWhite); n != 0 {
				t.Errorf("%d white pixels below the fifth via", n)
			}
		}},
		{"info only: yellow hinweis, no time", map[string]interface{}{"abw": "", "vonnach": "", "nr": "", "via": " ", "hinweis": "Bitte nicht einsteigen"}, func(t *testing.T, img *image.NRGBA) {
			if sbbCount(img, 0, 40, 160, 120, sbbIsYellow) == 0 {
				t.Error("yellow info text missing")
			}
			if n := sbbCount(img, 0, 0, 64, 35, sbbIsWhite); n != 0 {
				t.Errorf("%d white pixels in the time area, want 0", n)
			}
			if img.NRGBAAt(4, 106) == sbbRed {
				t.Error("red bar in info mode")
			}
		}},
		// PHP: "0" is falsy, so abw 0 still means info only.
		{"abw 0 is info only", map[string]interface{}{"abw": float64(0), "vonnach": "", "nr": "", "via": ""}, func(t *testing.T, img *image.NRGBA) {
			if sbbCount(img, 0, 40, 160, 120, sbbIsYellow) == 0 {
				t.Error("yellow info text missing")
			}
			if img.NRGBAAt(4, 106) == sbbRed {
				t.Error("red bar in info mode")
			}
		}},
		{"nr set is not info only", map[string]interface{}{"abw": "", "vonnach": "", "nr": "Bus", "via": ""}, func(t *testing.T, img *image.NRGBA) {
			if sbbCount(img, 0, 0, 64, 35, sbbIsWhite) == 0 {
				t.Error("time missing")
			}
			if img.NRGBAAt(4, 106) != sbbRed {
				t.Error("red hinweis bar missing")
			}
		}},
		{"abw with spaces counts as delay", map[string]interface{}{"abw": " 5", "hinweis": ""}, func(t *testing.T, img *image.NRGBA) {
			if sbbCount(img, 0, 36, 64, 58, sbbIsYellow) == 0 {
				t.Error("no yellow delay under the time")
			}
		}},
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
				z[k] = v
			}
			d["zug1"] = z
			img, err := r.Render(tmpl, d)
			if err != nil {
				t.Fatal(err)
			}
			for y := 0; y < 120; y++ {
				for x := 0; x < 240; x++ {
					if img.NRGBAAt(x, y) != img.NRGBAAt(x, y+120) {
						t.Fatalf("lower half differs from upper at (%d,%d)", x, y)
					}
				}
			}
			c.check(t, img)
		})
	}
}
