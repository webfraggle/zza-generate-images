package renderer

import (
	"bytes"
	"encoding/json"
	"image"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// clockTemplate loads a 90×16 template in a temp dir (with the Nimbus font)
// whose only layer is the given text value. layers is the YAML under `layers:`.
func clockTemplate(t *testing.T, animation, value string) *Template {
	t.Helper()
	dir := t.TempDir()
	font, err := os.ReadFile("../../templates/sbb-105-v1/NimbusSanL-Reg.otf")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "f.otf"), font, 0o644); err != nil {
		t.Fatal(err)
	}
	src := "meta:\n  timezone: UTC\n  canvas: {width: 90, height: 16}\n" + animation +
		"fonts:\n  - id: f\n    file: f.otf\nlayers:\n" +
		"  - type: rect\n    x: 0\n    y: 0\n    width: 90\n    height: 16\n    color: \"#000000\"\n" +
		"  - type: text\n    value: \"" + value + "\"\n    x: 1\n    y: 1\n    font: f\n    size: 12\n    color: \"#ffffff\"\n"
	tmpl, err := ParseTemplate([]byte(src), dir)
	if err != nil {
		t.Fatal(err)
	}
	return tmpl
}

func clockRenderer(at time.Time) *Renderer {
	return NewWithClock("/tmp", func() time.Time { return at })
}

// pngAt renders tmpl as a PNG with a renderer whose clock is fixed at t.
func pngAt(t *testing.T, tmpl *Template, at time.Time) *image.NRGBA {
	t.Helper()
	img, err := clockRenderer(at).Render(tmpl, nil)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	return img
}

func assertSameImage(t *testing.T, what string, got, want *image.NRGBA) {
	t.Helper()
	if !bytes.Equal(got.Pix, want.Pix) {
		t.Errorf("%s: frame differs from the PNG at the expected time", what)
	}
}

const clockAnim = "  animation: {frames: 60, frame_delay: 1}\n"

func TestFrameModeNowAdvances(t *testing.T) {
	tmpl := clockTemplate(t, clockAnim, "{{now | format('HH:mm:ss')}}")
	start := time.Date(2026, 11, 20, 10, 8, 37, 0, time.UTC)
	a := mustAnimate(t, clockRenderer(start), tmpl, nil, nil)
	assertSameImage(t, "frame 0", a.Frames[0], pngAt(t, tmpl, start))
	assertSameImage(t, "frame 23", a.Frames[23], pngAt(t, tmpl, time.Date(2026, 11, 20, 10, 9, 0, 0, time.UTC)))
	if bytes.Equal(a.Frames[0].Pix, a.Frames[1].Pix) {
		t.Error("frames 0 and 1 are identical, now did not advance")
	}
}

func TestFrameModeNowCrossesMidnight(t *testing.T) {
	tmpl := clockTemplate(t, clockAnim, "{{now.day}}.{{now.month}} {{now | format('HH:mm:ss')}}")
	start := time.Date(2026, 11, 20, 23, 59, 30, 0, time.UTC)
	a := mustAnimate(t, clockRenderer(start), tmpl, nil, nil)
	next := time.Date(2026, 11, 21, 0, 0, 0, 0, time.UTC)
	assertSameImage(t, "frame 30", a.Frames[30], pngAt(t, tmpl, next))
	assertSameImage(t, "frame 29", a.Frames[29], pngAt(t, tmpl, next.Add(-time.Second)))
}

func TestFrameModeFractionalDelay(t *testing.T) {
	tmpl := clockTemplate(t, "  animation: {frames: 60, frame_delay: 0.3}\n", "{{now | format('HH:mm:ss')}}")
	start := time.Date(2026, 11, 20, 10, 0, 0, 900_000_000, time.UTC)
	a := mustAnimate(t, clockRenderer(start), tmpl, nil, nil)
	sec0 := time.Date(2026, 11, 20, 10, 0, 0, 0, time.UTC)
	assertSameImage(t, "frame 3", a.Frames[3], pngAt(t, tmpl, sec0))
	assertSameImage(t, "frame 4", a.Frames[4], pngAt(t, tmpl, sec0.Add(time.Second)))
}

func TestSplitflapNowFrozen(t *testing.T) {
	dir := t.TempDir()
	font, err := os.ReadFile("../../templates/sbb-105-v1/NimbusSanL-Reg.otf")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "f.otf"), font, 0o644); err != nil {
		t.Fatal(err)
	}
	src := `meta:
  timezone: UTC
  canvas: {width: 30, height: 16}
  animation: {fps: 10, duration: 2}
fonts:
  - id: f
    file: f.otf
layers:
  - type: splitflap
    x: 0
    y: 0
    width: 14
    height: 16
    value: "{{now.second}}"
    var: v
    layers:
      - type: rect
        x: 0
        y: 0
        width: 14
        height: 16
        color: "#202020"
      - type: text
        value: "{{v}}"
        x: 1
        y: 1
        font: f
        size: 12
        color: "#ffffff"
`
	tmpl, err := ParseTemplate([]byte(src), dir)
	if err != nil {
		t.Fatal(err)
	}
	if !tmpl.HasSplitflap() {
		t.Fatal("template has no splitflap layer")
	}
	start := time.Date(2026, 11, 20, 10, 0, 7, 0, time.UTC)
	ticks := 0
	r := NewWithClock("/tmp", func() time.Time { ticks++; return start.Add(time.Duration(ticks-1) * time.Second) })
	a := mustAnimate(t, r, tmpl, nil, []byte("seed"))
	if len(a.Frames) < 2 {
		t.Fatalf("frames = %d, want an animation", len(a.Frames))
	}
	// The last frame is the target value, the second read at the start.
	assertSameImage(t, "last frame", a.Frames[len(a.Frames)-1], pngAt(t, tmpl, start))
	if ticks != 1 {
		t.Errorf("clock read %d times, want once", ticks)
	}
}

// The shipped clock templates describe the clock once (now); frame 3 of a
// GIF starting at 10:08:57 must equal the PNG at 10:09:00.
func TestShippedClockTemplates(t *testing.T) {
	start := time.Date(2026, 11, 20, 10, 8, 57, 0, time.UTC)
	for _, name := range []string{"umuc-105-v1", "nederland-105-v1", "oebb-105-v1"} {
		t.Run(name, func(t *testing.T) {
			tmpl, err := LoadTemplate("../../templates", name)
			if err != nil {
				t.Fatal(err)
			}
			raw, err := os.ReadFile("../../templates/" + name + "/default.json")
			if err != nil {
				t.Fatal(err)
			}
			var data map[string]interface{}
			if err := json.Unmarshal(raw, &data); err != nil {
				t.Fatal(err)
			}
			r := NewWithClock("../../templates", func() time.Time { return start })
			a := mustAnimate(t, r, tmpl, data, nil)
			want, err := NewWithClock("../../templates", func() time.Time {
				return time.Date(2026, 11, 20, 10, 9, 0, 0, time.UTC)
			}).Render(tmpl, data)
			if err != nil {
				t.Fatal(err)
			}
			assertSameImage(t, "frame 3", a.Frames[3], want)
		})
	}
}
