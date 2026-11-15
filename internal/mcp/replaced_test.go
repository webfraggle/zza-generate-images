package mcp

import (
	"encoding/json"
	"image/color"
	"path/filepath"
	"strings"
	"testing"
)

// firstJSON decodes the first text content (the JSON result) of a tool result.
func firstJSON(t *testing.T, text string) map[string]any {
	t.Helper()
	if i := strings.Index(text, "\n"); i >= 0 {
		text = text[:i]
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(text), &m); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, text)
	}
	return m
}

// svg_to_png, photo tools and import_image report whether they replaced an
// existing file, like upload_image.
func TestReplacedReported(t *testing.T) {
	dir := copyTemplates(t, "sbb-105-v1")
	cs := newSession(t, dir)
	svg := map[string]any{"template": "sbb-105-v1", "name": "r.png", "width": 8, "height": 8,
		"svg": `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 8 8"><rect width="8" height="8"/></svg>`}
	crop := map[string]any{"template": "sbb-105-v1", "src": "r.png", "dst": "c.png", "x": 0, "y": 0, "width": 4, "height": 4}
	src := filepath.Join(t.TempDir(), "p.png")
	writeTestPNG(t, src, solid(10, 10, color.NRGBA{1, 2, 3, 255}))
	imp := map[string]any{"template": "sbb-105-v1", "path": src, "name": "i.png"}
	for _, c := range []struct {
		tool string
		args map[string]any
	}{{"svg_to_png", svg}, {"crop_image", crop}, {"import_image", imp}} {
		for i, want := range []bool{false, true} {
			res := callTool(t, cs, c.tool, c.args)
			requireOK(t, res)
			got, ok := firstJSON(t, resultText(res))["replaced"].(bool)
			if !ok || got != want {
				t.Errorf("%s call %d: replaced = %v (present %v), want %v", c.tool, i+1, got, ok, want)
			}
		}
	}
}
