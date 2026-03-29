# Request Edit-Link via Modal — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a modal on the detail page that lets the owner of a template request a new edit link by entering their email address, with IP-based brute-force protection (6 failures → 6h block).

**Architecture:** New `IPLimiter` struct (in-memory, `sync.Mutex`) lives in `editorState`. New AJAX endpoint `POST /{template}/request-token` reuses `editor.RequestToken` for email verification and token generation. Modal UI is inline HTML + Vanilla JS, no page reload.

**Tech Stack:** Go, `sync.Mutex`, Vanilla JS, no new dependencies

---

## File Structure

| Action | File | Responsibility |
|---|---|---|
| Create | `internal/server/ip_limiter.go` | IPLimiter struct, Allow/RecordFailure/RecordSuccess |
| Create | `internal/server/ip_limiter_test.go` | Unit tests for IPLimiter |
| Create | `internal/server/request_token_handler.go` | `handleRequestToken`, `clientIP`, `writeTokenJSON` |
| Create | `internal/server/request_token_handler_test.go` | Handler tests |
| Modify | `internal/server/editor_handlers.go` | Add `ipLimiter` field + init + route |
| Modify | `web/static/app.css` | `.btn-level2`, `.info-right` flex-direction, modal CSS |
| Modify | `web/templates/detail.html` | Button, modal HTML, modal JS |

---

## Task A: IPLimiter

**Files:**
- Create: `internal/server/ip_limiter.go`
- Create: `internal/server/ip_limiter_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// internal/server/ip_limiter_test.go
package server

import (
	"testing"
	"time"
)

func TestIPLimiter_AllowBeforeLimit(t *testing.T) {
	l := NewIPLimiter()
	for i := 0; i < maxIPFailures-1; i++ {
		l.RecordFailure("1.2.3.4")
	}
	if !l.Allow("1.2.3.4") {
		t.Error("expected Allow=true after 5 failures")
	}
}

func TestIPLimiter_BlockedAtLimit(t *testing.T) {
	l := NewIPLimiter()
	for i := 0; i < maxIPFailures; i++ {
		l.RecordFailure("1.2.3.4")
	}
	if l.Allow("1.2.3.4") {
		t.Error("expected Allow=false after 6 failures")
	}
}

func TestIPLimiter_ExpiredBlock(t *testing.T) {
	l := NewIPLimiter()
	for i := 0; i < maxIPFailures; i++ {
		l.RecordFailure("1.2.3.4")
	}
	// Manually backdate the block so it appears expired.
	l.mu.Lock()
	l.entries["1.2.3.4"].blockedUntil = time.Now().Add(-time.Minute)
	l.mu.Unlock()
	if !l.Allow("1.2.3.4") {
		t.Error("expected Allow=true after block expired")
	}
}

func TestIPLimiter_RecordSuccessResets(t *testing.T) {
	l := NewIPLimiter()
	for i := 0; i < maxIPFailures; i++ {
		l.RecordFailure("1.2.3.4")
	}
	l.RecordSuccess("1.2.3.4")
	if !l.Allow("1.2.3.4") {
		t.Error("expected Allow=true after RecordSuccess")
	}
}
```

- [ ] **Step 2: Run tests — expect compile failure**

```bash
go test ./internal/server/ -run TestIPLimiter -v
```
Expected: `FAIL — NewIPLimiter undefined`

- [ ] **Step 3: Implement ip_limiter.go**

```go
// internal/server/ip_limiter.go
package server

import (
	"sync"
	"time"
)

const (
	maxIPFailures   = 6
	ipBlockDuration = 6 * time.Hour
)

type ipEntry struct {
	failures     int
	blockedUntil time.Time
}

// IPLimiter tracks failed email attempts per client IP.
// After maxIPFailures failures, the IP is blocked for ipBlockDuration.
// State is in-memory and does not survive server restarts.
type IPLimiter struct {
	mu      sync.Mutex
	entries map[string]*ipEntry
}

func NewIPLimiter() *IPLimiter {
	return &IPLimiter{entries: make(map[string]*ipEntry)}
}

// Allow returns false if the IP is currently blocked.
func (l *IPLimiter) Allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	e, ok := l.entries[ip]
	if !ok {
		return true
	}
	if !e.blockedUntil.IsZero() && time.Now().Before(e.blockedUntil) {
		return false
	}
	return true
}

// RecordFailure increments the failure count for ip.
// Once failures reach maxIPFailures, the IP is blocked for ipBlockDuration.
func (l *IPLimiter) RecordFailure(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	e, ok := l.entries[ip]
	if !ok {
		e = &ipEntry{}
		l.entries[ip] = e
	}
	// Don't count additional failures while blocked.
	if !e.blockedUntil.IsZero() && time.Now().Before(e.blockedUntil) {
		return
	}
	e.failures++
	if e.failures >= maxIPFailures {
		e.blockedUntil = time.Now().Add(ipBlockDuration)
	}
}

// RecordSuccess clears the failure record for ip.
func (l *IPLimiter) RecordSuccess(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.entries, ip)
}
```

