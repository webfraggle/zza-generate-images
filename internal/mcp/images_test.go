package mcp

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func solid(w, h int, c color.NRGBA) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetNRGBA(x, y, c)
		}
	}
	return img
}

func pngBytes(t *testing.T, img image.Image) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func jpegBytes(t *testing.T, img image.Image) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := jpeg.Encode(&b, img, nil); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func TestScaleNearest(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	a, b := color.NRGBA{255, 0, 0, 255}, color.NRGBA{0, 0, 255, 255}
	src.SetNRGBA(0, 0, a)
	src.SetNRGBA(1, 1, a)
	src.SetNRGBA(1, 0, b)
	src.SetNRGBA(0, 1, b)
	got := scaleNearest(src, 3)
	if got.Bounds().Dx() != 6 || got.Bounds().Dy() != 6 {
		t.Fatalf("size %v", got.Bounds())
	}
	if got.NRGBAAt(2, 2) != a || got.NRGBAAt(3, 3) != a || got.NRGBAAt(3, 2) != b || got.NRGBAAt(0, 5) != b {
		t.Errorf("pixels wrong")
	}
}

func TestFitWithin(t *testing.T) {
	got := fitWithin(solid(1000, 500, color.NRGBA{1, 2, 3, 255}), 768)
	if got.Bounds().Dx() != 768 || got.Bounds().Dy() != 384 {
		t.Errorf("1000x500 -> %v", got.Bounds())
	}
	small := solid(100, 50, color.NRGBA{})
	if fitWithin(small, 768) != image.Image(small) {
		t.Errorf("small image must be returned unchanged")
	}
}

func TestLoadImageFile_Limits(t *testing.T) {
	dir := t.TempDir()
	ok := filepath.Join(dir, "ok.png")
	if err := os.WriteFile(ok, pngBytes(t, solid(10, 10, color.NRGBA{9, 9, 9, 255})), 0o644); err != nil {
		t.Fatal(err)
	}
	img, format, err := loadImageFile(ok, 1<<20, 4096, 1<<24)
	if err != nil || format != "png" || img.Bounds().Dx() != 10 {
		t.Fatalf("ok: %v %q %v", img, format, err)
	}
	if _, _, err := loadImageFile(ok, 10, 4096, 1<<24); err == nil || !strings.Contains(err.Error(), "image file too large") {
		t.Errorf("bytes limit: %v", err)
	}
	wide := filepath.Join(dir, "wide.png")
	if err := os.WriteFile(wide, pngBytes(t, solid(20, 10, color.NRGBA{})), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := loadImageFile(wide, 1<<20, 16, 1<<24); err == nil || err.Error() != "image too large: 20x10 (max 16 px per side)" {
		t.Errorf("side limit: %v", err)
	}
	if _, _, err := loadImageFile(wide, 1<<20, 4096, 100); err == nil || !strings.Contains(err.Error(), "pixels") {
		t.Errorf("pixel limit: %v", err)
	}
	txt := filepath.Join(dir, "x.png")
	if err := os.WriteFile(txt, []byte("TOPSECRET"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, _, err = loadImageFile(txt, 1<<20, 4096, 1<<24)
	if err == nil || err.Error() != "not a PNG or JPEG image" || strings.Contains(err.Error(), "TOPSECRET") {
		t.Errorf("text file: %v", err)
	}
}

func TestCheckTemplateCapacity(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < maxFilesPerTemplate; i++ {
		mustWrite(t, filepath.Join(dir, fmt.Sprintf("f%d.json", i)), "{}")
	}
	if err := checkTemplateCapacity(dir, "new.png", 1); err == nil || err.Error() != "too many files in template (max 64)" {
		t.Errorf("count: %v", err)
	}
	if err := checkTemplateCapacity(dir, "f3.json", 1); err != nil {
		t.Errorf("replace existing: %v", err)
	}
	if err := checkTemplateCapacity(dir, "f3.json", maxTemplateBytes+1); err == nil || err.Error() != "template would exceed 30 MiB" {
		t.Errorf("size: %v", err)
	}
}

func TestWriteAssetBytes_RefusesNonRegular(t *testing.T) {
	dir := copyTemplates(t, "sbb-105-v1")
	e, _ := newEnv(Config{TemplatesDir: dir})
	tdir := filepath.Join(dir, "sbb-105-v1")
	if err := os.Symlink(filepath.Join(tdir, "bg.png"), filepath.Join(tdir, "ln.png")); err != nil {
		t.Skip("no symlinks")
	}
	if _, err := writeAssetBytes(e, "sbb-105-v1", "ln.png", []byte("x")); err == nil || !strings.Contains(err.Error(), "refusing to overwrite non-regular file") {
		t.Errorf("symlink: %v", err)
	}
	if err := os.Mkdir(filepath.Join(tdir, "d.png"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := writeAssetBytes(e, "sbb-105-v1", "d.png", []byte("x")); err == nil || !strings.Contains(err.Error(), "refusing to overwrite non-regular file") {
		t.Errorf("dir: %v", err)
	}
	if _, err := writeAssetBytes(e, "sbb-105-v1", "../x.png", []byte("x")); err == nil {
		t.Errorf("traversal accepted")
	}
	if err := writePNGFile(e, "sbb-105-v1", "new.png", solid(4, 4, color.NRGBA{1, 1, 1, 255})); err != nil {
		t.Errorf("writePNGFile: %v", err)
	}
	if err := writePNGFile(e, "sbb-105-v1", "new.jpg", solid(4, 4, color.NRGBA{})); err == nil {
		t.Errorf("writePNGFile accepted .jpg")
	}
}
