package server

import (
	"bytes"
	"context"
	"image/gif"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/webfraggle/zza-generate-images/internal/config"
	"github.com/webfraggle/zza-generate-images/web"
)

const flapTemplateYAML = `meta:
  canvas: {width: 20, height: 10}
layers:
  - type: splitflap
    x: 0
    y: 0
    width: 10
    height: 10
    cells: 2
    split: chars
    value: "AB"
    reel: " AB"
    var: ch
    layers:
      - if: "eq(ch, 'A')"
        layers: [{type: rect, x: 0, y: 0, width: 10, height: 10, color: "#ff0000"}]
`

// newTemplateServer starts a server on a temp templates dir holding the
// given templates: name -> template.yaml content (each with default.json {}).
// It returns the server and its cache directory.
func newTemplateServer(t *testing.T, files map[string]string) (*Server, string) {
	t.Helper()
	dir, cacheDir := t.TempDir(), t.TempDir()
	for name, yaml := range files {
		if err := os.MkdirAll(filepath.Join(dir, name), 0o755); err != nil {
			t.Fatal(err)
		}
		for fn, content := range map[string]string{"template.yaml": yaml, "default.json": `{}`} {
			if err := os.WriteFile(filepath.Join(dir, name, fn), []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	srv, err := New(&config.Config{
		Port: "8080", TemplatesDir: dir, CacheDir: cacheDir,
		CacheMaxAgeHours: 1, CacheMaxSizeMB: 10,
	}, web.FS)
	if err != nil {
		t.Fatal(err)
	}
	return srv, cacheDir
}

// newFlapServer starts a server on a temp templates dir holding the
// splitflap template "t" (with default.json).
func newFlapServer(t *testing.T) *Server {
	t.Helper()
	srv, _ := newTemplateServer(t, map[string]string{"t": flapTemplateYAML})
	return srv
}

func doRender(srv *Server, path, body string, hdr map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	return rr
}

func TestRender_FormatGIF(t *testing.T) {
	srv := newFlapServer(t)
	r1 := doRender(srv, "/t/render?format=gif", `{}`, nil)
	if r1.Code != 200 {
		t.Fatalf("got %d: %s", r1.Code, r1.Body.String())
	}
	if ct := r1.Header().Get("Content-Type"); ct != "image/gif" {
		t.Errorf("Content-Type = %q", ct)
	}
	if !bytes.HasPrefix(r1.Body.Bytes(), []byte("GIF89a")) {
		t.Error("body does not start with GIF89a")
	}
	g, err := gif.DecodeAll(bytes.NewReader(r1.Body.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if len(g.Image) < 2 {
		t.Errorf("frames = %d, want > 1", len(g.Image))
	}
	if r1.Header().Get("X-Cache") != "MISS" {
		t.Errorf("X-Cache = %q, want MISS", r1.Header().Get("X-Cache"))
	}
	r2 := doRender(srv, "/t/render?format=gif", `{}`, nil)
	if r2.Header().Get("X-Cache") != "HIT" {
		t.Errorf("second X-Cache = %q, want HIT", r2.Header().Get("X-Cache"))
	}
	if !bytes.Equal(r1.Body.Bytes(), r2.Body.Bytes()) {
		t.Error("cached GIF differs")
	}
}

func TestRender_DefaultStillPNG(t *testing.T) {
	srv := newFlapServer(t)
	a := doRender(srv, "/t/render", `{}`, nil)
	_ = doRender(srv, "/t/render?format=gif", `{}`, nil)
	b := doRender(srv, "/t/render?format=png", `{}`, nil)
	c := doRender(srv, "/t/render", `{}`, nil)
	for i, rr := range []*httptest.ResponseRecorder{a, b, c} {
		if rr.Code != 200 || rr.Header().Get("Content-Type") != "image/png" {
			t.Fatalf("resp %d: %d %q", i, rr.Code, rr.Header().Get("Content-Type"))
		}
		if !bytes.HasPrefix(rr.Body.Bytes(), []byte("\x89PNG")) {
			t.Fatalf("resp %d: no PNG magic", i)
		}
	}
	if !bytes.Equal(a.Body.Bytes(), b.Body.Bytes()) || !bytes.Equal(a.Body.Bytes(), c.Body.Bytes()) {
		t.Error("PNG bytes differ between default, format=png and after a GIF request")
	}
}

func TestRender_FormatInvalid(t *testing.T) {
	srv := newFlapServer(t)
	for _, f := range []string{"webp", "GIF", "Png"} {
		if rr := doRender(srv, "/t/render?format="+f, `{}`, nil); rr.Code != 400 {
			t.Errorf("format=%s: got %d, want 400", f, rr.Code)
		}
	}
	req := httptest.NewRequest(http.MethodGet, "/t/preview?format=webp", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	if rr.Code != 400 {
		t.Errorf("preview format=webp: got %d, want 400", rr.Code)
	}
}

func TestPreview_FormatGIF(t *testing.T) {
	srv := newFlapServer(t)
	req := httptest.NewRequest(http.MethodGet, "/t/preview?format=gif", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	if rr.Code != 200 || rr.Header().Get("Content-Type") != "image/gif" {
		t.Fatalf("got %d %q: %s", rr.Code, rr.Header().Get("Content-Type"), rr.Body.String())
	}
}

func TestRender_GIF_NoSplitflap(t *testing.T) {
	srv := newTestServer(t)
	raw, err := os.ReadFile(filepath.Join("..", "..", "templates", "sbb-105-v1", "default.json"))
	if err != nil {
		t.Fatal(err)
	}
	rr := doRender(srv, "/sbb-105-v1/render?format=gif", string(raw), nil)
	if rr.Code != 200 {
		t.Fatalf("got %d: %s", rr.Code, rr.Body.String())
	}
	g, err := gif.DecodeAll(bytes.NewReader(rr.Body.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if len(g.Image) != 1 {
		t.Errorf("frames = %d, want 1", len(g.Image))
	}
}

func TestRender_GIF_HTTPNoRedirect(t *testing.T) {
	srv := newFlapServer(t)
	rr := doRender(srv, "/t/render?format=gif", `{}`, map[string]string{"X-Forwarded-Proto": "http"})
	if rr.Code != 200 {
		t.Errorf("got %d, want 200 (no redirect)", rr.Code)
	}
}

func TestRender_GIF_Deterministic(t *testing.T) {
	// Two servers share templates content but not cache dirs.
	a := doRender(newFlapServer(t), "/t/render?format=gif", `{}`, nil)
	b := doRender(newFlapServer(t), "/t/render?format=gif", `{}`, nil)
	if a.Code != 200 || !bytes.Equal(a.Body.Bytes(), b.Body.Bytes()) {
		t.Error("GIF bytes differ between independent servers")
	}
}

func TestRender_GIF_ContextCancelledWhileWaiting(t *testing.T) {
	srv := newFlapServer(t)
	for i := 0; i < cap(srv.gifSem); i++ {
		srv.gifSem <- struct{}{}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	req := httptest.NewRequest(http.MethodPost, "/t/render?format=gif", strings.NewReader(`{}`)).WithContext(ctx)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("got %d, want 503", rr.Code)
	}
	// A cache hit must not wait for the semaphore.
	for i := 0; i < cap(srv.gifSem); i++ {
		<-srv.gifSem
	}
	if r := doRender(srv, "/t/render?format=gif", `{}`, nil); r.Code != 200 {
		t.Fatalf("warm-up: %d", r.Code)
	}
	for i := 0; i < cap(srv.gifSem); i++ {
		srv.gifSem <- struct{}{}
	}
	if r := doRender(srv, "/t/render?format=gif", `{}`, nil); r.Code != 200 || r.Header().Get("X-Cache") != "HIT" {
		t.Errorf("cache hit with full semaphore: %d %q", r.Code, r.Header().Get("X-Cache"))
	}
}

func TestDetail_AnimatedFlag(t *testing.T) {
	srv := newFlapServer(t)
	// The flag is set from HasSplitflap; checked via the view model builder.
	if !srv.isAnimated("t") {
		t.Error("splitflap template: Animated = false")
	}
	if srv.isAnimated("does-not-exist") {
		t.Error("missing template: Animated = true")
	}
}

// addPlainTemplate adds a non-animated template "p" next to the splitflap one.
func addPlainTemplate(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, "p"), 0o755); err != nil {
		t.Fatal(err)
	}
	yaml := "meta:\n  canvas: {width: 20, height: 10}\nlayers:\n  - type: rect\n    x: 0\n    y: 0\n    width: 20\n    height: 10\n    color: \"#ff0000\"\n"
	for name, content := range map[string]string{"template.yaml": yaml, "default.json": `{}`} {
		if err := os.WriteFile(filepath.Join(dir, "p", name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func getPage(srv *Server, path string) string {
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, path, nil))
	return rr.Body.String()
}

func TestDetail_AnimatedControls(t *testing.T) {
	srv := newFlapServer(t)
	addPlainTemplate(t, srv.templatesDir)

	anim := getPage(srv, "/t")
	for _, id := range []string{`id="replay-btn"`, `id="download-gif-btn"`, `class="tbtn tbtn-primary" disabled>`, "GIF herunterladen"} {
		if !strings.Contains(anim, id) {
			t.Errorf("animated detail page lacks %s", id)
		}
	}
	plain := getPage(srv, "/p")
	for _, id := range []string{`id="replay-btn"`, `id="download-gif-btn"`} {
		if strings.Contains(plain, id) {
			t.Errorf("plain detail page contains %s", id)
		}
	}
	if !strings.Contains(plain, `id="download-btn" class="tbtn tbtn-primary"`) {
		t.Error("plain detail page: PNG download must be the primary button")
	}
	if strings.Contains(anim, `id="download-btn" class="tbtn tbtn-primary"`) {
		t.Error("animated detail page: PNG download must not be primary (GIF is)")
	}
	for _, page := range []string{anim, plain} {
		if !strings.Contains(page, `id="download-src-btn"`) {
			t.Error("detail page lacks the template source download")
		}
	}
}

// getEditorPage calls the editor page handler directly: the /edit/ route is
// only wired up in the desktop build.
func getEditorPage(srv *Server, name string) string {
	req := httptest.NewRequest(http.MethodGet, "/edit/"+name, nil)
	req.SetPathValue("template", name)
	rr := httptest.NewRecorder()
	srv.handleEditorPage(rr, req)
	return rr.Body.String()
}

func TestEditor_AnimatedControls(t *testing.T) {
	srv := newFlapServer(t)
	addPlainTemplate(t, srv.templatesDir)

	anim := getEditorPage(srv, "t")
	for _, id := range []string{`id="preview-mode"`, `id="replay-btn"`, "Standbild", "Animation", "Erneut abspielen"} {
		if !strings.Contains(anim, id) {
			t.Errorf("animated editor page lacks %s", id)
		}
	}
	plain := getEditorPage(srv, "p")
	for _, id := range []string{`id="preview-mode"`, `id="replay-btn"`} {
		if strings.Contains(plain, id) {
			t.Errorf("plain editor page contains %s", id)
		}
	}
}

// A request that waited for a render slot must serve the GIF another request
// cached in the meantime instead of rendering it again.
func TestRender_GIF_CacheRecheckAfterWait(t *testing.T) {
	srv := newFlapServer(t)
	for i := 0; i < cap(srv.gifSem); i++ {
		srv.gifSem <- struct{}{}
	}
	done := make(chan *httptest.ResponseRecorder)
	go func() { done <- doRender(srv, "/t/render?format=gif", `{}`, nil) }()
	time.Sleep(100 * time.Millisecond) // let the request reach the semaphore

	key := srv.renderCacheKey("t", []byte(`{}`))
	if err := srv.cache.SetExt(key, formatGIF, []byte("SENTINEL")); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < cap(srv.gifSem); i++ {
		<-srv.gifSem
	}
	rr := <-done
	if rr.Code != 200 || rr.Header().Get("X-Cache") != "HIT" || rr.Body.String() != "SENTINEL" {
		t.Errorf("got %d, X-Cache %q, %d bytes; want the cached entry as HIT", rr.Code, rr.Header().Get("X-Cache"), rr.Body.Len())
	}
}

func TestRender_GIF_WaitTimeout(t *testing.T) {
	srv := newFlapServer(t)
	srv.gifWaitTimeout = 20 * time.Millisecond
	for i := 0; i < cap(srv.gifSem); i++ {
		srv.gifSem <- struct{}{}
	}
	rr := doRender(srv, "/t/render?format=gif", `{}`, nil)
	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("got %d, want 503", rr.Code)
	}
	if rr.Header().Get("Retry-After") == "" {
		t.Error("503 without Retry-After")
	}
}

const nowTemplateYAML = `meta:
  canvas: {width: 4, height: 4}
layers:
  - type: rect
    x: 0
    y: 0
    width: 4
    height: 4
    color: {if: "greaterThan(now.second, -1)", then: "#ff0000", else: "#00ff00"}
`

const frameTemplateYAML = `meta:
  canvas: {width: 4, height: 1}
  animation: {frames: 4, frame_delay: 1, loop: true}
layers:
  - {type: rect, x: 0, y: 0, width: 4, height: 1, color: "#000000"}
  - {type: rect, x: "{{anim.frame}}", y: 0, width: 1, height: 1, color: "#ff0000"}
`

func newTimeServer(t *testing.T) (*Server, string) {
	return newTemplateServer(t, map[string]string{"n": nowTemplateYAML, "f": frameTemplateYAML})
}

func TestRender_UsesNow_BypassesCache(t *testing.T) {
	srv, cacheDir := newTimeServer(t)
	for _, path := range []string{"/n/render", "/n/render?format=gif"} {
		for i := 0; i < 2; i++ {
			rr := doRender(srv, path, `{}`, nil)
			if rr.Code != 200 {
				t.Fatalf("%s: status %d: %s", path, rr.Code, rr.Body.String())
			}
			if got := rr.Header().Get("X-Cache"); got != "BYPASS" {
				t.Errorf("%s: X-Cache = %q, want BYPASS", path, got)
			}
			if got := rr.Header().Get("Cache-Control"); got != "no-store" {
				t.Errorf("%s: Cache-Control = %q, want no-store", path, got)
			}
		}
	}
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/n/preview", nil))
	if rr.Code != 200 || rr.Header().Get("X-Cache") != "BYPASS" {
		t.Errorf("preview: %d, X-Cache %q, want 200 BYPASS", rr.Code, rr.Header().Get("X-Cache"))
	}
	_ = filepath.WalkDir(cacheDir, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() && (strings.HasSuffix(p, ".png") || strings.HasSuffix(p, ".gif")) {
			t.Errorf("cache contains %s", p)
		}
		return nil
	})
}

func TestRender_UsesNow_GIFStillLimited(t *testing.T) {
	srv, _ := newTimeServer(t)
	srv.gifWaitTimeout = 50 * time.Millisecond
	for i := 0; i < cap(srv.gifSem); i++ {
		srv.gifSem <- struct{}{}
	}
	defer func() {
		for i := 0; i < cap(srv.gifSem); i++ {
			<-srv.gifSem
		}
	}()
	rr := doRender(srv, "/n/render?format=gif", `{}`, nil)
	if rr.Code != http.StatusServiceUnavailable || rr.Header().Get("Retry-After") == "" {
		t.Errorf("got %d, Retry-After %q; want 503 with Retry-After", rr.Code, rr.Header().Get("Retry-After"))
	}
}

func TestRender_FrameMode_GIF(t *testing.T) {
	srv, _ := newTimeServer(t)
	rr := doRender(srv, "/f/render?format=gif", `{}`, nil)
	if rr.Code != 200 {
		t.Fatalf("status %d: %s", rr.Code, rr.Body.String())
	}
	g, err := gif.DecodeAll(bytes.NewReader(rr.Body.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if len(g.Image) != 4 || g.LoopCount != 0 {
		t.Fatalf("frames = %d, LoopCount = %d, want 4 and 0", len(g.Image), g.LoopCount)
	}
	for i, d := range g.Delay {
		if d != 100 {
			t.Errorf("delay[%d] = %d, want 100", i, d)
		}
	}
	if r2 := doRender(srv, "/f/render?format=gif", `{}`, nil); r2.Header().Get("X-Cache") != "HIT" {
		t.Errorf("second X-Cache = %q, want HIT", r2.Header().Get("X-Cache"))
	}
}

func TestDetail_AnimatedFlag_FrameMode(t *testing.T) {
	srv, _ := newTimeServer(t)
	page := getPage(srv, "/f")
	for _, id := range []string{`id="replay-btn"`, `id="download-gif-btn"`} {
		if !strings.Contains(page, id) {
			t.Errorf("detail page lacks %s", id)
		}
	}
	if !strings.Contains(getEditorPage(srv, "f"), `id="preview-mode"`) {
		t.Error("editor page lacks preview-mode")
	}
}

func previewGIF(srv *Server, name string) *httptest.ResponseRecorder {
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/"+name+"/preview?format=gif", nil))
	return rr
}

func TestPreviewGIF_MicroCache(t *testing.T) {
	srv, _ := newTimeServer(t)
	clock := time.Unix(1000, 0)
	srv.now = func() time.Time { return clock }

	first := previewGIF(srv, "n")
	if first.Code != 200 || first.Header().Get("X-Cache") != "BYPASS" {
		t.Fatalf("first: %d, X-Cache %q, want 200 BYPASS", first.Code, first.Header().Get("X-Cache"))
	}
	clock = clock.Add(previewGIFTTL - time.Second)
	second := previewGIF(srv, "n")
	if second.Code != 200 || second.Header().Get("X-Cache") != "PREVIEW" {
		t.Fatalf("second: %d, X-Cache %q, want 200 PREVIEW", second.Code, second.Header().Get("X-Cache"))
	}
	if !bytes.Equal(first.Body.Bytes(), second.Body.Bytes()) {
		t.Error("PREVIEW hit differs from the rendered GIF")
	}
	if got := second.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", got)
	}

	// The render endpoint is unaffected.
	if rr := doRender(srv, "/n/render?format=gif", `{}`, nil); rr.Header().Get("X-Cache") != "BYPASS" {
		t.Errorf("render X-Cache = %q, want BYPASS", rr.Header().Get("X-Cache"))
	}

	// Expired entry re-renders.
	clock = clock.Add(2 * time.Second)
	if rr := previewGIF(srv, "n"); rr.Header().Get("X-Cache") != "BYPASS" {
		t.Errorf("after TTL: X-Cache = %q, want BYPASS (re-render)", rr.Header().Get("X-Cache"))
	}
}

func TestPreviewGIF_MicroCache_OnlyExistingNotCacheable(t *testing.T) {
	srv, _ := newTimeServer(t)
	if rr := previewGIF(srv, "missing"); rr.Code != http.StatusNotFound && rr.Code != http.StatusBadRequest {
		t.Errorf("missing: %d", rr.Code)
	}
	// Cacheable (animated, no now) template is served by the disk cache, not the micro-cache.
	previewGIF(srv, "n")
	previewGIF(srv, "f")
	if rr := previewGIF(srv, "f"); rr.Header().Get("X-Cache") != "HIT" {
		t.Errorf("cacheable template: X-Cache = %q, want HIT", rr.Header().Get("X-Cache"))
	}
	srv.previewMu.Lock()
	defer srv.previewMu.Unlock()
	if len(srv.previewGIFs) != 1 {
		t.Errorf("micro-cache holds %d entries, want 1 (only the time-dependent template)", len(srv.previewGIFs))
	}
}
