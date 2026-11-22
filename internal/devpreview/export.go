package devpreview

import (
	"net/http"
	"strconv"

	"github.com/webfraggle/zza-generate-images/internal/devexport"
	"github.com/webfraggle/zza-generate-images/internal/renderer"
)

// ExportPattern is the mux pattern of the editor's "Für ZZA exportieren"
// download: the template as a ZZA controller package (same ZIP as `zza export`).
const ExportPattern = "GET /edit/{template}/zza-export"

// ExportHandler returns the handler for ExportPattern. It shares the preview's
// slot limit, since an export does the same work as the first half of a
// preview. Errors map like the preview (422 with the reasons when the template
// is not ZZA-compatible).
func (h *Handler) ExportHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("template")
		if err := renderer.ValidateTemplateName(name); err != nil {
			http.Error(w, "invalid template name", http.StatusBadRequest)
			return
		}
		if !h.acquire(w, r) {
			return
		}
		defer h.release()

		pkg, err := devexport.Export(h.templatesDir, name)
		if err != nil {
			h.fail(w, name, err)
			return
		}
		z, err := pkg.ZIP()
		if err != nil {
			h.fail(w, name, err)
			return
		}
		// name passed ValidateTemplateName, so it is safe inside the quotes.
		w.Header().Set("Content-Disposition", `attachment; filename="`+name+`-zza.zip"`)
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Length", strconv.Itoa(len(z)))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(z)
	})
}
