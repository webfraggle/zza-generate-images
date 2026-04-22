package server

import (
	"archive/zip"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/webfraggle/zza-generate-images/internal/renderer"
)

// handleTemplateZip streams the template directory (template.yaml + default.json
// + all asset files) as a ZIP archive. Directories are not recursed.
func (s *Server) handleTemplateZip(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("template")
	if err := renderer.ValidateTemplateName(name); err != nil {
		http.Error(w, "invalid template name", http.StatusBadRequest)
		return
	}
	dir := filepath.Join(s.templatesDir, name)
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		http.Error(w, "template not found", http.StatusNotFound)
		return
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		log.Printf("zip: read dir %q: %v", name, err)
		http.Error(w, "could not read template", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.zip"`, name))

	zw := zip.NewWriter(w)
	defer zw.Close()

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if err := addFileToZip(zw, dir, e.Name()); err != nil {
			log.Printf("zip: add %q: %v", e.Name(), err)
			return
		}
	}
}

func addFileToZip(zw *zip.Writer, dir, name string) error {
	src, err := os.Open(filepath.Join(dir, name))
	if err != nil {
		return err
	}
	defer src.Close()
	header, err := zw.Create(name)
	if err != nil {
		return err
	}
	_, err = io.Copy(header, src)
	return err
}
