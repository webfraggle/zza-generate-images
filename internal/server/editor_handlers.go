package server

import (
	"database/sql"
	"encoding/json"
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
	TokenTTL time.Duration
	Mail     editor.MailConfig
}

// editorState holds DB and config for editor HTTP handlers.
type editorState struct {
	db        *sql.DB
	cfg       EditorConfig
	tmpl      *template.Template
	tdir      string // templates directory path
	ipLimiter *IPLimiter
}

var emailRe = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]{2,}$`)

// RegisterEditorRoutes wires the editor auth routes into the server.
// It is called optionally from cmd/zza/main.go when a DB is available.
// Routes under /edit/{token} are registered on a separate mux (s.editorHandler)
// to avoid conflicts with the wildcard pattern GET /{template}/preview on the main mux.
func (s *Server) RegisterEditorRoutes(db *sql.DB, cfg EditorConfig) {
	es := &editorState{db: db, cfg: cfg, tmpl: s.htmlTmpl, tdir: s.templatesDir, ipLimiter: NewIPLimiter()}

	// Start periodic cleanup of the IP limiter to prevent unbounded map growth.
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			es.ipLimiter.Cleanup()
		}
	}()

	// /{template}/edit stays on the main mux — no conflict.
	s.mux.HandleFunc("GET /{template}/edit", es.handleEditRequest)
	s.mux.HandleFunc("POST /{template}/edit", es.handleEditSubmit)
	s.mux.HandleFunc("POST /{template}/request-token", es.handleRequestToken)

	// /edit/{token} routes go on a dedicated mux dispatched via ServeHTTP pre-check.
	editMux := http.NewServeMux()
	editMux.HandleFunc("GET /edit/{token}", es.handleEditor)
	editMux.HandleFunc("GET /edit/{token}/files", es.handleListFiles)
	editMux.HandleFunc("GET /edit/{token}/file/{filename}", es.handleGetFile)
	editMux.HandleFunc("POST /edit/{token}/save", es.handleSave)
	editMux.HandleFunc("POST /edit/{token}/upload", es.handleUpload)
	editMux.HandleFunc("DELETE /edit/{token}/file/{filename}", es.handleDeleteFile)
	s.editorHandler = editMux
}

// requireToken validates the hex token in the request path and returns the
// associated template name. Returns ("", false) and writes an HTTP error if invalid.
func (es *editorState) requireToken(w http.ResponseWriter, r *http.Request) (string, bool) {
	tok := r.PathValue("token")
	if !isHexToken(tok) {
		http.Error(w, "invalid token", http.StatusBadRequest)
		return "", false
	}
	name, err := editor.ValidateToken(es.db, tok)
	if err != nil {
		http.Error(w, "Link ungültig oder abgelaufen.", http.StatusUnauthorized)
		return "", false
	}
	return name, true
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

	// Check IP rate limit.
	ip := clientIP(r)
	if !es.ipLimiter.Allow(ip) {
		d := editRequestData{
			TemplateName: templateName,
			Error:        "Zu viele Fehlversuche. Bitte versuche es in 6 Stunden erneut.",
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = es.tmpl.ExecuteTemplate(w, "edit-request.html", d)
		return
	}

	// Issue token.
	tok, err := editor.RequestToken(es.db, templateName, email, es.cfg.TokenTTL)
	if err != nil {
		var msg string
		switch {
		case errors.Is(err, editor.ErrEmailMismatch):
			es.ipLimiter.RecordFailure(ip)
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

	es.ipLimiter.RecordSuccess(ip)

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

// editorViewData is the view model for the editor page.
type editorViewData struct {
	Token        string
	TemplateName string
}

// handleEditor validates the token and shows the editor UI.
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

	// Ensure the template directory exists and has a starter template.yaml.
	if err := editor.InitTemplate(es.tdir, templateName); err != nil {
		log.Printf("edit-editor: init template %q: %v", templateName, err)
		http.Error(w, "could not initialise template directory", http.StatusInternalServerError)
		return
	}

	d := editorViewData{Token: tok, TemplateName: templateName}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := es.tmpl.ExecuteTemplate(w, "edit-editor.html", d); err != nil {
		log.Printf("edit-editor: execute template: %v", err)
	}
}

// handleListFiles returns a JSON list of files in the template directory.
func (es *editorState) handleListFiles(w http.ResponseWriter, r *http.Request) {
	templateName, ok := es.requireToken(w, r)
	if !ok {
		return
	}
	files, err := editor.ListFiles(es.tdir, templateName)
	if err != nil {
		log.Printf("editor: list files %q: %v", templateName, err)
		http.Error(w, "could not list files", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"files": files})
}

// handleGetFile returns the text content of a .yaml or .json file.
func (es *editorState) handleGetFile(w http.ResponseWriter, r *http.Request) {
	templateName, ok := es.requireToken(w, r)
	if !ok {
		return
	}
	filename := r.PathValue("filename")
	data, err := editor.ReadTextFile(es.tdir, templateName, filename)
	switch {
	case errors.Is(err, editor.ErrForbidden), errors.Is(err, editor.ErrInvalidName):
		http.Error(w, "forbidden", http.StatusForbidden)
	case errors.Is(err, editor.ErrFileNotFound):
		http.Error(w, "not found", http.StatusNotFound)
	case err != nil:
		log.Printf("editor: read file %q/%q: %v", templateName, filename, err)
		http.Error(w, "could not read file", http.StatusInternalServerError)
	default:
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write(data)
	}
}

// saveRequest is the JSON body for the save endpoint.
type saveRequest struct {
	Filename string `json:"filename"`
	Content  string `json:"content"`
}

// handleSave writes an edited .yaml or .json file back to disk.
func (es *editorState) handleSave(w http.ResponseWriter, r *http.Request) {
	templateName, ok := es.requireToken(w, r)
	if !ok {
		return
	}

	var req saveRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	err := editor.WriteTextFile(es.tdir, templateName, req.Filename, []byte(req.Content))
	switch {
	case errors.Is(err, editor.ErrForbidden), errors.Is(err, editor.ErrInvalidName):
		http.Error(w, "forbidden", http.StatusForbidden)
	case err != nil:
		log.Printf("editor: write file %q/%q: %v", templateName, req.Filename, err)
		http.Error(w, "could not save file", http.StatusInternalServerError)
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}

// handleUpload receives a multipart file upload and stores it in the template directory.
func (es *editorState) handleUpload(w http.ResponseWriter, r *http.Request) {
	templateName, ok := es.requireToken(w, r)
	if !ok {
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, editor.MaxUploadBytes+512)
	if err := r.ParseMultipartForm(editor.MaxUploadBytes); err != nil {
		http.Error(w, "request too large or invalid", http.StatusBadRequest)
		return
	}

	f, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "missing file field", http.StatusBadRequest)
		return
	}
	defer f.Close()

	if uploadErr := editor.UploadFile(es.tdir, templateName, header.Filename, f); uploadErr != nil {
		if errors.Is(uploadErr, editor.ErrForbidden) || errors.Is(uploadErr, editor.ErrInvalidName) {
			http.Error(w, "file type not allowed", http.StatusForbidden)
		} else {
			log.Printf("editor: upload %q/%q: %v", templateName, header.Filename, uploadErr)
			http.Error(w, "upload failed", http.StatusInternalServerError)
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleDeleteFile removes an asset file from the template directory.
func (es *editorState) handleDeleteFile(w http.ResponseWriter, r *http.Request) {
	templateName, ok := es.requireToken(w, r)
	if !ok {
		return
	}
	filename := r.PathValue("filename")
	err := editor.DeleteFile(es.tdir, templateName, filename)
	switch {
	case errors.Is(err, editor.ErrForbidden), errors.Is(err, editor.ErrInvalidName):
		http.Error(w, "forbidden", http.StatusForbidden)
	case errors.Is(err, editor.ErrFileNotFound):
		http.Error(w, "not found", http.StatusNotFound)
	case err != nil:
		log.Printf("editor: delete file %q/%q: %v", templateName, filename, err)
		http.Error(w, "could not delete file", http.StatusInternalServerError)
	default:
		w.WriteHeader(http.StatusNoContent)
	}
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
