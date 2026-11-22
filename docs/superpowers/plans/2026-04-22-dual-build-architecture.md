# Dual-Build-Architektur Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Split the single-binary ZZA into a slim server-build (preview + render only, no editor/auth/DB) and a full desktop-build (editor + preview + render + native GUI via Wails v2), from the same codebase.

**Architecture:** Two `cmd/`-main-packages share `internal/renderer`, `internal/config`, `internal/version`, `internal/server`, `internal/gallery`. Desktop-only code (`internal/editor`, `internal/desktop`, `internal/cli`) is imported only by `cmd/zza`. The deleted packages (`internal/admin`, `internal/db`) disappear entirely along with SMTP, token auth, admin TOTP, and SQLite. A single HTML preview template is shared between both builds via a boolean `EditorEnabled` flag.

**Tech Stack:** Go 1.26, Cobra CLI, Wails v2 (webview wrapper, CGO), `html/template`, `archive/zip`, existing vanilla JS + CodeMirror front-end.

**Spec:** `docs/superpowers/specs/2026-04-22-dual-build-architecture-design.md`

---

## File Structure After Refactor

**New files:**
- `cmd/zza-server/main.go` — slim server entrypoint
- `cmd/zza/main.go` — Cobra root (default=GUI, `serve`, `render`, `version`)
- `internal/server/zip.go` + `internal/server/zip_test.go` — template ZIP streaming handler
- `internal/editor/fs_handlers.go` + `internal/editor/fs_handlers_test.go` — auth-free file/YAML/upload/delete handlers (template-name addressed)
- `internal/desktop/paths.go` + `internal/desktop/paths_test.go` — templates-dir resolution (Windows/macOS app bundle/bare binary)
- `internal/desktop/run.go` — Wails bootstrap + browser fallback
- `wails.json` — Wails v2 config in repo root

**Modified files:**
- `internal/config/config.go` — strip SMTP/DB/Admin/Token fields, add `SMTPEnabled`-style pruning
- `internal/server/server.go` — register ZIP route, add `EditorEnabled` to preview data
- `web/templates/detail.html` — conditionally show Edit-button + ZIP-button via `{{if .EditorEnabled}}`
- `web/templates/gallery.html` — conditionally show admin link via `{{if .EditorEnabled}}`
- `web/templates/edit-editor.html` — replace `TOKEN` variable with `TEMPLATE`, rewrite fetch URLs to `/edit/{template}/...`
- `build.sh` — add Wails build targets, produce release ZIPs with templates folder
- `Dockerfile` — switch `cmd/zza` → `cmd/zza-server`, drop db/ca-certs volumes, simplify

**Deleted files/dirs:**
- `internal/admin/` (entire directory)
- `internal/db/` (entire directory)
- `internal/editor/auth.go`, `internal/editor/auth_test.go`
- `internal/editor/create.go`, `internal/editor/create_test.go`
- `internal/editor/mailer.go`, `internal/editor/mailer_test.go`
- `internal/server/admin_handlers.go`
- `internal/server/create_handlers.go`, `internal/server/create_handlers_test.go`
- `internal/server/request_token_handler.go`, `internal/server/request_token_handler_test.go`
- `internal/server/editor_handlers.go` (replaced by `internal/editor/fs_handlers.go`)
- `internal/server/ip_limiter.go`, `internal/server/ip_limiter_test.go`
- `web/templates/admin-*.html` (4 files)
- `web/templates/edit-request.html`, `web/templates/edit-sent.html`
- `web/templates/create-new.html`, `web/templates/create-sent.html`
- `cmd/zza-desktop/` (entire directory — superseded by new `cmd/zza`)

---

## Task 1: Create feature branch + snapshot baseline tests

**Files:** _(no code changes yet)_

- [ ] **Step 1: Create branch and confirm starting state**

Run:
```bash
git checkout develop
git pull
git checkout -b feature/dual-build
go test ./... 2>&1 | tee /tmp/zza-baseline-tests.log
```
Expected: current `develop` tests all pass. Note test count for later comparison.

- [ ] **Step 2: Record baseline**

Run:
```bash
grep -cE '^(=== RUN|--- FAIL|--- PASS|ok)' /tmp/zza-baseline-tests.log
```
Write the PASS/FAIL count into the commit message of the first real change so regressions become visible.

- [ ] **Step 3: Commit the branch marker**

```bash
git commit --allow-empty -m "chore: start dual-build refactor on feature/dual-build"
```

---

## Task 2: Move `cmd/zza` → `cmd/zza-server`, delete `cmd/zza-desktop`, stub new `cmd/zza`

Goal: rename without behavior change, so `go build ./...` still passes. The real new `cmd/zza` is finalised in Task 16.

**Files:**
- Rename: `cmd/zza/main.go` → `cmd/zza-server/main.go`
- Delete: `cmd/zza-desktop/`
- Create: `cmd/zza/main.go` (temporary Cobra stub that only calls Render CLI)

- [ ] **Step 1: Rename the server entrypoint**

```bash
git mv cmd/zza cmd/zza-server
# inside cmd/zza-server/main.go update cobra.Command.Use:
```

Edit `cmd/zza-server/main.go` line 32:
```go
		Use:   "zza-server",
		Short: "Zugzielanzeiger image generator (server)",
```

- [ ] **Step 2: Update Dockerfile build target + entrypoint**

Edit `Dockerfile` — change the two references from `./cmd/zza` / `/app/zza` to the new server path:
```dockerfile
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags="-s -w -X github.com/webfraggle/zza-generate-images/internal/version.Version=${ZZA_VERSION}" -o zza-server ./cmd/zza-server
```
And:
```dockerfile
COPY --from=builder /app/zza-server .
...
ENTRYPOINT ["/app/zza-server", "serve"]
```

- [ ] **Step 3: Delete old desktop CLI**

```bash
git rm -r cmd/zza-desktop
```

- [ ] **Step 4: Create placeholder `cmd/zza/main.go`**

```go
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/webfraggle/zza-generate-images/internal/cli"
)

func main() {
	if err := rootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func rootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "zza",
		Short: "Zugzielanzeiger desktop (editor + preview + render)",
	}
	root.AddCommand(cli.RenderCmd())
	return root
}
```

- [ ] **Step 5: Update `build.sh` binary names (minimal patch; Wails additions in Task 17)**

Replace all three `-o "$OUTDIR/zza-desktop-…"` with `-o "$OUTDIR/zza-…"` and all `./cmd/zza-desktop` with `./cmd/zza`. Rename `zza-desktop.exe` → `zza.exe` in the Windows target.

- [ ] **Step 6: Verify the build still works**

Run:
```bash
go build ./... && go test ./...
```
Expected: PASS. If not, fix imports before continuing.

- [ ] **Step 7: Commit**

```bash
git add -A
git commit -m "refactor: rename cmd/zza to cmd/zza-server, stub new cmd/zza"
```

---

## Task 3: Shared ZIP-download handler (TDD)

Route `GET /{template}.zip` streams `template.yaml` + `default.json` + assets as a ZIP. Lives in `internal/server/`, shared by both builds.

**Files:**
- Create: `internal/server/zip.go`
- Create: `internal/server/zip_test.go`
- Modify: `internal/server/server.go` (register route + disallow on path traversal)

- [ ] **Step 1: Write the failing test**

Create `internal/server/zip_test.go`:
```go
package server

import (
	"archive/zip"
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestServer_TemplateZip_ContainsExpectedFiles(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/sbb-096-v1.zip", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("zip: got %d, body: %s", rr.Code, rr.Body.String())
	}
	if ct := rr.Header().Get("Content-Type"); ct != "application/zip" {
		t.Errorf("Content-Type: got %q, want application/zip", ct)
	}
	if cd := rr.Header().Get("Content-Disposition"); cd == "" {
		t.Error("missing Content-Disposition header")
	}

	zr, err := zip.NewReader(bytes.NewReader(rr.Body.Bytes()), int64(rr.Body.Len()))
	if err != nil {
		t.Fatalf("read zip: %v", err)
	}
	want := map[string]bool{"template.yaml": false, "default.json": false}
	for _, f := range zr.File {
		if _, ok := want[f.Name]; ok {
			want[f.Name] = true
			rc, err := f.Open()
			if err != nil {
				t.Fatalf("open %q: %v", f.Name, err)
			}
			if _, err := io.Copy(io.Discard, rc); err != nil {
				t.Errorf("read %q: %v", f.Name, err)
			}
			_ = rc.Close()
		}
	}
	for name, found := range want {
		if !found {
			t.Errorf("zip missing %q", name)
		}
	}
}

func TestServer_TemplateZip_InvalidName(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/..%2Fevil.zip", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	if rr.Code == http.StatusOK {
		t.Error("path traversal should not return 200")
	}
}

func TestServer_TemplateZip_NotFound(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/does-not-exist.zip", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("got %d, want 404", rr.Code)
	}
}
```

