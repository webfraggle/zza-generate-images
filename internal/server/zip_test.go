package server

import (
	"archive/zip"
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/webfraggle/zza-generate-images/internal/config"
	"github.com/webfraggle/zza-generate-images/web"
)

func TestServer_TemplateZip_ContainsExpectedFiles(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/sbb-096-v1.zip", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("zip: got %d, body: %s", rr.Code, rr.Body.String())
	}
	if ct := rr.Header().Get("Content-Type"); ct != "application/zip" {
		t.Errorf("Content-Type: got %q, want application/zip", ct)
	}
	if cd := rr.Header().Get("Content-Disposition"); cd == "" {
		t.Error("missing Content-Disposition header")
	}

	zr, err := zip.NewReader(bytes.NewReader(rr.Body.Bytes()), int64(rr.Body.Len()))
	if err != nil {
		t.Fatalf("read zip: %v", err)
	}
	want := map[string]bool{"template.yaml": false, "default.json": false}
	for _, f := range zr.File {
		if _, ok := want[f.Name]; ok {
			want[f.Name] = true
			rc, err := f.Open()
			if err != nil {
				t.Fatalf("open %q: %v", f.Name, err)
			}
			if _, err := io.Copy(io.Discard, rc); err != nil {
				t.Errorf("read %q: %v", f.Name, err)
			}
			_ = rc.Close()
		}
	}
	for name, found := range want {
		if !found {
			t.Errorf("zip missing %q", name)
		}
	}
}

func TestServer_TemplateZip_InvalidName(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/..%2Fevil.zip", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	if rr.Code == http.StatusOK {
		t.Error("path traversal should not return 200")
	}
}

func TestServer_TemplateZip_NotFound(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/does-not-exist.zip", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("got %d, want 404", rr.Code)
	}
}

func TestServer_TemplateZip_SkipsSymlinks(t *testing.T) {
	// Build an isolated templates dir with one real file and one symlink
	// pointing at a secret outside the template directory.
	tmp := t.TempDir()
	secretDir := t.TempDir()
	secret := filepath.Join(secretDir, "secret.txt")
	if err := os.WriteFile(secret, []byte("TOP SECRET"), 0o600); err != nil {
		t.Fatal(err)
	}

	templatesDir := filepath.Join(tmp, "templates")
	tplDir := filepath.Join(templatesDir, "leaky")
	if err := os.MkdirAll(tplDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tplDir, "template.yaml"),
		[]byte("meta:\n  canvas: {width: 10, height: 10}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(secret, filepath.Join(tplDir, "leak.txt")); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		Port:             "8080",
		TemplatesDir:     templatesDir,
		CacheDir:         t.TempDir(),
		CacheMaxAgeHours: 1,
		CacheMaxSizeMB:   10,
	}
	srv, err := New(cfg, web.FS)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/leaky.zip", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("zip: got %d, body: %s", rr.Code, rr.Body.String())
	}

	zr, err := zip.NewReader(bytes.NewReader(rr.Body.Bytes()), int64(rr.Body.Len()))
	if err != nil {
		t.Fatalf("read zip: %v", err)
	}
	for _, f := range zr.File {
		if f.Name == "leak.txt" {
			t.Error("symlinked file must not be in the ZIP")
		}
		if rc, err := f.Open(); err == nil {
			b, _ := io.ReadAll(rc)
			_ = rc.Close()
			if bytes.Contains(b, []byte("TOP SECRET")) {
				t.Errorf("ZIP contains secret content (file %q)", f.Name)
			}
		}
	}
}
