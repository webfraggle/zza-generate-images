package editor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// yamlEscapeStr escapes a string for use inside YAML double-quoted scalars.
// Only `\` and `"` need escaping in YAML double-quoted strings.
func yamlEscapeStr(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return s
}

// generateStarterYAML builds a starter template.yaml with the given meta fields.
// canvasW and canvasH define the image dimensions; layers are scaled accordingly.
func generateStarterYAML(name, description, display string, canvasW, canvasH int) []byte {
	halfH := canvasH / 2
	return []byte(fmt.Sprintf(`meta:
  name: "%s"
  description: "%s"
  author: ""
  version: "1.0"
  display: "%s"
  canvas:
    width: %d
    height: %d

layers:
  # Obere Hälfte
  - type: rect
    x: 0
    y: 0
    width: %d
    height: %d
    color: "#1a1a1a"

  # Untere Hälfte (Kopie der oberen)
  - type: copy
    src_x: 0
    src_y: 0
    src_width: %d
    src_height: %d
    x: 0
    y: %d
`,
		yamlEscapeStr(name),
		yamlEscapeStr(description),
		yamlEscapeStr(display),
		canvasW, canvasH,
		canvasW, halfH,
		canvasW, halfH,
		halfH,
	))
}

// CreateTemplate creates a new template directory seeded with a customised
// template.yaml and a starter default.json.
// Returns an error if the directory already exists (race-condition guard).
func CreateTemplate(templatesDir, templateName, name, description, display string, canvasW, canvasH int) error {
	dir, err := templateDir(templatesDir, templateName)
	if err != nil {
		return err
	}
	// os.Mkdir (not MkdirAll) fails atomically if dir exists — race guard.
	if err := os.Mkdir(dir, 0o755); err != nil {
		return fmt.Errorf("editor: template %q already exists or cannot be created: %w", templateName, err)
	}
	yamlData := generateStarterYAML(name, description, display, canvasW, canvasH)
	if err := os.WriteFile(filepath.Join(dir, "template.yaml"), yamlData, 0o644); err != nil {
		_ = os.RemoveAll(dir)
		return fmt.Errorf("editor: writing template.yaml: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "default.json"), starterDefaultJSON, 0o644); err != nil {
		_ = os.RemoveAll(dir)
		return fmt.Errorf("editor: writing default.json: %w", err)
	}
	return nil
}
