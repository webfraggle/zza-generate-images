# Create-New-Template Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a `/create-new` page with a form (email, display size, template-ID, title, description) that creates a template directory with customised YAML and triggers the existing email-token editor flow.

**Architecture:** New `createHandler` in `internal/server/create_handlers.go` (mirrors `editorState` pattern). Template creation logic in new `internal/editor/create.go`. Two new HTML templates. Route registration called from `cmd/zza/main.go` alongside `RegisterEditorRoutes`.

**Tech Stack:** Go stdlib (`net/http`, `os`, `path/filepath`), `database/sql`, existing `renderer.ValidateTemplateName`, `editor.RequestToken`, `editor.SendTokenMail`.

---

## File Map

| Action | Path | Responsibility |
|---|---|---|
| Create | `internal/editor/create.go` | `CreateTemplate` + YAML generation |
| Create | `internal/editor/create_test.go` | Tests for `CreateTemplate` |
| Create | `internal/server/create_handlers.go` | `createHandler`, routes, 3 handlers |
| Create | `internal/server/create_handlers_test.go` | HTTP handler tests |
| Create | `web/templates/create-new.html` | Form page |
| Create | `web/templates/create-sent.html` | Success page |
| Modify | `cmd/zza/main.go` | Call `RegisterCreateRoutes` |
| Modify | `web/templates/gallery.html` | Add "+ Neues Template" link |

---

## Task 1: `editor.CreateTemplate` — write failing tests

**Files:**
- Create: `internal/editor/create_test.go`

- [ ] **Step 1: Write failing tests**

```go
// internal/editor/create_test.go
package editor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCreateTemplate_CreatesFiles(t *testing.T) {
	dir := t.TempDir()
	err := CreateTemplate(dir, "my-tmpl", "Mein Template", "Eine Beschreibung", `1.05"`, 240, 240)
	if err != nil {
		t.Fatalf("CreateTemplate: %v", err)
	}

	// Directory must exist.
	info, err := os.Stat(filepath.Join(dir, "my-tmpl"))
	if err != nil || !info.IsDir() {
		t.Fatal("template directory not created")
	}

	// template.yaml must exist and contain meta fields.
	yaml, err := os.ReadFile(filepath.Join(dir, "my-tmpl", "template.yaml"))
	if err != nil {
		t.Fatalf("template.yaml not written: %v", err)
	}
	yamlStr := string(yaml)
	for _, want := range []string{
		`name: "Mein Template"`,
		`description: "Eine Beschreibung"`,
		`display: "1.05\""`,
		`width: 240`,
		`height: 240`,
	} {
		if !strings.Contains(yamlStr, want) {
			t.Errorf("template.yaml missing %q\ngot:\n%s", want, yamlStr)
		}
	}

	// default.json must exist.
	if _, err := os.Stat(filepath.Join(dir, "my-tmpl", "default.json")); err != nil {
		t.Fatal("default.json not written")
	}
}

func TestCreateTemplate_SmallDisplay(t *testing.T) {
	dir := t.TempDir()
	err := CreateTemplate(dir, "small", "Klein", "", `0.96"`, 160, 160)
	if err != nil {
		t.Fatalf("CreateTemplate: %v", err)
	}
	yaml, _ := os.ReadFile(filepath.Join(dir, "small", "template.yaml"))
	yamlStr := string(yaml)
	for _, want := range []string{`display: "0.96\""`, `width: 160`, `height: 160`} {
		if !strings.Contains(yamlStr, want) {
			t.Errorf("missing %q in yaml:\n%s", want, yamlStr)
		}
	}
}

func TestCreateTemplate_FailsIfExists(t *testing.T) {
	dir := t.TempDir()
	// Create the dir manually.
	if err := os.Mkdir(filepath.Join(dir, "exists"), 0o755); err != nil {
		t.Fatal(err)
	}
	err := CreateTemplate(dir, "exists", "X", "", `1.05"`, 240, 240)
	if err == nil {
		t.Fatal("expected error when directory already exists, got nil")
	}
}

