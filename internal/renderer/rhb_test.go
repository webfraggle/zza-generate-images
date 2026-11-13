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

// yellowCount counts yellow pixels in columns x0..x1-1 and rows y0..y1.
func yellowCount(img *image.NRGBA, x0, x1, y0, y1 int) int {
	n := 0
	for y := y0; y <= y1; y++ {
		for x := x0; x < x1; x++ {
			if isYellow(img, x, y) {
				n++
			}
		}
	}
	return n
}

func TestRhb105_RendersAllCases(t *testing.T) {
	r := New("../../templates")
	tmpl, err := LoadTemplate("../../templates", "rhb-105-v1")
	if err != nil {
		t.Fatal(err)
	}
	numberField := func(t *testing.T, img *image.NRGBA) {
		t.Helper()
		if yellowCount(img, 10, 20, 0, 36) == 0 {
			t.Error("number field (x 10-19, rows 0-36) is empty, want dots")
		}
	}
	hasVia := func(t *testing.T, img *image.NRGBA) {
		t.Helper()
		numberField(t, img)
		if yellowCount(img, 50, 240, 6, 16) == 0 {
			t.Error("no via text right of the number field")
		}
	}
	cases := []struct {
		name    string
		nr, via string
		check   func(t *testing.T, img *image.NRGBA)
	}{
		{"default", "-keep-", "-keep-", hasVia},
		{"IR", "IR", "-keep-", hasVia},
		{"R", "R", "-keep-", hasVia},
		{"REX", "REX", "-keep-", hasVia},
		{"novia", "-keep-", "", func(t *testing.T, img *image.NRGBA) {
			numberField(t, img)
			if n := yellowCount(img, 40, 240, 6, 16); n != 0 {
				t.Errorf("no via: %d yellow pixels in rows 6-16 right of the number field", n)
			}
		}},
		{"five", "-keep-", "Chur - Thusis - Filisur - Samedan - Pontresina", hasVia},
		{"emptynr", "", "-keep-", func(t *testing.T, img *image.NRGBA) {
			if n := yellowCount(img, 4, 20, 0, 36); n != 0 {
				t.Errorf("empty nr: %d yellow pixels in x 4-19, rows 0-36 (number field must be gone)", n)
			}
			if yellowCount(img, 20, 240, 6, 16) == 0 {
				t.Error("empty nr: via text missing")
			}
		}},
	}
	for _, c := range cases {
		img, err := r.Render(tmpl, rhbData(t, c.nr, c.via))
		if err != nil {
			t.Errorf("%s: %v", c.name, err)
			continue
		}
		if b := img.Bounds(); b.Dx() != 240 || b.Dy() != 240 {
			t.Errorf("%s: size %v", c.name, b)
		}
		t.Run(c.name, func(t *testing.T) { c.check(t, img) })
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
