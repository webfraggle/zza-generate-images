package devpreview

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/webfraggle/zza-generate-images/internal/devexport"
)

func serveExport(h *Handler, name string) *httptest.ResponseRecorder {
	mux := http.NewServeMux()
	mux.Handle(ExportPattern, h.ExportHandler())
	req := httptest.NewRequest(http.MethodGet, "/edit/"+name+"/zza-export", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func TestExportZIP(t *testing.T) {
	rec := serveExport(New(shipped), "faltblatt-105-v1")
	if rec.Code != 200 {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/zip" {
		t.Errorf("content-type %q", ct)
	}
	if cd := rec.Header().Get("Content-Disposition"); cd != `attachment; filename="faltblatt-105-v1-zza.zip"` {
		t.Errorf("content-disposition %q", cd)
	}
	if cl := rec.Header().Get("Content-Length"); cl != strconv.Itoa(rec.Body.Len()) {
		t.Errorf("content-length %q, body %d", cl, rec.Body.Len())
	}
	if _, err := devexport.ReadZIP(rec.Body.Bytes()); err != nil {
		t.Fatalf("response is not a readable package: %v", err)
	}
}

func TestExportIncompatible422(t *testing.T) {
	rec := serveExport(New(incompatibleTemplate(t)), "small")
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	if body := rec.Body.String(); !strings.HasPrefix(body, "nicht ZZA-kompatibel:\n") || !strings.Contains(body, "240 × 240") {
		t.Errorf("body %q", body)
	}
}

func TestExportRequestErrors(t *testing.T) {
	h := New(shipped)
	if rec := serveExport(h, "bad$name"); rec.Code != 400 {
		t.Errorf("invalid name: %d", rec.Code)
	}
	if rec := serveExport(h, "does-not-exist"); rec.Code != 404 {
		t.Errorf("missing template: %d", rec.Code)
	}
}
