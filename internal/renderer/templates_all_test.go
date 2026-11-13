package renderer

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestAllTemplates_Render is a safety net: every shipped template must load
// and render (static and animated) with its default.json.
func TestAllTemplates_Render(t *testing.T) {
	dirs, err := filepath.Glob("../../templates/*/template.yaml")
	if err != nil || len(dirs) == 0 {
		t.Fatalf("no templates found: %v", err)
	}
	for _, yamlPath := range dirs {
		dir := filepath.Dir(yamlPath)
		name := filepath.Base(dir)
		t.Run(name, func(t *testing.T) {
			data := map[string]interface{}{}
			body := []byte("{}")
			if b, err := os.ReadFile(filepath.Join(dir, "default.json")); err == nil {
				body = b
				if err := json.Unmarshal(b, &data); err != nil {
					t.Fatalf("default.json: %v", err)
				}
			}
			tmpl, err := LoadTemplate("../../templates", name)
			if err != nil {
				t.Fatalf("LoadTemplate: %v", err)
			}
			r := New("../../templates")
			if _, err := r.Render(tmpl, data); err != nil {
				t.Fatalf("Render: %v", err)
			}
			anim, err := r.RenderAnimation(tmpl, data, AnimationSeed(name, body))
			if err != nil {
				t.Fatalf("RenderAnimation: %v", err)
			}
			if len(anim.Frames) < 1 {
				t.Fatalf("no frames")
			}
		})
	}
}
