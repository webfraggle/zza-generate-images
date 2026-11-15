package editor

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

func saveReq(t *testing.T, h *FSHandlers, filename, content, ifMatch string) *httptest.ResponseRecorder {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"filename": filename, "content": content})
	req := httptest.NewRequest(http.MethodPost, "/edit/mine/save", bytes.NewReader(body))
	req.SetPathValue("template", "mine")
	if ifMatch != "" {
		req.Header.Set("If-Match", ifMatch)
	}
	rr := httptest.NewRecorder()
	h.Save(rr, req)
	return rr
}

func TestFSHandlers_GetFile_ETag(t *testing.T) {
	h := NewFSHandlers(newEditorTestDir(t), nil)
	req := httptest.NewRequest(http.MethodGet, "/edit/mine/file/default.json", nil)
	req.SetPathValue("template", "mine")
	req.SetPathValue("filename", "default.json")
	rr := httptest.NewRecorder()
	h.GetFile(rr, req)
	want := `"` + FileVersion([]byte(`{"x":1}`)) + `"`
	if got := rr.Header().Get("ETag"); got != want {
		t.Errorf("ETag = %q, want %q", got, want)
	}
}

func TestFSHandlers_Save_IfMatchOK(t *testing.T) {
	h := NewFSHandlers(newEditorTestDir(t), nil)
	cur := `"` + FileVersion([]byte(`{"x":1}`)) + `"`
	rr := saveReq(t, h, "default.json", `{"x":2}`, cur)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("got %d: %s", rr.Code, rr.Body.String())
	}
	want := `"` + FileVersion([]byte(`{"x":2}`)) + `"`
	if got := rr.Header().Get("ETag"); got != want {
		t.Errorf("ETag = %q, want %q", got, want)
	}
}

func TestFSHandlers_Save_IfMatchConflict(t *testing.T) {
	dir := newEditorTestDir(t)
	called := false
	h := NewFSHandlers(dir, func(string) { called = true })
	rr := saveReq(t, h, "default.json", `{"x":2}`, `"0000000000000000"`)
	if rr.Code != http.StatusConflict {
		t.Fatalf("got %d", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("content-type %q", ct)
	}
	var out map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out["error"] != "conflict" || out["current_version"] != FileVersion([]byte(`{"x":1}`)) {
		t.Errorf("body %v", out)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "mine", "default.json"))
	if string(data) != `{"x":1}` {
		t.Errorf("file changed: %s", data)
	}
	if called {
		t.Error("Invalidate called on conflict")
	}
}

func TestFSHandlers_Save_IfMatchWeakPrefix(t *testing.T) {
	h := NewFSHandlers(newEditorTestDir(t), nil)
	cur := `W/"` + FileVersion([]byte(`{"x":1}`)) + `"`
	if rr := saveReq(t, h, "default.json", `{"x":2}`, cur); rr.Code != http.StatusNoContent {
		t.Fatalf("got %d", rr.Code)
	}
}

func TestFSHandlers_Save_IfMatchMissingFile(t *testing.T) {
	h := NewFSHandlers(newEditorTestDir(t), nil)
	rr := saveReq(t, h, "new.json", `{}`, `"abc"`)
	if rr.Code != http.StatusConflict || !strings.Contains(rr.Body.String(), `"current_version":""`) {
		t.Fatalf("got %d %s", rr.Code, rr.Body.String())
	}
}

func TestFSHandlers_Save_IfMatchStarAndAbsent(t *testing.T) {
	h := NewFSHandlers(newEditorTestDir(t), nil)
	if rr := saveReq(t, h, "default.json", `{"x":3}`, "*"); rr.Code != http.StatusNoContent {
		t.Errorf("star: got %d", rr.Code)
	}
	if rr := saveReq(t, h, "default.json", `{"x":4}`, ""); rr.Code != http.StatusNoContent {
		t.Errorf("absent: got %d", rr.Code)
	}
}

type fakeFeed struct {
	gotSince   uint64
	gotTimeout time.Duration
	waited     bool
}

func (f *fakeFeed) Wait(_ context.Context, _ string, since uint64, timeout time.Duration) ChangeBatch {
	f.waited, f.gotSince, f.gotTimeout = true, since, timeout
	return ChangeBatch{Seq: 7, Files: []string{"a.png"}}
}
func (f *fakeFeed) Current() uint64 { return 5 }

func eventsGet(h *FSHandlers, url string) *httptest.ResponseRecorder {
	mux := http.NewServeMux()
	h.Register(mux)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, url, nil))
	return rr
}

func TestFSHandlers_Events(t *testing.T) {
	h := NewFSHandlers(newEditorTestDir(t), nil)
	f := &fakeFeed{}
	h.Feed = f

	rr := eventsGet(h, "/edit/mine/events?since=3")
	if rr.Code != 200 || strings.TrimSpace(rr.Body.String()) != `{"seq":7,"files":["a.png"]}` {
		t.Fatalf("got %d %s", rr.Code, rr.Body.String())
	}
	if rr.Header().Get("Cache-Control") != "no-store" || rr.Header().Get("Content-Type") != "application/json" {
		t.Errorf("headers %v", rr.Header())
	}
	if f.gotSince != 3 || f.gotTimeout != 25*time.Second {
		t.Errorf("since=%d timeout=%v", f.gotSince, f.gotTimeout)
	}

	f.waited = false
	rr = eventsGet(h, "/edit/mine/events")
	if rr.Code != 200 || strings.TrimSpace(rr.Body.String()) != `{"seq":5,"files":[]}` || f.waited {
		t.Errorf("no since: %d %s waited=%v", rr.Code, rr.Body.String(), f.waited)
	}
	if rr := eventsGet(h, "/edit/mine/events?since=abc"); rr.Code != 400 {
		t.Errorf("bad since: %d", rr.Code)
	}
	if rr := eventsGet(h, "/edit/bad!name/events"); rr.Code != 400 {
		t.Errorf("bad name: %d", rr.Code)
	}
	h.Feed = nil
	if rr := eventsGet(h, "/edit/mine/events"); rr.Code != 404 {
		t.Errorf("nil feed: %d", rr.Code)
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
