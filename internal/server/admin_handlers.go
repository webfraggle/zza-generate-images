package server

import (
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"errors"
	"html/template"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/webfraggle/zza-generate-images/internal/admin"
	"github.com/webfraggle/zza-generate-images/internal/editor"
	"github.com/webfraggle/zza-generate-images/internal/gallery"
	"github.com/webfraggle/zza-generate-images/internal/renderer"
)

// AdminConfig holds configuration for the admin auth flow.
type AdminConfig struct {
	AdminToken    string // ADMIN_TOKEN env var
	TOTPSecret    string // TOTP_SECRET env var (Base32)
	SecureCookies bool   // set true in production (HTTPS); false for localhost dev
}

// adminState holds dependencies for admin HTTP handlers.
type adminState struct {
	cfg         AdminConfig
	sessions    *admin.SessionStore
	limiter     *admin.LoginLimiter
	replayGuard *admin.TOTPReplayGuard
	tmpl        *template.Template
	tdir        string
	cache       *Cache
	db          *sql.DB // may be nil
}

// RegisterAdminRoutes wires all /admin/... routes into the server.
// It is a no-op when cfg.AdminToken is empty (admin disabled).
func (s *Server) RegisterAdminRoutes(db *sql.DB, cfg AdminConfig) {
	if cfg.AdminToken == "" {
		log.Println("admin: ADMIN_TOKEN not set — admin interface disabled")
		return
	}
	if cfg.TOTPSecret == "" {
		log.Println("admin: TOTP_SECRET not set — admin interface disabled")
		return
	}

	as := &adminState{
		cfg:         cfg,
		sessions:    admin.NewSessionStore(),
		limiter:     admin.NewLoginLimiter(),
		replayGuard: admin.NewTOTPReplayGuard(),
		tmpl:        s.htmlTmpl,
		tdir:        s.templatesDir,
		cache:       s.cache,
		db:          db,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /admin/login", as.handleLogin)
	mux.HandleFunc("POST /admin/login", as.handleLoginSubmit)
	mux.HandleFunc("POST /admin/logout", as.handleLogout)
	mux.HandleFunc("GET /admin/cache", as.requireSession(as.handleCacheStats))
	mux.HandleFunc("POST /admin/cache/flush", as.requireSession(as.handleCacheFlush))
	mux.HandleFunc("GET /admin/{name}/files", as.requireSession(as.handleListFiles))
	mux.HandleFunc("GET /admin/{name}/file/{filename}", as.requireSession(as.handleGetFile))
	mux.HandleFunc("POST /admin/{name}/save", as.requireSession(as.handleSave))
	mux.HandleFunc("POST /admin/{name}/upload", as.requireSession(as.handleUpload))
	mux.HandleFunc("DELETE /admin/{name}/file/{filename}", as.requireSession(as.handleDeleteFile))
	mux.HandleFunc("DELETE /admin/{name}", as.requireSession(as.handleDeleteTemplate))
	mux.HandleFunc("GET /admin/{name}", as.requireSession(as.handleAdminEditor))
	mux.HandleFunc("GET /admin", as.requireSession(as.handleOverview))

	s.adminHandler = mux
}

// requireSession wraps a handler to enforce admin session authentication.
// Unauthenticated requests are redirected to /admin/login.
func (as *adminState) requireSession(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(admin.AdminCookieName)
		if err != nil || !as.sessions.Validate(cookie.Value) {
			http.Redirect(w, r, "/admin/login", http.StatusFound)
			return
		}
		next(w, r)
	}
}

// clientIP returns the remote IP from the TCP connection for rate limiting.
// X-Forwarded-For is intentionally ignored to prevent spoofing.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr // fallback (no port in addr)
	}
	return host
}

// ── Login ─────────────────────────────────────────────────────────────────────

type loginData struct {
	Error string
}

func (as *adminState) handleLogin(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = as.tmpl.ExecuteTemplate(w, "admin-login.html", loginData{})
}

