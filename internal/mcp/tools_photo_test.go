package mcp

import (
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

const photoTpl = "sbb-105-v1"

func writeTestPNG(t *testing.T, path string, img image.Image) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}

// photoSession returns a session with a copy of sbb-105-v1 containing photo.png
// (400x300, white with a red square at 100..200 x 100..200 and a noisy gradient).
func photoSession(t *testing.T) (*sdk.ClientSession, string) {
	t.Helper()
	dir := copyTemplates(t, photoTpl)
	img := image.NewNRGBA(image.Rect(0, 0, 400, 300))
	for y := 0; y < 300; y++ {
		for x := 0; x < 400; x++ {
			c := color.NRGBA{255, 255, 255, 255}
			if x >= 100 && x < 200 && y >= 100 && y < 200 {
				c = color.NRGBA{uint8(200 + (x*7+y*3)%50), uint8((x + y) % 40), uint8((x * y) % 30), 255}
			}
			img.SetNRGBA(x, y, c)
		}
	}
	writeTestPNG(t, filepath.Join(dir, photoTpl, "photo.png"), img)
	return newSession(t, dir), filepath.Join(dir, photoTpl)
}

func loadOut(t *testing.T, path string) image.Image {
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

func TestPerspectiveCorrect_Tool(t *testing.T) {
	cs, dir := photoSession(t)
	res := callTool(t, cs, "perspective_correct", map[string]any{
		"template": photoTpl, "src": "photo.png", "dst": "straight.png",
		"corners": []map[string]any{{"x": 100, "y": 100}, {"x": 199, "y": 100}, {"x": 199, "y": 199}, {"x": 100, "y": 199}},
		"width":   240, "height": 120,
	})
	requireOK(t, res)
	img := loadOut(t, filepath.Join(dir, "straight.png"))
	if img.Bounds().Dx() != 240 || img.Bounds().Dy() != 120 {
		t.Fatalf("size %v", img.Bounds())
	}
	resultImage(t, res)
	var out photoOut
	if err := json.Unmarshal([]byte(resultText(res)), &out); err != nil || out.Name != "straight.png" || out.Width != 240 || out.Height != 120 {
		t.Fatalf("out %+v %v", out, err)
	}

	res = callTool(t, cs, "perspective_correct", map[string]any{
		"template": photoTpl, "src": "photo.png", "dst": "s2.png",
		"corners": []map[string]any{{"x": 0, "y": 0}, {"x": 9, "y": 0}, {"x": 9, "y": 9}},
		"width":   10, "height": 10,
	})
	requireToolError(t, res, "corners must contain exactly 4 points")
}

func TestCropImage_Tool(t *testing.T) {
	cs, dir := photoSession(t)
	res := callTool(t, cs, "crop_image", map[string]any{
		"template": photoTpl, "src": "photo.png", "dst": "c.png", "x": 10, "y": 10, "width": 50, "height": 40,
	})
	requireOK(t, res)
	if b := loadOut(t, filepath.Join(dir, "c.png")).Bounds(); b.Dx() != 50 || b.Dy() != 40 {
		t.Fatalf("size %v", b)
	}
	res = callTool(t, cs, "crop_image", map[string]any{
		"template": photoTpl, "src": "photo.png", "dst": "c2.png", "x": 390, "y": 0, "width": 20, "height": 10,
	})
	requireToolError(t, res, "crop rectangle exceeds image bounds 0,0–400,300")
}

func TestResizeImage_Tool(t *testing.T) {
	cs, dir := photoSession(t)
	res := callTool(t, cs, "resize_image", map[string]any{
		"template": photoTpl, "src": "photo.png", "dst": "r.png", "width": 40, "height": 30, "mode": "nearest",
	})
	requireOK(t, res)
	if b := loadOut(t, filepath.Join(dir, "r.png")).Bounds(); b.Dx() != 40 || b.Dy() != 30 {
		t.Fatalf("size %v", b)
	}
	res = callTool(t, cs, "resize_image", map[string]any{
		"template": photoTpl, "src": "photo.png", "dst": "r2.png", "width": 40, "height": 30, "mode": "smooth",
	})
	requireOK(t, res)
	res = callTool(t, cs, "resize_image", map[string]any{
		"template": photoTpl, "src": "photo.png", "dst": "r3.png", "width": 40, "height": 30, "mode": "fancy",
	})
	requireToolError(t, res, `mode must be "nearest" or "smooth"`)
	res = callTool(t, cs, "resize_image", map[string]any{
		"template": photoTpl, "src": "photo.png", "dst": "r3.png", "width": 2049, "height": 30,
	})
	requireToolError(t, res, "width and height must be 1-2048")
}

func TestQuantizeColors_Tool(t *testing.T) {
	cs, dir := photoSession(t)
	res := callTool(t, cs, "quantize_colors", map[string]any{
		"template": photoTpl, "src": "photo.png", "dst": "q.png", "colors": 4,
	})
	requireOK(t, res)
	var out photoOut
	if err := json.Unmarshal([]byte(resultText(res)), &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Palette) == 0 || len(out.Palette) > 4 {
		t.Fatalf("palette %v", out.Palette)
	}
	re := regexp.MustCompile(`^#[0-9a-f]{6}$`)
	for _, h := range out.Palette {
		if !re.MatchString(h) {
			t.Fatalf("bad hex %q", h)
		}
	}
	img := loadOut(t, filepath.Join(dir, "q.png"))
	seen := map[color.NRGBA]bool{}
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			seen[color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)] = true
		}
	}
	if len(seen) > 4 {
		t.Fatalf("%d distinct colours", len(seen))
	}
	res = callTool(t, cs, "quantize_colors", map[string]any{
		"template": photoTpl, "src": "photo.png", "dst": "q2.png", "colors": 1,
	})
	requireToolError(t, res, "colors must be 2-64")
}

