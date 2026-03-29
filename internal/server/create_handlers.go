// internal/server/create_handlers.go
package server

import (
	"database/sql"
	"encoding/json"
	"errors"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/webfraggle/zza-generate-images/internal/editor"
	"github.com/webfraggle/zza-generate-images/internal/renderer"
)

// createHandler handles the /create-new routes.
type createHandler struct {
	db   *sql.DB
	cfg  EditorConfig
	tmpl *template.Template
	tdir string
}

// RegisterCreateRoutes wires the create-new routes into the server.
// Call this from main alongside RegisterEditorRoutes when a DB is available.
func (s *Server) RegisterCreateRoutes(db *sql.DB, cfg EditorConfig) {
	ch := &createHandler{db: db, cfg: cfg, tmpl: s.htmlTmpl, tdir: s.templatesDir}
	s.mux.HandleFunc("GET /create-new", ch.handleCreateNew)
	s.mux.HandleFunc("POST /create-new", ch.handleCreateSubmit)
	s.mux.HandleFunc("GET /create-new/check", ch.handleCreateCheck)
}

// createFormData is the view model for the create-new page.
type createFormData struct {
	Error       string
	ID          string
	Email       string
	Title       string
	Description string
	Display     string // "1.05" or "0.96"
}

// createSentData is the view model for the create-sent confirmation page.
type createSentData struct {
	Email        string
	TemplateName string
}

// handleCreateNew shows the new-template form.
func (ch *createHandler) handleCreateNew(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := ch.tmpl.ExecuteTemplate(w, "create-new.html", createFormData{Display: "1.05"}); err != nil {
		log.Printf("create-new: execute template: %v", err)
	}
}

// handleCreateCheck answers GET /create-new/check?id=foo with JSON availability.
func (ch *createHandler) handleCreateCheck(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.URL.Query().Get("id"))
	w.Header().Set("Content-Type", "application/json")

	if err := renderer.ValidateTemplateName(id); err != nil {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"available": false,
			"reason":    "Ungültiges Format (nur a–z, 0–9, Bindestriche; max 64 Zeichen).",
		})
		return
	}

	// Check filesystem.
	if _, err := os.Stat(filepath.Join(ch.tdir, id)); err == nil {
		_ = json.NewEncoder(w).Encode(map[string]any{"available": false, "reason": "Bereits vergeben."})
		return
	}

	// Check DB.
	var dummy string
	err := ch.db.QueryRow(`SELECT name FROM templates WHERE name = ?`, id).Scan(&dummy)
	if err == nil {
		_ = json.NewEncoder(w).Encode(map[string]any{"available": false, "reason": "Bereits vergeben."})
		return
	}
	if !errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	_ = json.NewEncoder(w).Encode(map[string]any{"available": true})
}

// handleCreateSubmit processes the create-new form POST.
func (ch *createHandler) handleCreateSubmit(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}

	id := strings.TrimSpace(r.FormValue("id"))
	email := strings.ToLower(strings.TrimSpace(r.FormValue("email")))
	title := strings.TrimSpace(r.FormValue("title"))
	desc := strings.TrimSpace(r.FormValue("description"))
	display := r.FormValue("display") // "1.05" or "0.96"

	renderForm := func(errMsg string) {
		d := createFormData{
			Error:       errMsg,
			ID:          id,
			Email:       r.FormValue("email"),
			Title:       title,
			Description: desc,
			Display:     display,
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = ch.tmpl.ExecuteTemplate(w, "create-new.html", d)
	}

	// Determine canvas size from display value.
	var canvasW, canvasH int
	var displayStr string
	switch display {
	case "1.05":
		canvasW, canvasH = 240, 240
		displayStr = `1.05"`
	case "0.96":
		canvasW, canvasH = 160, 160
		displayStr = `0.96"`
	default:
		renderForm("Bitte wähle eine Display-Größe aus.")
		return
	}

	// Validate other fields.
	if err := renderer.ValidateTemplateName(id); err != nil {
		renderForm("Ungültige Template-ID (nur a–z, 0–9, Bindestriche; max 64 Zeichen).")
		return
	}
	if !emailRe.MatchString(email) {
		renderForm("Bitte gib eine gültige E-Mail-Adresse ein.")
		return
	}
	if title == "" {
		renderForm("Bitte gib einen Titel an.")
		return
	}
	if len(title) > 80 {
		renderForm("Titel ist zu lang (max 80 Zeichen).")
		return
	}
	if len(desc) > 300 {
		renderForm("Beschreibung ist zu lang (max 300 Zeichen).")
		return
	}

	// Server-side uniqueness check (guards against race with async JS check).
	if _, err := os.Stat(filepath.Join(ch.tdir, id)); err == nil {
		renderForm("Diese Template-ID ist bereits vergeben.")
		return
	}
	var dummy string
	if err := ch.db.QueryRow(`SELECT name FROM templates WHERE name = ?`, id).Scan(&dummy); err == nil {
		renderForm("Diese Template-ID ist bereits vergeben.")
		return
	}

	// Create template directory + files.
	if err := editor.CreateTemplate(ch.tdir, id, title, desc, displayStr, canvasW, canvasH); err != nil {
		log.Printf("create-new: CreateTemplate %q: %v", id, err)
		renderForm("Konnte Template nicht anlegen — bitte versuche es erneut.")
		return
	}

	// Register ownership + issue edit token.
	tok, err := editor.RequestToken(ch.db, id, email, ch.cfg.TokenTTL)
	if err != nil {
		log.Printf("create-new: RequestToken %q: %v", id, err)
		if rmErr := os.RemoveAll(filepath.Join(ch.tdir, id)); rmErr != nil {
			log.Printf("create-new: cleanup after RequestToken failure %q: %v", id, rmErr)
		}
		http.Error(w, "Interner Fehler.", http.StatusInternalServerError)
		return
	}

	// Send email (dev fallback: log the link).
	if ch.cfg.Mail.Host != "" {
		if mailErr := editor.SendTokenMail(ch.cfg.Mail, email, id, tok, ch.cfg.TokenTTL); mailErr != nil {
			log.Printf("create-new: send mail to %q: %v", email, mailErr)
		}
	} else {
		log.Printf("[DEV] edit link for new template %q: %s/edit/%s", id, ch.cfg.Mail.BaseURL, tok) //nolint:gosec
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = ch.tmpl.ExecuteTemplate(w, "create-sent.html", createSentData{Email: email, TemplateName: id})
}
