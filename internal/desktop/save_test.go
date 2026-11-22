package desktop

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newTestSaver(path string, gotName *string) *Saver {
	s := &Saver{dialog: func(_ context.Context, name string) (string, error) {
		if gotName != nil {
			*gotName = name
		}
		return path, nil
	}}
	s.attach(context.Background())
	return s
}

func postSave(s *Saver, name, body string) *httptest.ResponseRecorder {
	rr := httptest.NewRecorder()
	enc := base64.StdEncoding.EncodeToString([]byte(body))
	req := httptest.NewRequest(http.MethodPost, "/edit/_desktop/save-file?name="+name, strings.NewReader(enc))
	s.ServeHTTP(rr, req)
	return rr
}

func TestSaver_WritesFileToChosenPath(t *testing.T) {
	target := filepath.Join(t.TempDir(), "out.png")
	var suggested string
	rr := postSave(newTestSaver(target, &suggested), "sbb-105-v1.png", "PNGDATA")

	if rr.Code != http.StatusOK {
		t.Fatalf("got %d: %s", rr.Code, rr.Body.String())
	}
	if suggested != "sbb-105-v1.png" {
		t.Errorf("dialog got name %q", suggested)
	}
	got, err := os.ReadFile(target)
	if err != nil || string(got) != "PNGDATA" {
		t.Fatalf("file = %q, %v", got, err)
	}
	var out struct {
		Saved bool
		Path  string
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil || !out.Saved || out.Path != target {
		t.Errorf("response %s (%v)", rr.Body.String(), err)
	}
}

func TestSaver_CancelWritesNothing(t *testing.T) {
	rr := postSave(newTestSaver("", nil), "a.zip", "x")
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"saved":false`) {
		t.Fatalf("got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestSaver_NoWindowIs501(t *testing.T) {
	s := NewSaver() // never attached: browser fallback
	rr := postSave(s, "a.zip", "x")
	if rr.Code != http.StatusNotImplemented {
		t.Fatalf("got %d", rr.Code)
	}
}

func TestSaver_NameIsReducedToBaseName(t *testing.T) {
	var suggested string
	target := filepath.Join(t.TempDir(), "x")
	postSave(newTestSaver(target, &suggested), "..%2F..%2Fetc%2Fpasswd", "x")
	if suggested != "passwd" {
		t.Errorf("dialog got name %q", suggested)
	}
	if rr := postSave(newTestSaver(target, nil), "", "x"); rr.Code != http.StatusBadRequest {
		t.Errorf("empty name: got %d", rr.Code)
	}
	if rr := postSave(newTestSaver(target, nil), "..", "x"); rr.Code != http.StatusBadRequest {
		t.Errorf("dot-dot name: got %d", rr.Code)
	}
}

func TestSaver_EmptyOrBrokenBodyWritesNothing(t *testing.T) {
	target := filepath.Join(t.TempDir(), "out.zip")
	if rr := postSave(newTestSaver(target, nil), "a.zip", ""); rr.Code != http.StatusBadRequest {
		t.Errorf("empty body: got %d", rr.Code)
	}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/edit/_desktop/save-file?name=a.zip", strings.NewReader("\x00\x01 not base64"))
	newTestSaver(target, nil).ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("raw body: got %d", rr.Code)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Errorf("file was written: %v", err)
	}
}

func TestSaver_BinaryRoundTrip(t *testing.T) {
	target := filepath.Join(t.TempDir(), "out.bin")
	data := make([]byte, 70000)
	for i := range data {
		data[i] = byte(i * 7)
	}
	if rr := postSave(newTestSaver(target, nil), "out.bin", string(data)); rr.Code != http.StatusOK {
		t.Fatalf("got %d: %s", rr.Code, rr.Body.String())
	}
	got, err := os.ReadFile(target)
	if err != nil || string(got) != string(data) {
		t.Fatalf("round trip differs (%d bytes, %v)", len(got), err)
	}
}
