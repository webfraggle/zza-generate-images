package renderer

import (
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
