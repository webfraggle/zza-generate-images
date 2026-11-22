package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildHandler_EventsRoute(t *testing.T) {
	dir := t.TempDir()
	dst := filepath.Join(dir, "sbb-105-v1")
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	src, err := os.ReadFile(filepath.Join("..", "..", "templates", "sbb-105-v1", "template.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dst, "template.yaml"), src, 0o644); err != nil {
		t.Fatal(err)
	}

	h, stop, err := buildHandler(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(stop)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/edit/sbb-105-v1/events", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("got %d: %s", rr.Code, rr.Body.String())
	}
	var out map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if _, ok := out["seq"]; !ok {
		t.Errorf("no seq in %s", rr.Body.String())
	}
}

// The GUI's pages must route downloads to the native save dialog; `zza serve`
// (a real browser) must not.
func TestBuildHandler_NativeSaveFlagOnPages(t *testing.T) {
	dir := t.TempDir()
	dst := filepath.Join(dir, "sbb-105-v1")
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	src, err := os.ReadFile(filepath.Join("..", "..", "templates", "sbb-105-v1", "template.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dst, "template.yaml"), src, 0o644); err != nil {
		t.Fatal(err)
	}
	h, stop, err := buildHandler(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(stop)

	page := func(path string) string {
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, path, nil))
		if rr.Code != http.StatusOK {
			t.Fatalf("%s: got %d", path, rr.Code)
		}
		return rr.Body.String()
	}
	for _, path := range []string{"/sbb-105-v1", "/edit/sbb-105-v1"} {
		body := page(path)
		if !strings.Contains(body, `src="/static/save.js"`) {
			t.Errorf("%s: save.js not included", path)
		}
		if strings.Contains(body, "ZZA_NATIVE_SAVE = true") {
			t.Errorf("%s: native save set without SetNativeSave", path)
		}
	}
	h.SetNativeSave(true)
	for _, path := range []string{"/sbb-105-v1", "/edit/sbb-105-v1"} {
		if !strings.Contains(page(path), "ZZA_NATIVE_SAVE = true") {
			t.Errorf("%s: native save flag missing", path)
		}
	}
	if !strings.Contains(page("/static/save.js"), "window.zzaSave") {
		t.Error("save.js not served")
	}
}