- [ ] **Step 2: Run test, confirm failure**

Run: `go test ./internal/server/ -run TestServer_TemplateZip -v`
Expected: FAIL — route not registered.

- [ ] **Step 3: Implement `internal/server/zip.go`**

```go
package server

import (
	"archive/zip"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/webfraggle/zza-generate-images/internal/renderer"
)

// handleTemplateZip streams the template directory (template.yaml + default.json
// + all asset files) as a ZIP archive. Directories are not recursed.
func (s *Server) handleTemplateZip(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("template")
	if err := renderer.ValidateTemplateName(name); err != nil {
		http.Error(w, "invalid template name", http.StatusBadRequest)
		return
	}
	dir := filepath.Join(s.templatesDir, name)
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		http.Error(w, "template not found", http.StatusNotFound)
		return
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		log.Printf("zip: read dir %q: %v", name, err)
		http.Error(w, "could not read template", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.zip"`, name))

	zw := zip.NewWriter(w)
	defer zw.Close()

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if err := addFileToZip(zw, dir, e.Name()); err != nil {
			log.Printf("zip: add %q: %v", e.Name(), err)
			return
		}
	}
}

func addFileToZip(zw *zip.Writer, dir, name string) error {
	src, err := os.Open(filepath.Join(dir, name))
	if err != nil {
		return err
	}
	defer src.Close()
	header, err := zw.Create(name)
	if err != nil {
		return err
	}
	_, err = io.Copy(header, src)
	return err
}
```

- [ ] **Step 4: Register the route in `server.go`**

Edit `internal/server/server.go` `registerRoutes` — add a line that matches `/{template}.zip`. Because Go's ServeMux does not match extensions inside a wildcard, dispatch via the main `ServeHTTP` pre-mux block.

Add to `ServeHTTP` after the `/edit/` block:
```go
	if strings.HasSuffix(r.URL.Path, ".zip") && r.Method == http.MethodGet {
		name := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/"), ".zip")
		r2 := r.Clone(r.Context())
		r2.SetPathValue("template", name)
		s.handleTemplateZip(w, r2)
		return
	}
```

- [ ] **Step 5: Run tests**

Run: `go test ./internal/server/ -run TestServer_TemplateZip -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/server/zip.go internal/server/zip_test.go internal/server/server.go
git commit -m "feat(server): add /{template}.zip streaming download"
```

---

## Task 4: Preview-template `EditorEnabled` flag + ZIP button

The single `detail.html` template must work for both server (no edit, with ZIP) and desktop (edit + ZIP). Introduce `EditorEnabled` in `detailData`.

**Files:**
- Modify: `internal/server/server.go` — add `EditorEnabled bool` to `detailData`, plus `Server.editorEnabled` field & setter
- Modify: `web/templates/detail.html` — conditional blocks + new ZIP-button

- [ ] **Step 1: Update `detailData` struct and Server struct**

Edit `internal/server/server.go`:
```go
// Server struct — add field:
	editorEnabled bool
```

```go
// detailData struct — add field:
	EditorEnabled bool
```

Add a setter (used by the desktop build):
```go
// SetEditorEnabled toggles the Edit-button on the preview page. Desktop sets true.
func (s *Server) SetEditorEnabled(v bool) { s.editorEnabled = v }
```

In `handleDetail`, set the flag into the view model:
```go
	d := detailData{
		Name:          templateName,
		Meta:          tmpl.Meta,
		DefaultJSON:   string(jsonBytes),
		HasDefault:    len(jsonBytes) > 0,
		EditorEnabled: s.editorEnabled,
	}
```

- [ ] **Step 2: Update `detail.html` — rewrite the right-side action buttons**

Replace lines 52–58 (the `<div class="info-right">…</div>` block):
```html
        <div class="info-right">
          {{if .EditorEnabled}}
          <a href="/edit/{{.Name}}" class="btn btn-level2">Bearbeiten</a>
          {{end}}
          <a href="/{{.Name}}.zip" class="btn btn-level2" download>Als ZIP</a>
          <button id="download-btn" class="btn btn-primary" disabled>
            <svg width="16" height="16" viewBox="0 0 16 16" fill="none" style="vertical-align:-2px;margin-right:6px"><path d="M8 2v8m0 0L5 7m3 3 3-3M3 12h10" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/></svg>PNG herunterladen
          </button>
        </div>
```

- [ ] **Step 3: Delete the entire `<div id="edit-modal">…</div>` block and the `OWNER EDIT MODAL` JS IIFE**

Remove `detail.html` lines 61–78 (modal HTML) and lines 188–259 (the trailing IIFE `// ── OWNER EDIT MODAL …` through its closing `}());`). The server-build never issues tokens; the desktop-build now links directly.

- [ ] **Step 4: Update existing server test to confirm no edit-button by default**

In `internal/server/server_test.go` extend `TestServer_Detail`:
```go
func TestServer_Detail(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/sbb-096-v1", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("detail: got %d, want 200", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "sbb-096-v1") {
		t.Error("detail page should contain template name")
	}
	if !strings.Contains(body, "json-input") {
		t.Error("detail page should contain JSON textarea")
	}
	if strings.Contains(body, "Bearbeiten") {
		t.Error("server build should NOT show edit button")
	}
	if !strings.Contains(body, "/sbb-096-v1.zip") {
		t.Error("detail page should show ZIP download link")
	}
}

func TestServer_Detail_EditorEnabled_ShowsEditButton(t *testing.T) {
	srv := newTestServer(t)
	srv.SetEditorEnabled(true)

	req := httptest.NewRequest(http.MethodGet, "/sbb-096-v1", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if !strings.Contains(rr.Body.String(), `href="/edit/sbb-096-v1"`) {
		t.Error("desktop build should link to /edit/sbb-096-v1")
	}
}
```

- [ ] **Step 5: Run tests**

Run: `go test ./internal/server/ -run TestServer_Detail -v`
Expected: PASS both new tests.

- [ ] **Step 6: Commit**

```bash
git add internal/server/server.go internal/server/server_test.go web/templates/detail.html
git commit -m "feat(server): EditorEnabled flag + ZIP download button in preview"
```

---

## Task 5: Config — remove SMTP/DB/Admin/Token fields

**Files:**
- Modify: `internal/config/config.go`

- [ ] **Step 1: Update config struct to only server-relevant fields**

Overwrite `internal/config/config.go`:
```go
package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds all runtime configuration loaded from environment variables.
type Config struct {
	Port             string
	TemplatesDir     string
	CacheDir         string
	CacheMaxAgeHours int
	CacheMaxSizeMB   int64
	BaseURL          string
}

// Load reads configuration from environment variables and applies defaults.
//
//	PORT                 default "8080"
//	TEMPLATES_DIR        default "./templates"
//	CACHE_DIR            default "./cache"
//	CACHE_MAX_AGE_HOURS  default 24
//	CACHE_MAX_SIZE_MB    default 500
//	BASE_URL             default "http://localhost:8080"
func Load() *Config {
	return &Config{
		Port:             envStr("PORT", "8080"),
		TemplatesDir:     envStr("TEMPLATES_DIR", "./templates"),
		CacheDir:         envStr("CACHE_DIR", "./cache"),
		CacheMaxAgeHours: envInt("CACHE_MAX_AGE_HOURS", 24),
		CacheMaxSizeMB:   int64(envInt("CACHE_MAX_SIZE_MB", 500)),
		BaseURL:          envStr("BASE_URL", "http://localhost:8080"),
	}
}

// ValidatePort checks that the port string is a valid TCP port number (1–65535).
func ValidatePort(port string) error {
	n, err := strconv.Atoi(port)
	if err != nil || n < 1 || n > 65535 {
		return fmt.Errorf("config: invalid PORT value %q (must be 1–65535)", port)
	}
	return nil
}

func envStr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return def
}
```

- [ ] **Step 2: Do not build or test yet — other packages still reference the deleted fields. Commit after Task 6.**

---

## Task 6: Server main — strip editor/admin/db registration

**Files:**
- Modify: `cmd/zza-server/main.go`

- [ ] **Step 1: Overwrite `cmd/zza-server/main.go`**

