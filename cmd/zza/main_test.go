package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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

	h, err := buildHandler(dir)
	if err != nil {
		t.Fatal(err)
	}
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
