package editor

import (
	"encoding/json"
	"image/color"
	"os"
	"path/filepath"
	"testing"

	"github.com/webfraggle/zza-generate-images/internal/renderer"
)

// TestStarterTemplate_MirroredLayout checks that the starter shows the
// mirrored front/back layout: platform field on the outside (top half left,
// bottom half right) and the content field on the inside.
func TestStarterTemplate_MirroredLayout(t *testing.T) {
	dir := t.TempDir()
	const name = "s-105-v1"
	if err := InitTemplate(dir, name); err != nil {
		t.Fatal(err)
	}
	tmpl, err := renderer.LoadTemplate(dir, name)
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dir, name, "default.json"))
	if err != nil {
		t.Fatal(err)
	}
	var data map[string]any
	if err := json.Unmarshal(b, &data); err != nil {
		t.Fatal(err)
	}
	img, err := renderer.New(dir).Render(tmpl, data)
	if err != nil {
		t.Fatal(err)
	}
	if got := img.Bounds().Size(); got.X != 240 || got.Y != 240 {
		t.Fatalf("canvas %v, want 240x240", got)
	}

	accent := color.NRGBA{0xff, 0xff, 0xff, 0xff}
	bg := color.NRGBA{0x14, 0x2a, 0x9b, 0xff}
	for _, c := range []struct {
		x, y int
		want color.NRGBA
		what string
	}{
		{10, 60, accent, "platform field top left"},
		{229, 180, accent, "platform field bottom right"},
		{230, 60, bg, "content field top right"},
		{10, 180, bg, "content field bottom left"},
		{43, 5, accent, "platform field top, last column"},
		{44, 5, bg, "content field top, first column"},
		{195, 125, bg, "content field bottom, last column"},
		{196, 125, accent, "platform field bottom, first column"},
	} {
		if got := img.NRGBAAt(c.x, c.y); got != c.want {
			t.Errorf("%s: pixel (%d,%d) = %v, want %v", c.what, c.x, c.y, got, c.want)
		}
	}
}
