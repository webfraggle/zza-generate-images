package renderer

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestElseMarker_* tests verify that the ElseMarker type correctly handles
// the `else:` YAML key in a Layer context.
//
// Note: yaml.v3 does not call UnmarshalYAML for null nodes on named scalar
// types. Null handling (bare `else:` with no value) is therefore implemented
// in Layer.UnmarshalYAML, which scans the raw mapping node for a null "else"
// key and sets Else = true. These tests exercise that via Layer.

func TestElseMarker_UnmarshalNull(t *testing.T) {
	// `else:` with no value → YAML null → must parse as true.
	input := "type: rect\nelse:\n"
	var l Layer
	if err := yaml.Unmarshal([]byte(input), &l); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if !bool(l.Else) {
		t.Errorf("expected Else=true for null value, got false")
	}
}

func TestElseMarker_UnmarshalTrue(t *testing.T) {
	input := "type: rect\nelse: true\n"
	var l Layer
	if err := yaml.Unmarshal([]byte(input), &l); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if !bool(l.Else) {
		t.Errorf("expected Else=true, got false")
	}
}

func TestElseMarker_UnmarshalFalse(t *testing.T) {
	input := "type: rect\nelse: false\n"
	var l Layer
	if err := yaml.Unmarshal([]byte(input), &l); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if bool(l.Else) {
		t.Errorf("expected Else=false, got true")
	}
}

func TestLoadTemplate_ColorsValidation(t *testing.T) {
	dir := t.TempDir()
	write := func(yaml string) error {
		return os.WriteFile(filepath.Join(dir, "template.yaml"), []byte(yaml), 0644)
	}

	// colors: 0 (default, kein Feld) → valid
	if err := write("meta:\n  canvas:\n    width: 10\n    height: 10\n"); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadTemplate(filepath.Dir(dir), filepath.Base(dir)); err != nil {
		t.Errorf("colors omitted: unexpected error: %v", err)
	}

	// colors: 32 → valid
	if err := write("meta:\n  canvas:\n    width: 10\n    height: 10\n    colors: 32\n"); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadTemplate(filepath.Dir(dir), filepath.Base(dir)); err != nil {
		t.Errorf("colors: 32: unexpected error: %v", err)
	}

	// colors: 1 → invalid
	if err := write("meta:\n  canvas:\n    width: 10\n    height: 10\n    colors: 1\n"); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadTemplate(filepath.Dir(dir), filepath.Base(dir)); err == nil {
		t.Error("colors: 1: expected error, got nil")
	}

	// colors: 257 → invalid
	if err := write("meta:\n  canvas:\n    width: 10\n    height: 10\n    colors: 257\n"); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadTemplate(filepath.Dir(dir), filepath.Base(dir)); err == nil {
		t.Error("colors: 257: expected error, got nil")
	}
}
