package gallery

import (
	"log"
	"os"
	"path/filepath"
	"sort"

	"github.com/webfraggle/zza-generate-images/internal/renderer"
)

// TemplateInfo summarises a template for the gallery listing.
type TemplateInfo struct {
	Name       string
	Meta       renderer.Meta
	HasDefault bool // true when default.json exists
}

// ListTemplates reads all valid template directories under templatesDir and
// returns their metadata sorted by name. Broken or non-template directories
// are silently skipped.
func ListTemplates(templatesDir string) ([]TemplateInfo, error) {
	entries, err := os.ReadDir(templatesDir)
	if err != nil {
		return nil, err
	}

	var infos []TemplateInfo
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if err := renderer.ValidateTemplateName(name); err != nil {
			continue
		}
		tmpl, err := renderer.LoadTemplate(templatesDir, name)
		if err != nil {
			log.Printf("gallery: skipping template %q: %v", name, err)
			continue
		}
		defaultPath := filepath.Join(templatesDir, name, "default.json")
		_, statErr := os.Stat(defaultPath)
		infos = append(infos, TemplateInfo{
			Name:       name,
			Meta:       tmpl.Meta,
			HasDefault: statErr == nil,
		})
	}

	sort.Slice(infos, func(i, j int) bool {
		return infos[i].Name < infos[j].Name
	})
	return infos, nil
}

// LoadDefaultJSON reads default.json for the named template.
// The template name must already be validated by the caller.
func LoadDefaultJSON(templatesDir, name string) ([]byte, error) {
	path, err := renderer.SafeTemplatePath(templatesDir, name)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(filepath.Join(path, "default.json"))
}
