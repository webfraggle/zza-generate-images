package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image/png"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/webfraggle/zza-generate-images/internal/config"
	"github.com/webfraggle/zza-generate-images/internal/renderer"
)

const maxRequestBodyBytes = 1 << 20 // 1 MiB

// Server handles HTTP requests for the ZZA image renderer.
type Server struct {
	mux          *http.ServeMux
	rend         *renderer.Renderer
	cache        *Cache
	templatesDir string
}

// New creates and initialises a Server from cfg.
// The cache cleanup goroutine is started automatically and runs until the
// process exits (no shutdown hook needed for cache cleanup).
func New(cfg *config.Config) (*Server, error) {
	cache, err := NewCache(
		cfg.CacheDir,
		time.Duration(cfg.CacheMaxAgeHours)*time.Hour,
		cfg.CacheMaxSizeMB,
	)
	if err != nil {
		return nil, fmt.Errorf("server: %w", err)
	}

	s := &Server{
		mux:          http.NewServeMux(),
		rend:         renderer.New(cfg.TemplatesDir),
		cache:        cache,
		templatesDir: cfg.TemplatesDir,
	}
	s.registerRoutes()
	return s, nil
}

// ServeHTTP implements http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

// StartCleanup delegates to the cache's cleanup goroutine.
func (s *Server) StartCleanup(ctx context.Context, interval time.Duration) {
	s.cache.StartCleanup(ctx, interval)
}

func (s *Server) registerRoutes() {
	// POST /{template}/render — render JSON to PNG
	s.mux.HandleFunc("POST /{template}/render", s.handleRender)
	// OPTIONS /{template}/render — CORS preflight
	s.mux.HandleFunc("OPTIONS /{template}/render", s.handleOptions)
	// GET /health — liveness check
	s.mux.HandleFunc("GET /health", s.handleHealth)
}

// corsHeaders sets permissive CORS headers.
// This server is intended for local/intranet use, so wildcard origin is acceptable.
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

	// Validate JSON (parse once for validation, reuse raw bytes for cache key).
	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Cache lookup.
	// Note: there is no lock between Get and Set, so concurrent requests with
	// the same key may each render and write the same PNG. This is intentional
	// (last-write-wins with identical bytes) and avoids the complexity of
	// singleflight deduplication for a low-traffic local server.
	key := s.cache.Key(templateName, body)
	if cached, hit := s.cache.Get(key); hit {
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Content-Length", strconv.Itoa(len(cached)))
		w.Header().Set("X-Cache", "HIT")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(cached)
		return
	}

	// Load template.
	tmpl, err := renderer.LoadTemplate(s.templatesDir, templateName)
	if err != nil {
		// Distinguish "not found" from other errors.
		http.Error(w, "template not found: "+templateName, http.StatusNotFound)
		log.Printf("render: load template %q: %v", templateName, err)
		return
	}

	// Render.
	img, err := s.rend.Render(tmpl, data)
	if err != nil {
		http.Error(w, "render error", http.StatusInternalServerError)
		log.Printf("render: %q: %v", templateName, err)
		return
	}

	// Encode PNG into a buffer so we can cache it and serve it.
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		http.Error(w, "PNG encode error", http.StatusInternalServerError)
		log.Printf("render: png encode %q: %v", templateName, err)
		return
	}
	pngBytes := buf.Bytes()

	// Persist to cache (best-effort: don't fail the request on cache write error).
	if err := s.cache.Set(key, pngBytes); err != nil {
		log.Printf("cache: set %q: %v", key, err)
	}

	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Content-Length", strconv.Itoa(len(pngBytes)))
	w.Header().Set("X-Cache", "MISS")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(pngBytes)
}
