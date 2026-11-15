package mcp

import (
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
