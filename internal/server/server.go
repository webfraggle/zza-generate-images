package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"image"
	"image/png"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/webfraggle/zza-generate-images/internal/config"
	"github.com/webfraggle/zza-generate-images/internal/editor"
	"github.com/webfraggle/zza-generate-images/internal/gallery"
	"github.com/webfraggle/zza-generate-images/internal/renderer"
	"github.com/webfraggle/zza-generate-images/internal/version"
)

const maxRequestBodyBytes = 1 << 20 // 1 MiB

// maxConcurrentGIFRenders bounds simultaneous GIF cache-miss renders, which
// are far more expensive than PNG renders.
const maxConcurrentGIFRenders = 2

// defaultGIFWaitTimeout is how long a GIF cache miss waits for a render slot
// before it is answered with 503 and Retry-After.
const defaultGIFWaitTimeout = 15 * time.Second

// previewGIFTTL is how long GET /{template}/preview?format=gif serves the last
// GIF of a template that cannot use the disk cache (UsesNow), so gallery
// views do not each render a fresh animation.
const previewGIFTTL = 10 * time.Second

// previewEntry is one entry of the in-memory preview micro-cache.
type previewEntry struct {
	data    []byte
	created time.Time
}

// captureWriter records status and body of a response while passing it on.
type captureWriter struct {
	http.ResponseWriter
	status int
	body   bytes.Buffer
}

func (c *captureWriter) WriteHeader(code int) {
	c.status = code
	c.ResponseWriter.WriteHeader(code)
}

func (c *captureWriter) Write(p []byte) (int, error) {
	c.body.Write(p)
	return c.ResponseWriter.Write(p)
}

// Output formats selectable with the "format" query parameter.
const (
	formatPNG = "png"
	formatGIF = "gif"
)

// Server handles HTTP requests for the ZZA image renderer.
type Server struct {
	mux            *http.ServeMux
	staticHandler  http.Handler
	editorHandler  http.Handler // set by RegisterEditor; desktop-only
	rend           *renderer.Renderer
	cache          *Cache
	gifSem         chan struct{} // limits concurrent GIF renders
	gifWaitTimeout time.Duration // max wait for a slot in gifSem
	templatesDir   string
	now            func() time.Time // clock of the preview micro-cache (injectable in tests)
	previewMu      sync.Mutex
	previewGIFs    map[string]previewEntry // at most one entry per existing template
	htmlTmpl       *template.Template
	editorEnabled  bool
}

// New creates and initialises a Server from cfg.
// webFS must contain "templates/*.html" and "static/" for the frontend.
func New(cfg *config.Config, webFS fs.FS) (*Server, error) {
	cache, err := NewCache(
		cfg.CacheDir,
		time.Duration(cfg.CacheMaxAgeHours)*time.Hour,
		cfg.CacheMaxSizeMB,
	)
	if err != nil {
		return nil, fmt.Errorf("server: %w", err)
	}

	funcMap := template.FuncMap{
		"appVersion": func() string { return version.Version },
		"formatBytes": func(b int64) string {
			switch {
			case b >= 1<<20:
				return fmt.Sprintf("%.1f MB", float64(b)/(1<<20))
			case b >= 1<<10:
				return fmt.Sprintf("%.1f KB", float64(b)/(1<<10))
			default:
				return fmt.Sprintf("%d B", b)
			}
		},
	}
	tmpl, err := template.New("").Funcs(funcMap).ParseFS(webFS, "templates/*.html")
	if err != nil {
		return nil, fmt.Errorf("server: parsing HTML templates: %w", err)
	}

	staticFS, err := fs.Sub(webFS, "static")
	if err != nil {
		return nil, fmt.Errorf("server: sub FS for static: %w", err)
	}

	s := &Server{
		mux:            http.NewServeMux(),
		staticHandler:  http.StripPrefix("/static/", http.FileServerFS(staticFS)),
		rend:           renderer.New(cfg.TemplatesDir),
		cache:          cache,
		gifSem:         make(chan struct{}, maxConcurrentGIFRenders),
		gifWaitTimeout: defaultGIFWaitTimeout,
		now:            time.Now,
		previewGIFs:    map[string]previewEntry{},
		templatesDir:   cfg.TemplatesDir,
		htmlTmpl:       tmpl,
	}
	s.registerRoutes()
	return s, nil
}

