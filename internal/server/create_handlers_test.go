// internal/server/create_handlers_test.go
package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/webfraggle/zza-generate-images/internal/config"
	"github.com/webfraggle/zza-generate-images/internal/db"
	"github.com/webfraggle/zza-generate-images/web"
)

// newTestCreateServer returns a Server with create routes registered,
// using an isolated temp dir for templates and an in-memory SQLite DB.
// Returns the server and the temp templates dir path.
func newTestCreateServer(t *testing.T) (*Server, string) {
	t.Helper()
	tdir := t.TempDir()
	srv, err := New(&config.Config{
		Port:             "8080",
		TemplatesDir:     tdir,
		CacheDir:         t.TempDir(),
		CacheMaxAgeHours: 1,
		CacheMaxSizeMB:   10,
	}, web.FS)
	if err != nil {
		t.Fatal(err)
	}
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	srv.RegisterCreateRoutes(database, EditorConfig{TokenTTL: time.Hour})
	return srv, tdir
}

func TestCreateCheck_Available(t *testing.T) {
	srv, _ := newTestCreateServer(t)

	req := httptest.NewRequest(http.MethodGet, "/create-new/check?id=mein-template", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("check: got %d, want 200", rr.Code)
	}
	var resp map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["available"] != true {
		t.Errorf("expected available=true, got %v", resp)
	}
}

func TestCreateCheck_InvalidFormat(t *testing.T) {
	srv, _ := newTestCreateServer(t)

	for _, badID := range []string{"", "UPPER", "has space", "dot.here", strings.Repeat("a", 65)} {
		req := httptest.NewRequest(http.MethodGet, "/create-new/check?id="+url.QueryEscape(badID), nil)
		rr := httptest.NewRecorder()
		srv.ServeHTTP(rr, req)

		var resp map[string]any
		json.NewDecoder(rr.Body).Decode(&resp)
		if resp["available"] != false {
			t.Errorf("id %q: expected available=false, got %v", badID, resp)
		}
	}
}

func TestCreateCheck_DirectoryExists(t *testing.T) {
	srv, tdir := newTestCreateServer(t)

	// Pre-create the directory.
	if err := os.Mkdir(filepath.Join(tdir, "taken"), 0o755); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/create-new/check?id=taken", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	var resp map[string]any
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp["available"] != false {
		t.Errorf("expected available=false for existing dir, got %v", resp)
	}
}

func TestCreateNew_Get(t *testing.T) {
	srv, _ := newTestCreateServer(t)

	req := httptest.NewRequest(http.MethodGet, "/create-new", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("GET /create-new: got %d, want 200", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "Template-ID") {
		t.Error("form page should contain 'Template-ID'")
	}
}

func TestCreateSubmit_CreatesTemplate(t *testing.T) {
	srv, tdir := newTestCreateServer(t)

	body := strings.NewReader("id=mein-template&email=owner@example.com&title=Mein+Template&description=Desc&display=1.05")
	req := httptest.NewRequest(http.MethodPost, "/create-new", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("POST /create-new: got %d, want 200\nbody: %s", rr.Code, rr.Body.String())
	}
	// Response should be the success page.
	if !strings.Contains(rr.Body.String(), "owner@example.com") {
		t.Error("success page should mention the email address")
	}
	// Template directory must have been created.
	if _, err := os.Stat(filepath.Join(tdir, "mein-template", "template.yaml")); err != nil {
		t.Errorf("template.yaml not created: %v", err)
	}
}

func TestCreateSubmit_InvalidID(t *testing.T) {
	srv, _ := newTestCreateServer(t)

	body := strings.NewReader("id=INVALID+ID&email=x@example.com&title=T&description=&display=1.05")
	req := httptest.NewRequest(http.MethodPost, "/create-new", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "Ungültige Template-ID") {
		t.Errorf("expected error message, got: %s", rr.Body.String())
	}
}

func TestCreateSubmit_DuplicateID(t *testing.T) {
	srv, tdir := newTestCreateServer(t)

	// Pre-create the directory.
	if err := os.Mkdir(filepath.Join(tdir, "taken"), 0o755); err != nil {
		t.Fatal(err)
	}

	body := strings.NewReader("id=taken&email=x@example.com&title=T&description=&display=1.05")
	req := httptest.NewRequest(http.MethodPost, "/create-new", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if !strings.Contains(rr.Body.String(), "bereits vergeben") {
		t.Errorf("expected 'bereits vergeben', got: %s", rr.Body.String())
	}
}
