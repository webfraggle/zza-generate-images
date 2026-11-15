package mcp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/webfraggle/zza-generate-images/docs"
	"github.com/webfraggle/zza-generate-images/internal/editor"
)

func TestGetTemplateSpec(t *testing.T) {
	cs := newSession(t, copyTemplates(t))
	res := callTool(t, cs, "get_template_spec", nil)
	requireOK(t, res)
	if got := resultText(res); got != docs.TemplateSpec {
		t.Errorf("text differs from docs.TemplateSpec (len %d vs %d)", len(got), len(docs.TemplateSpec))
	}
}

func TestGetDesignGuide(t *testing.T) {
	cs := newSession(t, copyTemplates(t))
	res := callTool(t, cs, "get_design_guide", nil)
	requireOK(t, res)
	if got := resultText(res); got != docs.DesignGuide {
		t.Errorf("text differs from docs.DesignGuide (len %d vs %d)", len(got), len(docs.DesignGuide))
	}
}

func TestListTemplates(t *testing.T) {
	dir := copyTemplates(t, "sbb-105-v1", "faltblatt-105-v1")
	mustWrite(t, filepath.Join(dir, "broken-105-v1", "template.yaml"), "layers: [::")
	mustWrite(t, filepath.Join(dir, "Not_Valid", "template.yaml"), "layers: []")
	if err := os.MkdirAll(filepath.Join(dir, "no-yaml"), 0o755); err != nil {
		t.Fatal(err)
	}
	cs := newSession(t, dir)
	res := callTool(t, cs, "list_templates", nil)
	requireOK(t, res)
	var out struct {
		Templates []templateSummary `json:"templates"`
	}
	if err := json.Unmarshal([]byte(resultText(res)), &out); err != nil {
		t.Fatalf("result is not JSON: %v\n%s", err, resultText(res))
	}
	if len(out.Templates) != 3 {
		t.Fatalf("got %d templates, want 3: %+v", len(out.Templates), out.Templates)
	}
	b, f, s := out.Templates[0], out.Templates[1], out.Templates[2]
	if b.Name != "broken-105-v1" || b.Error == "" {
		t.Errorf("broken entry: %+v", b)
	}
	if f.Name != "faltblatt-105-v1" || !f.Animated || f.Width != 240 || f.Height != 240 || !f.HasDefault || f.Error != "" {
		t.Errorf("faltblatt entry: %+v", f)
	}
	if s.Name != "sbb-105-v1" || s.Animated || !s.HasDefault || s.Error != "" {
		t.Errorf("sbb entry: %+v", s)
	}
}

func TestReadTemplate(t *testing.T) {
	dir := copyTemplates(t, "sbb-105-v1")
	cs := newSession(t, dir)
	res := callTool(t, cs, "read_template", map[string]any{"template": "sbb-105-v1"})
	requireOK(t, res)
	var out struct {
		Template     string            `json:"template"`
		TemplateYAML string            `json:"template_yaml"`
		DefaultJSON  string            `json:"default_json"`
		Files        []editor.FileInfo `json:"files"`
		Versions     map[string]string `json:"versions"`
	}
	if err := json.Unmarshal([]byte(resultText(res)), &out); err != nil {
		t.Fatalf("result is not JSON: %v", err)
	}
	yaml, _ := os.ReadFile(filepath.Join(dir, "sbb-105-v1", "template.yaml"))
	def, _ := os.ReadFile(filepath.Join(dir, "sbb-105-v1", "default.json"))
	if out.Template != "sbb-105-v1" || out.TemplateYAML != string(yaml) || out.DefaultJSON != string(def) {
		t.Errorf("content mismatch")
	}
	if out.Versions["template.yaml"] != editor.FileVersion(yaml) || out.Versions["default.json"] != editor.FileVersion(def) {
		t.Errorf("versions: %v", out.Versions)
	}
	found := false
	for _, f := range out.Files {
		if f.Name == "bg.png" {
			found = true
			if f.Editable {
				t.Errorf("bg.png must not be editable")
			}
		}
	}
	if !found {
		t.Errorf("bg.png missing from files: %+v", out.Files)
	}
}

func TestReadTemplate_NotFound(t *testing.T) {
	cs := newSession(t, copyTemplates(t))
	res := callTool(t, cs, "read_template", map[string]any{"template": "nope-105-v1"})
	requireToolError(t, res, `template "nope-105-v1" not found`)
}

func TestReadTemplate_InvalidName(t *testing.T) {
	cs := newSession(t, copyTemplates(t))
	res := callTool(t, cs, "read_template", map[string]any{"template": "../etc"})
	requireToolError(t, res, "invalid characters")
}

func TestReadFile(t *testing.T) {
	dir := copyTemplates(t, "sbb-105-v1")
	cs := newSession(t, dir)
	res := callTool(t, cs, "read_file", map[string]any{"template": "sbb-105-v1", "name": "default.json"})
	requireOK(t, res)
	var out struct{ Name, Content, Version string }
	if err := json.Unmarshal([]byte(resultText(res)), &out); err != nil {
		t.Fatal(err)
	}
	def, _ := os.ReadFile(filepath.Join(dir, "sbb-105-v1", "default.json"))
	if out.Name != "default.json" || out.Content != string(def) || out.Version != editor.FileVersion(def) {
		t.Errorf("read_file mismatch: name=%q version=%q", out.Name, out.Version)
	}

	res = callTool(t, cs, "read_file", map[string]any{"template": "sbb-105-v1", "name": "bg.png"})
	requireToolError(t, res, "only .yaml and .json files can be read as text")

	res = callTool(t, cs, "read_file", map[string]any{"template": "sbb-105-v1", "name": "missing.json"})
	requireToolError(t, res, "not found")
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
