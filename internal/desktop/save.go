package desktop

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// SavePattern is the mux pattern of the native "save as" route. The Wails
// window's web view (WKWebView on macOS) ignores downloads, so the pages send
// the file here and the app writes it where the user chooses.
// "_desktop" is not a valid template name, so it cannot collide with one.
const SavePattern = "POST /edit/_desktop/save-file"

// maxSaveBytes bounds one saved file (PNG, GIF or a template ZIP).
const maxSaveBytes = 64 << 20

// errNoWindow means there is no Wails window to show a dialog in (browser
// fallback); the page then falls back to a normal browser download.
var errNoWindow = errors.New("no native window")

// Saver handles SavePattern: it asks for a target with the native save dialog
// and writes the file there. The file name comes from ?name=. The body is the
// file as base64 text: WKWebView delivers text bodies to the app, but drops
// binary Blob bodies (the request arrives empty).
//
// Responses: 200 {"saved":true,"path":…}, 200 {"saved":false} when the user
// cancels, 501 when no native window exists.
type Saver struct {
	mu  sync.Mutex
	ctx context.Context

	// dialog returns the chosen path, "" on cancel. Replaced in tests.
	dialog func(ctx context.Context, defaultName string) (string, error)
}

// NewSaver returns a Saver that shows the Wails save dialog once RunGUI has
// attached the window.
func NewSaver() *Saver {
	return &Saver{dialog: func(ctx context.Context, defaultName string) (string, error) {
		return wailsruntime.SaveFileDialog(ctx, wailsruntime.SaveDialogOptions{
			Title:                "Speichern unter",
			DefaultFilename:      defaultName,
			CanCreateDirectories: true,
		})
	}}
}

// attach stores the Wails context (from OnStartup); dialogs need it.
func (s *Saver) attach(ctx context.Context) {
	s.mu.Lock()
	s.ctx = ctx
	s.mu.Unlock()
}

func (s *Saver) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	name := saveName(r.URL.Query().Get("name"))
	if name == "" {
		http.Error(w, "missing file name", http.StatusBadRequest)
		return
	}
	// base64 needs 4 bytes per 3; the reader limit is on the encoded text.
	enc, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxSaveBytes/3*4+4))
	if err != nil {
		http.Error(w, "file too large or unreadable", http.StatusRequestEntityTooLarge)
		return
	}
	data, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(enc)))
	if err != nil {
		http.Error(w, "body is not base64", http.StatusBadRequest)
		return
	}
	// Never write an empty file: an empty body means the data got lost on the way.
	if len(data) == 0 {
		http.Error(w, "Die Datei ist leer angekommen und wurde nicht gespeichert.", http.StatusBadRequest)
		return
	}

	path, err := s.choose(name)
	if errors.Is(err, errNoWindow) {
		http.Error(w, "no native window", http.StatusNotImplemented)
		return
	}
	if err != nil {
		log.Printf("desktop: save dialog: %v", err)
		http.Error(w, "save dialog failed", http.StatusInternalServerError)
		return
	}
	if path == "" {
		writeSaveResult(w, false, "")
		return
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		log.Printf("desktop: save %q: %v", path, err)
		http.Error(w, "Datei konnte nicht gespeichert werden: "+err.Error(), http.StatusInternalServerError)
		return
	}
	writeSaveResult(w, true, path)
}

func (s *Saver) choose(name string) (string, error) {
	s.mu.Lock()
	ctx := s.ctx
	s.mu.Unlock()
	if ctx == nil {
		return "", errNoWindow
	}
	return s.dialog(ctx, name)
}

// saveName reduces the requested name to a plain file name (no directories).
// The dialog decides the folder; the name is only its suggestion.
func saveName(raw string) string {
	name := filepath.Base(strings.ReplaceAll(strings.TrimSpace(raw), "\\", "/"))
	if name == "." || name == ".." || name == "/" {
		return ""
	}
	return name
}

func writeSaveResult(w http.ResponseWriter, saved bool, path string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	out := map[string]any{"saved": saved}
	if saved {
		out["path"] = path
	}
	_ = json.NewEncoder(w).Encode(out)
}
