package editor

import (
	"database/sql"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	schema := `
	CREATE TABLE templates (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL UNIQUE,
		email TEXT NOT NULL,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);
	CREATE TABLE edit_tokens (
		token TEXT PRIMARY KEY,
		template_id INTEGER NOT NULL REFERENCES templates(id) ON DELETE CASCADE,
		expires_at DATETIME NOT NULL,
		used INTEGER NOT NULL DEFAULT 0,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);`
	if _, err := db.Exec(schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestGenerateToken_Unique(t *testing.T) {
	tok1, err1 := GenerateToken()
	tok2, err2 := GenerateToken()
	if err1 != nil || err2 != nil {
		t.Fatal("GenerateToken failed")
	}
	if tok1 == tok2 {
		t.Error("tokens should be unique")
	}
	if len(tok1) != 64 {
		t.Errorf("expected 64-char hex token, got %d", len(tok1))
	}
}

func TestRequestToken_NewTemplate(t *testing.T) {
	db := openTestDB(t)
	tok, err := RequestToken(db, "my-template", "owner@example.com", time.Hour)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tok) != 64 {
		t.Errorf("expected 64-char token, got %q", tok)
	}
}

func TestRequestToken_ExistingTemplate_WrongEmail(t *testing.T) {
	db := openTestDB(t)
	_, err := RequestToken(db, "tmpl", "owner@example.com", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	// Second request with wrong email.
	_, err = RequestToken(db, "tmpl", "other@example.com", time.Hour)
	if err != ErrEmailMismatch {
		t.Errorf("expected ErrEmailMismatch, got %v", err)
	}
}

func TestRequestToken_ExistingTemplate_CorrectEmail(t *testing.T) {
	db := openTestDB(t)
	_, _ = RequestToken(db, "tmpl", "owner@example.com", time.Hour)
	tok, err := RequestToken(db, "tmpl", "owner@example.com", time.Hour)
	if err != nil {
		t.Fatalf("same owner should get a new token: %v", err)
	}
	if len(tok) != 64 {
		t.Error("expected valid token")
	}
}

func TestRequestToken_EmailCaseInsensitive(t *testing.T) {
	db := openTestDB(t)
	_, _ = RequestToken(db, "tmpl", "Owner@Example.COM", time.Hour)
	_, err := RequestToken(db, "tmpl", "owner@example.com", time.Hour)
	if err != nil {
		t.Errorf("email comparison should be case-insensitive, got %v", err)
	}
}

func TestRequestToken_RateLimit(t *testing.T) {
	db := openTestDB(t)
	email := "owner@example.com"
	for i := 0; i < maxTokensPerHour; i++ {
		if _, err := RequestToken(db, "tmpl", email, time.Hour); err != nil {
			t.Fatalf("request %d failed unexpectedly: %v", i+1, err)
		}
	}
	_, err := RequestToken(db, "tmpl", email, time.Hour)
	if err != ErrRateLimited {
		t.Errorf("expected ErrRateLimited after %d requests, got %v", maxTokensPerHour, err)
	}
}

func TestValidateToken_Valid(t *testing.T) {
	db := openTestDB(t)
	tok, _ := RequestToken(db, "tmpl", "owner@example.com", time.Hour)

	name, err := ValidateToken(db, tok)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "tmpl" {
		t.Errorf("got template name %q, want %q", name, "tmpl")
	}
}

func TestValidateToken_Expired(t *testing.T) {
	db := openTestDB(t)
	tok, _ := RequestToken(db, "tmpl", "owner@example.com", -time.Second) // already expired
	_, err := ValidateToken(db, tok)
	if err != ErrTokenInvalid {
		t.Errorf("expected ErrTokenInvalid for expired token, got %v", err)
	}
}

func TestValidateToken_Unknown(t *testing.T) {
	db := openTestDB(t)
	_, err := ValidateToken(db, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	if err != ErrTokenInvalid {
		t.Errorf("expected ErrTokenInvalid for unknown token, got %v", err)
	}
}
