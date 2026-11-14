package renderer

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReferencedPaths_ValuesOnly(t *testing.T) {
	src := []byte(`
# gleis
mode: 1
layers:
  - type: text
    value: "{{zug1.nr | lower}}"
    if: "not(isEmpty(zug1.hinweis))"
  - [ "{{abschnitt}}" ]
`)
	got, err := ReferencedPaths(src)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"zug1.nr", "zug1.hinweis", "lower", "abschnitt"} {
		if !got[want] {
			t.Errorf("missing %q in %v", want, got)
		}
	}
	for _, notWant := range []string{"gleis", "mode", "layers", "value", "if"} {
		if got[notWant] {
			t.Errorf("unexpected %q", notWant)
		}
	}
}

func TestReferencedPaths_InvalidYAML(t *testing.T) {
	if _, err := ReferencedPaths([]byte("a: [unclosed")); err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func TestReferencedPaths_AllTemplates(t *testing.T) {
	paths, err := filepath.Glob("../../templates/*/template.yaml")
	if err != nil || len(paths) == 0 {
		t.Fatalf("no templates found: %v", err)
	}
	for _, p := range paths {
		src, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		got, err := ReferencedPaths(src)
		if err != nil {
			t.Errorf("%s: %v", p, err)
			continue
		}
		if len(got) == 0 {
			t.Errorf("%s: no tokens", p)
		}
	}
}

func TestReferencedPaths_Faltblatt(t *testing.T) {
	src, err := os.ReadFile("../../templates/faltblatt-105-v1/template.yaml")
	if err != nil {
		t.Fatal(err)
	}
	got, err := ReferencedPaths(src)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"gleis", "zug1.vonnach", "zug1.nr", "zug1.zeit", "zug1.via"} {
		if !got[want] {
			t.Errorf("missing %q", want)
		}
	}
	if got["zug1.reihung"] {
		t.Error("zug1.reihung should not be referenced")
	}
}