func TestColorToAlpha_Tool(t *testing.T) {
	cs, dir := photoSession(t)
	res := callTool(t, cs, "color_to_alpha", map[string]any{
		"template": photoTpl, "src": "photo.png", "dst": "a.png", "color": "#ffffff",
	})
	requireOK(t, res)
	img := loadOut(t, filepath.Join(dir, "a.png"))
	if _, _, _, a := img.At(0, 0).RGBA(); a != 0 {
		t.Fatalf("corner alpha = %d", a)
	}
	if _, _, _, a := img.At(150, 150).RGBA(); a == 0 {
		t.Fatal("red area must stay opaque")
	}
	res = callTool(t, cs, "color_to_alpha", map[string]any{
		"template": photoTpl, "src": "photo.png", "dst": "a2.png", "color": "white",
	})
	requireToolError(t, res, "color must be #rrggbb")
}

func TestSampleColors_Tool(t *testing.T) {
	cs, dir := photoSession(t)
	before, _ := os.ReadDir(dir)
	res := callTool(t, cs, "sample_colors", map[string]any{
		"template": photoTpl, "src": "photo.png", "points": []map[string]any{{"x": 5, "y": 5}},
	})
	requireOK(t, res)
	var out sampleOut
	if err := json.Unmarshal([]byte(resultText(res)), &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Colors) != 1 || out.Colors[0].Hex != "#ffffff" {
		t.Fatalf("out %+v", out)
	}
	if after, _ := os.ReadDir(dir); len(after) != len(before) {
		t.Fatal("sample_colors must not write files")
	}
	res = callTool(t, cs, "sample_colors", map[string]any{
		"template": photoTpl, "src": "photo.png", "points": []map[string]any{{"x": 400, "y": 0}},
	})
	requireToolError(t, res, "point 400,0 is outside the image 400x300")
	pts := make([]map[string]any, 65)
	for i := range pts {
		pts[i] = map[string]any{"x": 1, "y": 1}
	}
	res = callTool(t, cs, "sample_colors", map[string]any{"template": photoTpl, "src": "photo.png", "points": pts})
	requireToolError(t, res, "at most 64 points")
	res = callTool(t, cs, "sample_colors", map[string]any{
		"template": photoTpl, "src": "photo.png", "points": []map[string]any{{"x": 1, "y": 1}}, "radius": 6,
	})
	requireToolError(t, res, "radius must be 0-5")
}

func TestPhotoTools_Common(t *testing.T) {
	cs, dir := photoSession(t)
	base := func(src, dst string) map[string]any {
		return map[string]any{"template": photoTpl, "src": src, "dst": dst, "x": 0, "y": 0, "width": 10, "height": 10}
	}
	requireToolError(t, callTool(t, cs, "crop_image", base("photo.png", "photo.png")), "dst must differ from src — originals are kept")
	requireToolError(t, callTool(t, cs, "crop_image", base("photo.png", "x.jpg")), "dst must end in .png")
	requireToolError(t, callTool(t, cs, "crop_image", base("missing.png", "x.png")), "cannot read missing.png")
	requireToolError(t, callTool(t, cs, "crop_image", base("../photo.png", "x.png")), "invalid file name")
	requireToolError(t, callTool(t, cs, "crop_image", base("template.yaml", "x.png")), "src must be a")

	// text content with an image extension is rejected without leaking content
	if err := os.WriteFile(filepath.Join(dir, "fake.png"), []byte("SECRET-TEXT"), 0o644); err != nil {
		t.Fatal(err)
	}
	res := callTool(t, cs, "crop_image", base("fake.png", "x.png"))
	requireToolError(t, res, "not a PNG or JPEG")
	if got := resultText(res); regexp.MustCompile("SECRET").MatchString(got) {
		t.Fatalf("content leaked: %s", got)
	}

	writeTestPNG(t, filepath.Join(dir, "huge.png"), image.NewNRGBA(image.Rect(0, 0, 5001, 5001)))
	requireToolError(t, callTool(t, cs, "crop_image", base("huge.png", "x.png")), "image too large: 5001x5001 (max 25 megapixels)")
}