- [ ] **Step 4: Run tests — expect all pass**

```bash
go test ./internal/server/ -run TestIPLimiter -v
```
Expected: `PASS` (4 tests)

- [ ] **Step 5: Commit**

```bash
git add internal/server/ip_limiter.go internal/server/ip_limiter_test.go
git commit -m "feat: add in-memory IP rate limiter for email brute-force protection"
```

---

## Task B: Request-Token Handler

**Files:**
- Create: `internal/server/request_token_handler.go`
- Create: `internal/server/request_token_handler_test.go`
- Modify: `internal/server/editor_handlers.go`

- [ ] **Step 1: Write the failing handler tests**

```go
// internal/server/request_token_handler_test.go
package server

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
	body := strings.NewReader("email=" + email)
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
	resp := postRequestToken(t, srv, "no-owner-tmpl", "test%40example.com")
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
	resp := postRequestToken(t, srv, "my-tmpl", "owner%40example.com")
	if resp["ok"] != true {
		t.Errorf("expected ok=true, got %v", resp)
	}
}

func TestRequestToken_WrongEmail(t *testing.T) {
	srv, database := newTestEditorServer(t)
	if _, err := database.Exec(`INSERT INTO templates (name, email) VALUES (?, ?)`, "my-tmpl2", "owner@example.com"); err != nil {
		t.Fatal(err)
	}
	resp := postRequestToken(t, srv, "my-tmpl2", "wrong%40example.com")
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
		postRequestToken(t, srv, "my-tmpl3", "wrong%40example.com")
	}
	// 7th attempt with correct email must still be blocked.
	resp := postRequestToken(t, srv, "my-tmpl3", "owner%40example.com")
	if resp["ok"] != false {
		t.Errorf("expected ok=false (IP blocked), got %v", resp)
	}
	if msg, _ := resp["error"].(string); msg != "Zu viele Fehlversuche. Bitte versuche es in 6 Stunden erneut." {
		t.Errorf("unexpected error msg: %q", msg)
	}
}
```

- [ ] **Step 2: Run tests — expect compile failure**

```bash
go test ./internal/server/ -run TestRequestToken -v
```
Expected: `FAIL — handleRequestToken undefined`

- [ ] **Step 3: Add `ipLimiter` field to `editorState` in `editor_handlers.go`**

Change the struct (lines 25–30 of `internal/server/editor_handlers.go`):

```go
// editorState holds DB and config for editor HTTP handlers.
type editorState struct {
	db        *sql.DB
	cfg       EditorConfig
	tmpl      *template.Template
	tdir      string // templates directory path
	ipLimiter *IPLimiter
}
```

Change the initialisation inside `RegisterEditorRoutes` (line 39):

```go
es := &editorState{db: db, cfg: cfg, tmpl: s.htmlTmpl, tdir: s.templatesDir, ipLimiter: NewIPLimiter()}
```

Add the new route after the existing `/{template}/edit` routes (after line 43):

```go
s.mux.HandleFunc("POST /{template}/request-token", es.handleRequestToken)
```

- [ ] **Step 4: Implement `request_token_handler.go`**

```go
// internal/server/request_token_handler.go
package server

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net"
	"net/http"
	"strings"

	"github.com/webfraggle/zza-generate-images/internal/editor"
	"github.com/webfraggle/zza-generate-images/internal/renderer"
)

// clientIP extracts the real client IP from the request.
// Uses the first value of X-Forwarded-For (set by Traefik), falling back to RemoteAddr.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return strings.TrimSpace(strings.SplitN(xff, ",", 2)[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

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
		writeTokenJSON(w, false, "Für dieses Template ist keine E-Mail hinterlegt.")
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
			writeTokenJSON(w, false, "Diese E-Mail-Adresse ist nicht als Besitzer registriert.")
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
		}
	} else {
		log.Printf("[DEV] edit link for %q: %s/edit/%s", templateName, es.cfg.Mail.BaseURL, tok) //nolint:gosec
	}

	writeTokenJSON(w, true, "")
}
```

