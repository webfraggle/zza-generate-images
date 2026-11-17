package devpreview

import (
	"bytes"
	"image/gif"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	shipped = "../../templates"
	testFnt = "../../templates/faltblatt-105-v1/NimbusSanL-Bol.otf"
)

func serve(h *Handler, name, query, body string) *httptest.ResponseRecorder {
	mux := http.NewServeMux()
	mux.Handle(Pattern, h)
	req := httptest.NewRequest(http.MethodPost, "/edit/"+name+"/zza-render"+query, strings.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func defaultJSON(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(shipped, name, "default.json"))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestRenderPNG(t *testing.T) {
	rec := serve(New(shipped), "faltblatt-105-v1", "", defaultJSON(t, "faltblatt-105-v1"))
	if rec.Code != 200 || rec.Header().Get("Content-Type") != "image/png" {
		t.Fatalf("status %d type %q body %q", rec.Code, rec.Header().Get("Content-Type"), rec.Body.String())
	}
	img, err := png.Decode(rec.Body)
	if err != nil {
		t.Fatal(err)
	}
	if b := img.Bounds(); b.Dx() != 240 || b.Dy() != 240 {
		t.Errorf("size %v", b)
	}
}

func TestRenderGIF(t *testing.T) {
	// faltblatt animates (split-flap); the GIF has several frames.
	rec := serve(New(shipped), "faltblatt-105-v1", "?format=gif", defaultJSON(t, "faltblatt-105-v1"))
	if rec.Code != 200 || rec.Header().Get("Content-Type") != "image/gif" {
		t.Fatalf("status %d type %q body %q", rec.Code, rec.Header().Get("Content-Type"), rec.Body.String())
	}
	g, err := gif.DecodeAll(bytes.NewReader(rec.Body.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if len(g.Image) < 2 {
		t.Errorf("frames = %d, want > 1", len(g.Image))
	}
}

func TestIncompatible422(t *testing.T) {
	dir := t.TempDir()
	td := filepath.Join(dir, "small")
	if err := os.MkdirAll(td, 0o755); err != nil {
		t.Fatal(err)
	}
	font, err := os.ReadFile(testFnt)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(td, "f.otf"), font, 0o644); err != nil {
		t.Fatal(err)
	}
	y := "meta:\n  name: t\n  canvas:\n    width: 160\n    height: 80\nfonts:\n  - id: b\n    file: f.otf\nlayers:\n" +
		"  - type: text\n    font: b\n    size: 12\n    x: 0\n    y: 0\n    color: \"#fff\"\n    value: \"€\"\n"
	if err := os.WriteFile(filepath.Join(td, "template.yaml"), []byte(y), 0o644); err != nil {
		t.Fatal(err)
	}
	rec := serve(New(dir), "small", "", "{}")
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	body := rec.Body.String()
	for _, w := range []string{"nicht ZZA-kompatibel", "240 × 240", "Latin-1"} {
		if !strings.Contains(body, w) {
			t.Errorf("body %q lacks %q", body, w)
		}
	}
	if strings.Count(body, "\n") < 2 {
		t.Errorf("reasons not one per line: %q", body)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/plain") {
		t.Errorf("content-type %q", ct)
	}
}

func TestRequestErrors(t *testing.T) {
	h := New(shipped)
	if rec := serve(h, "..%2Fx", "", "{}"); rec.Code != 400 && rec.Code != 404 {
		t.Errorf("bad name: %d", rec.Code)
	}
	if rec := serve(h, "bad$name", "", "{}"); rec.Code != 400 {
		t.Errorf("invalid name: %d", rec.Code)
	}
	if rec := serve(h, "does-not-exist", "", "{}"); rec.Code != 404 {
		t.Errorf("missing template: %d", rec.Code)
	}
	if rec := serve(h, "faltblatt-105-v1", "", "{nope"); rec.Code != 400 {
		t.Errorf("bad JSON: %d", rec.Code)
	}
	if rec := serve(h, "faltblatt-105-v1", "?format=bmp", "{}"); rec.Code != 400 {
		t.Errorf("bad format: %d", rec.Code)
	}
	big := `{"x":"` + strings.Repeat("a", maxBodyBytes) + `"}`
	if rec := serve(h, "faltblatt-105-v1", "", big); rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("oversize: %d", rec.Code)
	}
}

func TestDataOver256KB400(t *testing.T) {
	big := `{"x":"` + strings.Repeat("a", 300<<10) + `"}`
	rec := serve(New(shipped), "faltblatt-105-v1", "", big)
	if rec.Code != 400 {
		t.Errorf("status %d: %.200s", rec.Code, rec.Body)
	}
}
