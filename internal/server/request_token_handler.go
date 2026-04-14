// internal/server/request_token_handler.go
package server

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/webfraggle/zza-generate-images/internal/editor"
	"github.com/webfraggle/zza-generate-images/internal/renderer"
)

// writeTokenJSON writes a JSON response for the request-token endpoint.
func writeTokenJSON(w http.ResponseWriter, ok bool, errMsg string) {
	w.Header().Set("Content-Type", "application/json")
	if ok {
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": errMsg})
}

// handleRequestToken handles POST /{template}/request-token.
// It verifies the caller's email matches the registered owner and issues a new edit link.
// Always responds HTTP 200 with JSON {"ok": bool, "error": "..."}.
func (es *editorState) handleRequestToken(w http.ResponseWriter, r *http.Request) {
	templateName := r.PathValue("template")
	if err := renderer.ValidateTemplateName(templateName); err != nil {
		writeTokenJSON(w, false, "Ungültiger Template-Name.")
		return
	}

	if !checkOrigin(r, es.cfg.Mail.BaseURL) {
		writeTokenJSON(w, false, "Forbidden")
		return
	}

	ip := clientIP(r)
	if !es.ipLimiter.Allow(ip) {
		writeTokenJSON(w, false, "Zu viele Fehlversuche. Bitte versuche es in 6 Stunden erneut.")
		return
	}

	if err := r.ParseForm(); err != nil {
		writeTokenJSON(w, false, "Ungültige Anfrage.")
		return
	}
	email := strings.ToLower(strings.TrimSpace(r.FormValue("email")))
	if !emailRe.MatchString(email) {
		writeTokenJSON(w, false, "Bitte gib eine gültige E-Mail-Adresse ein.")
		return
	}

	// Ensure the template already has a registered owner.
	// (RequestToken would create a new record for unknown templates — we don't want that here.)
	var dummy string
	err := es.db.QueryRow(`SELECT name FROM templates WHERE name = ?`, templateName).Scan(&dummy)
	if errors.Is(err, sql.ErrNoRows) {
		// Use the same generic message as for email mismatch to prevent template-owner enumeration.
		writeTokenJSON(w, false, "Falls eine E-Mail für dieses Template hinterlegt ist und deine Adresse übereinstimmt, erhältst du in Kürze einen Link.")
		return
	}
	if err != nil {
		log.Printf("request-token: db lookup %q: %v", templateName, err)
		writeTokenJSON(w, false, "Interner Fehler. Bitte versuche es später erneut.")
		return
	}

	tok, err := editor.RequestToken(es.db, templateName, email, es.cfg.TokenTTL)
	if err != nil {
		switch {
		case errors.Is(err, editor.ErrEmailMismatch):
			es.ipLimiter.RecordFailure(ip)
			writeTokenJSON(w, false, "Falls eine E-Mail für dieses Template hinterlegt ist und deine Adresse übereinstimmt, erhältst du in Kürze einen Link.")
		case errors.Is(err, editor.ErrRateLimited):
			writeTokenJSON(w, false, "Zu viele Anfragen. Bitte versuche es in einer Stunde erneut.")
		default:
			log.Printf("request-token: RequestToken %q: %v", templateName, err)
			writeTokenJSON(w, false, "Interner Fehler. Bitte versuche es später erneut.")
		}
		return
	}

	es.ipLimiter.RecordSuccess(ip)

	if es.cfg.Mail.Host != "" {
		if mailErr := editor.SendTokenMail(es.cfg.Mail, email, templateName, tok, es.cfg.TokenTTL); mailErr != nil {
			log.Printf("request-token: send mail to %q: %v", email, mailErr)
		} else {
			log.Printf("request-token: mail sent to %q for %q", email, templateName)
		}
	} else {
		log.Printf("[DEV] edit link for %q: %s/edit/%s", templateName, es.cfg.Mail.BaseURL, tok) //nolint:gosec
	}

	writeTokenJSON(w, true, "")
}
