package mcp

import (
	"fmt"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDeviceWarnings_ShippedTemplatesFit(t *testing.T) {
	dir := copyTemplates(t, "sbb-105-v1", "faltblatt-105-v1")
	e, err := newEnv(Config{TemplatesDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"sbb-105-v1", "faltblatt-105-v1"} {
		if w := deviceWarnings(e, name); len(w) != 0 {
			t.Errorf("%s: unexpected device warnings: %v", name, w)
		}
	}
}

// manyImagesTemplate adds n different full-canvas image layers. The exporter
// scales images to their drawn size, so each costs about 225 KB of core
// memory on the device; six of them exceed the QUAD budget of 1.5 MiB.
func manyImagesTemplate(t *testing.T, e *env, dir string, n int) string {
	t.Helper()
	layers := "layers:\n"
	for k := 0; k < n; k++ {
		img := image.NewNRGBA(image.Rect(0, 0, 240, 240))
		for i := range img.Pix {
			img.Pix[i] = byte(i*7 + k*31)
		}
		img.Set(0, 0, color.NRGBA{byte(k), 2, 3, 255})
		name := fmt.Sprintf("big%d.png", k)
		if _, err := writePNGFile(e, "sbb-105-v1", name, img); err != nil {
			t.Fatal(err)
		}
		layers += fmt.Sprintf("  - type: image\n    file: %s\n    x: 0\n    y: 0\n", name)
	}
	src, err := os.ReadFile(filepath.Join(dir, "sbb-105-v1", "template.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	out := strings.Replace(string(src), "layers:\n", layers, 1)
	if out == string(src) {
		t.Fatal("no layers: in sbb template")
	}
	return out
}

func TestDeviceWarnings_MemoryOverQUADBudget(t *testing.T) {
	dir := copyTemplates(t, "sbb-105-v1")
	e, err := newEnv(Config{TemplatesDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	src := manyImagesTemplate(t, e, dir, 6)
	if err := os.WriteFile(filepath.Join(dir, "sbb-105-v1", "template.yaml"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	w := deviceWarnings(e, "sbb-105-v1")
	if len(w) == 0 {
		t.Fatal("want a memory warning")
	}
	joined := strings.Join(w, "\n")
	for _, want := range []string{"memory", "1.5 MiB", "built-in layout"} {
		if !strings.Contains(joined, want) {
			t.Errorf("warning lacks %q:\n%s", want, joined)
		}
	}
}

func TestDeviceWarnings_NotCompatible(t *testing.T) {
	dir := copyTemplates(t, "sbb-105-v1")
	e, err := newEnv(Config{TemplatesDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, "sbb-105-v1", "template.yaml")
	src, _ := os.ReadFile(p)
	// A drawn constant outside Latin-1 (written past the MCP check on purpose)
	out := strings.Replace(string(src), "layers:\n", "layers:\n  - type: text\n    value: \"\u2192\"\n    x: 1\n    y: 1\n    font: bold\n    size: 12\n    color: \"#ffffff\"\n", 1)
	if err := os.WriteFile(p, []byte(out), 0o644); err != nil {
		t.Fatal(err)
	}
	w := strings.Join(deviceWarnings(e, "sbb-105-v1"), "\n")
	if !strings.Contains(w, "not ZZA-compatible") {
		t.Errorf("want a compatibility warning, got:\n%s", w)
	}
}

func TestWriteFile_DeviceWarning(t *testing.T) {
	dir := copyTemplates(t, "sbb-105-v1")
	cs := newSession(t, dir)
	e, err := newEnv(Config{TemplatesDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	src := manyImagesTemplate(t, e, dir, 6)
	res := callTool(t, cs, "write_file", map[string]any{"template": "sbb-105-v1", "name": "template.yaml", "content": src})
	requireOK(t, res)
	var out writeFileOut
	parseJSONResult(t, res, resultText(res), &out)
	if !strings.Contains(strings.Join(out.Warnings, "\n"), "1.5 MiB") {
		t.Errorf("write_file must warn about device memory, warnings: %v", out.Warnings)
	}
}

func TestDeviceWarnings_WorkOverLimit(t *testing.T) {
	// worst-layers (device stress template) costs 23.7 M operations per still.
	src := filepath.Join("..", "..", "testdata", "devtemplates", "worst-layers")
	dir := filepath.Join(t.TempDir(), "templates")
	dst := filepath.Join(dir, "worst-layers")
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range entries {
		b, err := os.ReadFile(filepath.Join(src, f.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dst, f.Name()), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	e, err := newEnv(Config{TemplatesDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	w := strings.Join(deviceWarnings(e, "worst-layers"), "\n")
	if !strings.Contains(w, "M operations") || !strings.Contains(w, "15 M") {
		t.Errorf("want a work-limit warning, got:\n%s", w)
	}
}
