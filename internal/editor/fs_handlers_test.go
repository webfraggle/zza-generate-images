package editor

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newEditorTestDir(t *testing.T) string {
	t.Helper()
	d := t.TempDir()
	tpl := filepath.Join(d, "mine")
	if err := os.MkdirAll(tpl, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tpl, "template.yaml"),
		[]byte("meta:\n  canvas: {width: 10, height: 10}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tpl, "default.json"),
		[]byte(`{"x":1}`), 0o644); err != nil {
		t.Fatal(err)
	}
	return d
}

func TestFSHandlers_ListFiles(t *testing.T) {
	h := NewFSHandlers(newEditorTestDir(t), nil)
	req := httptest.NewRequest(http.MethodGet, "/edit/mine/files", nil)
	req.SetPathValue("template", "mine")
	rr := httptest.NewRecorder()
	h.ListFiles(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("got %d", rr.Code)
	}
	var out struct{ Files []FileInfo }
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Files) != 2 {
		t.Errorf("want 2 files, got %d", len(out.Files))
	}
}

func TestFSHandlers_GetFile(t *testing.T) {
	h := NewFSHandlers(newEditorTestDir(t), nil)
	req := httptest.NewRequest(http.MethodGet, "/edit/mine/file/template.yaml", nil)
	req.SetPathValue("template", "mine")
	req.SetPathValue("filename", "template.yaml")
	rr := httptest.NewRecorder()
	h.GetFile(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("got %d, body: %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "canvas:") {
		t.Error("body should contain YAML")
	}
}

func TestFSHandlers_Save_WritesAndInvalidates(t *testing.T) {
	dir := newEditorTestDir(t)
	var cacheCalls int
	h := NewFSHandlers(dir, func(string) { cacheCalls++ })

	body, _ := json.Marshal(map[string]string{
		"filename": "template.yaml",
		"content":  "meta:\n  canvas: {width: 20, height: 20}\n",
	})
	req := httptest.NewRequest(http.MethodPost, "/edit/mine/save", bytes.NewReader(body))
	req.SetPathValue("template", "mine")
	rr := httptest.NewRecorder()
	h.Save(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("got %d, body: %s", rr.Code, rr.Body.String())
	}
	got, _ := os.ReadFile(filepath.Join(dir, "mine", "template.yaml"))
	if !strings.Contains(string(got), "width: 20") {
		t.Errorf("file not written: %q", got)
	}
	if cacheCalls != 1 {
		t.Errorf("want 1 cache invalidation, got %d", cacheCalls)
	}
}

func TestFSHandlers_Save_InvalidYAML(t *testing.T) {
	h := NewFSHandlers(newEditorTestDir(t), nil)
	body, _ := json.Marshal(map[string]string{
		"filename": "template.yaml",
		"content":  "this: is: not valid: yaml: ][",
	})
	req := httptest.NewRequest(http.MethodPost, "/edit/mine/save", bytes.NewReader(body))
	req.SetPathValue("template", "mine")
	rr := httptest.NewRecorder()
	h.Save(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("invalid YAML should give 400, got %d", rr.Code)
	}
}

func TestFSHandlers_Upload(t *testing.T) {
	dir := newEditorTestDir(t)
	h := NewFSHandlers(dir, nil)

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, _ := mw.CreateFormFile("file", "logo.png")
	// 8-byte PNG magic so file contents are non-zero:
	_, _ = io.Copy(fw, bytes.NewReader([]byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}))
	mw.Close()

	req := httptest.NewRequest(http.MethodPost, "/edit/mine/upload", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.SetPathValue("template", "mine")
	rr := httptest.NewRecorder()
	h.Upload(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("got %d, body: %s", rr.Code, rr.Body.String())
	}
	if _, err := os.Stat(filepath.Join(dir, "mine", "logo.png")); err != nil {
		t.Errorf("uploaded file missing: %v", err)
	}
}

func TestFSHandlers_DeleteFile(t *testing.T) {
	dir := newEditorTestDir(t)
	_ = os.WriteFile(filepath.Join(dir, "mine", "extra.png"), []byte("x"), 0o644)
	h := NewFSHandlers(dir, nil)

	req := httptest.NewRequest(http.MethodDelete, "/edit/mine/file/extra.png", nil)
	req.SetPathValue("template", "mine")
	req.SetPathValue("filename", "extra.png")
	rr := httptest.NewRecorder()
	h.DeleteFile(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Errorf("got %d", rr.Code)
	}
	if _, err := os.Stat(filepath.Join(dir, "mine", "extra.png")); !os.IsNotExist(err) {
		t.Error("file should be removed")
	}
}

func TestFSHandlers_DeleteProtectedRefused(t *testing.T) {
	h := NewFSHandlers(newEditorTestDir(t), nil)
	req := httptest.NewRequest(http.MethodDelete, "/edit/mine/file/template.yaml", nil)
	req.SetPathValue("template", "mine")
	req.SetPathValue("filename", "template.yaml")
	rr := httptest.NewRecorder()
	h.DeleteFile(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Errorf("template.yaml delete should be forbidden, got %d", rr.Code)
	}
}