func (as *adminState) handleLoginSubmit(w http.ResponseWriter, r *http.Request) {
	if !as.limiter.Allow(clientIP(r)) {
		http.Error(w, "Zu viele Versuche. Bitte warte 15 Minuten.", http.StatusTooManyRequests)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	submittedToken := r.FormValue("token")
	submittedCode := r.FormValue("totp")

	// Evaluate TOTP first (always runs) to avoid timing side-channel.
	// Token comparison is constant-time; both checks always execute.
	totpOK := admin.ValidateTOTP(as.cfg.TOTPSecret, submittedCode, as.replayGuard)
	tokenOK := subtle.ConstantTimeCompare([]byte(as.cfg.AdminToken), []byte(submittedToken)) == 1

	if !tokenOK || !totpOK {
		log.Printf("admin: failed login attempt from %s", clientIP(r))
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = as.tmpl.ExecuteTemplate(w, "admin-login.html", loginData{Error: "Falscher Token oder TOTP-Code."})
		return
	}

	tok, err := as.sessions.Create()
	if err != nil {
		log.Printf("admin: session create failed: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     admin.AdminCookieName,
		Value:    tok,
		Path:     "/admin",
		MaxAge:   int(admin.SessionTTL.Seconds()),
		HttpOnly: true,
		Secure:   as.cfg.SecureCookies,
		SameSite: http.SameSiteStrictMode,
	})
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

func (as *adminState) handleLogout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(admin.AdminCookieName); err == nil {
		as.sessions.Destroy(cookie.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     admin.AdminCookieName,
		Value:    "",
		Path:     "/admin",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})
	http.Redirect(w, r, "/admin/login", http.StatusFound)
}

// ── Overview ──────────────────────────────────────────────────────────────────

type adminTemplateInfo struct {
	Name     string
	MetaName string
	Owner    string // email from DB, or "" if not registered
}

type adminOverviewData struct {
	Templates []adminTemplateInfo
}

func (as *adminState) handleOverview(w http.ResponseWriter, r *http.Request) {
	infos, err := gallery.ListTemplates(as.tdir)
	if err != nil {
		http.Error(w, "could not list templates", http.StatusInternalServerError)
		log.Printf("admin: overview: %v", err)
		return
	}

	var items []adminTemplateInfo
	for _, info := range infos {
		item := adminTemplateInfo{Name: info.Name, MetaName: info.Meta.Name}
		if as.db != nil {
			var email string
			if dbErr := as.db.QueryRow(`SELECT email FROM templates WHERE name = ?`, info.Name).Scan(&email); dbErr == nil {
				item.Owner = email
			}
		}
		items = append(items, item)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = as.tmpl.ExecuteTemplate(w, "admin-overview.html", adminOverviewData{Templates: items})
}

// ── Delete Template ───────────────────────────────────────────────────────────

func (as *adminState) handleDeleteTemplate(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if err := renderer.ValidateTemplateName(name); err != nil {
		http.Error(w, "invalid template name", http.StatusBadRequest)
		return
	}

	// Remove from filesystem with secondary path-traversal guard.
	dir := filepath.Join(as.tdir, name)
	base := filepath.Clean(as.tdir) + string(filepath.Separator)
	if !strings.HasPrefix(filepath.Clean(dir)+string(filepath.Separator), base) {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}
	if err := os.RemoveAll(dir); err != nil {
		log.Printf("admin: delete template %q: %v", name, err)
		http.Error(w, "could not delete template", http.StatusInternalServerError)
		return
	}

	// Remove from DB (best-effort).
	if as.db != nil {
		if _, err := as.db.Exec(`DELETE FROM templates WHERE name = ?`, name); err != nil {
			log.Printf("admin: delete template %q from db: %v", name, err)
		}
	}

	log.Printf("admin: deleted template %q", name)
	w.WriteHeader(http.StatusNoContent)
}

// ── Cache ──────────────────────────────────────────────────────────────────────

type adminCacheData struct {
	Stats CacheStats
}

func (as *adminState) handleCacheStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = as.tmpl.ExecuteTemplate(w, "admin-cache.html", adminCacheData{Stats: as.cache.Stats()})
}

func (as *adminState) handleCacheFlush(w http.ResponseWriter, r *http.Request) {
	if err := as.cache.Flush(); err != nil {
		log.Printf("admin: cache flush: %v", err)
		http.Error(w, "flush failed", http.StatusInternalServerError)
		return
	}
	log.Println("admin: cache flushed")
	http.Redirect(w, r, "/admin/cache", http.StatusFound)
}

// ── Admin Editor ──────────────────────────────────────────────────────────────

type adminEditorData struct {
	TemplateName string
}

func (as *adminState) handleAdminEditor(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if err := renderer.ValidateTemplateName(name); err != nil {
		http.Error(w, "invalid template name", http.StatusBadRequest)
		return
	}

	if err := editor.InitTemplate(as.tdir, name); err != nil {
		log.Printf("admin: init template %q: %v", name, err)
		http.Error(w, "could not initialise template directory", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = as.tmpl.ExecuteTemplate(w, "admin-editor.html", adminEditorData{TemplateName: name})
}

// ── Admin File API (same logic as editor, but admin-auth) ─────────────────────

func (as *adminState) handleListFiles(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if err := renderer.ValidateTemplateName(name); err != nil {
		http.Error(w, "invalid template name", http.StatusBadRequest)
		return
	}
	files, err := editor.ListFiles(as.tdir, name)
	if err != nil {
		log.Printf("admin: list files %q: %v", name, err)
		http.Error(w, "could not list files", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"files": files})
}

func (as *adminState) handleGetFile(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if err := renderer.ValidateTemplateName(name); err != nil {
		http.Error(w, "invalid template name", http.StatusBadRequest)
		return
	}
	filename := r.PathValue("filename")
	data, err := editor.ReadTextFile(as.tdir, name, filename)
	switch {
	case errors.Is(err, editor.ErrForbidden), errors.Is(err, editor.ErrInvalidName):
		http.Error(w, "forbidden", http.StatusForbidden)
	case errors.Is(err, editor.ErrFileNotFound):
		http.Error(w, "not found", http.StatusNotFound)
	case err != nil:
		log.Printf("admin: read file %q/%q: %v", name, filename, err)
		http.Error(w, "could not read file", http.StatusInternalServerError)
	default:
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write(data)
	}
}

func (as *adminState) handleSave(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if err := renderer.ValidateTemplateName(name); err != nil {
		http.Error(w, "invalid template name", http.StatusBadRequest)
		return
	}
	var req struct {
		Filename string `json:"filename"`
		Content  string `json:"content"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	err := editor.WriteTextFile(as.tdir, name, req.Filename, []byte(req.Content))
	switch {
	case errors.Is(err, editor.ErrForbidden), errors.Is(err, editor.ErrInvalidName):
		http.Error(w, "forbidden", http.StatusForbidden)
	case err != nil:
		log.Printf("admin: write file %q/%q: %v", name, req.Filename, err)
		http.Error(w, "could not save file", http.StatusInternalServerError)
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}

func (as *adminState) handleUpload(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if err := renderer.ValidateTemplateName(name); err != nil {
		http.Error(w, "invalid template name", http.StatusBadRequest)
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
	if uploadErr := editor.UploadFile(as.tdir, name, header.Filename, f); uploadErr != nil {
		if errors.Is(uploadErr, editor.ErrForbidden) || errors.Is(uploadErr, editor.ErrInvalidName) {
			http.Error(w, "file type not allowed", http.StatusForbidden)
		} else {
			log.Printf("admin: upload %q/%q: %v", name, header.Filename, uploadErr)
			http.Error(w, "upload failed", http.StatusInternalServerError)
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (as *adminState) handleDeleteFile(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if err := renderer.ValidateTemplateName(name); err != nil {
		http.Error(w, "invalid template name", http.StatusBadRequest)
		return
	}
	filename := r.PathValue("filename")
	err := editor.DeleteFile(as.tdir, name, filename)
	switch {
	case errors.Is(err, editor.ErrForbidden), errors.Is(err, editor.ErrInvalidName):
		http.Error(w, "forbidden", http.StatusForbidden)
	case errors.Is(err, editor.ErrFileNotFound):
		http.Error(w, "not found", http.StatusNotFound)
	case err != nil:
		log.Printf("admin: delete file %q/%q: %v", name, filename, err)
		http.Error(w, "could not delete file", http.StatusInternalServerError)
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}
