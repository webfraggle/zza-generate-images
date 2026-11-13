package renderer

import (
	"encoding/json"
	"image"
	"os"
	"testing"
)

func rhbData(t *testing.T, nr, via string) map[string]interface{} {
	t.Helper()
	raw, err := os.ReadFile("../../templates/rhb-105-v1/default.json")
	if err != nil {
		t.Fatal(err)
	}
	var data map[string]interface{}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if nr != "-keep-" {
		setZug(data, "zug1", "nr", nr)
	}
	if via != "-keep-" {
		setZug(data, "zug1", "via", via)
	}
	return data
}

func isYellow(img *image.NRGBA, x, y int) bool {
	c := img.NRGBAAt(x, y)
	return c.R > 200 && c.G > 170 && c.B < 90
}

// firstYellowX returns the first column >= from with a yellow pixel in rows y0..y1.
func firstYellowX(img *image.NRGBA, from, y0, y1 int) int {
	for x := from; x < img.Bounds().Dx(); x++ {
		for y := y0; y <= y1; y++ {
			if isYellow(img, x, y) {
				return x
			}
		}
	}
	return -1
}

func TestRhb105_Loads(t *testing.T) {
	if _, err := LoadTemplate("../../templates", "rhb-105-v1"); err != nil {
		t.Fatalf("LoadTemplate: %v", err)
	}
}

func TestRhb105_RendersAllCases(t *testing.T) {
	r := New("../../templates")
	tmpl, err := LoadTemplate("../../templates", "rhb-105-v1")
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string][2]string{
		"default": {"-keep-", "-keep-"},
		"IR":      {"IR", "-keep-"},
		"R":       {"R", "-keep-"},
		"REX":     {"REX", "-keep-"},
		"novia":   {"-keep-", ""},
		"five":    {"-keep-", "Chur - Thusis - Filisur - Samedan - Pontresina"},
		"emptynr": {"", "-keep-"},
	}
	for name, c := range cases {
		img, err := r.Render(tmpl, rhbData(t, c[0], c[1]))
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if b := img.Bounds(); b.Dx() != 240 || b.Dy() != 240 {
			t.Errorf("%s: size %v", name, b)
		}
	}
}

func TestRhb105_ViaFollowsNumberWidth(t *testing.T) {
	r := New("../../templates")
	tmpl, err := LoadTemplate("../../templates", "rhb-105-v1")
	if err != nil {
		t.Fatal(err)
	}
	start := func(nr string) int {
		img, err := r.Render(tmpl, rhbData(t, nr, "-keep-"))
		if err != nil {
			t.Fatal(err)
		}
		// Rectangle start = column after the last yellow pixel in rows 20-30
		// (only dots live there); the via text starts at the first yellow
		// column in rows 6-16 right of it.
		rect := 0
		for x := 0; x < 240; x++ {
			for y := 20; y <= 30; y++ {
				if isYellow(img, x, y) {
					rect = x + 1
				}
			}
		}
		return firstYellowX(img, rect, 6, 16)
	}
	re, rex := start("RE"), start("REX")
	if re < 0 || rex < 0 {
		t.Fatalf("no yellow via pixels found: RE=%d REX=%d", re, rex)
	}
	if rex <= re {
		t.Errorf("via start for REX (%d) not right of RE (%d)", rex, re)
	}
}
