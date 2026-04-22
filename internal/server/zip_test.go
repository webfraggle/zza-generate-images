package server

import (
	"archive/zip"
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
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