- [ ] **Step 5: Run tests — expect all pass**

```bash
go test ./internal/server/ -run TestRequestToken -v
```
Expected: `PASS` (4 tests)

- [ ] **Step 6: Run full test suite**

```bash
go test ./...
```
Expected: all tests pass, no compilation errors.

- [ ] **Step 7: Commit**

```bash
git add internal/server/ip_limiter.go internal/server/ip_limiter_test.go \
        internal/server/request_token_handler.go internal/server/request_token_handler_test.go \
        internal/server/editor_handlers.go
git commit -m "feat: add POST /{template}/request-token endpoint with IP rate limiting"
```

---

## Task C: CSS — Button + Modal

**Files:**
- Modify: `web/static/app.css`

The `.btn-level2` class and modal CSS go in `app.css`. Also update `.info-right` to stack its buttons vertically.

- [ ] **Step 1: Add `.btn-level2` after `.btn-secondary:hover` line**

Find this in `app.css`:
```css
.btn-secondary:hover { color: var(--ink); border-color: var(--border-strong); }
```

Add immediately after:
```css
.btn-level2 {
  background: #ffffff;
  color: #444444;
  border-color: var(--border);
}
.btn-level2:hover { border-color: var(--border-strong); color: var(--ink); }
```

- [ ] **Step 2: Update `.info-right` to stack buttons vertically**

Find:
```css
.info-right {
  display: flex;
  align-items: flex-end;
  justify-content: flex-end;
}
```

Replace with:
```css
.info-right {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  justify-content: flex-end;
  gap: 8px;
}
```

- [ ] **Step 3: Add modal CSS at the end of `app.css`**

Append:
```css
/* ── MODAL ──────────────────────────────────────────────────────────────────── */
.modal-overlay {
  display: none;
  position: fixed;
  inset: 0;
  background: rgba(0,0,0,.55);
  z-index: 1000;
  align-items: center;
  justify-content: center;
}
.modal-overlay.is-open { display: flex; }

.modal-panel {
  background: var(--surface);
  border-radius: var(--radius-md);
  padding: 28px 32px;
  width: 100%;
  max-width: 400px;
  position: relative;
  box-shadow: 0 8px 32px rgba(0,0,0,.2);
}

.modal-close {
  position: absolute;
  top: 12px;
  right: 14px;
  background: none;
  border: none;
  font-size: 1.3rem;
  color: var(--light-text);
  cursor: pointer;
  line-height: 1;
  padding: 2px 6px;
}
.modal-close:hover { color: var(--ink); }

.modal-title {
  font-family: var(--font-display);
  font-size: 1.05rem;
  font-weight: 700;
  margin-bottom: 8px;
  color: var(--ink);
}

.modal-desc {
  font-size: .875rem;
  color: var(--light-text);
  margin-bottom: 16px;
  line-height: 1.5;
}

.modal-error {
  font-size: .8rem;
  color: var(--red);
  margin-top: 8px;
  min-height: 1.2em;
}

.modal-success {
  font-size: .9rem;
  color: var(--green);
  line-height: 1.5;
  padding: 8px 0;
}

.modal-form-group { margin-bottom: 12px; }
```

- [ ] **Step 4: Verify CSS loads without errors**

Open a browser devtools console on any page served locally — no CSS parse errors expected.

- [ ] **Step 5: Commit**

```bash
git add web/static/app.css
git commit -m "feat: add btn-level2 style and modal CSS"
```

---

## Task D: Detail Page — Button, Modal HTML, Modal JS

**Files:**
- Modify: `web/templates/detail.html`

- [ ] **Step 1: Add the owner-edit button above the download button**

Find in `detail.html` (inside `<div class="info-right">`):
```html
        <div class="info-right">
          <button id="download-btn" class="btn btn-primary" disabled>
```

Replace with:
```html
        <div class="info-right">
          <button id="owner-edit-btn" class="btn btn-level2">Edit durch Besitzer</button>
          <button id="download-btn" class="btn btn-primary" disabled>
```

- [ ] **Step 2: Add modal HTML before `<span class="app-version">`**

Find:
```html
  <span class="app-version">{{appVersion}}</span>
```