// isRenderRoute reports whether r targets a POST /{template}/render endpoint.
func isRenderRoute(r *http.Request) bool {
	return r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/render")
}

// ServeHTTP implements http.Handler.
// /static/... and /{template}.zip are dispatched before the mux to avoid
// routing conflicts with wildcard patterns like "GET /{template}/preview".
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Redirect HTTP → HTTPS for all routes except /render (called by microcontrollers).
	if r.Header.Get("X-Forwarded-Proto") == "http" && !isRenderRoute(r) {
		target := "https://" + r.Host + r.RequestURI
		http.Redirect(w, r, target, http.StatusMovedPermanently)
		return
	}

	if strings.HasPrefix(r.URL.Path, "/static/") {
		s.staticHandler.ServeHTTP(w, r)
		return
	}
	if strings.HasPrefix(r.URL.Path, "/edit/") {
		if s.editorHandler == nil {
			http.NotFound(w, r)
			return
		}
		s.editorHandler.ServeHTTP(w, r)
		return
	}
	if strings.HasSuffix(r.URL.Path, ".zip") && r.Method == http.MethodGet {
		name := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/"), ".zip")
		s.handleTemplateZip(w, name)
		return
	}
	s.mux.ServeHTTP(w, r)
}

// StartCleanup delegates to the cache's cleanup goroutine.
func (s *Server) StartCleanup(ctx context.Context, interval time.Duration) {
	s.cache.StartCleanup(ctx, interval)
}

// SetEditorEnabled toggles the Edit-button on the preview page. Desktop sets true.
// Call once during startup before ServeHTTP — not concurrency-safe.
func (s *Server) SetEditorEnabled(v bool) { s.editorEnabled = v }

// EditorRoute is an additional route of the editor mux (full ServeMux pattern,
// e.g. "POST /edit/{template}/zza-render"). It lets the desktop build add
// routes without the server package importing their packages.
type EditorRoute struct {
	Pattern string
	Handler http.Handler
}

// RegisterEditor attaches an FSHandlers set to this server and wires the
// editor landing page on top (owned by the server package so it can reuse
// the shared html/template set). Desktop-build calls this once at startup;
// server-build does not. Not concurrency-safe — call before ServeHTTP.
func (s *Server) RegisterEditor(h *editor.FSHandlers, extra ...EditorRoute) {
	mux := http.NewServeMux()
	h.Register(mux)
	for _, rt := range extra {
		mux.Handle(rt.Pattern, rt.Handler)
	}
	mux.HandleFunc("GET /edit/{template}", s.handleEditorPage)
	s.editorHandler = mux
}

// InvalidateTemplateCache touches template.yaml so the next render recomputes.
// The existing renderAndServe key includes template.yaml's mod-time, so bumping
// it invalidates all cached PNGs for the template.
func (s *Server) InvalidateTemplateCache(name string) {
	if err := renderer.ValidateTemplateName(name); err != nil {
		return
	}
	p := filepath.Join(s.templatesDir, name, "template.yaml")
	now := time.Now()
	if err := os.Chtimes(p, now, now); err != nil {
		log.Printf("invalidate cache for %q: %v", name, err)
	}
}

type editorPageData struct {
	TemplateName string
	Animated     bool
}

// isAnimated reports whether the template is animated (splitflap or frame
// mode). A template that fails to load counts as not animated.
func (s *Server) isAnimated(name string) bool {
	tmpl, err := renderer.LoadTemplate(s.templatesDir, name)
	return err == nil && tmpl.IsAnimated()
}

