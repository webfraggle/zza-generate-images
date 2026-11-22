package mcp

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteTools_ReservedAndHiddenNamesRejected(t *testing.T) {
	dir := copyTemplates(t, "sbb-105-v1")
	cs := newSession(t, dir)
	for _, n := range []string{"CON.json", "nul.yaml", ".json", "Aux.yaml"} {
		res := callTool(t, cs, "write_file", map[string]any{"template": "sbb-105-v1", "name": n, "content": "{}"})
		requireToolError(t, res, "invalid file name")
	}
	for _, n := range []string{"CON.png", ".png", "lpt1.png"} {
		res := callTool(t, cs, "svg_to_png", map[string]any{"template": "sbb-105-v1", "name": n, "width": 8, "height": 8,
			"svg": `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 8 8"><rect width="8" height="8"/></svg>`})
		requireToolError(t, res, "invalid file name")
	}
	for _, n := range []string{"CON.json", "nul.yaml", "CON.png", "lpt1.png"} {
		if _, err := os.Stat(filepath.Join(dir, "sbb-105-v1", n)); err == nil {
			t.Errorf("%s was written", n)
		}
	}
}