Add immediately before it:
```html
  <div id="edit-modal" class="modal-overlay" role="dialog" aria-modal="true" aria-labelledby="modal-title">
    <div class="modal-panel">
      <button class="modal-close" id="modal-close" aria-label="Schließen">&#x00D7;</button>
      <h2 class="modal-title" id="modal-title">Edit-Link anfordern</h2>
      <p class="modal-desc">Gib deine E-Mail-Adresse ein. Wir schicken dir einen neuen Editier-Link.</p>
      <div id="modal-form">
        <div class="modal-form-group">
          <label for="modal-email">E-Mail-Adresse</label>
          <input type="email" id="modal-email" autocomplete="email" placeholder="name@example.com">
        </div>
        <div id="modal-error" class="modal-error"></div>
        <button type="button" id="modal-submit" class="btn btn-primary btn-full" style="margin-top:12px">Senden</button>
      </div>
      <div id="modal-success" class="modal-success" style="display:none">
        Wir haben dir einen Link an deine E-Mail geschickt.
      </div>
    </div>
  </div>
```

- [ ] **Step 3: Add modal JS at the bottom of the existing `<script>` block**

Find the closing `</script>` tag (after all the existing JS) and insert before it:

```javascript
    // ── OWNER EDIT MODAL ────────────────────────────────────────────────────
    (function () {
      const overlay    = document.getElementById('edit-modal');
      const closeBtn   = document.getElementById('modal-close');
      const submitBtn  = document.getElementById('modal-submit');
      const emailInput = document.getElementById('modal-email');
      const errorEl    = document.getElementById('modal-error');
      const formEl     = document.getElementById('modal-form');
      const successEl  = document.getElementById('modal-success');
      const openBtn    = document.getElementById('owner-edit-btn');

      function openModal() {
        emailInput.value      = '';
        errorEl.textContent   = '';
        formEl.style.display  = '';
        successEl.style.display = 'none';
        submitBtn.disabled    = false;
        submitBtn.textContent = 'Senden';
        overlay.classList.add('is-open');
        emailInput.focus();
      }

      function closeModal() {
        overlay.classList.remove('is-open');
      }

      openBtn.addEventListener('click', openModal);
      closeBtn.addEventListener('click', closeModal);
      overlay.addEventListener('click', function (e) {
        if (e.target === overlay) { closeModal(); }
      });
      document.addEventListener('keydown', function (e) {
        if (e.key === 'Escape') { closeModal(); }
      });

      submitBtn.addEventListener('click', async function () {
        errorEl.textContent = '';
        const email = emailInput.value.trim();
        if (!email) {
          errorEl.textContent = 'Bitte gib deine E-Mail-Adresse ein.';
          return;
        }
        submitBtn.disabled    = true;
        submitBtn.textContent = 'Wird gesendet\u2026';

        try {
          const resp = await fetch('/' + templateName + '/request-token', {
            method:  'POST',
            headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
            body:    'email=' + encodeURIComponent(email),
          });
          const data = await resp.json();
          if (data.ok) {
            formEl.style.display    = 'none';
            successEl.style.display = '';
          } else {
            errorEl.textContent   = data.error || 'Unbekannter Fehler.';
            submitBtn.disabled    = false;
            submitBtn.textContent = 'Senden';
          }
        } catch (e) {
          errorEl.textContent   = 'Netzwerkfehler: ' + e.message;
          submitBtn.disabled    = false;
          submitBtn.textContent = 'Senden';
        }
      });
    }());
```

Note: `templateName` is already defined at the top of the outer `<script>` block as `const templateName = "{{.Name}}";` and is accessible inside the IIFE via closure.

- [ ] **Step 4: Build and smoke-test**

```bash
go build ./cmd/zza/
```
Expected: clean build, no errors.

Start server locally and open any template detail page. Verify:
1. „Edit durch Besitzer"-Button erscheint oberhalb von „PNG herunterladen"
2. Klick öffnet Modal
3. × und Klick auf Overlay schließen Modal
4. Escape-Taste schließt Modal
5. Falsche E-Mail → Fehlermeldung erscheint im Modal
6. Korrekte E-Mail → Bestätigungstext erscheint (oder Dev-Log zeigt Edit-Link)
7. PNG-Download-Button funktioniert weiterhin

- [ ] **Step 5: Commit**

```bash
git add web/templates/detail.html
git commit -m "feat: add owner edit-link modal to detail page"
```
