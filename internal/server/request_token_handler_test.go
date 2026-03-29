package server

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/webfraggle/zza-generate-images/internal/config"
	"github.com/webfraggle/zza-generate-images/internal/db"
	"github.com/webfraggle/zza-generate-images/web"
)

func newTestEditorServer(t *testing.T) (*Server, *sql.DB) {
	t.Helper()
	srv, err := New(&config.Config{
		Port:             "8080",
		TemplatesDir:     t.TempDir(),
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
	srv.RegisterEditorRoutes(database, EditorConfig{TokenTTL: time.Hour})
	return srv, database
}

func postRequestToken(t *testing.T, srv *Server, templateName, email string) map[string]any {
	t.Helper()
	body := strings.NewReader(url.Values{"email": {email}}.Encode())
	req := httptest.NewRequest(http.MethodPost, "/"+templateName+"/request-token", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("got status %d, want 200", rr.Code)
	}
	var resp map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return resp
}

func TestRequestToken_NoOwner(t *testing.T) {
	srv, _ := newTestEditorServer(t)
	resp := postRequestToken(t, srv, "no-owner-tmpl", "test@example.com")
	if resp["ok"] != false {
		t.Errorf("expected ok=false, got %v", resp)
	}
	if msg, _ := resp["error"].(string); msg != "Für dieses Template ist keine E-Mail hinterlegt." {
		t.Errorf("unexpected error msg: %q", msg)
	}
}

func TestRequestToken_CorrectEmail(t *testing.T) {
	srv, database := newTestEditorServer(t)
	if _, err := database.Exec(`INSERT INTO templates (name, email) VALUES (?, ?)`, "my-tmpl", "owner@example.com"); err != nil {
		t.Fatal(err)
	}
	resp := postRequestToken(t, srv, "my-tmpl", "owner@example.com")
	if resp["ok"] != true {
		t.Errorf("expected ok=true, got %v", resp)
	}
}

func TestRequestToken_WrongEmail(t *testing.T) {
	srv, database := newTestEditorServer(t)
	if _, err := database.Exec(`INSERT INTO templates (name, email) VALUES (?, ?)`, "my-tmpl2", "owner@example.com"); err != nil {
		t.Fatal(err)
	}
	resp := postRequestToken(t, srv, "my-tmpl2", "wrong@example.com")
	if resp["ok"] != false {
		t.Errorf("expected ok=false, got %v", resp)
	}
	if msg, _ := resp["error"].(string); msg != "Diese E-Mail-Adresse ist nicht als Besitzer registriert." {
		t.Errorf("unexpected error msg: %q", msg)
	}
}

func TestRequestToken_IPBlockedAfterSixFailures(t *testing.T) {
	srv, database := newTestEditorServer(t)
	if _, err := database.Exec(`INSERT INTO templates (name, email) VALUES (?, ?)`, "my-tmpl3", "owner@example.com"); err != nil {
		t.Fatal(err)
	}
	// 6 wrong-email attempts from the same IP (httptest default: 192.0.2.1).
	for i := 0; i < maxIPFailures; i++ {
		postRequestToken(t, srv, "my-tmpl3", "wrong@example.com")
	}
	// 7th attempt with correct email must still be blocked.
	resp := postRequestToken(t, srv, "my-tmpl3", "owner@example.com")
	if resp["ok"] != false {
		t.Errorf("expected ok=false (IP blocked), got %v", resp)
	}
	if msg, _ := resp["error"].(string); msg != "Zu viele Fehlversuche. Bitte versuche es in 6 Stunden erneut." {
		t.Errorf("unexpected error msg: %q", msg)
	}
}