```go
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/webfraggle/zza-generate-images/internal/config"
	"github.com/webfraggle/zza-generate-images/internal/server"
	"github.com/webfraggle/zza-generate-images/web"
)

func main() {
	if err := rootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func rootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "zza-server",
		Short: "Zugzielanzeiger image generator (server)",
	}
	root.AddCommand(serveCmd())
	root.AddCommand(versionCmd())
	return root
}

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version and exit",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintln(cmd.OutOrStdout(), serverVersion())
		},
	}
}

func serverVersion() string {
	// imported here to keep main imports minimal
	return importedVersion()
}

func serveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Start the HTTP render server",
		Long: `Start the HTTP server. Configuration via environment variables:
  PORT                    (default: 8080)
  TEMPLATES_DIR           (default: ./templates)
  CACHE_DIR               (default: ./cache)
  CACHE_MAX_AGE_HOURS     (default: 24)
  CACHE_MAX_SIZE_MB       (default: 500)
  BASE_URL                (default: http://localhost:8080)`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.Load()
			if err := config.ValidatePort(cfg.Port); err != nil {
				return err
			}

			srv, err := server.New(cfg, web.FS)
			if err != nil {
				return fmt.Errorf("serve: %w", err)
			}
			// EditorEnabled stays false — server build has no editor.

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			srv.StartCleanup(ctx, 15*time.Minute)

			httpSrv := &http.Server{
				Addr:         ":" + cfg.Port,
				Handler:      srv,
				ReadTimeout:  30 * time.Second,
				WriteTimeout: 60 * time.Second,
				IdleTimeout:  120 * time.Second,
			}

			quit := make(chan os.Signal, 1)
			signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

			go func() {
				log.Printf("zza-server listening on :%s (templates: %s, cache: %s)",
					cfg.Port, cfg.TemplatesDir, cfg.CacheDir)
				if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
					log.Fatalf("server error: %v", err)
				}
			}()

			<-quit
			log.Println("shutting down...")
			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer shutdownCancel()
			return httpSrv.Shutdown(shutdownCtx)
		},
	}
}
```

Replace the top-level `serverVersion` → direct import. Drop the indirection and use the `version` package directly:
```go
import "github.com/webfraggle/zza-generate-images/internal/version"
// …
func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use: "version", Short: "Print version and exit",
		Run: func(cmd *cobra.Command, _ []string) { fmt.Fprintln(cmd.OutOrStdout(), version.Version) },
	}
}
```
Remove the `serverVersion()` / `importedVersion()` shim — the direct import above is the real implementation.

- [ ] **Step 2: Remove `RegisterAdminRoutes`/`RegisterEditorRoutes`/`RegisterCreateRoutes` from `internal/server/server.go` exports**

These are defined in separate files (`admin_handlers.go`, `editor_handlers.go`, `create_handlers.go`, `request_token_handler.go`). They will be deleted in Task 8 & 9. For now we have compile errors — that's fine; we fix the whole server package as one commit in Task 8.

- [ ] **Step 3: Commit WIP (intentionally broken — next tasks fix it)**

```bash
git add internal/config/config.go cmd/zza-server/main.go
git commit -m "wip(server): strip editor/admin registration from main + config

