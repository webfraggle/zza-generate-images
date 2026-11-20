package mcp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/webfraggle/zza-generate-images/internal/renderer"
)

func TestSetMetaScalars(t *testing.T) {
	src := "# top comment\nmeta:\n  name: Old # keep\n  canvas: {width: 240, height: 240}\nlayers: []\n"
	out, err := setMetaScalars([]byte(src), map[string]string{"name": "A: \"tricky\"\n# not a comment", "description": "D"})
	if err != nil {
		t.Fatal(err)
	}
	tmpl, err := renderer.ParseTemplate(out, "/x")
	if err != nil {
		t.Fatalf("result invalid: %v\n%s", err, out)
	}
	if tmpl.Meta.Name != "A: \"tricky\"\n# not a comment" || tmpl.Meta.Description != "D" || tmpl.Meta.Canvas.Width != 240 {
		t.Errorf("meta = %+v", tmpl.Meta)
	}
	if !strings.Contains(string(out), "# top comment") {
		t.Errorf("comment lost:\n%s", out)
	}
	if _, err := setMetaScalars([]byte("- a\n"), map[string]string{"name": "x"}); err == nil {
		t.Errorf("non-mapping accepted")
	}
}

// Only the meta.name / meta.description lines change; blank lines, comments
// and everything else of the starter stay byte-identical.
func TestSetMetaScalars_StarterTextual(t *testing.T) {
	starter, err := os.ReadFile(filepath.Join("..", "editor", "starter", "template.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	out, err := setMetaScalars(starter, map[string]string{"name": "Berlin: \"S-Bahn\"\nx", "description": "Kurz & gut"})
	if err != nil {
		t.Fatal(err)
	}
	a, b := strings.Split(string(starter), "\n"), strings.Split(string(out), "\n")
	if len(a) != len(b) {
		t.Fatalf("line count %d -> %d:\n%s", len(a), len(b), out)
	}
	for i := range a {
		if a[i] == b[i] {
			continue
		}
		if !strings.HasPrefix(a[i], "  name:") && !strings.HasPrefix(a[i], "  description:") {
			t.Errorf("line %d changed: %q -> %q", i+1, a[i], b[i])
		}
	}
	tmpl, err := renderer.ParseTemplate(out, "/x")
	if err != nil {
		t.Fatal(err)
	}
	if tmpl.Meta.Name != "Berlin: \"S-Bahn\"\nx" || tmpl.Meta.Description != "Kurz & gut" {
		t.Errorf("meta = %+v", tmpl.Meta)
	}
}

// A missing key is inserted as a line into the meta block.
func TestSetMetaScalars_InsertsMissingKey(t *testing.T) {
	src := "meta:\n  name: A\n\n  canvas: {width: 240, height: 240}\n# c\nlayers: []\n"
	out, err := setMetaScalars([]byte(src), map[string]string{"name": "B", "description": "D"})
	if err != nil {
		t.Fatal(err)
	}
	want := "meta:\n  name: \"B\"\n  description: \"D\"\n\n  canvas: {width: 240, height: 240}\n# c\nlayers: []\n"
	if string(out) != want {
		t.Errorf("got\n%s\nwant\n%s", out, want)
	}
}

// Values the line edit cannot handle fall back to re-encoding.
func TestSetMetaScalars_Fallbacks(t *testing.T) {
	for _, src := range []string{
		"meta:\n  name: a\n    b\n  canvas: {width: 240, height: 240}\nlayers: []\n",
		"meta:\n  name: |\n    a\n  canvas: {width: 240, height: 240}\nlayers: []\n",
		"meta: {name: a, canvas: {width: 240, height: 240}}\nlayers: []\n",
	} {
		out, err := setMetaScalars([]byte(src), map[string]string{"name": "N"})
		if err != nil {
			t.Fatal(err)
		}
		tmpl, err := renderer.ParseTemplate(out, "/x")
		if err != nil || tmpl.Meta.Name != "N" || tmpl.Meta.Canvas.Width != 240 {
			t.Errorf("%q -> %q (%v)", src, out, err)
		}
	}
}
