package editor

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

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
	// Feed serves GET /edit/{template}/events. nil answers 404 (no watcher).
	Feed ChangeFeed
}

// NewFSHandlers constructs handlers. invalidate may be nil in tests.
func NewFSHandlers(templatesDir string, invalidate InvalidateCacheFn) *FSHandlers {
	return &FSHandlers{TemplatesDir: templatesDir, Invalidate: invalidate}
}

// Register attaches the five sub-routes (files, file read/write/delete, save,
// upload) onto mux under the /edit/{template}/ prefix. The editor landing page
// itself (GET /edit/{template}) is owned by the server package — it needs the
// shared html/template set and is wired in Server.RegisterEditor.
func (h *FSHandlers) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /edit/{template}/files", h.ListFiles)
	mux.HandleFunc("GET /edit/{template}/file/{filename}", h.GetFile)
	mux.HandleFunc("GET /edit/{template}/events", h.Events)
	mux.HandleFunc("POST /edit/{template}/save", h.Save)
	mux.HandleFunc("POST /edit/{template}/upload", h.Upload)
	mux.HandleFunc("DELETE /edit/{template}/file/{filename}", h.DeleteFile)
	mux.HandleFunc("POST /edit/{template}/rename", h.Rename)
}

// Rename renames the template to the name in the JSON body {"name": "..."}.
// 200 {"name": new}; 400 (characters, length), 404 and 409 (name taken) carry
// a German text for the rename dialog.
func (h *FSHandlers) Rename(w http.ResponseWriter, r *http.Request) {
	name, ok := h.requireTemplate(w, r)
	if !ok {
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	newName := strings.TrimSpace(req.Name)
	switch err := RenameTemplate(h.TemplatesDir, name, newName); {
	case err == nil:
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"name": newName})
	case errors.Is(err, ErrInvalidName):
		http.Error(w, "Name ungültig: nur Kleinbuchstaben, Ziffern und Bindestriche.", http.StatusBadRequest)
	case errors.Is(err, ErrNameTooLong):
		http.Error(w, fmt.Sprintf("Name zu lang (%d Zeichen, höchstens %d): Längere Namen kann der Anzeiger nicht installieren.",
			len(newName), MaxTemplateNameLen), http.StatusBadRequest)
	case errors.Is(err, ErrTemplateExists):
		http.Error(w, "Ein Template mit diesem Namen gibt es schon.", http.StatusConflict)
	case errors.Is(err, ErrTemplateNotFound):
		http.Error(w, "Template nicht gefunden.", http.StatusNotFound)
	default:
		log.Printf("editor: rename %q -> %q: %v", name, newName, err)
		http.Error(w, "Umbenennen fehlgeschlagen.", http.StatusInternalServerError)
	}
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
		w.Header().Set("ETag", `"`+FileVersion(data)+`"`)
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
		// A yaml.Node only checks the syntax. Decoding into a Go value would also
		// refuse repeated mapping keys, and the template language repeats
		// "elif"/"then" in conditional values.
		var probe yaml.Node
		if err := yaml.Unmarshal([]byte(req.Content), &probe); err != nil {
			http.Error(w, "invalid YAML: "+err.Error(), http.StatusBadRequest)
			return
		}
	}

	expected := strings.TrimPrefix(strings.TrimSpace(r.Header.Get("If-Match")), "W/")
	expected = strings.Trim(expected, `"`)
	newVersion, err := WriteTextFileIfMatch(h.TemplatesDir, name, req.Filename, []byte(req.Content), expected)
	if err != nil {
		switch {
		case errors.Is(err, ErrVersionConflict):
			var vce *VersionConflictError
			cur := ""
			if errors.As(err, &vce) {
				cur = vce.Current
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusConflict)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "conflict", "current_version": cur})
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
	w.Header().Set("ETag", `"`+newVersion+`"`)
	w.WriteHeader(http.StatusNoContent)
}

// eventsTimeout is how long the long-poll waits for changes.
const eventsTimeout = 25 * time.Second

// Events is the long-poll endpoint for file changes of a template.
func (h *FSHandlers) Events(w http.ResponseWriter, r *http.Request) {
	if h.Feed == nil {
		http.NotFound(w, r)
		return
	}
	name, ok := h.requireTemplate(w, r)
	if !ok {
		return
	}
	var batch ChangeBatch
	if s := r.URL.Query().Get("since"); s == "" {
		batch = ChangeBatch{Seq: h.Feed.Current()}
	} else {
		since, err := strconv.ParseUint(s, 10, 64)
		if err != nil {
			http.Error(w, "invalid since", http.StatusBadRequest)
			return
		}
		batch = h.Feed.Wait(r.Context(), name, since, eventsTimeout)
	}
	if batch.Files == nil {
		batch.Files = []string{}
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(batch)
}

func (h *FSHandlers) Upload(w http.ResponseWriter, r *http.Request) {
	name, ok := h.requireTemplate(w, r)
	if !ok {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, MaxUploadBytes+512)

	// Two forms: the file as the raw request body with ?name=<file name> (what
	// the editor page sends), or a multipart form with the field "file". The
	// app window's web view (WKWebView) delivers a form with a file as an empty
	// body, so the page cannot use the form.
	var (
		filename string
		content  io.Reader
	)
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/") {
		filename = r.URL.Query().Get("name")
		if filename == "" {
			http.Error(w, "missing file name", http.StatusBadRequest)
			return
		}
		content = r.Body
	} else {
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
		filename, content = header.Filename, f
	}

	if err := UploadFile(h.TemplatesDir, name, filename, content); err != nil {
		if errors.Is(err, ErrForbidden) || errors.Is(err, ErrInvalidName) {
			http.Error(w, "file type not allowed", http.StatusForbidden)
		} else {
			log.Printf("editor: upload %q/%q: %v", name, filename, err)
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
