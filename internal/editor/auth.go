// Package editor implements the token-based authentication for template editing.
package editor

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	tokenBytes       = 32 // 256 bits of entropy
	maxTokensPerHour = 3  // per template per hour
)

// Sentinel errors returned by auth functions.
var (
	ErrRateLimited   = errors.New("editor: too many token requests — please try again later")
	ErrEmailMismatch = errors.New("editor: email address does not match the template owner")
	ErrTokenInvalid  = errors.New("editor: token is invalid or has expired")
)

// GenerateToken returns a cryptographically random 64-character hex token.
func GenerateToken() (string, error) {
	b := make([]byte, tokenBytes)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("editor: generating token: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// RequestToken issues an edit token for templateName/email.
//
// For templates with no DB record yet, a new ownership record is created.
// For templates that already have an owner, the supplied email is verified
// against the stored address — a mismatch returns ErrEmailMismatch.
//
// Returns ErrRateLimited when more than maxTokensPerHour tokens have been
// issued for this template within the last hour.
func RequestToken(db *sql.DB, templateName, email string, ttl time.Duration) (string, error) {
	email = strings.ToLower(strings.TrimSpace(email))

	// Look up existing ownership record.
	var templateID int64
	var storedEmail string
	err := db.QueryRow(
		`SELECT id, email FROM templates WHERE name = ?`, templateName,
	).Scan(&templateID, &storedEmail)

	switch {
	case errors.Is(err, sql.ErrNoRows):
		// New template: register ownership.
		res, insertErr := db.Exec(
			`INSERT INTO templates (name, email) VALUES (?, ?)`,
			templateName, email,
		)
		if insertErr != nil {
			return "", fmt.Errorf("editor: registering template: %w", insertErr)
		}
		templateID, _ = res.LastInsertId()

	case err != nil:
		return "", fmt.Errorf("editor: looking up template: %w", err)

	default:
		// Existing template: verify ownership.
		if !strings.EqualFold(storedEmail, email) {
			return "", ErrEmailMismatch
		}
	}

	// Rate-limit: count tokens issued for this template in the last hour.
	var count int
	if err := db.QueryRow(
		`SELECT COUNT(*) FROM edit_tokens
		 WHERE template_id = ? AND created_at > datetime('now', '-1 hour')`,
		templateID,
	).Scan(&count); err != nil {
		return "", fmt.Errorf("editor: checking rate limit: %w", err)
	}
	if count >= maxTokensPerHour {
		return "", ErrRateLimited
	}

	// Generate and persist the token.
	tok, err := GenerateToken()
	if err != nil {
		return "", err
	}
	expiresAt := time.Now().UTC().Add(ttl)
	if _, err := db.Exec(
		`INSERT INTO edit_tokens (token, template_id, expires_at) VALUES (?, ?, ?)`,
		tok, templateID, expiresAt,
	); err != nil {
		return "", fmt.Errorf("editor: storing token: %w", err)
	}
	return tok, nil
}

// ValidateToken checks that the token is known, not expired, and not revoked.
// Returns the associated template name.
func ValidateToken(db *sql.DB, token string) (templateName string, err error) {
	var templateID int64
	var expiresAt time.Time
	var used bool

	err = db.QueryRow(
		`SELECT template_id, expires_at, used FROM edit_tokens WHERE token = ?`, token,
	).Scan(&templateID, &expiresAt, &used)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrTokenInvalid
	}
	if err != nil {
		return "", fmt.Errorf("editor: validating token: %w", err)
	}
	if used || time.Now().UTC().After(expiresAt) {
		return "", ErrTokenInvalid
	}

	err = db.QueryRow(
		`SELECT name FROM templates WHERE id = ?`, templateID,
	).Scan(&templateName)
	if err != nil {
		return "", fmt.Errorf("editor: resolving template name: %w", err)
	}
	return templateName, nil
}