func (s *Server) handleEditorPage(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("template")
	if err := renderer.ValidateTemplateName(name); err != nil {
		http.Error(w, "invalid template name", http.StatusBadRequest)
		return
	}
	if err := editor.InitTemplate(s.templatesDir, name); err != nil {
		log.Printf("editor page: init %q: %v", name, err)
		http.Error(w, "could not init template", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.htmlTmpl.ExecuteTemplate(w, "edit-editor.html", editorPageData{TemplateName: name, Animated: s.isAnimated(name)}); err != nil {
		log.Printf("editor page: execute template: %v", err)
	}
}

func (s *Server) registerRoutes() {
	// API
	s.mux.HandleFunc("POST /{template}/render", s.handleRender)
	s.mux.HandleFunc("OPTIONS /{template}/render", s.handleOptions)

	// Gallery UI
	s.mux.HandleFunc("GET /", s.handleGallery)
	s.mux.HandleFunc("GET /health", s.handleHealth)
	s.mux.HandleFunc("GET /{template}/preview", s.handlePreview)
	s.mux.HandleFunc("GET /{template}", s.handleDetail)
}

// corsHeaders sets permissive CORS headers.
// This server is intended for local/intranet use, so wildcard origin is acceptable.
// No credentials are involved, so Access-Control-Allow-Credentials is not set.
func corsHeaders(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Access-Control-Max-Age", "86400")
}

func (s *Server) handleOptions(w http.ResponseWriter, r *http.Request) {
	corsHeaders(w)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprintln(w, "ok")
}

// galleryData is the view model for the gallery overview page.
type galleryData struct {
	Templates     []gallery.TemplateInfo
	EditorEnabled bool
}

// handleGallery renders the template gallery overview page.
func (s *Server) handleGallery(w http.ResponseWriter, r *http.Request) {
	infos, err := gallery.ListTemplates(s.templatesDir)
	if err != nil {
		http.Error(w, "could not list templates", http.StatusInternalServerError)
		log.Printf("gallery: list: %v", err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.htmlTmpl.ExecuteTemplate(w, "gallery.html",
		galleryData{Templates: infos, EditorEnabled: s.editorEnabled}); err != nil {
		log.Printf("gallery: execute template: %v", err)
	}
}

// detailData is the view model for the detail/try-it page.
type detailData struct {
	Name          string
	Meta          renderer.Meta
	DefaultJSON   string
	HasDefault    bool
	EditorEnabled bool
	Animated      bool        // template has splitflap layers (GIF output is animated)
	Form          []formGroup // input fields the template actually uses; nil if unavailable
}

// handleDetail renders the try-it detail page for a single template.
func (s *Server) handleDetail(w http.ResponseWriter, r *http.Request) {
	templateName := r.PathValue("template")
	if err := renderer.ValidateTemplateName(templateName); err != nil {
		http.Error(w, "invalid template name", http.StatusBadRequest)
		return
	}

	d, err := s.loadDetailData(templateName)
	if err != nil {
		http.Error(w, "template not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.htmlTmpl.ExecuteTemplate(w, "detail.html", d); err != nil {
		log.Printf("detail: execute template: %v", err)
	}
}

// loadDetailData builds the detail page view model. It fails only if the
// template itself cannot be loaded; problems with default.json or the form
// derivation are logged and leave the corresponding fields empty.
func (s *Server) loadDetailData(templateName string) (detailData, error) {
	tmpl, err := renderer.LoadTemplate(s.templatesDir, templateName)
	if err != nil {
		return detailData{}, err
	}

	jsonBytes, err := gallery.LoadDefaultJSON(s.templatesDir, templateName)
	if err != nil {
		log.Printf("detail: load default.json %q: %v", templateName, err)
	}

	d := detailData{
		Name:          templateName,
		Meta:          tmpl.Meta,
		DefaultJSON:   string(jsonBytes),
		HasDefault:    len(jsonBytes) > 0,
		EditorEnabled: s.editorEnabled,
		Animated:      tmpl.IsAnimated(),
	}
	if len(jsonBytes) == 0 {
		return d, nil
	}

	dir, err := renderer.SafeTemplatePath(s.templatesDir, templateName)
	if err != nil {
		log.Printf("detail: template path %q: %v", templateName, err)
		return d, nil
	}
	yamlSrc, err := os.ReadFile(filepath.Join(dir, "template.yaml"))
	if err != nil {
		log.Printf("detail: read template.yaml %q: %v", templateName, err)
		return d, nil
	}
	if d.Form, err = buildForm(yamlSrc, jsonBytes); err != nil {
		log.Printf("detail: build form %q: %v", templateName, err)
		d.Form = nil
	}
	return d, nil
}

// requestFormat returns the output format selected by the "format" query
// parameter (exact match; empty means PNG). On an unknown value it writes a
// 400 response and returns false.
func requestFormat(w http.ResponseWriter, r *http.Request) (string, bool) {
	switch f := r.URL.Query().Get("format"); f {
	case "", formatPNG:
		return formatPNG, true
	case formatGIF:
		return formatGIF, true
	default:
		http.Error(w, "unknown format (use png or gif)", http.StatusBadRequest)
		return "", false
	}
}

// handlePreview renders a template using its default.json and returns PNG,
// or an animated GIF with ?format=gif.
func (s *Server) handlePreview(w http.ResponseWriter, r *http.Request) {
	templateName := r.PathValue("template")
	if err := renderer.ValidateTemplateName(templateName); err != nil {
		http.Error(w, "invalid template name", http.StatusBadRequest)
		return
	}
	format, ok := requestFormat(w, r)
	if !ok {
		return
	}

	jsonBytes, err := gallery.LoadDefaultJSON(s.templatesDir, templateName)
	if err != nil {
		http.Error(w, "no default.json for this template", http.StatusNotFound)
		return
	}

	if format != formatGIF {
		// Reuse the render pipeline (includes caching).
		s.renderAndServe(r.Context(), w, templateName, jsonBytes, format)
		return
	}

	// GIF: time-dependent templates bypass the disk cache; keep their last
	// preview GIF in memory for previewGIFTTL.
	s.previewMu.Lock()
	e, ok := s.previewGIFs[templateName]
	s.previewMu.Unlock()
	if ok && s.now().Sub(e.created) < previewGIFTTL {
		writeImage(w, formatGIF, "PREVIEW", e.data)
		return
	}
	cw := &captureWriter{ResponseWriter: w, status: http.StatusOK}
	s.renderAndServe(r.Context(), cw, templateName, jsonBytes, format)
	// Only BYPASS responses (successful render of an existing, non-cacheable
	// template) are stored, so the map is bounded by the number of templates.
	if cw.status == http.StatusOK && w.Header().Get("X-Cache") == "BYPASS" {
		s.previewMu.Lock()
		s.previewGIFs[templateName] = previewEntry{data: cw.body.Bytes(), created: s.now()}
		s.previewMu.Unlock()
	}
}

func (s *Server) handleRender(w http.ResponseWriter, r *http.Request) {
	corsHeaders(w)

	templateName := r.PathValue("template")
	if err := renderer.ValidateTemplateName(templateName); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	format, ok := requestFormat(w, r)
	if !ok {
		return
	}

	// Read and size-limit the body.
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "request body too large or unreadable", http.StatusBadRequest)
		return
	}

	// Validate JSON.
	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	s.renderAndServe(r.Context(), w, templateName, body, format)
}

// renderCacheKey returns the cache key for rendering templateName with body.
// Besides the body it covers the build version (a new renderer may draw the
// same input differently) and the name, size and mod-time of every file in
// the template directory (template.yaml, images, fonts), so updating either
// the server or a template never serves an image cached by the old state.
func (s *Server) renderCacheKey(templateName string, body []byte) string {
	stamp := append([]byte(version.Version), 0)
	if entries, err := os.ReadDir(filepath.Join(s.templatesDir, templateName)); err == nil {
		for _, e := range entries { // ReadDir sorts by name: stable order
			info, err := e.Info()
			if err != nil || !info.Mode().IsRegular() {
				continue
			}
			stamp = append(stamp, e.Name()...)
			stamp = append(stamp, 0)
			stamp = strconv.AppendInt(stamp, info.Size(), 10)
			stamp = append(stamp, 0)
			stamp = strconv.AppendInt(stamp, info.ModTime().UnixNano(), 10)
			stamp = append(stamp, 0)
		}
	}
	// body must not be aliased: append to a fresh slice.
	return s.cache.Key(templateName, append(stamp, body...))
}

// renderAndServe renders templateName with the given JSON body and writes the
// result to w as PNG or, for formatGIF, as an animated GIF (a single frame for
// templates that are not animated). format must be formatPNG or formatGIF.
// It checks and populates the cache automatically; both formats share one
// cache key and differ in file extension. GIF cache misses wait for a slot
// in gifSem (503 if ctx ends or the wait times out) and then check the cache
// again; cache hits never wait.
//
// The template is loaded before the cache lookup. Time-dependent templates
// (UsesNow) bypass the cache completely: no lookup, no store, not even after
// the GIF wait; the response carries X-Cache: BYPASS and Cache-Control:
// no-store. GIF renders still go through gifSem.
func (s *Server) renderAndServe(ctx context.Context, w http.ResponseWriter, templateName string, body []byte, format string) {
	// Cache lookup.
	// Note: there is no lock between Get and Set, so concurrent requests with
	// the same key may each render and write the same PNG. This is intentional
	// (last-write-wins with identical bytes) and avoids the complexity of
	// singleflight deduplication for a low-traffic local server.
	key := s.renderCacheKey(templateName, body)

	// Load template (404 if missing); it decides whether the cache is usable.
	tmpl, err := renderer.LoadTemplate(s.templatesDir, templateName)
	if err != nil {
		log.Printf("render: load template %q: %v", templateName, err)
		http.Error(w, "template not found", http.StatusNotFound)
		return
	}
	cacheable := !tmpl.UsesNow()

	if cacheable {
		if cached, hit := s.cache.GetExt(key, format); hit {
			writeImage(w, format, "HIT", cached)
			return
		}
	}

	// Parse JSON data.
	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		http.Error(w, "Ungültiges JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	if format == formatGIF {
		s.renderGIF(ctx, w, templateName, body, key, tmpl, data, cacheable)
		return
	}

	// Render.
	img, err := s.rend.Render(tmpl, data)
	if err != nil {
		log.Printf("render: %q: %v", templateName, err)
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}

	// Optionally reduce color palette.
	var encImg image.Image = img
	if tmpl.Meta.Canvas.Colors > 0 {
		encImg = renderer.Quantize(img, tmpl.Meta.Canvas.Colors)
	}

	// Encode PNG.
	var buf bytes.Buffer
	if err := png.Encode(&buf, encImg); err != nil {
		http.Error(w, "PNG encode error", http.StatusInternalServerError)
		log.Printf("render: png encode %q: %v", templateName, err)
		return
	}
	pngBytes := buf.Bytes()

	cacheState := "BYPASS"
	if cacheable {
		cacheState = "MISS"
		// Persist to cache (best-effort).
		if err := s.cache.Set(key, pngBytes); err != nil {
			log.Printf("cache: set %q: %v", key, err)
		}
	}
	writeImage(w, formatPNG, cacheState, pngBytes)
}

// writeImage writes data as an image response; format is formatPNG or formatGIF,
// which are also the MIME subtypes. cacheState "BYPASS" also forbids caching
// by clients and proxies.
func writeImage(w http.ResponseWriter, format, cacheState string, data []byte) {
	if cacheState == "BYPASS" || cacheState == "PREVIEW" {
		w.Header().Set("Cache-Control", "no-store")
	}
	w.Header().Set("Content-Type", "image/"+format)
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.Header().Set("X-Cache", cacheState)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

// renderGIF renders the animation of tmpl and writes it; when cacheable it
// caches the result under key, otherwise it never touches the cache.
func (s *Server) renderGIF(ctx context.Context, w http.ResponseWriter, templateName string, body []byte, key string, tmpl *renderer.Template, data map[string]interface{}, cacheable bool) {
	wait := time.NewTimer(s.gifWaitTimeout)
	defer wait.Stop()
	select {
	case s.gifSem <- struct{}{}:
		defer func() { <-s.gifSem }()
	case <-ctx.Done():
		http.Error(w, "request cancelled while waiting for render slot", http.StatusServiceUnavailable)
		return
	case <-wait.C:
		w.Header().Set("Retry-After", "5")
		http.Error(w, "all render slots busy, try again", http.StatusServiceUnavailable)
		return
	}
	if ctx.Err() != nil {
		return // client gone: do not spend the slot on it
	}
	// Another request may have rendered the same GIF while this one waited.
	if cacheable {
		if cached, hit := s.cache.GetExt(key, formatGIF); hit {
			writeImage(w, formatGIF, "HIT", cached)
			return
		}
	}

	anim, err := s.rend.RenderAnimation(tmpl, data, renderer.AnimationSeed(templateName, body))
	if err != nil {
		log.Printf("render: animation %q: %v", templateName, err)
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}
	var buf bytes.Buffer
	if err := renderer.EncodeGIF(&buf, anim, tmpl.Meta.Canvas.Colors); err != nil {
		log.Printf("render: gif encode %q: %v", templateName, err)
		http.Error(w, "GIF encode error", http.StatusInternalServerError)
		return
	}
	gifBytes := buf.Bytes()

	if !cacheable {
		writeImage(w, formatGIF, "BYPASS", gifBytes)
		return
	}
	if err := s.cache.SetExt(key, formatGIF, gifBytes); err != nil {
		log.Printf("cache: set %q: %v", key, err)
	}

	writeImage(w, formatGIF, "MISS", gifBytes)
}
