package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/webfraggle/zza-generate-images/internal/config"
	"github.com/webfraggle/zza-generate-images/web"
)

// testConfig returns a Config pointing at the repo's test templates.
func testConfig(t *testing.T) *config.Config {
	t.Helper()
	return &config.Config{
		Port:             "8080",
		TemplatesDir:     filepath.Join("..", "..", "templates"),
		CacheDir:         t.TempDir(),
		CacheMaxAgeHours: 1,
		CacheMaxSizeMB:   10,
	}
}

func newTestServer(t *testing.T) *Server {
	t.Helper()
	srv, err := New(testConfig(t), web.FS)
	if err != nil {
		t.Fatal(err)
	}
	return srv
}

func TestServer_Health(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("health: got %d, want 200", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "ok") {
		t.Errorf("health body: %q", rr.Body.String())
	}
}

func TestServer_CORS_Preflight(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest(http.MethodOptions, "/sbb-096-v1/render", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Errorf("options: got %d, want 204", rr.Code)
	}
	if rr.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Error("missing CORS header")
	}
}

func TestServer_Gallery(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("gallery: got %d, want 200", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("gallery Content-Type: %q", ct)
	}
	if !strings.Contains(rr.Body.String(), "sbb-096-v1") {
		t.Error("gallery should list sbb-096-v1")
	}
}

func TestServer_Detail(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/sbb-096-v1", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("detail: got %d, want 200", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "sbb-096-v1") {
		t.Error("detail page should contain template name")
	}
	if !strings.Contains(body, "json-input") {
		t.Error("detail page should contain JSON textarea")
	}
}

func TestServer_Detail_NotFound(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/does-not-exist", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("detail 404: got %d, want 404", rr.Code)
	}
}

func TestServer_Preview(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/sbb-096-v1/preview", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("preview: got %d, body: %s", rr.Code, rr.Body.String())
	}
	if ct := rr.Header().Get("Content-Type"); ct != "image/png" {
		t.Errorf("preview Content-Type: %q", ct)
	}
	b := rr.Body.Bytes()
	if len(b) < 8 || b[0] != 0x89 || b[1] != 0x50 {
		t.Error("preview response is not a valid PNG")
	}
}

func TestServer_Render_InvalidTemplateName(t *testing.T) {
	srv := newTestServer(t)

	body := `{"gleis":"3"}`
	req := httptest.NewRequest(http.MethodPost, "/../../evil/render",
		strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code == http.StatusOK {
		t.Error("path traversal attempt should not return 200")
	}
}

func TestServer_Render_InvalidJSON(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest(http.MethodPost, "/sbb-096-v1/render",
		strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("invalid JSON: got %d, want 400", rr.Code)
	}
}

func TestServer_Render_UnknownTemplate(t *testing.T) {
	srv := newTestServer(t)

	body, _ := json.Marshal(map[string]interface{}{"x": 1})
	req := httptest.NewRequest(http.MethodPost, "/does-not-exist/render",
		strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("unknown template: got %d, want 404", rr.Code)
	}
}

func TestServer_Render_Success(t *testing.T) {
	srv := newTestServer(t)

	data, _ := json.Marshal(map[string]interface{}{
		"zug1": map[string]interface{}{
			"zeit":    "16:00",
			"vonnach": "Zürich HB",
			"nr":      "IC1",
			"hinweis": "",
			"abw":     0,
		},
		"gleis": "7",
	})
	req := httptest.NewRequest(http.MethodPost, "/sbb-096-v1/render",
		strings.NewReader(string(data)))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("render: got %d, body: %s", rr.Code, rr.Body.String())
	}
	if ct := rr.Header().Get("Content-Type"); ct != "image/png" {
		t.Errorf("Content-Type: got %q, want image/png", ct)
	}
	b := rr.Body.Bytes()
	if len(b) < 8 || b[0] != 0x89 || b[1] != 0x50 || b[2] != 0x4E || b[3] != 0x47 {
		t.Error("response body is not a valid PNG")
	}
}

func TestServer_Render_CacheHit(t *testing.T) {
	srv := newTestServer(t)

	data, _ := json.Marshal(map[string]interface{}{
		"zug1": map[string]interface{}{
			"zeit":    "16:00",
			"vonnach": "Bern",
			"nr":      "RE42",
			"hinweis": "",
			"abw":     0,
		},
		"gleis": "2",
	})

	makeReq := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/sbb-096-v1/render",
			strings.NewReader(string(data)))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		srv.ServeHTTP(rr, req)
		return rr
	}

	rr1 := makeReq()
	if rr1.Code != http.StatusOK {
		t.Fatalf("first request: got %d", rr1.Code)
	}
	rr2 := makeReq()
	if rr2.Code != http.StatusOK {
		t.Fatalf("second request: got %d", rr2.Code)
	}
	if rr2.Header().Get("X-Cache") != "HIT" {
		t.Errorf("second request should be cache HIT, got %q", rr2.Header().Get("X-Cache"))
	}
}
