package renderer

import (
	"bytes"
	"image"
	"image/draw"
	"image/png"
	"os"
	"testing"

	xdraw "golang.org/x/image/draw"
)

// TestContactSheet is a tool, not a check: it writes the faltblatt-105-v1
// animation with default data to the files named by environment variables
// and is skipped when none is set.
//
//	ZZA_STILL=<file.png>          still image
//	ZZA_CONTACT_SHEET=<file.png>  all frames, 4× nearest neighbour, 6 per row
//	ZZA_GIF=<file.gif>            encoded GIF
//
//	ZZA_CONTACT_SHEET=/tmp/sheet.png go test ./internal/renderer/ -run TestContactSheet
func TestContactSheet(t *testing.T) {
	stillPath, sheetPath, gifPath := os.Getenv("ZZA_STILL"), os.Getenv("ZZA_CONTACT_SHEET"), os.Getenv("ZZA_GIF")
	if stillPath == "" && sheetPath == "" && gifPath == "" {
		t.Skip("set ZZA_STILL, ZZA_CONTACT_SHEET and/or ZZA_GIF to write output files")
	}
	r, tmpl := loadFaltblatt(t)
	body, data := faltblattDefault(t)

	if stillPath != "" {
		writePNG(t, stillPath, mustRenderFaltblatt(t, r, tmpl, data))
	}
	if sheetPath == "" && gifPath == "" {
		return
	}
	anim, err := r.RenderAnimation(tmpl, data, AnimationSeed(faltblattName, body))
	if err != nil {
		t.Fatalf("RenderAnimation: %v", err)
	}
	if sheetPath != "" {
		writePNG(t, sheetPath, contactSheet(anim.Frames, 4, 6))
		t.Logf("contact sheet: %d frames", len(anim.Frames))
	}
	if gifPath != "" {
		var buf bytes.Buffer
		if err := EncodeGIF(&buf, anim, tmpl.Meta.Canvas.Colors); err != nil {
			t.Fatalf("EncodeGIF: %v", err)
		}
		if err := os.WriteFile(gifPath, buf.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("GIF: %d frames, %d bytes", len(anim.Frames), buf.Len())
	}
}

// contactSheet lays out frames left to right, top to bottom, cols per row,
// each scaled by zoom without smoothing and separated by a 2 px gap.
func contactSheet(frames []*image.NRGBA, zoom, cols int) *image.NRGBA {
	const gap = 2
	fw, fh := frames[0].Bounds().Dx()*zoom, frames[0].Bounds().Dy()*zoom
	rows := (len(frames) + cols - 1) / cols
	sheet := image.NewNRGBA(image.Rect(0, 0, cols*(fw+gap)-gap, rows*(fh+gap)-gap))
	draw.Draw(sheet, sheet.Bounds(), image.White, image.Point{}, draw.Src)
	for i, f := range frames {
		x, y := (i%cols)*(fw+gap), (i/cols)*(fh+gap)
		xdraw.NearestNeighbor.Scale(sheet, image.Rect(x, y, x+fw, y+fh), f, f.Bounds(), xdraw.Src, nil)
	}
	return sheet
}

func writePNG(t *testing.T, path string, img image.Image) {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode %s: %v", path, err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}
