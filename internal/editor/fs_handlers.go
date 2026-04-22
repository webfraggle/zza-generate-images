package editor

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/webfraggle/zza-generate-images/internal/renderer"
	"gopkg.in/yaml.v3"
)

// InvalidateCacheFn is called after a successful template-save so the render
// cache can be purged. nil is allowed (no-op).
type InvalidateCacheFn func(template string)

// FSHandlers serves the editor HTTP API backed by the local filesystem.
// No auth — the desktop build binds to 127.0.0.1 only.
type FSHandlers struct {
	TemplatesDir string
	Invalidate   InvalidateCacheFn
}

// NewFSHandlers constructs handlers. invalidate may be nil in tests.
func NewFSHandlers(templatesDir string, invalidate InvalidateCacheFn) *FSHandlers {
	return &FSHandlers{TemplatesDir: templatesDir, Invalidate: invalidate}
}

// Register attaches all editor routes onto mux. Pattern prefix is /edit/.
// The EditorPage (GET /edit/{template}) is NOT registered here because it
// needs access to the parent server's html/template set; the server package
// overrides that route in its own RegisterEditor wiring.
func (h *FSHandlers) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /edit/{template}", h.EditorPage)
	mux.HandleFunc("GET /edit/{template}/files", h.ListFiles)
	mux.HandleFunc("GET /edit/{template}/file/{filename}", h.GetFile)
	mux.HandleFunc("POST /edit/{template}/save", h.Save)
	mux.HandleFunc("POST /edit/{template}/upload", h.Upload)
	mux.HandleFunc("DELETE /edit/{template}/file/{filename}", h.DeleteFile)
}

// EditorPage is a placeholder — see Register's doc comment. It only runs
// when FSHandlers is mounted standalone (in tests without the server wrapper).
func (h *FSHandlers) EditorPage(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "EditorPage must be wired by the server package", http.StatusNotImplemented)
}

func (h *FSHandlers) ListFiles(w http.ResponseWriter, r *http.Request) {
	name, ok := h.requireTemplate(w, r)
	if !ok {
		return
	}
	if err := InitTemplate(h.TemplatesDir, name); err != nil {
		http.Error(w, "could not init template dir", http.StatusInternalServerError)
		return
	}
	files, err := ListFiles(h.TemplatesDir, name)
	if err != nil {
		log.Printf("editor: list %q: %v", name, err)
		http.Error(w, "could not list files", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"files": files})
}

func (h *FSHandlers) GetFile(w http.ResponseWriter, r *http.Request) {
	name, ok := h.requireTemplate(w, r)
	if !ok {
		return
	}
	filename := r.PathValue("filename")
	data, err := ReadTextFile(h.TemplatesDir, name, filename)
	switch {
	case errors.Is(err, ErrForbidden), errors.Is(err, ErrInvalidName):
		http.Error(w, "forbidden", http.StatusForbidden)
	case errors.Is(err, ErrFileNotFound):
		http.Error(w, "not found", http.StatusNotFound)
	case err != nil:
		log.Printf("editor: read %q/%q: %v", name, filename, err)
		http.Error(w, "could not read file", http.StatusInternalServerError)
	default:
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write(data)
	}
}

type saveRequest struct {
	Filename string `json:"filename"`
	Content  string `json:"content"`
}

func (h *FSHandlers) Save(w http.ResponseWriter, r *http.Request) {
	name, ok := h.requireTemplate(w, r)
	if !ok {
		return
	}
	var req saveRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Parse-check YAML before writing (invalid YAML must not clobber the file).
	// Empty content bypasses the gate on purpose — truncation is a valid edit;
	// the render pipeline will surface the missing-fields error on next load.
	if len(req.Content) > 0 && hasYAMLExt(req.Filename) {
		var probe any
		if err := yaml.Unmarshal([]byte(req.Content), &probe); err != nil {
			http.Error(w, "invalid YAML: "+err.Error(), http.StatusBadRequest)
			return
		}
	}

	if err := WriteTextFile(h.TemplatesDir, name, req.Filename, []byte(req.Content)); err != nil {
		switch {
		case errors.Is(err, ErrForbidden), errors.Is(err, ErrInvalidName):
			http.Error(w, "forbidden", http.StatusForbidden)
		default:
			log.Printf("editor: write %q/%q: %v", name, req.Filename, err)
			http.Error(w, "could not save file", http.StatusInternalServerError)
		}
		return
	}
	if h.Invalidate != nil {
		h.Invalidate(name)
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *FSHandlers) Upload(w http.ResponseWriter, r *http.Request) {
	name, ok := h.requireTemplate(w, r)
	if !ok {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, MaxUploadBytes+512)
	if err := r.ParseMultipartForm(MaxUploadBytes); err != nil {
		http.Error(w, "request too large or invalid", http.StatusBadRequest)
		return
	}
	f, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "missing file field", http.StatusBadRequest)
		return
	}
	defer f.Close()

	if err := UploadFile(h.TemplatesDir, name, header.Filename, f); err != nil {
		if errors.Is(err, ErrForbidden) || errors.Is(err, ErrInvalidName) {
			http.Error(w, "file type not allowed", http.StatusForbidden)
		} else {
			log.Printf("editor: upload %q/%q: %v", name, header.Filename, err)
			http.Error(w, "upload failed", http.StatusInternalServerError)
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *FSHandlers) DeleteFile(w http.ResponseWriter, r *http.Request) {
	name, ok := h.requireTemplate(w, r)
	if !ok {
		return
	}
	filename := r.PathValue("filename")
	err := DeleteFile(h.TemplatesDir, name, filename)
	switch {
	case errors.Is(err, ErrForbidden), errors.Is(err, ErrInvalidName):
		http.Error(w, "forbidden", http.StatusForbidden)
	case errors.Is(err, ErrFileNotFound):
		http.Error(w, "not found", http.StatusNotFound)
	case err != nil:
		log.Printf("editor: delete %q/%q: %v", name, filename, err)
		http.Error(w, "could not delete file", http.StatusInternalServerError)
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}

func (h *FSHandlers) requireTemplate(w http.ResponseWriter, r *http.Request) (string, bool) {
	name := r.PathValue("template")
	if err := renderer.ValidateTemplateName(name); err != nil {
		http.Error(w, "invalid template name", http.StatusBadRequest)
		return "", false
	}
	return name, true
}

func hasYAMLExt(filename string) bool {
	return strings.HasSuffix(filename, ".yaml") || strings.HasSuffix(filename, ".yml")
}
