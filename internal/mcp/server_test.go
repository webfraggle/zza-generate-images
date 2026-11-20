package mcp

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewServer_ListsReadTools(t *testing.T) {
	cs := newSession(t, copyTemplates(t, "sbb-105-v1"))
	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	have := map[string]bool{}
	for _, tool := range res.Tools {
		have[tool.Name] = true
		if strings.TrimSpace(tool.Description) == "" {
			t.Errorf("tool %s has empty description", tool.Name)
		}
	}
	for _, want := range []string{"get_template_spec", "get_design_guide", "list_templates", "read_template", "read_file", "create_template", "write_file", "upload_image", "import_image", "render_preview"} {
		if !have[want] {
			t.Errorf("missing tool %s (have %v)", want, have)
		}
	}
}

func TestNewServer_Instructions(t *testing.T) {
	cs := newSession(t, copyTemplates(t))
	got := cs.InitializeResult().Instructions
	if got != serverInstructions {
		t.Errorf("instructions differ from serverInstructions")
	}
	for _, want := range []string{"240x240", "-105-v1", "get_design_guide",
		"list_fonts -> add_font", "text layers need a font", "the starter has none",
		"svg_to_png", "import_image", "perspective_correct", "sample_colors", "crop_image",
		"color_to_alpha", "quantize_colors", "must exist before write_file",
		"Latin-1", "1.5 MiB", "15 M operations", `"device:"`} {
		if !strings.Contains(got, want) {
			t.Errorf("instructions lack %q", want)
		}
	}
	if len(got) > 2000 {
		t.Errorf("instructions too long: %d chars", len(got))
	}
}

func TestNewServer_DefaultFontsDir(t *testing.T) {
	e, err := newEnv(Config{TemplatesDir: "/x/templates"})
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join("/x", "fonts"); e.fontsDir != want {
		t.Errorf("fontsDir = %q, want %q", e.fontsDir, want)
	}
	e, err = newEnv(Config{TemplatesDir: "/x/templates", FontsDir: "/y/f"})
	if err != nil {
		t.Fatal(err)
	}
	if e.fontsDir != "/y/f" {
		t.Errorf("explicit fontsDir = %q", e.fontsDir)
	}
}