func TestCreateTemplate_EscapesYAMLSpecialChars(t *testing.T) {
	dir := t.TempDir()
	err := CreateTemplate(dir, "esc", `Say "hello"`, `Back\slash`, `1.05"`, 240, 240)
	if err != nil {
		t.Fatalf("CreateTemplate: %v", err)
	}
	yaml, _ := os.ReadFile(filepath.Join(dir, "esc", "template.yaml"))
	yamlStr := string(yaml)
	if !strings.Contains(yamlStr, `Say \"hello\"`) {
		t.Errorf("double quotes not escaped:\n%s", yamlStr)
	}
	if !strings.Contains(yamlStr, `Back\\slash`) {
		t.Errorf("backslash not escaped:\n%s", yamlStr)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /path/to/repo && go test ./internal/editor/... -run TestCreateTemplate -v
```
Expected: `FAIL` — `undefined: CreateTemplate`

---

## Task 2: `editor.CreateTemplate` — implement

**Files:**
- Create: `internal/editor/create.go`

- [ ] **Step 1: Write the implementation**

```go
// internal/editor/create.go
package editor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// yamlEscapeStr escapes a string for use inside YAML double-quoted scalars.
// Only `\` and `"` need escaping in YAML double-quoted strings.
func yamlEscapeStr(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return s
}

// generateStarterYAML builds a starter template.yaml with the given meta fields.
// canvasW and canvasH define the image dimensions; layers are scaled accordingly.
func generateStarterYAML(name, description, display string, canvasW, canvasH int) []byte {
	halfH := canvasH / 2
	return []byte(fmt.Sprintf(`meta:
  name: "%s"
  description: "%s"
  author: ""
  version: "1.0"
  display: "%s"
  canvas:
    width: %d
    height: %d

layers:
  # Obere Hälfte
  - type: rect
    x: 0
    y: 0
    width: %d
    height: %d
    color: "#1a1a1a"

  # Untere Hälfte (Kopie der oberen)
  - type: copy
    src_x: 0
    src_y: 0
    src_width: %d
    src_height: %d
    x: 0
    y: %d
`,
		yamlEscapeStr(name),
		yamlEscapeStr(description),
		yamlEscapeStr(display),
		canvasW, canvasH,
		canvasW, halfH,
		canvasW, halfH,
		halfH,
	))
}

// CreateTemplate creates a new template directory seeded with a customised
// template.yaml and a starter default.json.
// Returns an error if the directory already exists (race-condition guard).
func CreateTemplate(templatesDir, templateName, name, description, display string, canvasW, canvasH int) error {
	dir, err := templateDir(templatesDir, templateName)
	if err != nil {
		return err
	}
	// os.Mkdir (not MkdirAll) fails atomically if dir exists — race guard.
	if err := os.Mkdir(dir, 0o755); err != nil {
		return fmt.Errorf("editor: template %q already exists or cannot be created: %w", templateName, err)
	}
	yamlData := generateStarterYAML(name, description, display, canvasW, canvasH)
	if err := os.WriteFile(filepath.Join(dir, "template.yaml"), yamlData, 0o644); err != nil {
		return fmt.Errorf("editor: writing template.yaml: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "default.json"), starterDefaultJSON, 0o644); err != nil {
		return fmt.Errorf("editor: writing default.json: %w", err)
	}
	return nil
}
```

- [ ] **Step 2: Run tests to verify they pass**

```bash
go test ./internal/editor/... -run TestCreateTemplate -v
```
Expected: all 4 `TestCreateTemplate_*` tests `PASS`

- [ ] **Step 3: Run full editor test suite**

```bash
go test ./internal/editor/... -v
```
Expected: all tests `PASS`, no regressions

- [ ] **Step 4: Commit**

```bash
git add internal/editor/create.go internal/editor/create_test.go
git commit -m "feat(editor): add CreateTemplate for seeded template directory creation"
```

---

## Task 3: `createHandler` + check endpoint — write failing tests

**Files:**
- Create: `internal/server/create_handlers_test.go`

- [ ] **Step 1: Write failing tests for the check endpoint**

```go
// internal/server/create_handlers_test.go
package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/webfraggle/zza-generate-images/internal/config"
	"github.com/webfraggle/zza-generate-images/internal/db"
	"github.com/webfraggle/zza-generate-images/web"
)

// newTestCreateServer returns a Server with create routes registered,
// using an isolated temp dir for templates and an in-memory SQLite DB.
// Returns the server and the temp templates dir path.
func newTestCreateServer(t *testing.T) (*Server, string) {
	t.Helper()
	tdir := t.TempDir()
	srv, err := New(&config.Config{
		Port:             "8080",
		TemplatesDir:     tdir,
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
	srv.RegisterCreateRoutes(database, EditorConfig{TokenTTL: time.Hour})
	return srv, tdir
}

func TestCreateCheck_Available(t *testing.T) {
	srv, _ := newTestCreateServer(t)

	req := httptest.NewRequest(http.MethodGet, "/create-new/check?id=mein-template", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("check: got %d, want 200", rr.Code)
	}
	var resp map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["available"] != true {
		t.Errorf("expected available=true, got %v", resp)
	}
}

func TestCreateCheck_InvalidFormat(t *testing.T) {
	srv, _ := newTestCreateServer(t)

	for _, badID := range []string{"", "UPPER", "has space", "dot.here", strings.Repeat("a", 65)} {
		req := httptest.NewRequest(http.MethodGet, "/create-new/check?id="+badID, nil)
		rr := httptest.NewRecorder()
		srv.ServeHTTP(rr, req)

		var resp map[string]any
		json.NewDecoder(rr.Body).Decode(&resp)
		if resp["available"] != false {
			t.Errorf("id %q: expected available=false, got %v", badID, resp)
		}
	}
}

func TestCreateCheck_DirectoryExists(t *testing.T) {
	srv, tdir := newTestCreateServer(t)

	// Pre-create the directory.
	if err := os.Mkdir(filepath.Join(tdir, "taken"), 0o755); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/create-new/check?id=taken", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	var resp map[string]any
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp["available"] != false {
		t.Errorf("expected available=false for existing dir, got %v", resp)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/server/... -run TestCreate -v
```
Expected: `FAIL` — `undefined: RegisterCreateRoutes`

---

## Task 4: `createHandler` — implement check + GET handler

**Files:**
- Create: `internal/server/create_handlers.go`

- [ ] **Step 1: Write the handler file**

```go
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

	type createSentData struct {
		Email        string
		TemplateName string
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = ch.tmpl.ExecuteTemplate(w, "create-sent.html", createSentData{Email: email, TemplateName: id})
}
```

- [ ] **Step 2: Run check-endpoint tests**

```bash
go test ./internal/server/... -run "TestCreateCheck" -v
```
Expected: all 3 `TestCreateCheck_*` tests `PASS`

---

## Task 5: POST handler tests

**Files:**
- Modify: `internal/server/create_handlers_test.go` — add POST tests

- [ ] **Step 1: Add POST tests to the test file**

Append to `internal/server/create_handlers_test.go`:

```go
func TestCreateNew_Get(t *testing.T) {
	srv, _ := newTestCreateServer(t)

	req := httptest.NewRequest(http.MethodGet, "/create-new", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("GET /create-new: got %d, want 200", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "Template-ID") {
		t.Error("form page should contain 'Template-ID'")
	}
}

func TestCreateSubmit_CreatesTemplate(t *testing.T) {
	srv, tdir := newTestCreateServer(t)

	body := strings.NewReader("id=mein-template&email=owner@example.com&title=Mein+Template&description=Desc&display=1.05")
	req := httptest.NewRequest(http.MethodPost, "/create-new", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("POST /create-new: got %d, want 200\nbody: %s", rr.Code, rr.Body.String())
	}
	// Response should be the success page.
	if !strings.Contains(rr.Body.String(), "owner@example.com") {
		t.Error("success page should mention the email address")
	}
	// Template directory must have been created.
	if _, err := os.Stat(filepath.Join(tdir, "mein-template", "template.yaml")); err != nil {
		t.Errorf("template.yaml not created: %v", err)
	}
}

func TestCreateSubmit_InvalidID(t *testing.T) {
	srv, _ := newTestCreateServer(t)

	body := strings.NewReader("id=INVALID+ID&email=x@example.com&title=T&description=&display=1.05")
	req := httptest.NewRequest(http.MethodPost, "/create-new", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "Ungültige Template-ID") {
		t.Errorf("expected error message, got: %s", rr.Body.String())
	}
}

func TestCreateSubmit_DuplicateID(t *testing.T) {
	srv, tdir := newTestCreateServer(t)

	// Pre-create the directory.
	if err := os.Mkdir(filepath.Join(tdir, "taken"), 0o755); err != nil {
		t.Fatal(err)
	}

	body := strings.NewReader("id=taken&email=x@example.com&title=T&description=&display=1.05")
	req := httptest.NewRequest(http.MethodPost, "/create-new", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if !strings.Contains(rr.Body.String(), "bereits vergeben") {
		t.Errorf("expected 'bereits vergeben', got: %s", rr.Body.String())
	}
}
```

- [ ] **Step 2: Run all create handler tests**

```bash
go test ./internal/server/... -run "TestCreate" -v
```
Expected: all `TestCreate*` tests `PASS`

- [ ] **Step 3: Run full server test suite**

```bash
go test ./internal/server/... -v
```
Expected: all tests `PASS`, no regressions

- [ ] **Step 4: Commit**

```bash
git add internal/server/create_handlers.go internal/server/create_handlers_test.go
git commit -m "feat(server): add /create-new handlers with async ID check"
```

---

## Task 6: HTML templates

**Files:**
- Create: `web/templates/create-new.html`
- Create: `web/templates/create-sent.html`

- [ ] **Step 1: Create `web/templates/create-new.html`**

```html
<!DOCTYPE html>
<html lang="de">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Neues Template anlegen — Zugzielanzeiger</title>
  <link rel="stylesheet" href="https://use.typekit.net/ldx6jxj.css">
  <link rel="stylesheet" href="/static/app.css">
</head>
<body class="form-page">

<div class="form-card">
  <h1>Neues Template anlegen</h1>

  <p class="form-desc">
    Hier kannst du ein neues Template erstellen. Gib deine E-Mail-Adresse an —
    du bekommst einen Link zum Editor. Die Template-ID ist dein eindeutiger Name
    in der URL, z.&thinsp;B. <code>/mein-template</code>.
  </p>

  {{if .Error}}
  <p class="error-text">{{.Error}}</p>
  {{end}}

  <form method="POST" action="/create-new" id="create-form">

    <div class="form-group">
      <label for="template-id">Template-ID</label>
      <input type="text" id="template-id" name="id" required
             pattern="[a-z0-9\-]+" maxlength="64"
             placeholder="mein-template"
             value="{{.ID}}"
             autocomplete="off">
      <span id="id-status" class="field-hint"></span>
    </div>

    <div class="form-group">
      <label for="title">Titel</label>
      <input type="text" id="title" name="title" required maxlength="80"
             placeholder="Mein Zugzielanzeiger"
             value="{{.Title}}">
    </div>

    <div class="form-group">
      <label for="description">Beschreibung <span class="label-optional">(optional)</span></label>
      <textarea id="description" name="description" maxlength="300"
                placeholder="Kurze Beschreibung des Templates">{{.Description}}</textarea>
    </div>

    <div class="form-group">
      <label>Display-Größe</label>
      <div class="radio-group">
        <label class="radio-label">
          <input type="radio" name="display" value="1.05"
                 {{if or (eq .Display "1.05") (eq .Display "")}}checked{{end}}>
          1.05&Prime; — 240&times;240 px
        </label>
        <label class="radio-label">
          <input type="radio" name="display" value="0.96"
                 {{if eq .Display "0.96"}}checked{{end}}>
          0.96&Prime; — 160&times;160 px
        </label>
      </div>
    </div>

    <div class="form-group">
      <label for="email">E-Mail-Adresse</label>
      <input type="email" id="email" name="email" required autocomplete="email"
             placeholder="name@example.com"
             value="{{.Email}}">
    </div>

    <button type="submit" id="submit-btn" class="btn btn-primary btn-full" disabled>
      Template anlegen
    </button>
  </form>
</div>
<span class="app-version">{{appVersion}}</span>

<script>
(function () {
  const idInput  = document.getElementById('template-id');
  const idStatus = document.getElementById('id-status');
  const submitBtn = document.getElementById('submit-btn');
  let checkTimer = null;
  let idOk = false;

  function setStatus(msg, state) {
    // state: 'ok' | 'error' | 'checking' | ''
    idOk = state === 'ok';
    idStatus.textContent = msg;
    idStatus.className = 'field-hint' + (state ? ' field-hint--' + state : '');
    submitBtn.disabled = !idOk;
  }

  function check() {
    const val = idInput.value.trim();
    if (!val) { setStatus('', ''); return; }
    if (!/^[a-z0-9-]+$/.test(val) || val.length > 64) {
      setStatus('Nur Kleinbuchstaben, Ziffern und Bindestriche erlaubt.', 'error');
      return;
    }
    setStatus('Wird geprüft\u2026', 'checking');
    fetch('/create-new/check?id=' + encodeURIComponent(val))
      .then(function(r) { return r.json(); })
      .then(function(d) {
        if (d.available) {
          setStatus('\u2713 Verfügbar', 'ok');
        } else {
          setStatus('\u2717 ' + (d.reason || 'Nicht verfügbar'), 'error');
        }
      })
      .catch(function() { setStatus('Fehler beim Prüfen.', 'error'); });
  }

  idInput.addEventListener('input', function () {
    clearTimeout(checkTimer);
    setStatus('', '');
    checkTimer = setTimeout(check, 300);
  });

  // If the field is pre-filled (e.g. after a server-side error), trigger check.
  if (idInput.value) { check(); }
}());
</script>
</body>
</html>
```

- [ ] **Step 2: Create `web/templates/create-sent.html`**

```html
<!DOCTYPE html>
<html lang="de">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Template angelegt — Zugzielanzeiger</title>
  <link rel="stylesheet" href="https://use.typekit.net/ldx6jxj.css">
  <link rel="stylesheet" href="/static/app.css">
</head>
<body class="form-page">

<div class="form-card form-card-centered">
  <span class="sent-icon">✉️</span>
  <h1>Fast geschafft!</h1>

  <p class="form-desc">
    Das Template <strong>{{.TemplateName}}</strong> wurde angelegt.<br>
    Wir haben einen Editier-Link an <strong>{{.Email}}</strong> geschickt.
  </p>

  <p class="form-note">
    Klick den Link in der Mail, um deinen Template-Editor zu öffnen.
    Der Link ist für die konfigurierte Dauer gültig. Schau auch im Spam-Ordner nach.
  </p>

  <a href="/" class="form-link">← Zurück zur Galerie</a>
</div>
<span class="app-version">{{appVersion}}</span>
</body>
</html>
```

- [ ] **Step 3: Verify templates compile (build check)**

```bash
go build ./...
```
Expected: no errors (the HTML templates are embedded via `web.FS` and parsed at runtime — a build confirms no import issues)

- [ ] **Step 4: Commit**

```bash
git add web/templates/create-new.html web/templates/create-sent.html
git commit -m "feat(web): add create-new and create-sent HTML templates"
```

---

## Task 7: Wire up routes in `cmd/zza/main.go`

**Files:**
- Modify: `cmd/zza/main.go`

- [ ] **Step 1: Add `RegisterCreateRoutes` call**

In `cmd/zza/main.go`, find the block that calls `srv.RegisterEditorRoutes(...)` (around line 106). Add the create routes call directly after it:

```go
			// Register editor routes.
			srv.RegisterEditorRoutes(database, server.EditorConfig{
				TokenTTL: time.Duration(cfg.EditTokenTTLHours) * time.Hour,
				Mail: editor.MailConfig{
					Host:    cfg.SMTPHost,
					Port:    cfg.SMTPPort,
					User:    cfg.SMTPUser,
					Pass:    cfg.SMTPPass,
					From:    cfg.SMTPFrom,
					BaseURL: cfg.BaseURL,
				},
			})

			// Register create-new routes (same config as editor).
			srv.RegisterCreateRoutes(database, server.EditorConfig{
				TokenTTL: time.Duration(cfg.EditTokenTTLHours) * time.Hour,
				Mail: editor.MailConfig{
					Host:    cfg.SMTPHost,
					Port:    cfg.SMTPPort,
					User:    cfg.SMTPUser,
					Pass:    cfg.SMTPPass,
					From:    cfg.SMTPFrom,
					BaseURL: cfg.BaseURL,
				},
			})
```

- [ ] **Step 2: Build to verify**

```bash
go build ./cmd/zza/...
```
Expected: no errors

- [ ] **Step 3: Commit**

```bash
git add cmd/zza/main.go
git commit -m "feat(main): register /create-new routes on server startup"
```

---

## Task 8: Gallery header link

**Files:**
- Modify: `web/templates/gallery.html`

- [ ] **Step 1: Add "+ Neues Template" link to the gallery header nav**

In `web/templates/gallery.html`, find the `<nav class="header-nav">` block (lines 13–16) and add the create link:

```html
    <nav class="header-nav">
      <a href="/" class="nav-link active">Galerie</a>
      <a href="/create-new" class="btn btn-primary">+ Neues Template</a>
      <a href="/admin" class="nav-link muted">Admin</a>
    </nav>
```

- [ ] **Step 2: Build and full test run**

```bash
go build ./... && go test ./...
```
Expected: build succeeds, all tests `PASS`

- [ ] **Step 3: Manual smoke test**

Start the server in dev mode (no SMTP needed — edit links are logged):
```bash
go run ./cmd/zza serve
```
1. Open `http://localhost:8080` — verify "+ Neues Template" link appears in header
2. Click the link — verify form loads at `/create-new`
3. Type an ID in the Template-ID field — verify async check fires after 300ms
4. Fill all fields and submit — verify "Fast geschafft!" page appears with correct email
5. Check server log for `[DEV] edit link for new template "..." ...`
6. Open the logged link — verify the editor opens with the correct template name
7. In the editor, verify `template.yaml` contains the title, description, display, and canvas dimensions you entered

- [ ] **Step 4: Final commit**

```bash
git add web/templates/gallery.html
git commit -m "feat(gallery): add '+ Neues Template' header link to /create-new"
```
