// Package devpreview serves the desktop-only ZZA preview: it renders a
// template with the device core (internal/devicerender) so the editor can show
// what the controller will draw. It is never linked into the server build.
package devpreview

import (
	"bytes"
	"encoding/json"
	"errors"
	"image/png"
	"io"
	"io/fs"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/webfraggle/zza-generate-images/internal/devexport"
	"github.com/webfraggle/zza-generate-images/internal/devicerender"
	"github.com/webfraggle/zza-generate-images/internal/renderer"
)

const (
	// Pattern is the mux pattern of the preview route.
	Pattern = "POST /edit/{template}/zza-render"

	// maxBodyBytes is the request body limit; same as POST /{template}/render.
	maxBodyBytes = 1 << 20
	// memBudget is the core's memory budget for one preview render.
	memBudget = 16 << 20
	// maxConcurrent bounds simultaneous previews (export + core render), like
	// the server's GIF renders (maxConcurrentGIFRenders).
	maxConcurrent = 2
	// defaultWaitTimeout is how long a request waits for a free slot.
	defaultWaitTimeout = 15 * time.Second
)

// Handler renders ZZA previews of the templates under templatesDir.
type Handler struct {
	templatesDir string
	now          func() time.Time
	sem          chan struct{} // limits concurrent previews
	waitTimeout  time.Duration // max wait for a slot in sem
}

// New returns a handler for templates in templatesDir.
func New(templatesDir string) *Handler {
	return &Handler{templatesDir: templatesDir, now: time.Now, sem: make(chan struct{}, maxConcurrent),
		waitTimeout: defaultWaitTimeout}
}

// ServeHTTP implements http.Handler for POST /edit/{template}/zza-render.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("template")
	if err := renderer.ValidateTemplateName(name); err != nil {
		http.Error(w, "invalid template name", http.StatusBadRequest)
		return
	}
	gif := false
	switch r.URL.Query().Get("format") {
	case "", "png":
	case "gif":
		gif = true
	default:
		http.Error(w, "unknown format (use png or gif)", http.StatusBadRequest)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, "request body unreadable", http.StatusBadRequest)
		return
	}
	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
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
	tmpl, err := renderer.LoadTemplate(h.templatesDir, name)
	if err != nil {
		h.fail(w, name, err)
		return
	}
	loc, err := time.LoadLocation(orDefault(tmpl.Meta.Timezone, "Europe/Berlin"))
	if err != nil {
		h.fail(w, name, err)
		return
	}
	now := h.now().In(loc)

	rd := devicerender.New(memBudget)
	defer rd.Close()
	if err := rd.Load(pkg); err != nil {
		h.fail(w, name, err)
		return
	}

	var buf bytes.Buffer
	format := "png"
	if gif {
		anim, err := rd.RenderAnimation(body, now)
		if err != nil {
			h.fail(w, name, err)
			return
		}
		if err := renderer.EncodeGIF(&buf, anim, tmpl.Meta.Canvas.Colors); err != nil {
			h.fail(w, name, err)
			return
		}
		format = "gif"
	} else {
		img, err := rd.Render(body, now)
		if err != nil {
			h.fail(w, name, err)
			return
		}
		if err := png.Encode(&buf, img); err != nil {
			h.fail(w, name, err)
			return
		}
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "image/"+format)
	w.Header().Set("Content-Length", strconv.Itoa(buf.Len()))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(buf.Bytes())
}

// acquire waits for a free slot in h.sem. On failure it has written a 503 and
// returns false; on success the caller must call h.release.
func (h *Handler) acquire(w http.ResponseWriter, r *http.Request) bool {
	wait := time.NewTimer(h.waitTimeout)
	defer wait.Stop()
	select {
	case h.sem <- struct{}{}:
		return true
	case <-r.Context().Done():
		http.Error(w, "request cancelled while waiting for a preview slot", http.StatusServiceUnavailable)
		return false
	case <-wait.C:
		w.Header().Set("Retry-After", "5")
		http.Error(w, "all preview slots busy, try again", http.StatusServiceUnavailable)
		return false
	}
}

func (h *Handler) release() { <-h.sem }

func orDefault(s, d string) string {
	if s == "" {
		return d
	}
	return s
}

// fail maps an error of the export/render pipeline to an HTTP response.
func (h *Handler) fail(w http.ResponseWriter, name string, err error) {
	var inc *devexport.Incompatible
	switch {
	case errors.Is(err, fs.ErrNotExist):
		http.Error(w, "template not found", http.StatusNotFound)
	case errors.As(err, &inc):
		http.Error(w, "nicht ZZA-kompatibel:\n"+strings.Join(inc.Reasons, "\n"), http.StatusUnprocessableEntity)
	case errors.Is(err, devicerender.ErrTemplate):
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
	case errors.Is(err, devicerender.ErrData):
		http.Error(w, err.Error(), http.StatusBadRequest)
	default: // ErrNoMem, ErrFormat (exporter/core mismatch = bug), the rest
		log.Printf("zza-render: %q: %v", name, err)
		http.Error(w, "ZZA render failed: "+err.Error(), http.StatusInternalServerError)
	}
}
