package mcp

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func sbbEnv(t *testing.T) (*env, string) {
	t.Helper()
	dir := copyTemplates(t, "sbb-105-v1")
	e, err := newEnv(Config{TemplatesDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dir, "sbb-105-v1", "template.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	return e, string(b)
}

func TestValidateTemplateYAML_OK(t *testing.T) {
	e, src := sbbEnv(t)
	w, err := validateTemplateYAML(e, "sbb-105-v1", []byte(src))
	if err != nil || len(w) != 0 {
		t.Fatalf("warnings=%v err=%v", w, err)
	}
}

func TestValidateTemplateYAML_WrongCanvas(t *testing.T) {
	e, src := sbbEnv(t)
	src = strings.Replace(src, "width: 240\n    height: 240", "width: 296\n    height: 128", 1)
	_, err := validateTemplateYAML(e, "sbb-105-v1", []byte(src))
	want := `canvas must be 240x240 (only 1.05" displays are supported), got 296x128`
	if err == nil || err.Error() != want {
		t.Fatalf("err = %v, want %q", err, want)
	}
}

func TestValidateTemplateYAML_RenderError(t *testing.T) {
	e, _ := sbbEnv(t)
	src := "meta:\n  canvas: {width: 240, height: 240}\nlayers:\n  - {type: image, file: missing.png, x: 0, y: 0}\n"
	_, err := validateTemplateYAML(e, "sbb-105-v1", []byte(src))
	if err == nil || !strings.Contains(err.Error(), "missing.png") {
		t.Fatalf("err = %v", err)
	}
}

func TestValidateTemplateYAML_Warnings(t *testing.T) {
	e, _ := sbbEnv(t)
	src := `meta:
  canvas: {width: 240, height: 240}
fonts:
  - {id: bold, file: NimbusSanL-Bol.otf}
layers:
  - {type: rect, x: 0, y: 0, width: 240, height: 120, color: "#123456"}
  - {type: text, x: 5, y: 10, font: bold, size: 8, color: "#ffffff", value: "hello"}
  - {type: text, x: 5, y: 40, font: bold, size: 20, color: "#ffffff", value: "15:54"}
`
	w, err := validateTemplateYAML(e, "sbb-105-v1", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"bottom half (y 120-239) is empty — the back side of the display shows nothing; rebuild it with copy layers (see get_design_guide)",
		`layers[1] (text): size 8 px — below 10 px is hard to read on the 1.05" display`,
		`layers[2] (text): static text "15:54" looks like train data — use {{zug1.…}} placeholders`,
	}
	if !reflect.DeepEqual(w, want) {
		t.Errorf("warnings:\n got %q\nwant %q", w, want)
	}
}

func TestValidateTemplateYAML_NestedPath(t *testing.T) {
	e, _ := sbbEnv(t)
	src := `meta:
  canvas: {width: 240, height: 240}
fonts:
  - {id: bold, file: NimbusSanL-Bol.otf}
layers:
  - {type: rect, x: 0, y: 0, width: 240, height: 240, color: "#123456"}
  - type: loop
    value: "a,b"
    split_by: ","
    var: i
    layers:
      - {type: text, x: 5, y: 10, font: bold, size: 6, color: "#ffffff", value: "{{i}}"}
`
	w, err := validateTemplateYAML(e, "sbb-105-v1", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if len(w) != 1 || !strings.Contains(w[0], "layers[1].layers[0]") {
		t.Errorf("warnings = %q", w)
	}
}

func TestValidateJSONObject(t *testing.T) {
	if err := validateJSONObject([]byte(`{"a":1}`)); err != nil {
		t.Errorf("object: %v", err)
	}
	if err := validateJSONObject([]byte(`[1]`)); err == nil || err.Error() != "invalid JSON: must be an object" {
		t.Errorf("array: %v", err)
	}
	if err := validateJSONObject([]byte(`{`)); err == nil || !strings.HasPrefix(err.Error(), "invalid JSON: ") {
		t.Errorf("syntax: %v", err)
	}
}

func TestLoadTestData(t *testing.T) {
	dir := t.TempDir()
	if m := loadTestData(dir); m == nil || len(m) != 0 {
		t.Errorf("missing: %v", m)
	}
	mustWrite(t, filepath.Join(dir, "default.json"), `{"a":1}`)
	if m := loadTestData(dir); m["a"] != float64(1) {
		t.Errorf("valid: %v", m)
	}
	mustWrite(t, filepath.Join(dir, "default.json"), `[`)
	if m := loadTestData(dir); len(m) != 0 {
		t.Errorf("invalid: %v", m)
	}
}
