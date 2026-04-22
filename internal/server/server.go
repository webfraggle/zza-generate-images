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
	"time"

	"github.com/webfraggle/zza-generate-images/internal/config"
	"github.com/webfraggle/zza-generate-images/internal/editor"
	"github.com/webfraggle/zza-generate-images/internal/gallery"
	"github.com/webfraggle/zza-generate-images/internal/renderer"
	"github.com/webfraggle/zza-generate-images/internal/version"
)

const maxRequestBodyBytes = 1 << 20 // 1 MiB

// Server handles HTTP requests for the ZZA image renderer.
type Server struct {
	mux           *http.ServeMux
	staticHandler http.Handler
	editorHandler http.Handler // set by RegisterEditor; desktop-only
	rend          *renderer.Renderer
	cache         *Cache
	templatesDir  string
	htmlTmpl      *template.Template
	editorEnabled bool
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
		mux:           http.NewServeMux(),
		staticHandler: http.StripPrefix("/static/", http.FileServerFS(staticFS)),
		rend:          renderer.New(cfg.TemplatesDir),
		cache:         cache,
		templatesDir:  cfg.TemplatesDir,
		htmlTmpl:      tmpl,
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
	if strings.HasPrefix(r.URL.Path, "/edit/") && s.editorHandler != nil {
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

// RegisterEditor attaches an FSHandlers set to this server. The editor page
// itself is rendered here (to re-use the shared html/template set).
// Desktop-build calls this; server-build does not.
func (s *Server) RegisterEditor(h *editor.FSHandlers) {
	mux := http.NewServeMux()
	h.Register(mux)
	// Override the placeholder EditorPage with one that uses our html/template.
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
	if err := s.htmlTmpl.ExecuteTemplate(w, "edit-editor.html", editorPageData{TemplateName: name}); err != nil {
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

// handleGallery renders the template gallery overview page.
func (s *Server) handleGallery(w http.ResponseWriter, r *http.Request) {
	infos, err := gallery.ListTemplates(s.templatesDir)
	if err != nil {
		http.Error(w, "could not list templates", http.StatusInternalServerError)
		log.Printf("gallery: list: %v", err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.htmlTmpl.ExecuteTemplate(w, "gallery.html", infos); err != nil {
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
}

// handleDetail renders the try-it detail page for a single template.
func (s *Server) handleDetail(w http.ResponseWriter, r *http.Request) {
	templateName := r.PathValue("template")
	if err := renderer.ValidateTemplateName(templateName); err != nil {
		http.Error(w, "invalid template name", http.StatusBadRequest)
		return
	}

	tmpl, err := renderer.LoadTemplate(s.templatesDir, templateName)
	if err != nil {
		http.Error(w, "template not found", http.StatusNotFound)
		return
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
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.htmlTmpl.ExecuteTemplate(w, "detail.html", d); err != nil {
		log.Printf("detail: execute template: %v", err)
	}
}

// handlePreview renders a template using its default.json and returns PNG.
func (s *Server) handlePreview(w http.ResponseWriter, r *http.Request) {
	templateName := r.PathValue("template")
	if err := renderer.ValidateTemplateName(templateName); err != nil {
		http.Error(w, "invalid template name", http.StatusBadRequest)
		return
	}

	jsonBytes, err := gallery.LoadDefaultJSON(s.templatesDir, templateName)
	if err != nil {
		http.Error(w, "no default.json for this template", http.StatusNotFound)
		return
	}

	// Reuse the render pipeline (includes caching).
	s.renderAndServe(w, templateName, jsonBytes)
}

func (s *Server) handleRender(w http.ResponseWriter, r *http.Request) {
	corsHeaders(w)

	templateName := r.PathValue("template")
	if err := renderer.ValidateTemplateName(templateName); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
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

	s.renderAndServe(w, templateName, body)
}

// renderAndServe renders templateName with the given JSON body and writes PNG to w.
// It checks and populates the cache automatically.
func (s *Server) renderAndServe(w http.ResponseWriter, templateName string, body []byte) {
	// Cache lookup.
	// Note: there is no lock between Get and Set, so concurrent requests with
	// the same key may each render and write the same PNG. This is intentional
	// (last-write-wins with identical bytes) and avoids the complexity of
	// singleflight deduplication for a low-traffic local server.
	//
	// The template.yaml mod-time is included in the cache key so that saving
	// the YAML via the editor automatically invalidates stale PNG cache entries.
	var modStamp []byte
	if info, err := os.Stat(filepath.Join(s.templatesDir, templateName, "template.yaml")); err == nil {
		modStamp = strconv.AppendInt(nil, info.ModTime().UnixNano(), 10)
	}
	key := s.cache.Key(templateName, append(body, modStamp...))
	if cached, hit := s.cache.Get(key); hit {
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Content-Length", strconv.Itoa(len(cached)))
		w.Header().Set("X-Cache", "HIT")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(cached)
		return
	}

	// Parse JSON data.
	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		http.Error(w, "Ungültiges JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Load template.
	tmpl, err := renderer.LoadTemplate(s.templatesDir, templateName)
	if err != nil {
		log.Printf("render: load template %q: %v", templateName, err)
		http.Error(w, "template not found", http.StatusNotFound)
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

	// Persist to cache (best-effort).
	if err := s.cache.Set(key, pngBytes); err != nil {
		log.Printf("cache: set %q: %v", key, err)
	}

	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Content-Length", strconv.Itoa(len(pngBytes)))
	w.Header().Set("X-Cache", "MISS")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(pngBytes)
}