Intentionally broken build — internal/server still has orphan handler files
that reference the deleted config fields. Fixed in next commits."
```

---

## Task 7: Delete admin + db packages

**Files:**
- Delete: `internal/admin/`
- Delete: `internal/db/`
- Delete: `internal/server/admin_handlers.go`

- [ ] **Step 1: Delete the directories**

```bash
git rm -r internal/admin internal/db
git rm internal/server/admin_handlers.go
```

- [ ] **Step 2: Verify no stale imports**

Run:
```bash
grep -rn "internal/admin\|internal/db" --include="*.go"
```
Expected: no matches. If any found, fix them.

- [ ] **Step 3: Drop the SQLite driver and modernc deps from go.mod**

Run:
```bash
go mod tidy
```
Expected: `modernc.org/*`, `github.com/google/uuid`, `github.com/ncruces/go-strftime` disappear from `go.sum`/indirects (the `uuid` dep was only used by admin, sqlite by db).

- [ ] **Step 4: Commit**

```bash
git add -A
git commit -m "feat: delete internal/admin, internal/db, admin_handlers.go

SQLite and TOTP removed — no persistent state in the server build."
```

---

## Task 8: Delete server-side mail/create/request-token/editor-auth handlers & templates

**Files:**
- Delete: `internal/editor/auth.go`, `internal/editor/auth_test.go`
- Delete: `internal/editor/create.go`, `internal/editor/create_test.go`
- Delete: `internal/editor/mailer.go`, `internal/editor/mailer_test.go`
- Delete: `internal/server/editor_handlers.go` (will be replaced in Task 9 via the editor package)
- Delete: `internal/server/create_handlers.go`, `internal/server/create_handlers_test.go`
- Delete: `internal/server/request_token_handler.go`, `internal/server/request_token_handler_test.go`
- Delete: `internal/server/ip_limiter.go`, `internal/server/ip_limiter_test.go`
- Delete: `web/templates/admin-cache.html`, `admin-editor.html`, `admin-login.html`, `admin-overview.html`, `edit-request.html`, `edit-sent.html`, `create-new.html`, `create-sent.html`

- [ ] **Step 1: Delete the files**

```bash
git rm internal/editor/auth.go internal/editor/auth_test.go \
       internal/editor/create.go internal/editor/create_test.go \
       internal/editor/mailer.go internal/editor/mailer_test.go \
       internal/server/editor_handlers.go \
       internal/server/create_handlers.go internal/server/create_handlers_test.go \
       internal/server/request_token_handler.go internal/server/request_token_handler_test.go \
       internal/server/ip_limiter.go internal/server/ip_limiter_test.go \
       web/templates/admin-cache.html web/templates/admin-editor.html \
       web/templates/admin-login.html web/templates/admin-overview.html \
       web/templates/edit-request.html web/templates/edit-sent.html \
       web/templates/create-new.html web/templates/create-sent.html
```

- [ ] **Step 2: Remove `editorHandler` and `adminHandler` from Server struct**

Edit `internal/server/server.go`:
- Remove the two fields from the `Server` struct.
- Remove the two `if strings.HasPrefix(r.URL.Path, "/edit/") …` and `/admin` blocks from `ServeHTTP` (we'll re-add a new `/edit/` dispatch in Task 9).
- Remove the `checkOrigin` function and `TestCheckOrigin` test (was used by create/edit-submit — no longer needed; the desktop editor has no cross-origin concerns and the server has no write endpoints).

Delete the `TestCheckOrigin` test block from `internal/server/server_test.go` lines 246–279.

- [ ] **Step 3: Verify compile**

Run:
```bash
go build ./...
```
Expected: PASS. `internal/server`, `internal/editor` both compile (editor package now only has `files.go`, `files_test.go` and the `starter/` embed).

- [ ] **Step 4: Run tests**

Run: `go test ./...`
Expected: PASS (server tests, renderer tests, editor files test). If `files_test.go` references removed helpers from `auth.go`, remove those helper calls — files_test should only exercise `files.go`.

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "feat: remove server auth, mail, admin, create-new, token handlers"
```

---

## Task 9: Auth-free editor handlers under `/edit/{template}/…`

New handlers live in the `editor` package (shared location, used only by desktop main). URL scheme changes from `/edit/{token}/…` → `/edit/{template}/…`.

**Files:**
- Create: `internal/editor/fs_handlers.go`
- Create: `internal/editor/fs_handlers_test.go`
- Modify: `internal/server/server.go` — new dispatch block + `RegisterEditor`

- [ ] **Step 1: Write the failing test for the new handler set**

Create `internal/editor/fs_handlers_test.go`:
```go
package editor

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newEditorTestDir(t *testing.T) string {
	t.Helper()
	d := t.TempDir()
	tpl := filepath.Join(d, "mine")
	if err := os.MkdirAll(tpl, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tpl, "template.yaml"),
		[]byte("meta:\n  canvas: {width: 10, height: 10}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tpl, "default.json"),
		[]byte(`{"x":1}`), 0o644); err != nil {
		t.Fatal(err)
	}
	return d
}

func TestFSHandlers_ListFiles(t *testing.T) {
	h := NewFSHandlers(newEditorTestDir(t), nil)
	req := httptest.NewRequest(http.MethodGet, "/edit/mine/files", nil)
	req.SetPathValue("template", "mine")
	rr := httptest.NewRecorder()
	h.ListFiles(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("got %d", rr.Code)
	}
	var out struct{ Files []FileInfo }
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Files) != 2 {
		t.Errorf("want 2 files, got %d", len(out.Files))
	}
}

func TestFSHandlers_GetFile(t *testing.T) {
	h := NewFSHandlers(newEditorTestDir(t), nil)
	req := httptest.NewRequest(http.MethodGet, "/edit/mine/file/template.yaml", nil)
	req.SetPathValue("template", "mine")
	req.SetPathValue("filename", "template.yaml")
	rr := httptest.NewRecorder()
	h.GetFile(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("got %d, body: %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "canvas:") {
		t.Error("body should contain YAML")
	}
}

func TestFSHandlers_Save_WritesAndInvalidates(t *testing.T) {
	dir := newEditorTestDir(t)
	var cacheCalls int
	h := NewFSHandlers(dir, func(string) { cacheCalls++ })

	body, _ := json.Marshal(map[string]string{
		"filename": "template.yaml",
		"content":  "meta:\n  canvas: {width: 20, height: 20}\n",
	})
	req := httptest.NewRequest(http.MethodPost, "/edit/mine/save", bytes.NewReader(body))
	req.SetPathValue("template", "mine")
	rr := httptest.NewRecorder()
	h.Save(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("got %d, body: %s", rr.Code, rr.Body.String())
	}
	got, _ := os.ReadFile(filepath.Join(dir, "mine", "template.yaml"))
	if !strings.Contains(string(got), "width: 20") {
		t.Errorf("file not written: %q", got)
	}
	if cacheCalls != 1 {
		t.Errorf("want 1 cache invalidation, got %d", cacheCalls)
	}
}

func TestFSHandlers_Save_InvalidYAML(t *testing.T) {
	h := NewFSHandlers(newEditorTestDir(t), nil)
	body, _ := json.Marshal(map[string]string{
		"filename": "template.yaml",
		"content":  "this: is: not valid: yaml: ][",
	})
	req := httptest.NewRequest(http.MethodPost, "/edit/mine/save", bytes.NewReader(body))
	req.SetPathValue("template", "mine")
	rr := httptest.NewRecorder()
	h.Save(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("invalid YAML should give 400, got %d", rr.Code)
	}
}

func TestFSHandlers_Upload(t *testing.T) {
	dir := newEditorTestDir(t)
	h := NewFSHandlers(dir, nil)

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, _ := mw.CreateFormFile("file", "logo.png")
	// 8-byte PNG magic so file contents are non-zero:
	_, _ = io.Copy(fw, bytes.NewReader([]byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}))
	mw.Close()

	req := httptest.NewRequest(http.MethodPost, "/edit/mine/upload", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.SetPathValue("template", "mine")
	rr := httptest.NewRecorder()
	h.Upload(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("got %d, body: %s", rr.Code, rr.Body.String())
	}
	if _, err := os.Stat(filepath.Join(dir, "mine", "logo.png")); err != nil {
		t.Errorf("uploaded file missing: %v", err)
	}
}

func TestFSHandlers_DeleteFile(t *testing.T) {
	dir := newEditorTestDir(t)
	_ = os.WriteFile(filepath.Join(dir, "mine", "extra.png"), []byte("x"), 0o644)
	h := NewFSHandlers(dir, nil)

	req := httptest.NewRequest(http.MethodDelete, "/edit/mine/file/extra.png", nil)
	req.SetPathValue("template", "mine")
	req.SetPathValue("filename", "extra.png")
	rr := httptest.NewRecorder()
	h.DeleteFile(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Errorf("got %d", rr.Code)
	}
	if _, err := os.Stat(filepath.Join(dir, "mine", "extra.png")); !os.IsNotExist(err) {
		t.Error("file should be removed")
	}
}

func TestFSHandlers_DeleteProtectedRefused(t *testing.T) {
	h := NewFSHandlers(newEditorTestDir(t), nil)
	req := httptest.NewRequest(http.MethodDelete, "/edit/mine/file/template.yaml", nil)
	req.SetPathValue("template", "mine")
	req.SetPathValue("filename", "template.yaml")
	rr := httptest.NewRecorder()
	h.DeleteFile(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Errorf("template.yaml delete should be forbidden, got %d", rr.Code)
	}
}
```

- [ ] **Step 2: Run test, confirm failure**

Run: `go test ./internal/editor/ -run TestFSHandlers -v`
Expected: FAIL — `NewFSHandlers` undefined.

- [ ] **Step 3: Implement `internal/editor/fs_handlers.go`**

```go
package editor

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/webfraggle/zza-generate-images/internal/renderer"
	"gopkg.in/yaml.v3"
)

// InvalidateCacheFn is called after a successful template-save so the render
// cache can be purged. nil is allowed (no-op).
type InvalidateCacheFn func(template string)

// FSHandlers serves the editor HTTP API backed by the local filesystem.
// No auth — the desktop build binds to 127.0.0.1 only.
type FSHandlers struct {
	TemplatesDir string
	Invalidate   InvalidateCacheFn
}

// NewFSHandlers constructs handlers. invalidate may be nil in tests.
func NewFSHandlers(templatesDir string, invalidate InvalidateCacheFn) *FSHandlers {
	return &FSHandlers{TemplatesDir: templatesDir, Invalidate: invalidate}
}

// Register attaches all editor routes onto mux. Pattern prefix is /edit/.
func (h *FSHandlers) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /edit/{template}", h.EditorPage)
	mux.HandleFunc("GET /edit/{template}/files", h.ListFiles)
	mux.HandleFunc("GET /edit/{template}/file/{filename}", h.GetFile)
	mux.HandleFunc("POST /edit/{template}/save", h.Save)
	mux.HandleFunc("POST /edit/{template}/upload", h.Upload)
	mux.HandleFunc("DELETE /edit/{template}/file/{filename}", h.DeleteFile)
}

// EditorPage serves the editor HTML (rendered by the parent server with the
// shared html/template — see RegisterEditor in internal/server).
// This method is a placeholder: the parent server wires it via its own
// template set. Kept here so the mux registration lives in one place.
func (h *FSHandlers) EditorPage(w http.ResponseWriter, r *http.Request) {
	// The actual template execution happens in internal/server. This stub
	// is only reached if FSHandlers is mounted standalone (tests).
	http.Error(w, "EditorPage must be wired by the server package", http.StatusNotImplemented)
}

func (h *FSHandlers) ListFiles(w http.ResponseWriter, r *http.Request) {
	name, ok := h.requireTemplate(w, r)
	if !ok {
		return
	}
	if err := InitTemplate(h.TemplatesDir, name); err != nil {
		http.Error(w, "could not init template dir", http.StatusInternalServerError)
		return
	}
	files, err := ListFiles(h.TemplatesDir, name)
	if err != nil {
		log.Printf("editor: list %q: %v", name, err)
		http.Error(w, "could not list files", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"files": files})
}

func (h *FSHandlers) GetFile(w http.ResponseWriter, r *http.Request) {
	name, ok := h.requireTemplate(w, r)
	if !ok {
		return
	}
	filename := r.PathValue("filename")
	data, err := ReadTextFile(h.TemplatesDir, name, filename)
	switch {
	case errors.Is(err, ErrForbidden), errors.Is(err, ErrInvalidName):
		http.Error(w, "forbidden", http.StatusForbidden)
	case errors.Is(err, ErrFileNotFound):
		http.Error(w, "not found", http.StatusNotFound)
	case err != nil:
		log.Printf("editor: read %q/%q: %v", name, filename, err)
		http.Error(w, "could not read file", http.StatusInternalServerError)
	default:
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write(data)
	}
}

type saveRequest struct {
	Filename string `json:"filename"`
	Content  string `json:"content"`
}

func (h *FSHandlers) Save(w http.ResponseWriter, r *http.Request) {
	name, ok := h.requireTemplate(w, r)
	if !ok {
		return
	}
	var req saveRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Parse-check YAML before writing (invalid YAML must not clobber the file).
	if len(req.Content) > 0 && hasYAMLExt(req.Filename) {
		var probe any
		if err := yaml.Unmarshal([]byte(req.Content), &probe); err != nil {
			http.Error(w, "invalid YAML: "+err.Error(), http.StatusBadRequest)
			return
		}
	}

	if err := WriteTextFile(h.TemplatesDir, name, req.Filename, []byte(req.Content)); err != nil {
		switch {
		case errors.Is(err, ErrForbidden), errors.Is(err, ErrInvalidName):
			http.Error(w, "forbidden", http.StatusForbidden)
		default:
			log.Printf("editor: write %q/%q: %v", name, req.Filename, err)
			http.Error(w, "could not save file", http.StatusInternalServerError)
		}
		return
	}
	if h.Invalidate != nil {
		h.Invalidate(name)
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *FSHandlers) Upload(w http.ResponseWriter, r *http.Request) {
	name, ok := h.requireTemplate(w, r)
	if !ok {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, MaxUploadBytes+512)
	if err := r.ParseMultipartForm(MaxUploadBytes); err != nil {
		http.Error(w, "request too large or invalid", http.StatusBadRequest)
		return
	}
	f, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "missing file field", http.StatusBadRequest)
		return
	}
	defer f.Close()

	if err := UploadFile(h.TemplatesDir, name, header.Filename, f); err != nil {
		if errors.Is(err, ErrForbidden) || errors.Is(err, ErrInvalidName) {
			http.Error(w, "file type not allowed", http.StatusForbidden)
		} else {
			log.Printf("editor: upload %q/%q: %v", name, header.Filename, err)
			http.Error(w, "upload failed", http.StatusInternalServerError)
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *FSHandlers) DeleteFile(w http.ResponseWriter, r *http.Request) {
	name, ok := h.requireTemplate(w, r)
	if !ok {
		return
	}
	filename := r.PathValue("filename")
	err := DeleteFile(h.TemplatesDir, name, filename)
	switch {
	case errors.Is(err, ErrForbidden), errors.Is(err, ErrInvalidName):
		http.Error(w, "forbidden", http.StatusForbidden)
	case errors.Is(err, ErrFileNotFound):
		http.Error(w, "not found", http.StatusNotFound)
	case err != nil:
		log.Printf("editor: delete %q/%q: %v", name, filename, err)
		http.Error(w, "could not delete file", http.StatusInternalServerError)
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}

func (h *FSHandlers) requireTemplate(w http.ResponseWriter, r *http.Request) (string, bool) {
	name := r.PathValue("template")
	if err := renderer.ValidateTemplateName(name); err != nil {
		http.Error(w, "invalid template name", http.StatusBadRequest)
		return "", false
	}
	return name, true
}

func hasYAMLExt(filename string) bool {
	if len(filename) < 5 {
		return false
	}
	suf := filename[len(filename)-5:]
	return suf == ".yaml" || (len(filename) >= 4 && filename[len(filename)-4:] == ".yml")
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/editor/ -v`
Expected: PASS all FSHandlers tests + existing `files_test.go`.

- [ ] **Step 5: Wire up the editor in the server package**

Add to `internal/server/server.go`:
```go
// RegisterEditor attaches an FSHandlers set to this server. The editor page
// itself is rendered here (to re-use the shared html/template set).
// Desktop-build calls this; server-build does not.
func (s *Server) RegisterEditor(h *editor.FSHandlers) {
	mux := http.NewServeMux()
	h.Register(mux)
	// Override the placeholder EditorPage with one that uses our html/template.
	mux.HandleFunc("GET /edit/{template}", s.handleEditorPage)
	s.editorHandler = mux
}

type editorPageData struct {
	TemplateName string
}

func (s *Server) handleEditorPage(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("template")
	if err := renderer.ValidateTemplateName(name); err != nil {
		http.Error(w, "invalid template name", http.StatusBadRequest)
		return
	}
	if err := editor.InitTemplate(s.templatesDir, name); err != nil {
		log.Printf("editor page: init %q: %v", name, err)
		http.Error(w, "could not init template", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = s.htmlTmpl.ExecuteTemplate(w, "edit-editor.html", editorPageData{TemplateName: name})
}
```

Also re-add the `editorHandler http.Handler` field on `Server` (removed in Task 8) and re-add its dispatch in `ServeHTTP` before the mux:
```go
	if strings.HasPrefix(r.URL.Path, "/edit/") && s.editorHandler != nil {
		s.editorHandler.ServeHTTP(w, r)
		return
	}
```

Add imports: `"github.com/webfraggle/zza-generate-images/internal/editor"`.

- [ ] **Step 6: Add a cache-invalidation helper**

In `internal/server/cache.go` expose:
```go
// InvalidateTemplate removes all cached entries for a given template name.
// Entry keys are prefixed with the template name followed by ":".
func (c *Cache) InvalidateTemplate(name string) {
	// The cache key in server.go is s.cache.Key(templateName, body+modStamp) —
	// the key computation prefixes the name before hashing, so entries are not
	// distinguishable by template. Simplest correct implementation: bump the
	// mod-time of template.yaml which is already part of the key, making all
	// previous keys stale. Caller passes the templatesDir; do it in server.go.
}
```

Actually: the existing keying already includes `template.yaml` mod-time, so saving `template.yaml` naturally invalidates. For other files (assets), bump mod-time manually:

Add to `internal/server/server.go` a `Server`-level helper used by the editor wiring:
```go
// InvalidateTemplateCache touches template.yaml so the next render recomputes.
func (s *Server) InvalidateTemplateCache(name string) {
	if err := renderer.ValidateTemplateName(name); err != nil {
		return
	}
	p := filepath.Join(s.templatesDir, name, "template.yaml")
	now := time.Now()
	_ = os.Chtimes(p, now, now)
}
```

- [ ] **Step 7: Compile + tests**

Run: `go build ./... && go test ./...`
Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/editor/fs_handlers.go internal/editor/fs_handlers_test.go \
        internal/server/server.go
git commit -m "feat(editor): auth-free /edit/{template}/... handlers"
```

---

## Task 10: Frontend — rewrite `edit-editor.html` to template-addressed URLs

**Files:**
- Modify: `web/templates/edit-editor.html`

- [ ] **Step 1: Replace `TOKEN` variable and all fetch URLs**

Open `web/templates/edit-editor.html` and apply:

Line 93 (replace):
```js
const TEMPLATE = '{{.TemplateName}}';
```

Search-and-replace inside the file:
- `` `/edit/${TOKEN}/files` `` → `` `/edit/${TEMPLATE}/files` ``
- `` `/edit/${TOKEN}/file/${encodeURIComponent(name)}` `` → `` `/edit/${TEMPLATE}/file/${encodeURIComponent(name)}` ``
- `` `/edit/${TOKEN}/save` `` → `` `/edit/${TEMPLATE}/save` ``
- `` `/edit/${TOKEN}/upload` `` → `` `/edit/${TEMPLATE}/upload` ``
- `` `/edit/${TOKEN}/file/default.json` `` → `` `/edit/${TEMPLATE}/file/default.json` ``

- [ ] **Step 2: Update the back-link**

Line 13 already has `<a href="/{{.TemplateName}}">← Vorschau</a>` — leave as-is.

- [ ] **Step 3: Render a smoke-test check**

Add an integration test in `internal/server/server_test.go`:
```go
func TestServer_EditorPage_DesktopOnly(t *testing.T) {
	srv := newTestServer(t)
	// Without RegisterEditor, /edit/... should fall through to the mux and
	// therefore 404 (no editor route registered).
	req := httptest.NewRequest(http.MethodGet, "/edit/sbb-096-v1", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("server build /edit should 404, got %d", rr.Code)
	}
}

func TestServer_EditorPage_WithEditor_Renders(t *testing.T) {
	srv := newTestServer(t)
	tdir := filepath.Join("..", "..", "templates")
	srv.RegisterEditor(editor.NewFSHandlers(tdir, srv.InvalidateTemplateCache))

	req := httptest.NewRequest(http.MethodGet, "/edit/sbb-096-v1", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("got %d, body: %s", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	if !strings.Contains(body, "sbb-096-v1") {
		t.Error("editor page should include template name")
	}
	if !strings.Contains(body, "TEMPLATE") && !strings.Contains(body, "'sbb-096-v1'") {
		t.Error("editor page JS should reference template constant")
	}
}
```
Add `"github.com/webfraggle/zza-generate-images/internal/editor"` to the test file's imports.

- [ ] **Step 4: Run tests**

Run: `go test ./internal/server/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add web/templates/edit-editor.html internal/server/server_test.go
git commit -m "feat(editor): frontend uses /edit/{template}/... — no tokens"
```

---

## Task 11: Gallery template — hide admin/create links

**Files:**
- Modify: `web/templates/gallery.html`
- Modify: `internal/server/server.go` (gallery view data)
- Modify: `internal/gallery/gallery.go` (if needed; preferred: add a wrapper view-struct server-side)

- [ ] **Step 1: Wrap gallery data with `EditorEnabled`**

Edit `internal/server/server.go` `handleGallery`:
```go
type galleryData struct {
	Templates     []gallery.TemplateInfo
	EditorEnabled bool
}

func (s *Server) handleGallery(w http.ResponseWriter, r *http.Request) {
	infos, err := gallery.ListTemplates(s.templatesDir)
	if err != nil {
		http.Error(w, "could not list templates", http.StatusInternalServerError)
		log.Printf("gallery: list: %v", err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.htmlTmpl.ExecuteTemplate(w, "gallery.html",
		galleryData{Templates: infos, EditorEnabled: s.editorEnabled}); err != nil {
		log.Printf("gallery: execute template: %v", err)
	}
}
```

- [ ] **Step 2: Update `gallery.html`**

Rewrite top of file:
```html
  <header>
    <span class="brand">Anzeigen-Generator</span>
    <nav class="header-nav">
      <a href="/" class="nav-link active">Galerie</a>
    </nav>
  </header>
  <main class="gallery">
    {{range .Templates}}
```

(The `{{else}}` empty branch stays; last `</main>` stays.)

- [ ] **Step 3: Run tests**

Run: `go test ./internal/server/ -v`
Expected: PASS — `TestServer_Gallery` still asserts `sbb-096-v1` is listed.

- [ ] **Step 4: Commit**

```bash
git add internal/server/server.go web/templates/gallery.html
git commit -m "feat(server): gallery no longer shows admin/create links"
```

---

## Task 12: Desktop — templates directory resolution (TDD)

**Files:**
- Create: `internal/desktop/paths.go`
- Create: `internal/desktop/paths_test.go`

- [ ] **Step 1: Write the failing test**

```go
package desktop

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveTemplatesDir_FlagOverrideWins(t *testing.T) {
	override := t.TempDir()
	got, err := ResolveTemplatesDir(override, "/unused/exe")
	if err != nil {
		t.Fatal(err)
	}
	if got != override {
		t.Errorf("got %q, want %q", got, override)
	}
}

func TestResolveTemplatesDir_BareBinaryUsesExeDir(t *testing.T) {
	tmp := t.TempDir()
	fakeExe := filepath.Join(tmp, "zza")
	if err := os.WriteFile(fakeExe, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := ResolveTemplatesDir("", fakeExe)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(tmp, "templates")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestResolveTemplatesDir_AppBundleUsesSiblingDir(t *testing.T) {
	tmp := t.TempDir()
	bundle := filepath.Join(tmp, "ZZA.app", "Contents", "MacOS", "zza")
	if err := os.MkdirAll(filepath.Dir(bundle), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bundle, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := ResolveTemplatesDir("", bundle)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(tmp, "templates") // sibling of ZZA.app
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestEnsureTemplatesDir_CreatesAndSeeds(t *testing.T) {
	tmp := t.TempDir()
	dir := filepath.Join(tmp, "templates")
	if err := EnsureTemplatesDir(dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "default", "template.yaml")); err != nil {
		t.Errorf("starter template not created: %v", err)
	}
}
```

- [ ] **Step 2: Run, confirm failure**

Run: `go test ./internal/desktop/ -v`
Expected: FAIL — package missing.

- [ ] **Step 3: Implement `internal/desktop/paths.go`**

```go
// Package desktop provides desktop-build entrypoints: templates directory
// resolution, Wails bootstrap, and browser-fallback when no webview is available.
package desktop

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/webfraggle/zza-generate-images/internal/editor"
)

// ResolveTemplatesDir returns the absolute path to the templates folder.
// Priority: override (--templates-dir flag) → sibling-of-app-bundle (macOS .app)
// → sibling-of-executable (Windows and bare macOS/Linux binaries).
func ResolveTemplatesDir(override, exePath string) (string, error) {
	if override != "" {
		abs, err := filepath.Abs(override)
		if err != nil {
			return "", fmt.Errorf("desktop: resolving override: %w", err)
		}
		return abs, nil
	}

	exeAbs, err := filepath.Abs(exePath)
	if err != nil {
		return "", fmt.Errorf("desktop: resolving exe path: %w", err)
	}
	exeDir := filepath.Dir(exeAbs)

	// macOS .app bundle: binary lives at <Bundle>.app/Contents/MacOS/zza
	if strings.Contains(exeDir, ".app/Contents/MacOS") {
		// Walk up to find the directory containing the .app bundle.
		cur := exeDir
		for cur != "/" && cur != "." {
			parent := filepath.Dir(cur)
			if strings.HasSuffix(cur, ".app") {
				return filepath.Join(parent, "templates"), nil
			}
			cur = parent
		}
	}

	return filepath.Join(exeDir, "templates"), nil
}

// EnsureTemplatesDir creates dir if it doesn't exist and seeds a minimal
// starter template (default/) when empty.
func EnsureTemplatesDir(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("desktop: creating templates dir: %w", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("desktop: reading templates dir: %w", err)
	}
	// If there's at least one sub-directory that looks like a template, do nothing.
	for _, e := range entries {
		if e.IsDir() {
			return nil
		}
	}
	// Seed with a default starter template.
	return editor.InitTemplate(dir, "default")
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/desktop/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/desktop/paths.go internal/desktop/paths_test.go
git commit -m "feat(desktop): resolve templates dir (exe sibling, .app bundle sibling, --templates-dir)"
```

---

## Task 13: Desktop — Wails bootstrap + browser fallback

**Files:**
- Create: `internal/desktop/run.go`
- Create: `wails.json` (repo root)
- Modify: `go.mod` — add `github.com/wailsapp/wails/v2`

- [ ] **Step 1: Add Wails dependency**

Run:
```bash
go get github.com/wailsapp/wails/v2@v2.9.3
go mod tidy
```

If `go get` prompts about CGO, proceed — Wails requires CGO on macOS/Windows builds but we still keep CGO disabled for the Linux server build.

- [ ] **Step 2: Install Wails CLI (dev-time dependency; build-time)**

Run (one-time on the dev machine):
```bash
go install github.com/wailsapp/wails/v2/cmd/wails@v2.9.3
wails doctor
```
Expected: wails reports a green status for darwin/arm64. Fix anything red before continuing.

- [ ] **Step 3: Create `wails.json` in the repo root**

```json
{
  "$schema": "https://wails.io/schemas/config.v2.json",
  "name": "zza",
  "outputfilename": "zza",
  "frontend:install": "true",
  "frontend:dev:serverUrl": "auto",
  "wailsjsdir": "./internal/desktop",
  "author": {
    "name": "webfraggle"
  },
  "info": {
    "productName": "Zugzielanzeiger",
    "productVersion": "0.0.0"
  }
}
```

- [ ] **Step 4: Implement `internal/desktop/run.go`**

```go
package desktop

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"os/exec"
	"runtime"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

// RunGUI opens a Wails-powered native window hosting handler. When Wails
// cannot start (no WebView2 on Windows 10, exotic Linux), it falls back to
// opening the system default browser pointed at a localhost HTTP server.
func RunGUI(title string, handler http.Handler) error {
	err := wails.Run(&options.App{
		Title:  title,
		Width:  1400,
		Height: 900,
		AssetServer: &assetserver.Options{
			Handler: handler,
		},
	})
	if err == nil {
		return nil
	}
	log.Printf("wails unavailable (%v) — falling back to default browser", err)
	return RunBrowser(handler)
}

// RunBrowser starts an HTTP server on 127.0.0.1:0 and opens the default browser.
// Blocks until the server stops.
func RunBrowser(handler http.Handler) error {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("desktop: listen: %w", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	url := fmt.Sprintf("http://127.0.0.1:%d", port)
	log.Printf("zza editor running at %s (close terminal to quit)", url)

	go openInBrowser(url)
	return http.Serve(listener, handler)
}

// RunServerOnly starts an HTTP server on the given address without opening
// anything. Used by `zza serve`.
func RunServerOnly(addr string, handler http.Handler) error {
	log.Printf("zza serving on %s", addr)
	return http.ListenAndServe(addr, handler)
}

func openInBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	if err := cmd.Start(); err != nil {
		log.Printf("desktop: open browser: %v", err)
	}
}
```

- [ ] **Step 5: Verify compile (but skip running — Wails needs display to actually open)**

Run:
```bash
CGO_ENABLED=1 go build ./internal/desktop
```
Expected: PASS on the dev machine (macOS, has CGO).

- [ ] **Step 6: Commit**

```bash
git add internal/desktop/run.go wails.json go.mod go.sum
git commit -m "feat(desktop): Wails bootstrap with browser fallback"
```

---

## Task 14: `cmd/zza/main.go` — Cobra root with default=GUI, serve, render, version

**Files:**
- Overwrite: `cmd/zza/main.go`

- [ ] **Step 1: Overwrite the placeholder**

```go
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/spf13/cobra"
	"github.com/webfraggle/zza-generate-images/internal/cli"
	"github.com/webfraggle/zza-generate-images/internal/config"
	"github.com/webfraggle/zza-generate-images/internal/desktop"
	"github.com/webfraggle/zza-generate-images/internal/editor"
	"github.com/webfraggle/zza-generate-images/internal/server"
	"github.com/webfraggle/zza-generate-images/internal/version"
	"github.com/webfraggle/zza-generate-images/web"
)

var templatesDirFlag string

func main() {
	if err := rootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func rootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "zza",
		Short: "Zugzielanzeiger desktop — editor + preview + render",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Default: open GUI.
			return runGUI(templatesDirFlag)
		},
	}
	root.PersistentFlags().StringVar(&templatesDirFlag, "templates-dir", "",
		"path to templates directory (defaults to sibling of executable)")
	root.AddCommand(cli.RenderCmd())
	root.AddCommand(serveCmd())
	root.AddCommand(versionCmd())
	return root
}

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use: "version", Short: "Print version and exit",
		Run: func(cmd *cobra.Command, _ []string) { fmt.Fprintln(cmd.OutOrStdout(), version.Version) },
	}
}

func serveCmd() *cobra.Command {
	var port string
	c := &cobra.Command{
		Use:   "serve",
		Short: "Run the editor+preview server without opening a window",
		RunE: func(cmd *cobra.Command, args []string) error {
			handler, err := buildHandler(templatesDirFlag)
			if err != nil {
				return err
			}
			return desktop.RunServerOnly("127.0.0.1:"+port, handler)
		},
	}
	c.Flags().StringVar(&port, "port", "8080", "TCP port to listen on")
	return c
}

func runGUI(override string) error {
	handler, err := buildHandler(override)
	if err != nil {
		return err
	}
	return desktop.RunGUI("Zugzielanzeiger", handler)
}

// buildHandler wires the HTTP server with the editor handlers attached.
func buildHandler(templatesOverride string) (*server.Server, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("locating executable: %w", err)
	}
	tdir, err := desktop.ResolveTemplatesDir(templatesOverride, exe)
	if err != nil {
		return nil, err
	}
	if err := desktop.EnsureTemplatesDir(tdir); err != nil {
		return nil, err
	}
	log.Printf("templates: %s", tdir)

	cfg := &config.Config{
		Port:             "0",
		TemplatesDir:     tdir,
		CacheDir:         cacheDirFor(exe),
		CacheMaxAgeHours: 24,
		CacheMaxSizeMB:   500,
		BaseURL:          "http://127.0.0.1",
	}
	srv, err := server.New(cfg, web.FS)
	if err != nil {
		return nil, err
	}
	srv.SetEditorEnabled(true)
	srv.RegisterEditor(editor.NewFSHandlers(tdir, srv.InvalidateTemplateCache))
	return srv, nil
}

// cacheDirFor returns a cache directory next to the executable.
func cacheDirFor(exe string) string {
	// Use <user cache dir>/zza if available, else exe dir /cache.
	if u, err := os.UserCacheDir(); err == nil {
		return u + string(os.PathSeparator) + "zza"
	}
	return "./cache"
}
```

- [ ] **Step 2: Ensure `internal/server.Server` implements `http.Handler`**

Confirm (it already does via `ServeHTTP`). No change needed.

- [ ] **Step 3: Build both binaries**

Run:
```bash
go build -o /tmp/zza-server ./cmd/zza-server
CGO_ENABLED=1 go build -o /tmp/zza ./cmd/zza
```
Expected: both succeed. The server build must not pull in Wails — check:
```bash
go list -deps ./cmd/zza-server | grep -i wails || echo "OK: server has no Wails dep"
```
Expected: `OK: server has no Wails dep`.

- [ ] **Step 4: Smoke-test the server binary**

Run in another terminal:
```bash
PORT=18080 TEMPLATES_DIR=./templates /tmp/zza-server serve &
sleep 1
curl -sI http://localhost:18080/health
curl -sI http://localhost:18080/sbb-096-v1.zip
curl -s -o /dev/null -w "%{http_code}\n" http://localhost:18080/edit/sbb-096-v1
pkill -f '/tmp/zza-server' || true
```
Expected: `/health` → 200, `/sbb-096-v1.zip` → 200 + `application/zip`, `/edit/sbb-096-v1` → 404.

- [ ] **Step 5: Smoke-test the desktop binary in headless mode (serve only)**

```bash
/tmp/zza serve --port 18081 &
sleep 1
curl -s -o /dev/null -w "%{http_code}\n" http://localhost:18081/edit/default
pkill -f '/tmp/zza' || true
```
Expected: `/edit/default` → 200, templates dir auto-created next to the binary.

- [ ] **Step 6: Commit**

```bash
git add cmd/zza/main.go
git commit -m "feat(desktop): cmd/zza root cmd with GUI default, serve, render, version"
```

---

## Task 15: `build.sh` — Wails cross-compile + release ZIPs

**Files:**
- Modify: `build.sh`

- [ ] **Step 1: Replace the desktop section of `build.sh`**

Replace everything between `echo "=== Desktop CLI (zza-desktop) ==="` and `echo "=== Server Docker image (zza) ==="` with:

```bash
echo "=== Desktop build (zza) ==="

RELEASE_DIR="$OUTDIR/release"
mkdir -p "$RELEASE_DIR"

build_desktop() {
    local target_os="$1" target_arch="$2" zip_name="$3"
    local wails_platform="${target_os}/${target_arch}"
    echo "Building $wails_platform..."

    if ! wails build -platform "$wails_platform" \
            -ldflags "$LDFLAGS" -clean -trimpath \
            >"$OUTDIR/wails-${target_os}-${target_arch}.log" 2>&1; then
        echo "  FAILED (see $OUTDIR/wails-${target_os}-${target_arch}.log)"
        ((failed++))
        return
    fi

    # Wails output locations vary per target.
    local build_dir="build/bin"
    local staging="$OUTDIR/stage-${target_os}-${target_arch}"
    rm -rf "$staging"
    mkdir -p "$staging"

    # Copy binary or .app bundle
    case "$target_os/$target_arch" in
        darwin/*)
            cp -R "$build_dir/zza.app" "$staging/" ;;
        windows/*)
            cp "$build_dir/zza.exe" "$staging/" ;;
    esac

    # Copy templates folder (entire curated set)
    cp -R templates "$staging/templates"

    # README
    cat > "$staging/README.txt" <<'EOF'
Zugzielanzeiger Desktop
=======================

First run:
  macOS: right-click zza.app → "Öffnen" (bypasses Gatekeeper on unsigned apps)
  Windows: on the SmartScreen warning, click "Weitere Informationen" → "Trotzdem ausführen"

The "templates" folder next to this binary holds all your template directories.
Edit templates via the built-in web editor (opens automatically when you launch zza).
EOF

    (cd "$OUTDIR" && zip -r "release/$zip_name" "$(basename $staging)/"* >/dev/null)
    rm -rf "$staging"
    echo "  → $RELEASE_DIR/$zip_name"
    ((ok++))
}

# Pure-Go cross-compile for render CLI only (fast, no CGO) — useful for CI.
# Full Wails builds require per-platform toolchain:
build_desktop darwin  arm64 "zza-macos-arm64.zip"
build_desktop darwin  amd64 "zza-macos-intel.zip"
# Windows needs mingw-w64 on the Mac host: brew install mingw-w64
build_desktop windows amd64 "zza-windows-x64.zip"

echo ""
```

- [ ] **Step 2: Update the Docker section to point at `./cmd/zza-server`**

Already done in Task 2 Step 2 via the Dockerfile; build.sh passes through `.` so nothing to change here.

- [ ] **Step 3: Smoke-test one platform**

Run:
```bash
./build.sh 2>&1 | tail -20
ls -la dist/release/
unzip -l dist/release/zza-macos-arm64.zip | head
```
Expected: at least the macOS ARM64 ZIP exists, contains `zza.app/` and `templates/`. Windows may fail if mingw-w64 is absent — document in the README fallback.

- [ ] **Step 4: Commit**

```bash
git add build.sh
git commit -m "build: Wails cross-compile + release ZIPs (templates + README)"
```

---

## Task 16: Dockerfile — slim server image

**Files:**
- Modify: `Dockerfile`

- [ ] **Step 1: Remove the `db` volume dir creation and drop `ca-certificates`+`tzdata` if not needed**

Edit `Dockerfile`:
```dockerfile
# syntax=docker/dockerfile:1
FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS builder

ARG TARGETOS=linux
ARG TARGETARCH=amd64

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG ZZA_VERSION=dev
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags="-s -w -X github.com/webfraggle/zza-generate-images/internal/version.Version=${ZZA_VERSION}" -o zza-server ./cmd/zza-server

FROM alpine:3.21

RUN apk add --no-cache tzdata \
    && adduser -D -u 1000 zza

WORKDIR /app
COPY --from=builder /app/zza-server .

RUN mkdir -p /data/cache /data/templates \
    && chown -R zza:zza /data

USER zza
EXPOSE 8080

ENTRYPOINT ["/app/zza-server", "serve"]
```

- [ ] **Step 2: Build the Docker image locally**

Run:
```bash
./build.sh
docker images | grep zza-generate-images | head
```
Expected: `ghcr.io/webfraggle/zza-generate-images:v0.X.Y` is loaded locally.

- [ ] **Step 3: Smoke-test the Docker image**

```bash
docker run --rm -d --name zza-test -p 18082:8080 \
    -v "$(pwd)/templates:/data/templates:ro" \
    -e TEMPLATES_DIR=/data/templates \
    -e CACHE_DIR=/data/cache \
    "ghcr.io/webfraggle/zza-generate-images:$(cat VERSION)"
sleep 2
curl -sf http://localhost:18082/health
curl -sI http://localhost:18082/sbb-096-v1.zip | head -3
curl -sI http://localhost:18082/edit/sbb-096-v1 | head -1
docker stop zza-test
```
Expected: health → `ok`, `.zip` → 200 + `application/zip`, `/edit/...` → `HTTP/1.1 404`.

- [ ] **Step 4: Commit**

```bash
git add Dockerfile
git commit -m "build: Dockerfile drops sqlite volume and switches to cmd/zza-server"
```

---

## Task 17: Manual-test plan + security/code review

**Files:**
- Create: `docs/superpowers/plans/2026-04-22-dual-build-architecture-manual-tests.md`

- [ ] **Step 1: Write the manual-test checklist**

Create the file with this content:

```markdown
# Dual-Build Manual Test Plan

## Server build
- [ ] `GET /health` → 200 ok
- [ ] `GET /` lists templates, no "Admin" or "+ Neues Template" nav items
- [ ] `GET /sbb-096-v1` renders preview, shows "Als ZIP" button, shows "PNG herunterladen", **no** "Bearbeiten" button, **no** edit modal
- [ ] `GET /sbb-096-v1.zip` downloads a valid .zip containing template.yaml + default.json + assets
- [ ] `POST /sbb-096-v1/render` returns PNG
- [ ] `GET /edit/sbb-096-v1` → 404
- [ ] `GET /admin` → 404
- [ ] `GET /sbb-096-v1/edit` → 404
- [ ] HTTPS redirect still works for `/` when X-Forwarded-Proto=http, `/render` stays http

## Desktop build — GUI
- [ ] Double-click `zza.app` / `zza.exe` — native window opens
- [ ] Gallery page shown, templates listed
- [ ] Click a template — preview works, "Bearbeiten" button visible
- [ ] Click "Bearbeiten" — editor loads YAML + default.json
- [ ] Edit YAML, click "Speichern" — no error, preview refreshes
- [ ] Invalid YAML on save — shows 400-error, file on disk unchanged
- [ ] Upload a .png asset — file appears in templates/<name>/
- [ ] Delete an uploaded asset — file removed
- [ ] Try to delete template.yaml — "forbidden" error
- [ ] "Als ZIP" downloads a ZIP

## Desktop build — Browser fallback
- [ ] Rename the system webview (or temporarily break Wails init) → verify fallback message appears and default browser opens the editor URL
- [ ] Same functional tests as in GUI mode

## Desktop build — CLI
- [ ] `zza version` → prints version
- [ ] `zza render <tmpl> <json> out.png` → PNG generated
- [ ] `zza serve --port 9000` → server responds on :9000, no window opens

## Templates folder
- [ ] First run with no templates folder next to binary → folder created, `default/` seeded
- [ ] Existing templates folder preserved
- [ ] `--templates-dir /custom/path` override honoured

## Releases
- [ ] Unzip `zza-macos-arm64.zip` to a fresh folder — Gatekeeper dialog, right-click Open succeeds
- [ ] ZIP contains full `templates/` folder (all curated templates)
- [ ] README.txt is readable

## Docker
- [ ] `docker-compose up` on the VM — server starts, no DB errors, no SMTP errors
- [ ] Old `zza.db` file left on disk — ignored
```

- [ ] **Step 2: Run security review (per CLAUDE.md phase workflow)**

Dispatch the `security-reviewer` agent on the diff since `develop`. Fix findings before continuing.

- [ ] **Step 3: Run code review**

Dispatch the `code-reviewer` agent on the diff. Address findings.

- [ ] **Step 4: Run full test suite one more time**

```bash
go test ./... -race
```
Expected: PASS.

- [ ] **Step 5: Commit the manual test plan**

```bash
git add docs/superpowers/plans/2026-04-22-dual-build-architecture-manual-tests.md
git commit -m "docs: manual test checklist for dual-build release"
```

---

## Task 18: Merge to develop

- [ ] **Step 1: Execute the manual tests**

Work through every checkbox in the manual-test plan. Do not proceed on any failing item — fix and re-test.

- [ ] **Step 2: Merge**

```bash
git checkout develop
git merge --no-ff feature/dual-build
git push origin develop
```

- [ ] **Step 3: Tag a release**

```bash
./build.sh       # bumps patch, builds everything
DOCKER_PUSH=1 ./build.sh   # publishes Docker image to ghcr.io
VERSION=$(cat VERSION)
git tag "$VERSION"
git push origin "$VERSION"
gh release create "$VERSION" \
    dist/release/zza-macos-arm64.zip \
    dist/release/zza-macos-intel.zip \
    dist/release/zza-windows-x64.zip \
    --title "$VERSION" \
    --notes "Dual-build release: slim server + desktop editor. See docs/superpowers/specs/2026-04-22-dual-build-architecture-design.md"
```

- [ ] **Step 4: Deploy the server side**

On the VM:
```bash
docker-compose pull
docker-compose up -d
```

Delete the stale `zza.db` on the VM host (no longer mounted):
```bash
rm -f /path/to/zza/data/db/zza.db
```

---

## Self-Review Notes (before execution)

**Spec coverage check:**
- Server routes (`/`, `/{template}`, `/{template}/render`, `/{template}.zip`, `/health`) — Task 3, Task 4, Task 11.
- Server removes DB/SMTP/Admin — Tasks 5, 6, 7, 8.
- Desktop adds `/edit/{template}` and POST-save — Tasks 9, 10.
- No auth on desktop editor — Tasks 9, 10.
- HTTPS-redirect stays in server, not desktop — `ServeHTTP` keeps the redirect; desktop uses 127.0.0.1 so `X-Forwarded-Proto=http` is never set.
- Shared preview template with `EditorEnabled` — Task 4.
- Templates-folder resolution (4 cases) — Task 12.
- Wails v2 + browser fallback — Task 13.
- ZIP bundles full `templates/` — Task 15.
- Build-matrix (Windows x64, macOS Intel/ARM) — Task 15.
- No code-signing — README explains click-through (Task 15 Step 1 README).
- Ersatzlos gelöscht items (admin, db, email HTML, admin handlers, env vars) — Tasks 5, 7, 8.

**Placeholder scan:** no "TBD" / "TODO" / "add appropriate" patterns. Every code-bearing step has full code.

**Type consistency:**
- `FSHandlers` named consistently in Tasks 9, 10, 14.
- `SetEditorEnabled` + `InvalidateTemplateCache` + `RegisterEditor` all consistent between Tasks 4, 9, 14.
- `ResolveTemplatesDir`, `EnsureTemplatesDir`, `RunGUI`, `RunBrowser`, `RunServerOnly` consistent between Tasks 12, 13, 14.
- `detailData.EditorEnabled` and `galleryData.EditorEnabled` both bool, both sourced from `s.editorEnabled`.

**Known caveats:**
- Wails dev-time setup (`wails doctor`, mingw-w64 for Windows) is an environment prerequisite, not a code task. Covered in Task 13 Step 2 and Task 15.
- Wails v2 major version pinned to 2.9.3 — if upstream breaks, re-check `options.App` / `assetserver.Options` field names.
- The existing `renderer.ValidateTemplateName` handles path-traversal guards; ZIP handler relies on this (Task 3) plus a directory existence stat.
