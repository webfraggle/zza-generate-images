package mcp

import (
	"bytes"
	"image"
	"image/color"
	"path/filepath"
	"strings"
	"testing"
)

func previewMeta(t *testing.T, text string) previewOut {
	t.Helper()
	var out previewOut
	parseJSONResult(t, nil, text, &out)
	return out
}

func TestRenderPreview_PNGDefault(t *testing.T) {
	cs := newSession(t, copyTemplates(t, "sbb-105-v1"))
	res := callTool(t, cs, "render_preview", map[string]any{"template": "sbb-105-v1"})
	requireOK(t, res)
	if b := resultImage(t, res).Bounds(); b.Dx() != 480 || b.Dy() != 480 {
		t.Errorf("image %v", b)
	}
	m := previewMeta(t, resultText(res))
	if m.Width != 240 || m.Height != 240 || m.Scale != 2 || m.Frames != 1 {
		t.Errorf("meta %+v", m)
	}
}

func TestRenderPreview_Scale(t *testing.T) {
	cs := newSession(t, copyTemplates(t, "sbb-105-v1"))
	for scale, want := range map[int]int{1: 240, 4: 960} {
		res := callTool(t, cs, "render_preview", map[string]any{"template": "sbb-105-v1", "scale": scale})
		requireOK(t, res)
		if b := resultImage(t, res).Bounds(); b.Dx() != want {
			t.Errorf("scale %d: width %d, want %d", scale, b.Dx(), want)
		}
	}
	requireToolError(t, callTool(t, cs, "render_preview", map[string]any{"template": "sbb-105-v1", "scale": 5}), "scale must be 1-4")
}

func imageBytes(t *testing.T, img image.Image) []byte {
	t.Helper()
	return pngBytes(t, img)
}

func TestRenderPreview_Data(t *testing.T) {
	cs := newSession(t, copyTemplates(t, "sbb-105-v1"))
	def := imageBytes(t, resultImage(t, callTool(t, cs, "render_preview", map[string]any{"template": "sbb-105-v1"})))
	other := imageBytes(t, resultImage(t, callTool(t, cs, "render_preview", map[string]any{
		"template": "sbb-105-v1", "data": map[string]any{"zug1": map[string]any{"vonnach": "X"}}})))
	if bytes.Equal(def, other) {
		t.Errorf("custom data produced the same image")
	}
}

func TestRenderPreview_DataTooLarge(t *testing.T) {
	cs := newSession(t, copyTemplates(t, "sbb-105-v1"))
	res := callTool(t, cs, "render_preview", map[string]any{"template": "sbb-105-v1", "data": map[string]any{"a": strings.Repeat("x", 70<<10)}})
	requireToolError(t, res, "data too large (max 64 KiB)")
}

func TestRenderPreview_GIFContactSheet(t *testing.T) {
	cs := newSession(t, copyTemplates(t, "faltblatt-105-v1"))
	res := callTool(t, cs, "render_preview", map[string]any{"template": "faltblatt-105-v1", "format": "gif"})
	requireOK(t, res)
	m := previewMeta(t, resultText(res))
	if m.Frames < 2 || m.Scale != 1 {
		t.Errorf("meta %+v", m)
	}
	if w := resultImage(t, res).Bounds().Dx(); w != 4*240+3*2 {
		t.Errorf("sheet width %d", w)
	}
}

func TestRenderPreview_GIFNotAnimated(t *testing.T) {
	cs := newSession(t, copyTemplates(t, "sbb-105-v1"))
	res := callTool(t, cs, "render_preview", map[string]any{"template": "sbb-105-v1", "format": "gif"})
	requireOK(t, res)
	if !strings.Contains(resultText(res), "template is not animated") {
		t.Errorf("no note: %s", resultText(res))
	}
	if b := resultImage(t, res).Bounds(); b.Dx() != 240 {
		t.Errorf("image %v", b)
	}
}

func TestRenderPreview_Colors(t *testing.T) {
	cs := newSession(t, copyTemplates(t, "nederland-105-v1"))
	res := callTool(t, cs, "render_preview", map[string]any{"template": "nederland-105-v1", "scale": 1})
	requireOK(t, res)
	img := resultImage(t, res)
	seen := map[color.NRGBA]bool{}
	for y := 0; y < img.Bounds().Dy(); y++ {
		for x := 0; x < img.Bounds().Dx(); x++ {
			seen[color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)] = true
		}
	}
	if len(seen) > 32 {
		t.Errorf("%d distinct colours, want <= 32", len(seen))
	}
}

func TestRenderPreview_RenderError(t *testing.T) {
	dir := copyTemplates(t, "sbb-105-v1")
	mustWrite(t, filepath.Join(dir, "sbb-105-v1", "template.yaml"),
		"meta:\n  canvas: {width: 240, height: 240}\nlayers:\n  - {type: image, file: missing.png, x: 0, y: 0}\n")
	cs := newSession(t, dir)
	res := callTool(t, cs, "render_preview", map[string]any{"template": "sbb-105-v1"})
	requireToolError(t, res, "render failed: ")
	if strings.Contains(resultText(res), dir) {
		t.Errorf("error leaks the templates path")
	}
}
