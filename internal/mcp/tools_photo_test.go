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
	"time"

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

func TestPhotoTools_DstCaseInsensitiveSame(t *testing.T) {
	cs, _ := photoSession(t)
	res := callTool(t, cs, "crop_image", map[string]any{
		"template": photoTpl, "src": "PHOTO.png", "dst": "photo.png", "x": 0, "y": 0, "width": 10, "height": 10,
	})
	requireToolError(t, res, "dst must differ from src")
}

func TestPhotoTools_DstSameFileViaLink(t *testing.T) {
	cs, dir := photoSession(t)
	if err := os.Link(filepath.Join(dir, "photo.png"), filepath.Join(dir, "alias.png")); err != nil {
		t.Skip("hard links unsupported")
	}
	res := callTool(t, cs, "crop_image", map[string]any{
		"template": photoTpl, "src": "photo.png", "dst": "alias.png", "x": 0, "y": 0, "width": 10, "height": 10,
	})
	requireToolError(t, res, "dst must differ from src")
}

func TestPhotoTools_OutputLimit(t *testing.T) {
	cs, dir := photoSession(t)
	wide := image.NewNRGBA(image.Rect(0, 0, 3000, 100))
	for i := range wide.Pix {
		wide.Pix[i] = uint8(i * 7)
	}
	writeTestPNG(t, filepath.Join(dir, "wide.png"), wide)
	for _, tool := range []string{"color_to_alpha", "quantize_colors"} {
		args := map[string]any{"template": photoTpl, "src": "wide.png", "dst": tool + ".png"}
		if tool == "color_to_alpha" {
			args["color"] = "#ffffff"
		} else {
			args["colors"] = 8
		}
		res := callTool(t, cs, tool, args)
		requireOK(t, res)
		if b := loadOut(t, filepath.Join(dir, tool+".png")).Bounds(); b.Dx() > 2048 || b.Dy() > 2048 {
			t.Fatalf("%s: size %v", tool, b)
		}
		var out photoOut
		if err := json.Unmarshal([]byte(resultText(res)), &out); err != nil || out.Note == "" || out.Width != 2048 {
			t.Fatalf("%s: out %+v %v", tool, out, err)
		}
	}
}

func TestQuantizeColors_LargeInputFast(t *testing.T) {
	cs, dir := photoSession(t)
	big := image.NewNRGBA(image.Rect(0, 0, 6000, 4000))
	for i := range big.Pix {
		big.Pix[i] = uint8(i * 13)
	}
	writeTestPNG(t, filepath.Join(dir, "big.png"), big)
	start := time.Now()
	res := callTool(t, cs, "quantize_colors", map[string]any{"template": photoTpl, "src": "big.png", "dst": "bq.png", "colors": 16})
	requireOK(t, res)
	if d := time.Since(start); d > 30*time.Second {
		t.Fatalf("took %v", d)
	}
}

func TestQuantizeColors_KeepsTransparent(t *testing.T) {
	cs, dir := photoSession(t)
	img := image.NewNRGBA(image.Rect(0, 0, 20, 20))
	for y := 10; y < 20; y++ {
		for x := 0; x < 20; x++ {
			img.SetNRGBA(x, y, color.NRGBA{200, 30, 30, 255})
		}
	}
	writeTestPNG(t, filepath.Join(dir, "t.png"), img)
	res := callTool(t, cs, "quantize_colors", map[string]any{"template": photoTpl, "src": "t.png", "dst": "tq.png", "colors": 4})
	requireOK(t, res)
	out := loadOut(t, filepath.Join(dir, "tq.png"))
	if _, _, _, a := out.At(5, 2).RGBA(); a != 0 {
		t.Fatalf("transparent pixel became alpha %d", a)
	}
	if _, _, _, a := out.At(5, 15).RGBA(); a == 0 {
		t.Fatal("opaque pixel became transparent")
	}
}

func TestPerspectiveCorrect_BadCorners(t *testing.T) {
	cs, _ := photoSession(t)
	res := callTool(t, cs, "perspective_correct", map[string]any{
		"template": photoTpl, "src": "photo.png", "dst": "ccw.png",
		"corners": []map[string]any{{"x": 100, "y": 100}, {"x": 100, "y": 199}, {"x": 199, "y": 199}, {"x": 199, "y": 100}},
		"width":   20, "height": 20,
	})
	requireToolError(t, res, "top-left, top-right, bottom-right, bottom-left")
	res = callTool(t, cs, "perspective_correct", map[string]any{
		"template": photoTpl, "src": "photo.png", "dst": "huge.png",
		"corners": []map[string]any{{"x": 0, "y": 0}, {"x": 1e9, "y": 0}, {"x": 1e9, "y": 10}, {"x": 0, "y": 10}},
		"width":   20, "height": 20,
	})
	requireToolError(t, res, "corners")
}

func TestSampleColors_AlphaAndFloatRange(t *testing.T) {
	cs, dir := photoSession(t)
	img := image.NewNRGBA(image.Rect(0, 0, 2, 1))
	img.SetNRGBA(0, 0, color.NRGBA{255, 0, 0, 255})
	writeTestPNG(t, filepath.Join(dir, "al.png"), img)
	res := callTool(t, cs, "sample_colors", map[string]any{
		"template": photoTpl, "src": "al.png", "points": []map[string]any{{"x": 0, "y": 0}}, "radius": 1,
	})
	requireOK(t, res)
	var out sampleOut
	if err := json.Unmarshal([]byte(resultText(res)), &out); err != nil {
		t.Fatal(err)
	}
	if out.Colors[0].Hex != "#ff0000" || out.Colors[0].Alpha != 128 {
		t.Fatalf("got %+v", out.Colors[0])
	}
	for _, x := range []float64{-0.5, 1e300} {
		res = callTool(t, cs, "sample_colors", map[string]any{
			"template": photoTpl, "src": "al.png", "points": []map[string]any{{"x": x, "y": 0}},
		})
		requireToolError(t, res, "outside the image")
	}
}
