package server

import (
	"database/sql"
	"errors"
	"html/template"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/webfraggle/zza-generate-images/internal/editor"
	"github.com/webfraggle/zza-generate-images/internal/renderer"
)

// EditorConfig holds configuration for the editor auth flow.
type EditorConfig struct {
	HMACSecret string
	TokenTTL   time.Duration
	Mail       editor.MailConfig
}

// editorState holds DB and config for editor HTTP handlers.
type editorState struct {
	db   *sql.DB
	cfg  EditorConfig
	tmpl *template.Template
	tdir string // templates directory path
}

var emailRe = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]{2,}$`)

// RegisterEditorRoutes wires the editor auth routes into the server.
// It is called optionally from cmd/zza/main.go when a DB is available.
// Routes under /edit/{token} are registered on a separate mux (s.editorHandler)
// to avoid conflicts with the wildcard pattern GET /{template}/preview on the main mux.
func (s *Server) RegisterEditorRoutes(db *sql.DB, cfg EditorConfig) {
	es := &editorState{db: db, cfg: cfg, tmpl: s.htmlTmpl, tdir: s.templatesDir}

	// /{template}/edit stays on the main mux — no conflict.
	s.mux.HandleFunc("GET /{template}/edit", es.handleEditRequest)
	s.mux.HandleFunc("POST /{template}/edit", es.handleEditSubmit)

	// /edit/{token} routes go on a dedicated mux dispatched via ServeHTTP pre-check.
	editMux := http.NewServeMux()
	editMux.HandleFunc("GET /edit/{token}", es.handleEditor)
	editMux.HandleFunc("POST /edit/{token}/save", es.handleSave)
	s.editorHandler = editMux
}

// editRequestData is the view model for the edit-request page.
type editRequestData struct {
	TemplateName string
	MetaName     string // display name from template.yaml meta, or empty
	Error        string
	IsNew        bool // true when template has no owner yet in the DB
}

// handleEditRequest shows the email-entry form for requesting an edit token.
func (es *editorState) handleEditRequest(w http.ResponseWriter, r *http.Request) {
	templateName := r.PathValue("template")
	if err := renderer.ValidateTemplateName(templateName); err != nil {
		http.Error(w, "invalid template name", http.StatusBadRequest)
		return
	}

	// Check whether template exists in filesystem.
	tmpl, loadErr := renderer.LoadTemplate(es.tdir, templateName)
	metaName := ""
	if loadErr == nil {
		metaName = tmpl.Meta.Name
	}

	// Check whether this template already has a registered owner.
	var dummy string
	dbErr := es.db.QueryRow(`SELECT name FROM templates WHERE name = ?`, templateName).Scan(&dummy)
	isNew := errors.Is(dbErr, sql.ErrNoRows)

	d := editRequestData{
		TemplateName: templateName,
		MetaName:     metaName,
		IsNew:        isNew,
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := es.tmpl.ExecuteTemplate(w, "edit-request.html", d); err != nil {
		log.Printf("edit-request: execute template: %v", err)
	}
}

// handleEditSubmit processes the email form, issues a token and sends the mail.
func (es *editorState) handleEditSubmit(w http.ResponseWriter, r *http.Request) {
	templateName := r.PathValue("template")
	if err := renderer.ValidateTemplateName(templateName); err != nil {
		http.Error(w, "invalid template name", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	email := strings.TrimSpace(r.FormValue("email"))

	// Validate email format.
	if !emailRe.MatchString(email) {
		d := editRequestData{
			TemplateName: templateName,
			Error:        "Bitte gib eine gültige E-Mail-Adresse ein.",
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = es.tmpl.ExecuteTemplate(w, "edit-request.html", d)
		return
	}

	// Issue token.
	tok, err := editor.RequestToken(es.db, templateName, email, es.cfg.HMACSecret, es.cfg.TokenTTL)
	if err != nil {
		var msg string
		switch {
		case errors.Is(err, editor.ErrEmailMismatch):
			msg = "Diese E-Mail-Adresse ist nicht als Besitzer dieses Templates registriert."
		case errors.Is(err, editor.ErrRateLimited):
			msg = "Zu viele Anfragen. Bitte versuche es in einer Stunde erneut."
		default:
			log.Printf("edit-submit: RequestToken %q: %v", templateName, err)
			msg = "Interner Fehler. Bitte versuche es später erneut."
		}
		d := editRequestData{TemplateName: templateName, Error: msg}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = es.tmpl.ExecuteTemplate(w, "edit-request.html", d)
		return
	}

	// Send email (best-effort — don't block on SMTP misconfiguration in dev).
	if es.cfg.Mail.Host != "" {
		if mailErr := editor.SendTokenMail(es.cfg.Mail, email, templateName, tok, es.cfg.TokenTTL); mailErr != nil {
			log.Printf("edit-submit: send mail to %q: %v", email, mailErr)
		}
	} else {
		// Development fallback: log the link so it can be used without SMTP.
		log.Printf("[DEV] edit link for %q: %s/edit/%s", templateName, es.cfg.Mail.BaseURL, tok) //nolint:gosec // intentional dev-only output
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = es.tmpl.ExecuteTemplate(w, "edit-sent.html", templateName)
}

// editorViewData is the view model for the editor placeholder page.
type editorViewData struct {
	Token        string
	TemplateName string
}

// handleEditor validates the token and shows the editor (Phase 6 will add the full UI).
func (es *editorState) handleEditor(w http.ResponseWriter, r *http.Request) {
	tok := r.PathValue("token")
	if !isHexToken(tok) {
		http.Error(w, "invalid token", http.StatusBadRequest)
		return
	}

	templateName, err := editor.ValidateToken(es.db, tok)
	if err != nil {
		http.Error(w, "Link ungültig oder abgelaufen.", http.StatusUnauthorized)
		return
	}

	d := editorViewData{Token: tok, TemplateName: templateName}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := es.tmpl.ExecuteTemplate(w, "edit-editor.html", d); err != nil {
		log.Printf("edit-editor: execute template: %v", err)
	}
}

// handleSave is a Phase 5 stub; Phase 6 will implement actual file saving.
func (es *editorState) handleSave(w http.ResponseWriter, r *http.Request) {
	tok := r.PathValue("token")
	if !isHexToken(tok) {
		http.Error(w, "invalid token", http.StatusBadRequest)
		return
	}
	if _, err := editor.ValidateToken(es.db, tok); err != nil {
		http.Error(w, "Link ungültig oder abgelaufen.", http.StatusUnauthorized)
		return
	}
	// Phase 6 will implement the actual save logic.
	http.Error(w, "not implemented yet", http.StatusNotImplemented)
}

// isHexToken checks that s is a 64-character lowercase hex string
// (the expected format of a generated token).
func isHexToken(s string) bool {
	if len(s) != 64 {
		return false
	}
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return true
}
