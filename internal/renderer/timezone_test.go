package renderer

import (
	"image"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fixedNow is 09:15:30 UTC, i.e. 10:15:30 in Berlin (CET).
var fixedNow = time.Date(2026, 11, 13, 9, 15, 30, 0, time.UTC)

func TestLoadTimezone(t *testing.T) {
	ok := map[string]string{
		"":                 "Europe/Berlin",
		"Europe/Berlin":    "Europe/Berlin",
		"UTC":              "UTC",
		"America/New_York": "America/New_York",
		"Europe/Zurich":    "Europe/Zurich",
	}
	for in, want := range ok {
		loc, err := loadTimezone(in)
		if err != nil {
			t.Errorf("loadTimezone(%q): unexpected error %v", in, err)
			continue
		}
		if loc.String() != want {
			t.Errorf("loadTimezone(%q) = %q, want %q", in, loc.String(), want)
		}
	}
	bad := []string{"Mars/Olympus", "../../etc/passwd", "/etc/localtime", "Local", "Europe/Berlin\n",
		strings.Repeat("a", 65), "Europe//Berlin"}
	for _, in := range bad {
		if _, err := loadTimezone(in); err == nil {
			t.Errorf("loadTimezone(%q): expected error", in)
		}
	}
}

func TestLoadTemplate_Timezone(t *testing.T) {
	write := func(tz string) (string, string) {
		dir := t.TempDir()
		sub := filepath.Join(dir, "tz")
		if err := os.MkdirAll(sub, 0o755); err != nil {
			t.Fatal(err)
		}
		y := "meta:\n  canvas:\n    width: 1\n    height: 1\n  timezone: \"" + tz + "\"\nlayers: []\n"
		if err := os.WriteFile(filepath.Join(sub, "template.yaml"), []byte(y), 0o644); err != nil {
			t.Fatal(err)
		}
		return dir, "tz"
	}
	dir, name := write("Mars/Olympus")
	if _, err := LoadTemplate(dir, name); err == nil || !strings.Contains(err.Error(), "invalid meta.timezone") {
		t.Fatalf("want invalid meta.timezone error, got %v", err)
	}
	dir, name = write("UTC")
	tmpl, err := LoadTemplate(dir, name)
	if err != nil {
		t.Fatal(err)
	}
	if tmpl.location().String() != "UTC" {
		t.Errorf("location = %q, want UTC", tmpl.location())
	}
}

func TestRender_TemplateTimezone(t *testing.T) {
	newTmpl := func() *Template {
		l := rectLayer(0, 0, 1, 1, "")
		l.Color = StringOrCond{cond: &condMap{
			branches: []condBranch{{ifExpr: "equals(now.hour, '10')", then: "#ff0000"}},
			els:      "#00ff00",
		}}
		return &Template{Meta: Meta{Canvas: Canvas{Width: 1, Height: 1}}, Layers: []Layer{l}}
	}
	r := New("/tmp")
	r.nowFunc = func() time.Time { return fixedNow }
	px := func(tmpl *Template) [3]uint8 {
		img, err := r.Render(tmpl, map[string]interface{}{})
		if err != nil {
			t.Fatal(err)
		}
		c := img.NRGBAAt(0, 0)
		return [3]uint8{c.R, c.G, c.B}
	}
	red, green := [3]uint8{255, 0, 0}, [3]uint8{0, 255, 0}
	if got := px(newTmpl()); got != red {
		t.Errorf("default zone: %v, want red", got)
	}
	utc := newTmpl()
	utc.Meta.Timezone = "UTC"
	if got := px(utc); got != green {
		t.Errorf("UTC: %v, want green", got)
	}
	r.nowFunc = func() time.Time { return time.Date(2026, 7, 1, 8, 0, 0, 0, time.UTC) }
	if got := px(newTmpl()); got != red {
		t.Errorf("summer time: %v, want red", got)
	}
}

func TestRotateImageCW_ParityAndAxis(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 70, 70))
	for deg := 0; deg < 360; deg++ {
		b := rotateImageCW(src, float64(deg)).Bounds()
		if b.Dx()%2 != 0 || b.Dy()%2 != 0 {
			t.Fatalf("70x70 at %d deg: odd result %dx%d", deg, b.Dx(), b.Dy())
		}
	}
	odd := image.NewNRGBA(image.Rect(0, 0, 9, 9))
	for _, deg := range []float64{0, 30, 45, 90} {
		b := rotateImageCW(odd, deg).Bounds()
		if b.Dx()%2 == 0 || b.Dy()%2 == 0 {
			t.Errorf("9x9 at %v deg: even result %dx%d", deg, b.Dx(), b.Dy())
		}
	}

	blk := image.NewNRGBA(image.Rect(0, 0, 70, 70))
	for y := 34; y < 36; y++ {
		for x := 34; x < 36; x++ {
			blk.SetNRGBA(x, y, color.NRGBA{R: 255, G: 255, B: 255, A: 255})
		}
	}
	for _, deg := range []float64{0, 17, 45, 90, 133, 180, 271} {
		dst := rotateImageCW(blk, deg)
		var sx, sy, sa float64
		b := dst.Bounds()
		for y := b.Min.Y; y < b.Max.Y; y++ {
			for x := b.Min.X; x < b.Max.X; x++ {
				a := float64(dst.NRGBAAt(x, y).A)
				sx += a * (float64(x) + 0.5)
				sy += a * (float64(y) + 0.5)
				sa += a
			}
		}
		if sa == 0 {
			t.Fatalf("%v deg: empty result", deg)
		}
		cx := sx/sa - float64(b.Dx()/2)
		cy := sy/sa - float64(b.Dy()/2)
		if cx < -0.15 || cx > 0.15 || cy < -0.15 || cy > 0.15 {
			t.Errorf("%v deg: axis offset (%.3f, %.3f)", deg, cx, cy)
		}
	}
}
