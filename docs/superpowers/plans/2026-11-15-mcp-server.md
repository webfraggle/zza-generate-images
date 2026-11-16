# MCP-Server für den Desktop-Build — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `zza mcp` — ein lokaler MCP-Server (stdio) im Desktop-Build, mit dem Claude Desktop / Claude Code Templates für das 1,05"-Display lesen, anlegen, bearbeiten, mit Bildern/SVG/Fonts bestücken und als Bild-Vorschau prüfen; parallel laufende GUI zeigt die Änderungen live und erkennt Konflikte.

**Architecture:** Neues Desktop-Paket `internal/mcp` auf dem offiziellen Go-SDK; jede Tool-Gruppe ist eine eigene Datei, die sich per `init()` registriert. Dateizugriff über `internal/editor` (Pfadschutz, atomar, Versionen), Rendern über `internal/renderer`. Live-Updates: `internal/watch` (fsnotify) → Long-Poll `GET /edit/{t}/events`; Konflikte über ETag/If-Match → 409. Leitfaden und YAML-Spec sind per `docs`-Paket eingebettet.

**Tech Stack:** Go 1.26, `github.com/modelcontextprotocol/go-sdk` v1.8.0, `github.com/srwiley/oksvg` + `rasterx` (T4), `github.com/fsnotify/fsnotify` v1.10.1 (T7), `golang.org/x/image` (vorhanden), `gopkg.in/yaml.v3` (vorhanden), Vanilla-JS im Editor + `node --test`.

**Spec:** `docs/superpowers/specs/2026-11-15-mcp-server-design.md`

## Global Constraints

- Server-Build bleibt unverändert und schlank: nach jedem Task `CGO_ENABLED=0 go build ./cmd/zza-server` grün und `go list -deps ./cmd/zza-server | grep -E 'fsnotify|modelcontextprotocol|oksvg|rasterx|internal/mcp|internal/watch'` ohne Ausgabe.
- `internal/mcp` und `internal/watch` sind DESKTOP-ONLY; nur `cmd/zza` importiert sie.
- stdout gehört im `mcp`-Subcommand ausschließlich dem Protokoll; Logs nur über `log` (stderr).
- SDK-Version fest: `github.com/modelcontextprotocol/go-sdk v1.8.0`.
- Nur 1,05": Canvas **240×240**, Template-Namen enden auf **`-105-v1`** (nur von MCP erzwungen, nicht von der GUI).
- Limits exakt wie in der Spec-Tabelle „Limits“ (Konstanten in `internal/mcp/limits.go`, Task 1).
- Tool-Parameter `snake_case`; Tool-Fehler englisch, ohne Dateiinhalte; Dokumente Deutsch; Code, Bezeichner, Commits englisch.
- Kein Tool löscht Dateien oder Templates.
- Jeder Task: `go test ./...` grün, `go vet ./...` grün, ein Commit pro Task-Schritt „Commit“ mit Zeile `Co-Authored-By: Claude Fable 5.2 <noreply@anthropic.com>`.
- Gemeinsam berührte Dateien je Task (für parallele Branches nach Task 2):

| Task | gemeinsame Dateien | Konfliktpotenzial |
|---|---|---|
| 3 | `docs/ai-design-guide.md`, `internal/mcp/instructions.go`, `internal/editor/starter/template.yaml` | keins (nur T3 berührt sie nach T1) |
| 4 | `go.mod`, `go.sum` | mit T7: beim Merge beide `require`-Zeilen behalten, dann `go mod tidy` |
| 5 | — | keins |
| 6 | — | keins |
| 7 | `go.mod`, `go.sum`, `cmd/zza/main.go`, `internal/editor/fs_handlers.go`, `internal/server/server.go`, `internal/renderer/renderer.go` | `go.mod`/`go.sum` mit T4 (s. o.) |
| 8 | `web/templates/edit-editor.html`, `web/static/app.css` | keins |

## Review Focus

- **Nichts darf auf stdout landen außer JSON-RPC** — ein einziges `fmt.Println` oder ein Cobra-Usage-Text bricht die Verbindung still ab. Test in Task 1 (`TestMCPCmd_StdoutOnlyProtocol`).
- **Die KI schreibt ein kaputtes `template.yaml`** — die Datei auf der Platte muss danach unverändert sein und der Fehlertext muss die Ursache nennen. Test in Task 2 (`TestWriteFile_InvalidTemplateKeepsOldFile`).
- **Eigenes Speichern im Editor löst ein Watcher-Ereignis aus** — darf weder eine Konfliktmeldung noch ein Neuladen mit Cursor-Verlust bewirken; und `InvalidateTemplateCache` (Chtimes) darf keine Ereignis-Schleife erzeugen. Tests in Task 7 (`TestWatcher_IgnoresChmodOnly`) und Task 8 (`decide: own save echo is ignored`).
- **Pfade von außen** (`import_image`, `add_font`) — Symlink, Verzeichnis, FIFO, `../`, Datei mit Bild-Endung, aber Textinhalt: alles abgelehnt, Inhalt nie im Fehler. Tests in Task 2 und Task 6.
- **Parallele Tool-Aufrufe** (Clients dürfen Requests überlappen) — zwei gleichzeitige `write_file` auf dieselbe Datei mit gleicher `expected_version`: genau einer gewinnt, der andere bekommt `conflict`. Test in Task 2 (`TestWriteFile_ConcurrentSameVersion`).

---

### Task 1: MCP-Grundgerüst, lesende Tools, `zza mcp`

**Abhängigkeiten:** keine.

**Files:**
- Create: `docs/embed.go`, `docs/ai-design-guide.md` (erste Fassung), `internal/mcp/server.go`, `internal/mcp/limits.go`, `internal/mcp/instructions.go`, `internal/mcp/tools_read.go`, `internal/mcp/helpers_test.go`, `internal/mcp/server_test.go`, `internal/mcp/tools_read_test.go`, `cmd/zza/mcp_test.go`
- Modify: `go.mod`, `go.sum`, `cmd/zza/main.go`, `internal/editor/files.go`, `internal/editor/files_test.go`

**Interfaces:**
- Produces:
  - `docs.TemplateSpec string`, `docs.DesignGuide string` (package `docs`, import `github.com/webfraggle/zza-generate-images/docs`)
  - `editor.FileVersion(data []byte) string` — erste 16 Hex-Zeichen von SHA-256
  - `mcp.Config{TemplatesDir, FontsDir string}`; `mcp.NewServer(cfg Config) (*sdk.Server, error)`; `mcp.Run(ctx context.Context, cfg Config) error` (Paket `internal/mcp`, Paketname `mcp`; SDK importiert als `sdk "github.com/modelcontextprotocol/go-sdk/mcp"`)
  - `type env struct{ templatesDir, fontsDir string; writeMu sync.Mutex }`
  - `registerToolGroup(name string, register func(*sdk.Server, *env))`
  - `(*env).templateDir(name string) (string, error)` — validiert Namen, verlangt `template.yaml`, Fehler `template "<n>" not found`
  - `const serverInstructions string` (instructions.go)
  - Test-Helfer (helpers_test.go): `copyTemplates(t, names ...string) string`, `newSession(t, templatesDir string) *sdk.ClientSession`, `callTool(t, cs, name string, args map[string]any) *sdk.CallToolResult`, `resultText(res) string`, `resultImage(t, res) image.Image`, `requireOK(t, res)`, `requireToolError(t, res, substr string)`

- [ ] **Step 1: SDK-Abhängigkeit hinzufügen**

Run: `go get github.com/modelcontextprotocol/go-sdk@v1.8.0 && go get github.com/modelcontextprotocol/go-sdk/mcp@v1.8.0`
Expected: `go.mod` enthält `github.com/modelcontextprotocol/go-sdk v1.8.0`; `golang.org/x/sys` steigt auf ≥ v0.41.0. `go build ./...` grün.

- [ ] **Step 2: Failing test `FileVersion`** in `internal/editor/files_test.go`

```go
func TestFileVersion(t *testing.T) {
	v := FileVersion([]byte("hello"))
	if v != "2cf24dba5fb0a30e" { // sha256("hello")[:8] hex
		t.Errorf("got %q", v)
	}
	if FileVersion(nil) != "e3b0c44298fc1c14" {
		t.Errorf("empty: got %q", FileVersion(nil))
	}
}
```
Run: `go test ./internal/editor -run TestFileVersion` → FAIL (undefined). Implement `FileVersion` in `internal/editor/files.go`. Run → PASS.

- [ ] **Step 3: `docs`-Paket + erste Leitfaden-Fassung**

`docs/embed.go`:
```go
// Package docs embeds documentation that the desktop MCP server hands to AI clients.
package docs

import _ "embed"

//go:embed yaml-template-spec.md
var TemplateSpec string

//go:embed ai-design-guide.md
var DesignGuide string
```
`docs/ai-design-guide.md` erste Fassung: Überschrift `# KI-Design-Leitfaden für Zugzielanzeiger-Templates`, darunter die zwölf Abschnittsüberschriften der Spec (§ KI-Design-Leitfaden 1–12) mit je den Pflichtaussagen als Stichpunkte (wörtlich aus der Spec übernommen). Task 3 baut daraus den Fließtext.

- [ ] **Step 4: Failing tests Server-Grundgerüst** — `internal/mcp/helpers_test.go` (Helfer, s. Interfaces) und `internal/mcp/server_test.go`:

```go
func TestNewServer_ListsReadTools(t *testing.T) {
	cs := newSession(t, copyTemplates(t, "sbb-105-v1"))
	res, err := cs.ListTools(context.Background(), nil)
	// err == nil; names contain exactly (at least) get_template_spec, get_design_guide,
	// list_templates, read_template, read_file; every tool has a non-empty Description
}

func TestNewServer_Instructions(t *testing.T) {
	cs := newSession(t, copyTemplates(t))
	got := cs.InitializeResult().Instructions
	// got == serverInstructions; strings.Contains(got, "240x240") and "-105-v1" and "get_design_guide"
}

func TestNewServer_DefaultFontsDir(t *testing.T) {
	// NewServer(Config{TemplatesDir: "/x/templates"}) → env.fontsDir == "/x/fonts"
	// tested via exported helper-free path: newEnv(cfg) returns *env; assert fontsDir
}
```
`copyTemplates` kopiert die genannten Ordner aus `../../templates` nach `<t.TempDir()>/templates` und gibt diesen absoluten Pfad zurück (ohne Namen: leerer Ordner). `newSession` baut `NewServer(Config{TemplatesDir: dir})`, verbindet über `sdk.NewInMemoryTransports()` (Server zuerst `Connect`, dann Client) und registriert `t.Cleanup` für beide Sessions.

Run: `go test ./internal/mcp/` → FAIL (package does not exist).

- [ ] **Step 5: Implement `internal/mcp/server.go`, `limits.go`, `instructions.go`**

- `newEnv(cfg Config) (*env, error)`: `TemplatesDir` per `filepath.Abs`; `FontsDir` leer → `filepath.Join(filepath.Dir(templatesDir), "fonts")`.
- `NewServer`: `sdk.NewServer(&sdk.Implementation{Name: "zza", Title: "Zugzielanzeiger", Version: version.Version}, &sdk.ServerOptions{Instructions: serverInstructions, Capabilities: &sdk.ServerCapabilities{}})`; danach alle `toolGroups` nach `name` sortiert registrieren.
- `Run`: `log.Printf("zza mcp: templates=%s fonts=%s", …)`; `s.Run(ctx, &sdk.StdioTransport{})`.
- `limits.go`: alle Limits der Spec als Konstanten: `maxTextFileBytes = 256 << 10`, `maxFilesPerTemplate = 64`, `maxTemplateBytes = 30 << 20`, `maxUploadBytes = 5 << 20`, `maxUploadBase64Chars = 7 << 20`, `maxUploadSide = 4096`, `maxImportBytes = 20 << 20`, `maxImportSide = 8192`, `maxStoredImportSide = 2048`, `maxToolInputPixels = 25_000_000`, `maxToolInputSide = 8192`, `maxToolOutputSide = 2048`, `maxSVGBytes = 256 << 10`, `maxSVGSide = 1024`, `maxSVGPaths = 2000`, `maxPreviewDataBytes = 64 << 10`, `maxSheetFrames = 12`, `sheetColumns = 4`, `maxSheetSide = 2048`, `maxResultImageSide = 768`, `maxSamplePoints = 64`, `maxSampleRadius = 5`, `maxFontBytes = 50 << 20`, `maxFontResults = 200`, `canvasSize = 240`, `templateSuffix = "-105-v1"`.
- `instructions.go`: `const serverInstructions` = Wortlaut aus der Spec (§ KI-Design-Leitfaden, „Server-instructions“).

- [ ] **Step 6: Failing tests lesende Tools** in `internal/mcp/tools_read_test.go`

```go
func TestGetTemplateSpec(t *testing.T)   // text == docs.TemplateSpec
func TestGetDesignGuide(t *testing.T)    // text == docs.DesignGuide
func TestListTemplates(t *testing.T) {
	dir := copyTemplates(t, "sbb-105-v1", "faltblatt-105-v1")
	// add broken: dir/broken-105-v1/template.yaml = "layers: [::"
	// add ignored: dir/Not_Valid/template.yaml, dir/no-yaml/ (empty)
	// JSON: 3 entries sorted: broken-105-v1 (error non-empty), faltblatt-105-v1 (animated true,
	// width 240, height 240, has_default true), sbb-105-v1 (animated false)
}
func TestReadTemplate(t *testing.T) {
	// read_template{template:"sbb-105-v1"}: template_yaml == file bytes, default_json == file bytes,
	// versions["template.yaml"] == editor.FileVersion(yaml), files contains {name:"bg.png", editable:false}
}
func TestReadTemplate_NotFound(t *testing.T)   // "nope-105-v1" → tool error containing `template "nope-105-v1" not found`
func TestReadTemplate_InvalidName(t *testing.T) // "../etc" → tool error containing "invalid characters"
func TestReadFile(t *testing.T)                 // read_file sbb default.json → content + version; "bg.png" → error "only .yaml and .json files can be read as text"
```
Run: `go test ./internal/mcp/ -run 'TestGet|TestList|TestRead'` → FAIL.

- [ ] **Step 7: Implement `internal/mcp/tools_read.go`**

`func init() { registerToolGroup("read", registerReadTools) }`. Typen:
```go
type templateIn struct{ Template string `json:"template" jsonschema:"template name, e.g. sbb-105-v1"` }
type fileIn struct {
	Template string `json:"template" jsonschema:"template name"`
	Name     string `json:"name" jsonschema:"file name inside the template, e.g. default.json"`
}
type templateSummary struct {
	Name string `json:"name"`; Title string `json:"title"`; Description string `json:"description"`
	Width int `json:"width"`; Height int `json:"height"`; Animated bool `json:"animated"`
	HasDefault bool `json:"has_default"`; Error string `json:"error,omitempty"`
}
```
Listing liest `os.ReadDir(templatesDir)` selbst (nicht `gallery`, weil kaputte Templates mit `error` erscheinen sollen). Datei-Liste über `editor.ListFiles`, Text über `editor.ReadTextFile`.

Tool-Beschreibungen (wörtlich):
- `get_template_spec`: `Returns the YAML template specification (layers, expressions, filters, conditions, animation). Read it fully before writing template.yaml.`
- `get_design_guide`: `Returns the design guide for display templates: 240x240 canvas, front/back halves with the platform number on the outside, flat colour look, font sizes, placeholders, workflow and test data. Read it before creating or changing a template.`
- `list_templates`: `Lists all templates with title, size, animation flag and load errors. Use it to find similar templates to learn from.`
- `read_template`: `Returns template.yaml, default.json (test data), the file list and the version of each text file. Pass a version as expected_version to write_file to avoid overwriting changes made in the editor.`
- `read_file`: `Returns one .yaml or .json file of a template with its version (e.g. an extra test data set).`

Run → PASS.

- [ ] **Step 8: Failing test Subcommand** `cmd/zza/mcp_test.go`

```go
func TestMCPCmd_StdoutOnlyProtocol(t *testing.T) {
	// Build the binary: exec "go build -o <tmp>/zza ." in this package dir (skip with t.Skip in -short).
	// Start "<tmp>/zza mcp --templates-dir <copy of ../../templates/sbb-105-v1 in tmp>" with pipes.
	// Write one line: {"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"t","version":"1"}}}
	// Read first stdout line within 20 s: must json.Unmarshal into map with "id"==1 and "result".serverInfo.name=="zza".
	// Close stdin; process exits within 5 s; stderr contains "zza mcp: templates="; stdout has no further non-JSON line.
}
```
Run: `go test ./cmd/zza -run TestMCPCmd` → FAIL (unknown command "mcp").

- [ ] **Step 9: Implement `mcpCmd()` in `cmd/zza/main.go`**

`Use: "mcp"`, `Short: "Run an MCP server on stdin/stdout for AI clients (Claude Desktop, Claude Code)"`, Flag `--templates-dir` wie `serve`. `RunE`: `os.Executable` → `desktop.ResolveTemplatesDir` → `desktop.EnsureTemplatesDir` → `ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)` → `mcp.Run(ctx, mcp.Config{TemplatesDir: tdir})`. `cmd.SilenceUsage = true` und `cmd.SetOut(os.Stderr)`, damit Cobra nie nach stdout schreibt. `root.AddCommand(mcpCmd())`.

Run: `go test ./cmd/zza -run TestMCPCmd` → PASS. Dann `go test ./... && go vet ./...` und die Server-Build-Prüfung aus Global Constraints.

- [ ] **Step 10: Commit**

```bash
git add go.mod go.sum docs/embed.go docs/ai-design-guide.md internal/mcp cmd/zza internal/editor/files.go internal/editor/files_test.go
git commit -m "feat(mcp): add zza mcp subcommand with read-only template tools"
```

---

### Task 2: Schreibende Tools und Vorschau

**Abhängigkeiten:** Task 1.

**Files:**
- Create: `internal/mcp/tools_write.go`, `internal/mcp/tools_preview.go`, `internal/mcp/images.go`, `internal/mcp/validate.go`, `internal/mcp/yamledit.go`, Tests `internal/mcp/tools_write_test.go`, `internal/mcp/tools_preview_test.go`, `internal/mcp/validate_test.go`, `internal/mcp/images_test.go`
- Modify: `internal/renderer/renderer.go` (ParseTemplate), `internal/renderer/renderer_test.go`, `internal/editor/files.go`, `internal/editor/files_test.go`

**Interfaces:**
- Consumes: Task 1 (`env`, `registerToolGroup`, Helfer, `editor.FileVersion`, Limits).
- Produces (für Tasks 4–7):
  - `renderer.ParseTemplate(src []byte, dir string) (*renderer.Template, error)`
  - `editor.ErrVersionConflict`; `editor.WriteTextFileIfMatch(templatesDir, templateName, filename string, data []byte, expected string) (newVersion string, err error)` — `expected == ""` oder `"*"`: unbedingt; sonst Vergleich mit aktueller Version (fehlende Datei = Konflikt); prozessweiter `sync.Mutex`; Fehler bei Konflikt `fmt.Errorf("%w: current version %s", ErrVersionConflict, cur)`
  - `editor.CurrentVersion(templatesDir, templateName, filename string) (string, error)` — `""` + `nil` wenn Datei fehlt
  - images.go: `loadImageFile(path string, maxBytes int64, maxSide, maxPixels int) (image.Image, string /*format*/, error)`; `toNRGBA(img image.Image) *image.NRGBA`; `writePNGFile(e *env, template, name string, img image.Image) error` (atomar, Endung `.png`, prüft Dateianzahl/Ordnergröße, kein Überschreiben von Nicht-regulären Dateien); `writeAssetBytes(e *env, template, name string, data []byte) (replaced bool, err error)`; `scaleNearest(img image.Image, factor int) *image.NRGBA`; `fitWithin(img image.Image, maxSide int) image.Image` (CatmullRom, nur verkleinern); `imageContent(img image.Image) (*sdk.ImageContent, error)` (PNG-kodiert); `checkTemplateCapacity(dir string, newName string, newSize int64) error`
  - validate.go: `validateTemplateYAML(e *env, template string, content []byte) (warnings []string, err error)`; `validateJSONObject(content []byte) error`; `loadTestData(dir string) map[string]any` (default.json oder `{}`)
  - preview: `renderStill(e *env, template string, data map[string]any) (*image.NRGBA, *renderer.Template, error)` (frischer `renderer.New`, Quantize wie Server)

- [ ] **Step 1: Failing test `ParseTemplate`** in `internal/renderer/renderer_test.go`

```go
func TestParseTemplate(t *testing.T) {
	tmpl, err := ParseTemplate([]byte("meta:\n  canvas: {width: 240, height: 240}\nlayers: []\n"), "/abs/dir")
	// err nil; tmpl.Dir == "/abs/dir"; tmpl.Meta.Canvas.Width == 240
	_, err = ParseTemplate([]byte("meta:\n  canvas: {width: 240, height: 240, colors: 1}\n"), "/abs/dir")
	// err contains "renderer: ParseTemplate: canvas.colors must be between 2 and 256"
	_, err = ParseTemplate([]byte("layers: [::"), "/abs/dir")
	// err contains "decoding YAML"
}
```
Run → FAIL. Implement: unexportiertes `parseTemplate(r io.Reader, dir string) (*Template, error)` mit allen bisherigen Prüfungen aus `LoadTemplate` (Fehler ohne Präfix); `ParseTemplate` wickelt mit `"renderer: ParseTemplate: %w"`, `LoadTemplate` mit `"renderer: LoadTemplate: %w"` (bisherige Meldungen bleiben wortgleich). Run `go test ./internal/renderer/` → PASS.

- [ ] **Step 2: Failing tests `WriteTextFileIfMatch` / `CurrentVersion`** in `internal/editor/files_test.go`

```go
func TestWriteTextFileIfMatch(t *testing.T) {
	dir, name := makeTestTemplate(t)
	v0, _ := CurrentVersion(dir, name, "template.yaml")
	v1, err := WriteTextFileIfMatch(dir, name, "template.yaml", []byte("a: 1\n"), v0) // ok, v1 == FileVersion("a: 1\n")
	_, err = WriteTextFileIfMatch(dir, name, "template.yaml", []byte("a: 2\n"), v0)   // errors.Is(err, ErrVersionConflict); file still "a: 1\n"; err.Error() contains v1
	_, err = WriteTextFileIfMatch(dir, name, "template.yaml", []byte("a: 3\n"), "")   // ok (unconditional)
	_, err = WriteTextFileIfMatch(dir, name, "template.yaml", []byte("a: 4\n"), "*")  // ok
	_, err = WriteTextFileIfMatch(dir, name, "new.json", []byte("{}"), "abc")         // ErrVersionConflict (missing file)
	v, err := CurrentVersion(dir, name, "missing.json")                              // v == "", err == nil
}
```
Run → FAIL. Implement in `internal/editor/files.go` (nutzt `WriteTextFile` intern, Mutex `writeMu` auf Paketebene). Run → PASS.

- [ ] **Step 3: Failing tests Validierung** in `internal/mcp/validate_test.go`

Testinhalt basiert auf einer Kopie von `sbb-105-v1`:
```go
func TestValidateTemplateYAML_OK(t *testing.T)          // original sbb template.yaml → warnings empty, err nil
func TestValidateTemplateYAML_WrongCanvas(t *testing.T)  // canvas 296x128 → err == `canvas must be 240x240 (only 1.05" displays are supported), got 296x128`
func TestValidateTemplateYAML_RenderError(t *testing.T)  // image layer file: missing.png → err contains "missing.png"
func TestValidateTemplateYAML_Warnings(t *testing.T) {
	// YAML: rect top half only (no copy), text layer size 8 with font from sbb, text layer value "15:54" (no {{)
	// warnings (exact, in this order):
	//  "bottom half (y 120-239) is empty — the back side of the display shows nothing; rebuild it with copy layers (see get_design_guide)"
	//  "text layer layers[1] uses size 8 px — below 10 px is hard to read on the 1.05\" display"
	//  "layer layers[2] draws static text \"15:54\" that looks like train data — use {{zug1.…}} placeholders"
}
func TestValidateJSONObject(t *testing.T) // `{"a":1}` ok; `[1]` → "invalid JSON: must be an object"; `{` → error prefix "invalid JSON: "
```
Layer-Pfade: `layers[i]`, verschachtelt `layers[i].layers[j]`. Uhrzeit-Muster `^\d{1,2}:\d{2}$` auf `Value.String()` wenn kein `{{` enthalten. „Untere Hälfte leer“ = alle Pixel mit y ≥ 120 haben Alpha 0.

Run → FAIL. Implement `validate.go`: `ParseTemplate(content, dir)` → Canvas-Prüfung → `renderer.New(e.templatesDir).Render(tmpl, loadTestData(dir))` → Warnungen. Fehlertexte: Parse/Render-Fehler unverändert durchreichen. Run → PASS.

- [ ] **Step 4: Failing tests Bild-Helfer** in `internal/mcp/images_test.go`

```go
func TestScaleNearest(t *testing.T)   // 2x2 checker scaled ×3 → 6x6, pixel (2,2) == src(0,0), (3,3) == src(1,1)
func TestFitWithin(t *testing.T)      // 1000x500 maxSide 768 → 768x384; 100x50 → unchanged pointer-equal bounds
func TestLoadImageFile_Limits(t *testing.T) {
	// PNG 10x10 ok, format "png"; file > maxBytes → "image file too large"; PNG 20x10 with maxSide 16 → "image too large: 20x10 (max 16 px per side)"
	// text file named x.png → "not a PNG or JPEG image" (and error text must not contain the file content)
}
func TestCheckTemplateCapacity(t *testing.T) // 64 files present + new name → "too many files in template (max 64)"; replacing existing name ok; total > 30 MiB → "template would exceed 30 MiB"
```
Run → FAIL. Implement `images.go`. `loadImageFile` prüft Größe per `Stat` vor dem Lesen, dann `image.DecodeConfig` (Maße/Pixel) vor `image.Decode`. Run → PASS.

- [ ] **Step 5: Failing tests `create_template` / `write_file`** in `internal/mcp/tools_write_test.go`

```go
func TestCreateTemplate(t *testing.T) {
	// create_template{name:"test-105-v1", title:"Test", description:"D"} → ok;
	// files template.yaml + default.json exist; ParseTemplate ok; meta.name == "Test", meta.description == "D";
	// result has one ImageContent (480x480); text JSON has versions["template.yaml"]
}
func TestCreateTemplate_Rules(t *testing.T) {
	// "test" → error `template names must end in "-105-v1" (only 1.05" displays are supported)`
	// "Bad-105-v1" → "invalid characters"; existing "sbb-105-v1" → `template "sbb-105-v1" already exists`
}
func TestWriteFile_TemplateYAML(t *testing.T)            // valid change (rect color) → version == FileVersion(content); file content equal
func TestWriteFile_InvalidTemplateKeepsOldFile(t *testing.T) {
	// content "layers: [::" → error starts "template.yaml rejected: "; file bytes unchanged
	// canvas 128x64 → error contains "canvas must be 240x240"; file bytes unchanged
}
func TestWriteFile_JSON(t *testing.T)                    // "test-long.json" with `{"zug1":{}}` ok; "[1]" → "invalid JSON: must be an object"
func TestWriteFile_DefaultJSONRenderWarning(t *testing.T) // template.yaml broken on disk (written directly) + valid default.json → ok, warnings[0] starts "probe render failed: "
func TestWriteFile_ExpectedVersion(t *testing.T) {
	// read_template → v; write with expected_version v → ok; second write with same v →
	// error starts "conflict: template.yaml was changed since you read it (current version "
}
func TestWriteFile_ConcurrentSameVersion(t *testing.T)   // 2 goroutines write different content with same expected_version → exactly one ok, one conflict
func TestWriteFile_Rejects(t *testing.T) {
	// name "bg.png" → "only .yaml and .json files can be written"; name "../x.yaml" → "invalid file name";
	// content of 256 KiB + 1 byte → "file too large (max 256 KiB)"
}
```
Run → FAIL.

- [ ] **Step 6: Implement `create_template` + `write_file` in `internal/mcp/tools_write.go`** (+ `yamledit.go`)

`func init() { registerToolGroup("write", registerWriteTools) }`. Alle schreibenden Handler halten `e.writeMu`.
- `yamledit.go`: `setMetaScalars(src []byte, kv map[string]string) ([]byte, error)` — dekodiert in `yaml.Node`, setzt/ergänzt Skalare unter `meta`, kodiert mit `yaml.NewEncoder` + `SetIndent(2)`.
- `create_template`: Namensregeln → `os.Stat` auf Ordner → `editor.InitTemplate` → `setMetaScalars` (`name` = `title` oder Template-Name; `description` nur wenn gesetzt) → `WriteTextFile` → Vorschau via `renderStill` + `scaleNearest(…, 2)`.
- `write_file` In: `Template, Name, Content string; ExpectedVersion string \`json:"expected_version,omitempty"\``. Reihenfolge: Endung → Größe → Validierung (`template.yaml`: `validateTemplateYAML`; andere `.yaml`: `yaml.Unmarshal`; `.json`: `validateJSONObject`) → `editor.WriteTextFileIfMatch` → bei `default.json` Probe-Render, Fehler als Warnung `probe render failed: <err>`. Fehlerabbildung: `ErrVersionConflict` → Text aus Spec; `ErrInvalidName` → `invalid file name`.

Beschreibungen (wörtlich):
- `create_template`: `Creates a new template from the starter (canvas 240x240, mirrored front/back layout). Name: lowercase letters, digits and hyphens, must end in "-105-v1", e.g. "berlin-sbahn-105-v1". Returns a preview image.`
- `write_file`: `Writes template.yaml, default.json or another .json/.yaml file of a template. template.yaml is validated (YAML, canvas must be 240x240, test render with default.json) and NOT saved if invalid — the error explains why. Pass expected_version from read_template/read_file to avoid overwriting edits made in the editor meanwhile. Returns the new version and design warnings. Call render_preview afterwards.`

Run: `go test ./internal/mcp/ -run 'TestCreate|TestWriteFile'` → PASS.

- [ ] **Step 7: Failing tests `upload_image` / `import_image`** (gleiche Testdatei)

```go
func TestUploadImage(t *testing.T) {
	// 10x10 PNG base64 → {name:"logo.png", width:10, height:10, bytes:n, replaced:false}; second call replaced:true
	// "data:image/png;base64,<b64>" accepted; name "my logo.PNG" → stored as "my_logo.PNG"
	// JPEG bytes with name x.png → "file content is jpeg but name ends in .png"
	// name "x.gif" → "only .png, .jpg and .jpeg images are allowed"; invalid base64 → "invalid base64"
	// decoded > 5 MiB → "image too large (max 5 MiB)"
}
func TestImportImage(t *testing.T) {
	// 3000x1000 JPEG in t.TempDir() → stored as photo.jpg 2048x683, scaled_from {3000,1000}, has ImageContent ≤768 px
	// small PNG with name override "ref.png" → byte-identical copy
	// relative path "photo.jpg" → "path must be absolute"
	// symlink to image → "not a regular file"; directory → "not a regular file"
	// text file "secret.png" containing "TOPSECRET" → "not a PNG or JPEG image"; error text does not contain "TOPSECRET"
	// missing file → "cannot read <path>"
}
```
Run → FAIL. Implement both in `tools_write.go` (Base64: `strings.TrimSpace`, Präfix bis `base64,` entfernen, `base64.StdEncoding` dann `RawStdEncoding`/`URLEncoding` probieren; `~/` über `os.UserHomeDir`; `os.Lstat` + `Mode().IsRegular()`).

Beschreibungen (wörtlich):
- `upload_image`: `Stores a PNG or JPEG (base64) in the template, e.g. a logo you generated. For photos the user attached in the chat, prefer import_image with the file path — base64 copies of attachments are unreliable.`
- `import_image`: `Copies a PNG/JPEG from the user's computer (absolute path, ask the user for it) into the template, scaling it down to max 2048 px. Use it for reference photos of real displays; then use perspective_correct, sample_colors and crop_image. Returns the stored size and a preview.`

Run → PASS.

- [ ] **Step 8: Failing tests `render_preview`** in `internal/mcp/tools_preview_test.go`

```go
func TestRenderPreview_PNGDefault(t *testing.T)  // sbb → image 480x480; text JSON width 240, height 240, scale 2, frames 1
func TestRenderPreview_Scale(t *testing.T)       // scale 1 → 240x240; scale 4 → 960x960; scale 5 → schema/handler error "scale must be 1-4"
func TestRenderPreview_Data(t *testing.T)        // data {"zug1":{"vonnach":"X"}} renders different pixels than default (compare bytes)
func TestRenderPreview_DataTooLarge(t *testing.T) // data with 70 KiB string → "data too large (max 64 KiB)"
func TestRenderPreview_GIFContactSheet(t *testing.T) // faltblatt format gif → frames ≥ 2 in JSON; image width == 4*240+3*2 (scale 1)
func TestRenderPreview_GIFNotAnimated(t *testing.T)  // sbb format gif → text contains "template is not animated"; image 480x480
func TestRenderPreview_Colors(t *testing.T)         // nederland (colors: 32) → count distinct colours in 240x240 still ≤ 32
func TestRenderPreview_RenderError(t *testing.T)    // template.yaml with missing image written directly → error starts "render failed: "
```
Run → FAIL.

- [ ] **Step 9: Implement `internal/mcp/tools_preview.go`**

`func init() { registerToolGroup("preview", registerPreviewTools) }`. In: `Template string; Data map[string]any \`json:"data,omitempty"\`; Format string \`json:"format,omitempty" jsonschema:"png (default) or gif"\`; Scale int \`json:"scale,omitempty" jsonschema:"1-4, nearest neighbour; default 2 for png, 1 for gif"\``. Seed für GIF: `renderer.AnimationSeed(template, json.Marshal(data))`. Frame-Auswahl für den Kontaktbogen: `n = min(len(frames), 12)`, Index `i*(len-1)/(n-1)` für i = 0…n−1. Bogen: 4 Spalten, 2 px weißer Abstand, Gesamtbreite/-höhe > 2048 → `scale` schrittweise senken bis es passt. `render_ms` aus `time.Since`.

Beschreibung (wörtlich): `Renders the template and returns the image (default: default.json as data, PNG scaled x2). Call it after EVERY change and compare with the reference. Pass data to test other trains (long destination, delay, empty fields). format "gif" returns a contact sheet of animation frames.`

Run: `go test ./internal/mcp/ && go test ./... && go vet ./...` → PASS. Server-Build-Prüfung.

- [ ] **Step 10: Commit**

```bash
git add internal/mcp internal/renderer/renderer.go internal/renderer/renderer_test.go internal/editor/files.go internal/editor/files_test.go
git commit -m "feat(mcp): add template writing, image upload/import and preview tools"
```

---

### Task 3: KI-Design-Leitfaden, Server-Instructions, Starter-Template

**Abhängigkeiten:** Task 2 (nutzt `render_preview`/`write_file`-Verhalten für die Beispiele). Unabhängig von Tasks 4–8.

**Files:**
- Modify: `docs/ai-design-guide.md` (Volltext), `internal/mcp/instructions.go` (nur falls der Spec-Wortlaut abweicht), `internal/editor/starter/template.yaml`
- Create: `internal/mcp/guide_test.go`, `internal/editor/starter_test.go`

**Interfaces:**
- Consumes: `docs.DesignGuide`, `serverInstructions`, `editor.InitTemplate`, `renderer.LoadTemplate`, `renderer.New(...).Render`.
- Produces: nichts Neues für andere Tasks (Tasks 4–6 verweisen in ihren Beschreibungen nur per Namen auf `get_design_guide`).

- [ ] **Step 1: Beispiele rendern** — zur Anschauung und um Zahlen im Leitfaden zu prüfen:

Run: `for t in templates/*-105-v1; do go run ./cmd/zza render -t $(basename $t) -i $t/default.json -o /tmp/zza-$(basename $t).png; done`
Expected: sechs PNGs; umuc, nederland, faltblatt zeigen das Gleis oben links und unten rechts; rhb, sbb, oebb sind oben/unten gleich.

- [ ] **Step 2: Failing test Leitfaden-Pflichtinhalte** `internal/mcp/guide_test.go`

```go
func TestDesignGuide_MandatoryContent(t *testing.T) {
	g := docs.DesignGuide
	for _, s := range []string{
		"240×120", "240×240", "-105-v1",
		"y 0–119", "y 120–239", "außen",
		"umuc-105-v1", "nederland-105-v1", "faltblatt-105-v1", "rhb-105-v1",
		"copy", "src_x: 120", "x: 162",            // umuc swap example
		"Minimum 10 px",
		"{{zug1.vonnach}}", "{{zug1.abw}}", "{{gleis}}",
		"isEmpty", "Frankfurt (Main) Flughafen Fernbahnhof", `"abw": 25`,
		"render_preview", "get_template_spec", "svg_to_png", "add_font", "perspective_correct",
		"keine Verläufe",
	} { if !strings.Contains(g, s) { t.Errorf("guide lacks %q", s) } }
	// every JSON code block (```json … ```) in the guide parses with json.Unmarshal into map[string]any
}
func TestDesignGuide_YAMLExamplesParse(t *testing.T) {
	// every ```yaml block that starts with "meta:" parses via renderer.ParseTemplate with dir "/tmp"
}
```
Run → FAIL (Erstfassung aus Task 1 hat keinen Fließtext/keine Codeblöcke).

- [ ] **Step 3: Leitfaden-Volltext schreiben** — `docs/ai-design-guide.md`, Gliederung und Pflichtaussagen exakt nach Spec § „KI-Design-Leitfaden“ 1–12. Zusätzlich verbindlich:
  - Abschnitt 2 enthält ein vollständiges Minimal-Beispiel (`meta:`-Block, 240×240, eine Schrift, Gleis-Feld links + Ziel/Zeit rechts, zwei `copy`-Layer zum Spiegeln) als ```yaml-Block, und je ein Zitat der `copy`-Layer aus umuc und faltblatt mit Koordinaten.
  - Abschnitt 2 erklärt das Lesen physisch: „Wer vor Seite A steht, sieht das Gleis links; wer auf der anderen Seite steht, sieht Seite B mit dem Gleis rechts — beides ist dasselbe Geräte-Ende.“
  - Abschnitt 9 enthält die vier Testdatensätze als vollständige ```json-Blöcke mit allen Feldern aus `internal/editor/starter/default.json`.
  - Abschnitt 11 nennt die Werkzeugreihenfolge mit Parametern (Ecken-Reihenfolge oben-links, oben-rechts, unten-rechts, unten-links).
  - Abschnitt 12 als Checkliste `- [ ]`.

Run: `go test ./internal/mcp/ -run TestDesignGuide` → PASS.

- [ ] **Step 4: Failing test Starter** `internal/editor/starter_test.go`

```go
func TestStarterTemplate_MirroredLayout(t *testing.T) {
	dir := t.TempDir()
	// InitTemplate(dir, "s-105-v1"); LoadTemplate; Render with default.json
	// canvas 240x240
	// pixel (10,60) == accent colour (Gleis-Feld oben links) and (229,180) == accent colour (unten rechts)
	// pixel (230,60) == background colour and (10,180) == background colour
}
```
Akzentfarbe `#ffffff`, Hintergrund `#142a9b`, Inhaltsfeld bleibt Hintergrund. Run → FAIL (heutiger Starter hat kein Gleis-Feld).

- [ ] **Step 5: Starter umbauen** — `internal/editor/starter/template.yaml`: `meta` wie bisher (`name: "Neues Template"`, Canvas 240×240), Layer: `rect 0,0 240×120 #142a9b`; `rect 0,0 44×120 #ffffff` (Gleis-Feld); unten Hintergrund `rect 0,120 240×120 #142a9b`; `copy src 44,0 196×120 → 0,120`; `copy src 0,0 44×120 → 196,120`. Kommentare: Seite A/B, „Gleis-Feld außen: oben links, unten rechts“, „Text-Layer brauchen eine Schrift — unter fonts: eintragen (KI: add_font)“, „copy-Layer immer am Ende“. Run → PASS; zusätzlich `go test ./internal/desktop/ ./internal/server/` (nutzen den Starter) → PASS.

- [ ] **Step 6: Instructions abgleichen** — `TestNewServer_Instructions` (Task 1) prüft den Spec-Wortlaut; nur anpassen, falls die Spec inzwischen geändert wurde. Run `go test ./...` → PASS.

- [ ] **Step 7: Commit**

```bash
git add docs/ai-design-guide.md internal/mcp/guide_test.go internal/mcp/instructions.go internal/editor/starter/template.yaml internal/editor/starter_test.go
git commit -m "docs(mcp): full AI design guide and mirrored starter template"
```

---

### Task 4: `svg_to_png`

**Abhängigkeiten:** Task 2. Parallel zu 3, 5, 6, 7.

**Files:**
- Create: `internal/mcp/tools_svg.go`, `internal/mcp/tools_svg_test.go`
- Modify: `go.mod`, `go.sum`

**Interfaces:**
- Consumes: `env`, `registerToolGroup`, `writePNGFile`, `scaleNearest`, `imageContent`, Limits `maxSVGBytes`, `maxSVGSide`, `maxSVGPaths`.
- Produces: `checkSVGSource(svg string) error`, `rasterizeSVG(svg string, w, h int, antialias bool) (*image.NRGBA, error)` (nur intern).

- [ ] **Step 1: Abhängigkeit** — `go get github.com/srwiley/oksvg@v0.0.0-20221011165216-be6e8873101c github.com/srwiley/rasterx@v0.0.0-20220730225603-2ab79fcdd4ef`; `golang.org/x/image` bleibt ≥ v0.18.0 (MVS). `go build ./...` grün.

- [ ] **Step 2: Failing tests** `internal/mcp/tools_svg_test.go`

```go
const redSquare = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10"><rect x="2" y="2" width="6" height="6" fill="#ff0000"/></svg>`

func TestCheckSVGSource(t *testing.T) {
	// ok: redSquare; ok: `<use href="#a"/>`; ok: `fill="url(#g)"`
	// errors (substring): `<text>` → "svg must not contain text — put text in a text layer of template.yaml"
	//   `<tspan` → same; `<image href="a.png"/>` → "svg must not reference images or external files"
	//   `<!DOCTYPE svg>` / `<!ENTITY` → "svg must not contain DOCTYPE or ENTITY declarations"
	//   `xlink:href="http://x"` → "svg must not reference images or external files"
	//   `<style>@import url(x.css)</style>` → "svg must not contain style sheets, scripts or foreignObject"
	//   `fill="url(http://x)"` → "svg must not reference images or external files"
	//   upper-case `<TEXT>` → text error
}
func TestRasterizeSVG_Antialias(t *testing.T)   // 20x20: pixel (10,10) == #ff0000 opaque; some edge pixel has 0<alpha<255
func TestRasterizeSVG_NoAntialias(t *testing.T) // 20x20 circle r=7: every pixel alpha is 0 or 255; two overlapping circles red/blue → no pixel colour other than red, blue, transparent
func TestRasterizeSVG_TooManyPaths(t *testing.T) // 2001 <rect> → "svg has too many shapes (max 2000)"
func TestSVGToPNG_Tool(t *testing.T) {
	// svg_to_png{template:"sbb-105-v1", name:"logo.png", svg:redSquare, width:40, height:20} → file exists, decodes 40x20;
	// result image 80x40 (×2 because ≤240); text JSON width 40, height 20
	// name "logo.jpg" → "name must end in .png"; width 0 or 1025 → "width and height must be 1-1024"
	// svg of 256 KiB + 1 → "svg too large (max 256 KiB)"; oksvg parse error passes through prefixed "svg parse error: "
}
```
Run → FAIL.

- [ ] **Step 3: Implement `tools_svg.go`**

`func init() { registerToolGroup("svg", registerSVGTools) }`. `checkSVGSource`: lowercase-Kopie, Prüfreihenfolge DOCTYPE/ENTITY → text/tspan → image/href/url → style/script/foreignObject/@import; `href`-Werte per Regex `(?:xlink:)?href\s*=\s*["']([^"']*)["']`, `url\(\s*['"]?([^)'"]*)` — Wert muss mit `#` beginnen. `rasterizeSVG`: `oksvg.ReadIconStream(strings.NewReader(svg), oksvg.StrictErrorMode)`; `len(icon.SVGPaths) > maxSVGPaths` → Fehler; `icon.SetTarget(0, 0, float64(w), float64(h))`; AA: `icon.Draw(rasterx.NewDasher(w, h, rasterx.NewScannerGV(w, h, dst, dst.Bounds())), 1)`. Ohne AA: für jeden Pfad `p := icon.SVGPaths[i]` eine leere NRGBA-Ebene, `p.DrawTransformed(dasher, 1, icon.Transform)`, dann jedes Pixel mit `A >= 128` als `color.NRGBA{R,G,B,255}` ins Ergebnis.

Beschreibung (wörtlich): `Rasterises SVG you write into a PNG in the template — for logos, pictograms, arrows, frames, dot grids. NO text in the SVG (use text layers), no external references. width/height = pixel size of the PNG (max 1024). antialias=false gives hard pixel edges (good for LED/dot-matrix looks; semi-transparency is dropped). Returns the image.`

Run: `go test ./internal/mcp/ -run 'SVG'` → PASS; `go test ./...`, Server-Build-Prüfung (oksvg darf nicht in `go list -deps ./cmd/zza-server` erscheinen).

- [ ] **Step 4: Commit**

```bash
git add go.mod go.sum internal/mcp/tools_svg.go internal/mcp/tools_svg_test.go
git commit -m "feat(mcp): add svg_to_png tool"
```

---

### Task 5: Foto-Werkzeuge

**Abhängigkeiten:** Task 2. Parallel zu 3, 4, 6, 7.

**Files:**
- Create: `internal/mcp/tools_photo.go`, `internal/mcp/photo_math.go`, `internal/mcp/tools_photo_test.go`, `internal/mcp/photo_math_test.go`

**Interfaces:**
- Consumes: `env`, `registerToolGroup`, `loadImageFile`, `toNRGBA`, `writePNGFile`, `fitWithin`, `imageContent`, `renderer.Quantize`, Limits `maxToolInputPixels`, `maxToolInputSide`, `maxToolOutputSide`, `maxResultImageSide`, `maxSamplePoints`, `maxSampleRadius`.
- Produces (intern): `type point struct{ X, Y float64 \`json:"x"\`/\`json:"y"\` }`; `homography(src [4]point, w, h int) (h [9]float64, err error)` (bildet Zielpixel auf Quellpunkte ab); `warpPerspective(src *image.NRGBA, corners [4]point, w, h int) (*image.NRGBA, error)`; `colorToAlpha(img *image.NRGBA, c color.NRGBA, tol int) *image.NRGBA`; `sampleAt(img *image.NRGBA, x, y, r int) color.NRGBA`; `parseHexColor(s string) (color.NRGBA, error)` (`#rrggbb`, sonst `color must be #rrggbb`).

- [ ] **Step 1: Failing tests Mathematik** `internal/mcp/photo_math_test.go`

```go
func TestHomography_Identity(t *testing.T)  // corners (0,0),(99,0),(99,49),(0,49), w=100,h=50 → maps (x,y) to (x,y) ±1e-6
func TestHomography_Degenerate(t *testing.T) // three collinear corners → "corners do not form a valid quadrilateral"
func TestWarpPerspective_Trapezoid(t *testing.T) {
	// 200x100 source: red quad with corners (40,10),(160,20),(150,90),(50,80) on white
	// warp to 60x30 → centre pixel (30,15) red; ≥ 95% of pixels red-dominant (R>200,G<60,B<60)
}
func TestColorToAlpha(t *testing.T)  // #00ff00 pixel with tol 0 → alpha 0; #10ef10 with tol 16 → alpha 0, tol 15 → unchanged
func TestSampleAt(t *testing.T)      // 2x2 black/white checker centre radius 1 at (0,0) → mean of 4 clipped pixels = #808080 ±1
func TestParseHexColor(t *testing.T) // "#FF8000" ok; "ff8000", "#fff", "#gg0000" → error "color must be #rrggbb"
```
Run → FAIL. Implement `photo_math.go`: Homographie per 8×8-Gleichungssystem (Gauß mit Pivotsuche, |Pivot| < 1e-9 → degeneriert), Abtastung bilinear am Pixelmittelpunkt (`x+0.5`), außerhalb → transparent. Run → PASS.

- [ ] **Step 2: Failing tests Tools** `internal/mcp/tools_photo_test.go` (Template-Kopie `sbb-105-v1`, Testbild `photo.png` 400×300 per Code erzeugt und direkt in den Template-Ordner geschrieben)

```go
func TestPerspectiveCorrect_Tool(t *testing.T) // → dst "straight.png" 240x120 exists; result image present; corners count 3 → "corners must contain exactly 4 points"
func TestCropImage_Tool(t *testing.T)          // 10,10,50,40 → 50x40; 390,0,20,10 → "crop rectangle exceeds image bounds 0,0–400,300"
func TestResizeImage_Tool(t *testing.T)        // nearest 40x30 ok; mode "fancy" → `mode must be "nearest" or "smooth"`; width 2049 → "width and height must be 1-2048"
func TestQuantizeColors_Tool(t *testing.T)     // colors 4 → ≤4 distinct colours in output, JSON palette len ≤ 4, each "#rrggbb"; colors 1 → "colors must be 2-64"
func TestColorToAlpha_Tool(t *testing.T)       // white bg removed → corner pixel alpha 0
func TestSampleColors_Tool(t *testing.T)       // points [{x:5,y:5}] → colors[0].hex == "#ffffff"; point (400,0) → "point 400,0 is outside the image 400x300"; 65 points → "at most 64 points"
func TestPhotoTools_Common(t *testing.T) {
	// dst == src → "dst must differ from src — originals are kept"
	// dst "x.jpg" → "dst must end in .png"; src "missing.png" → "cannot read missing.png"
	// src 5001x5001 PNG (written directly) → "image too large: 5001x5001 (max 25 megapixels)"
}
```
Run → FAIL.

- [ ] **Step 3: Implement `tools_photo.go`** — `func init() { registerToolGroup("photo", registerPhotoTools) }`. Gemeinsamer Helfer `photoIO(e, template, src, dst) (img *image.NRGBA, write func(*image.NRGBA) (*sdk.CallToolResult, error), err error)`; alle Handler halten `e.writeMu` (außer `sample_colors`). Ergebnis-Text JSON `{name, width, height}` (+ `palette` bei quantize) und `imageContent(fitWithin(out, 768))`.

Beschreibungen (wörtlich):
- `perspective_correct`: `Straightens a photo of a display: corners = the 4 corners of the display area in source pixels, order top-left, top-right, bottom-right, bottom-left; width/height = output size (e.g. 240x120 for one side). Writes a new PNG; the original is kept.`
- `crop_image`: `Cuts a rectangle out of an image (e.g. an original logo) into a new PNG.`
- `resize_image`: `Scales an image to width x height. mode "nearest" keeps hard pixels (LED/pixel look), "smooth" (default) for photos and logos.`
- `quantize_colors`: `Reduces an image to N colours (2-64), removing photo noise; returns the palette as hex values for use in template.yaml.`
- `color_to_alpha`: `Makes one colour (#rrggbb, ± tolerance 0-255 per channel, default 24) transparent, e.g. the background of an icon.`
- `sample_colors`: `Reads the average colour around points of an image (radius 0-5 px). Use it to take exact hex colours from a reference photo.`

Run: `go test ./internal/mcp/ -run 'Photo|Perspective|Crop|Resize|Quantize|ColorToAlpha|Sample|Homography|Warp|ParseHex'` → PASS; `go test ./...`.

- [ ] **Step 4: Commit**

```bash
git add internal/mcp/tools_photo.go internal/mcp/photo_math.go internal/mcp/tools_photo_test.go internal/mcp/photo_math_test.go
git commit -m "feat(mcp): add photo tools (perspective, crop, resize, quantize, colour to alpha, sample)"
```

---

### Task 6: Fonts (`list_fonts`, `add_font`)

**Abhängigkeiten:** Task 2. Parallel zu 3, 4, 5, 7.

**Files:**
- Create: `internal/mcp/tools_fonts.go`, `internal/mcp/tools_fonts_test.go`

**Interfaces:**
- Consumes: `env` (`fontsDir`), `registerToolGroup`, `renderer.ParseTemplate`, `editor.WriteTextFile`, `editor.SanitizeFilename`, `golang.org/x/image/font/sfnt`, `golang.org/x/image/font/opentype`.
- Produces (intern): `type fontEntry struct{ Key, File, Family, Style, Source string; FoundIn []string; Bytes int64; License string }` (JSON-Tags `snake_case`); `scanFonts(e *env) (fonts []fontEntry, skipped []skippedFont)`; `resolveFontKey(e *env, key string) (path string, err error)`; `defaultFontID(file string) string`; `addFontDef(src []byte, id, file string) (out []byte, existingID string, err error)`.

- [ ] **Step 1: Failing tests** `internal/mcp/tools_fonts_test.go` — Templates-Kopie `sbb-105-v1`, `umuc-105-v1`, `oebb-105-v1`; Fonts-Ordner `<tmp>/fonts/` mit Kopie von `templates/rhb-105-v1/LLPIXEL3.ttf` und einer Textdatei `broken.ttf`.

```go
func TestDefaultFontID(t *testing.T) // "NimbusSanL-Bol.otf"→"nimbussanl-bol"; "04B_03__.TTF"→"04b-03"; "Roboto Bold.ttf"→"roboto-bold"
func TestListFonts(t *testing.T) {
	// fonts_dir == <tmp>/fonts, fonts_dir_exists true
	// first entry key "@fonts/LLPIXEL3.ttf", source "fonts-dir"
	// Roboto-Bold.ttf appears once with found_in ["oebb-105-v1","umuc-105-v1"] (identical bytes), key "oebb-105-v1/Roboto-Bold.ttf"
	// family non-empty for every entry; skipped contains {key:"@fonts/broken.ttf", reason starts "not a font: "}
	// query "nimbus" → only NimbusSanL-* entries
}
func TestListFonts_NoFontsDir(t *testing.T) // remove fonts dir → fonts_dir_exists false; text contains "Put .ttf/.otf files into"
func TestAddFont_FromFontsDir(t *testing.T) {
	// add_font{template:"sbb-105-v1", font:"@fonts/LLPIXEL3.ttf"} → {id:"llpixel3", file:"LLPIXEL3.ttf", added:true, copied:true}
	// sbb/LLPIXEL3.ttf exists; template.yaml still contains the comment lines of the original; LoadTemplate ok;
	// tmpl.Fonts last == {llpixel3 LLPIXEL3.ttf}
	// second identical call → added:false, copied:false, id "llpixel3"; fonts count unchanged
}
func TestAddFont_FromTemplate(t *testing.T)  // font "oebb-105-v1/Roboto-Light.ttf", id "light" into sbb → ok
func TestAddFont_Conflicts(t *testing.T) {
	// id "bold" already used in sbb for another file → `font id "bold" is already used for NimbusSanL-Bol.otf`
	// target already has different file with same name (write junk "LLPIXEL3.ttf" into sbb first) → "LLPIXEL3.ttf already exists in sbb-105-v1 with different content"
}
func TestAddFont_NoFontsKey(t *testing.T)    // template.yaml without fonts: → fonts: inserted before layers:, LoadTemplate ok
func TestAddFont_BadKeys(t *testing.T) {
	// "@fonts/../x.ttf", "@fonts/sub/x.ttf", "/etc/x.ttf", "sbb-105-v1/bg.png", "nope-105-v1/x.ttf" → error "unknown font key"
	// "@fonts/broken.ttf" → "not a font: "
	// symlink "@fonts/link.ttf" → "unknown font key"
}
```
Run → FAIL.

- [ ] **Step 2: Implement `tools_fonts.go`** — `func init() { registerToolGroup("fonts", registerFontTools) }`. Name-Tabelle: `sfnt.Parse` + `Name(buf, 16)`, Fallback `Name(buf, sfnt.NameIDFamily)`; Style `17` → `sfnt.NameIDSubfamily`; Lizenz `sfnt.NameIDLicense` (13), auf 200 Zeichen gekürzt. Gruppierung der Template-Fonts über SHA-256 der Bytes; `found_in` sortiert; Key nach erstem Fundort. `resolveFontKey`: Präfix `@fonts/` → Rest muss `safeName` ohne Separator sein, `os.Lstat` regulär; sonst `<template>/<file>` mit `ValidateTemplateName` + Dateiname; Endung `.ttf`/`.otf` case-insensitive; alles andere `unknown font key "<key>" — use a key from list_fonts`. `addFontDef` per `yaml.Node`: `fonts`-Sequenz suchen bzw. vor `layers` einfügen, Mapping `{id, file}` anhängen, Encoder `SetIndent(2)`; danach `renderer.ParseTemplate` mit dem Template-Ordner. Datei-Kopie atomar (`.tmp` + Rename); hält `e.writeMu`.

Beschreibungen (wörtlich):
- `list_fonts`: `Lists fonts you can add to a template: fonts already used in any template and fonts the user put into the "fonts" folder next to the templates folder. No system or web fonts. Returns key, family, style and licence info.`
- `add_font`: `Copies a font (key from list_fonts) into the template AND registers it under fonts: in template.yaml in one step; returns the font id to use in text layers (font: <id>). Existing entries are reused, never duplicated.`

Run: `go test ./internal/mcp/ -run 'Font'` → PASS; `go test ./...`.

- [ ] **Step 3: Commit**

```bash
git add internal/mcp/tools_fonts.go internal/mcp/tools_fonts_test.go
git commit -m "feat(mcp): add list_fonts and add_font tools"
```

---

### Task 7: Parallelbetrieb — Backend (Watcher, Long-Poll, ETag/If-Match, Caches)

**Abhängigkeiten:** Task 2 (`editor.WriteTextFileIfMatch`, `CurrentVersion`). Parallel zu 3, 4, 5, 6.

**Files:**
- Create: `internal/watch/watch.go`, `internal/watch/watch_test.go`, `internal/editor/feed.go`, `internal/editor/feed_test.go`
- Modify: `go.mod`, `go.sum`, `internal/editor/fs_handlers.go`, `internal/editor/fs_handlers_test.go`, `internal/server/server.go`, `internal/server/server_test.go`, `internal/renderer/renderer.go`, `internal/renderer/renderer_test.go`, `cmd/zza/main.go`

**Interfaces:**
- Consumes: `editor.WriteTextFileIfMatch`, `editor.CurrentVersion`, `editor.FileVersion`, `editor.ErrVersionConflict`.
- Produces:
  - `internal/editor/feed.go`: `type ChangeBatch struct{ Seq uint64 \`json:"seq"\`; Files []string \`json:"files"\`; Reset bool \`json:"reset,omitempty"\` }`; `type ChangeFeed interface{ Wait(ctx context.Context, template string, since uint64, timeout time.Duration) ChangeBatch; Current() uint64 }`; Feld `FSHandlers.Feed ChangeFeed` (nil = keine `/events`-Route-Funktion → 404)
  - `internal/watch`: `type Watcher struct`; `New(templatesDir string, debounce time.Duration) (*Watcher, error)`; `(*Watcher).Run(ctx context.Context)`; `(*Watcher).OnChange(fn func(template string, files []string))` (vor `Run` aufrufen); `(*Watcher).Wait(...)` und `Current()` erfüllen `editor.ChangeFeed`; `const ringSize = 256`
  - `(*server.Server).ForgetPreview(name string)`
  - Font-Cache-Schlüssel `absPath + "\x00" + size + "\x00" + mtimeUnixNano`

- [ ] **Step 1: Abhängigkeit** — `go get github.com/fsnotify/fsnotify@v1.10.1`. `go build ./...` grün.

- [ ] **Step 2: Failing tests Watcher** `internal/watch/watch_test.go` (Debounce 50 ms in Tests, Wartezeiten ≤ 2 s)

```go
func TestWatcher_ReportsFileChange(t *testing.T)   // write tdir/a-105-v1/template.yaml → Wait(ctx,"a-105-v1",0,2s) returns Files ["template.yaml"], Seq ≥ 1
func TestWatcher_Debounce(t *testing.T)            // 5 writes to two files within 20 ms → exactly one batch with Files ["default.json","template.yaml"]
func TestWatcher_IgnoresTmpHiddenAndChmodOnly(t *testing.T) // write "x.yaml.tmp", ".DS_Store", os.Chtimes(template.yaml) → Wait times out with Files empty (len 0), Seq == since
func TestWatcher_NewTemplateDir(t *testing.T)      // mkdir b-105-v1 → batch for "b-105-v1" Files ["*"]; then write b-105-v1/template.yaml → batch ["template.yaml"]
func TestWatcher_OtherTemplateNotReported(t *testing.T) // change in a-105-v1 → Wait for "c-105-v1" times out empty
func TestWatcher_SinceAndReset(t *testing.T) {
	// since == Current() after a change → waits; since 0 after 300 batches (ring 256) → Reset true, Files ["*"]
}
func TestWatcher_OnChangeCallback(t *testing.T)    // OnChange fn gets ("a-105-v1", ["template.yaml"])
func TestWatcher_ContextCancel(t *testing.T)       // Wait with cancelled ctx returns immediately, Files empty
```
Run → FAIL. Implement `internal/watch/watch.go` (fsnotify auf Wurzel + Unterordner, Filter laut Spec, Debounce je Template per `time.AfterFunc`, Ring + `sync.Cond`-freies Broadcast über „close and replace channel“ unter Mutex). Run → PASS (`go test -race ./internal/watch/`).

- [ ] **Step 3: Failing tests Editor-HTTP** `internal/editor/fs_handlers_test.go`, `internal/editor/feed_test.go`

```go
func TestFSHandlers_GetFile_ETag(t *testing.T)       // header ETag == `"` + FileVersion(content) + `"`
func TestFSHandlers_Save_IfMatchOK(t *testing.T)     // If-Match with current ETag (quoted) → 204, new ETag header == quoted FileVersion(new content)
func TestFSHandlers_Save_IfMatchConflict(t *testing.T) {
	// If-Match "\"0000000000000000\"" → 409, Content-Type application/json,
	// body {"error":"conflict","current_version":"<v>"}; file unchanged; Invalidate not called
}
func TestFSHandlers_Save_IfMatchStarAndAbsent(t *testing.T) // "*" → 204; no header → 204 (existing tests stay green)
func TestFSHandlers_Events(t *testing.T) {
	// fake feed (struct implementing ChangeFeed) returning ChangeBatch{Seq:7, Files:["a.png"]}:
	// GET /edit/mine/events?since=3 → 200 JSON {"seq":7,"files":["a.png"]}, Cache-Control no-store; fake got since==3, timeout==25s
	// no since → fake.Current() used, response {"seq":<cur>,"files":[]} immediately (Wait not called)
	// since=abc → 400; Feed nil → 404; invalid template name → 400
}
```
Run → FAIL. Implement: `GetFile` setzt ETag; `Save` liest `If-Match` (Anführungszeichen trimmen) und ruft `WriteTextFileIfMatch` (bisherige YAML-Prüfung bleibt davor); Erfolg setzt `ETag`; Konflikt → 409 JSON. Neuer Handler `Events`, Route `GET /edit/{template}/events` in `Register`. `files` immer als Array kodieren (nie `null`). Run → PASS.

- [ ] **Step 4: Failing tests Caches**

```go
// internal/renderer/renderer_test.go
func TestGetFont_ReloadsChangedFile(t *testing.T) {
	// copy Roboto-Bold.ttf as f.ttf → getFont; overwrite f.ttf with RobotoCondensed-Regular.ttf bytes and
	// set mtime +2s → getFont returns a different *opentype.Font (pointer differs); unchanged file → same pointer
}
// internal/server/server_test.go
func TestForgetPreview(t *testing.T) // put entry in s.previewGIFs["x"], ForgetPreview("x") → entry gone; unknown name no panic
```
Run → FAIL. Implement: `getFont` macht `os.Stat` vor dem Cache-Lookup und nutzt den neuen Schlüssel; `ForgetPreview` löscht unter `previewMu`. Run → PASS.

- [ ] **Step 5: Verdrahtung in `cmd/zza/main.go`** — `buildHandler` erzeugt `w, err := watch.New(tdir, 200*time.Millisecond)`; Fehler → `log.Printf("watch disabled: %v", err)` und ohne Feed weiter; sonst `w.OnChange(func(t string, _ []string){ srv.ForgetPreview(t) })`, `go w.Run(context.Background())`, `h := editor.NewFSHandlers(tdir, srv.InvalidateTemplateCache); h.Feed = w`. `zza mcp` startet **keinen** Watcher.

Test `cmd/zza` (neu in `cmd/zza/main_test.go`): `TestBuildHandler_EventsRoute` — `buildHandler(<tmp copy of sbb>)`, `httptest` `GET /edit/sbb-105-v1/events` → 200 JSON mit `seq`. Run → PASS; `go test -race ./...`; Server-Build-Prüfung (fsnotify darf nicht in `go list -deps ./cmd/zza-server` erscheinen).

- [ ] **Step 6: Commit**

```bash
git add go.mod go.sum internal/watch internal/editor internal/server/server.go internal/server/server_test.go internal/renderer/renderer.go internal/renderer/renderer_test.go cmd/zza
git commit -m "feat(editor): live file change feed, ETag/If-Match conflict detection, font cache keyed by mtime"
```

---

### Task 8: Parallelbetrieb — Editor-Frontend

**Abhängigkeiten:** Task 7.

**Files:**
- Create: `web/static/editor-sync.js`, `web/static/test/editor-sync.test.mjs`
- Modify: `web/templates/edit-editor.html`, `web/static/app.css`

**Interfaces:**
- Consumes: `GET /edit/{t}/events?since=N` → `{seq, files, reset?}`; `ETag` an `GET /edit/{t}/file/{name}`; `409` + `{"error":"conflict","current_version"}` an `POST /edit/{t}/save`; `If-Match`-Header.
- Produces (`web/static/editor-sync.js`, ES-Modul):
  - `normalizeEtag(h: string|null): string` — entfernt `W/` und Anführungszeichen, `null` → `''`
  - `decide({files: string[], reset: boolean, currentFile: string, dirty: boolean, jsonDirty: boolean}) → {reloadList: boolean, currentFile: 'none'|'silent'|'ask', refreshJson: boolean, refreshPreview: boolean}`
  - `nextDelay(failures: number): number` — 0 bei 0 Fehlern, sonst 3000 ms

- [ ] **Step 1: Failing tests** `web/static/test/editor-sync.test.mjs` (Stil wie `node-parser.test.mjs`, Kopfkommentar `// Run with: node --test web/static/test/editor-sync.test.mjs`)

```js
test('empty batch changes nothing')                  // files [] → reloadList false, currentFile 'none', refreshPreview false
test('other file changed reloads list and preview')  // files ['logo.png'], currentFile 'template.yaml' → reloadList true, 'none', refreshPreview true
test('open clean file reloads silently')            // files ['template.yaml'], dirty false → 'silent'
test('open dirty file asks')                        // dirty true → 'ask'
test('star and reset touch everything')             // files ['*'] or reset true → reloadList, currentFile 'silent' (clean), refreshJson true (jsonDirty false)
test('default.json refreshes test json only when untouched') // files ['default.json'], jsonDirty false → refreshJson true; jsonDirty true → false
test('normalizeEtag')                               // '"abc"'→'abc', 'W/"abc"'→'abc', null→''
test('nextDelay')                                   // 0→0, 1→3000, 5→3000
```
Run: `node --test web/static/test/editor-sync.test.mjs` → FAIL (module not found).

- [ ] **Step 2: Implement `web/static/editor-sync.js`** (reine Funktionen, keine DOM-Zugriffe). Run → PASS.

- [ ] **Step 3: Editor einbinden** — `web/templates/edit-editor.html`:
  - Import `{ decide, normalizeEtag, nextDelay } from '/static/editor-sync.js'`.
  - Zustand `currentVersion`, `loadedContent`, `jsonLoaded` (Text von `default.json` beim Laden), `eventsSince`.
  - `openFile`: `currentVersion = normalizeEtag(res.headers.get('ETag'))`, `loadedContent = content`; Hinweisleiste ausblenden.
  - `dirty()` = `cmView.state.doc.toString() !== loadedContent`; `jsonDirty()` = `jsonInput.value !== jsonLoaded`.
  - `saveFile(force = false)`: Header `If-Match: "<currentVersion>"` außer bei `force`; `204` → `currentVersion` aus `ETag`, `loadedContent` = gespeicherter Inhalt; `409` → Dialog öffnen.
  - Neues `<dialog id="conflict-dialog">` mit Text „Die Datei wurde außerhalb des Editors geändert (z. B. von der KI).“ und Buttons „Neu laden“ (`openFile(currentFile)`), „Überschreiben“ (`saveFile(true)`), „Abbrechen“.
  - Neue Hinweisleiste `<div id="external-change" class="external-change" hidden>` unter der Code-Toolbar: „Datei wurde extern geändert.“ + Buttons „Neu laden“ / „Ignorieren“.
  - `watchChanges()`: Schleife `fetch(/edit/${TEMPLATE}/events` + `(eventsSince === null ? '' : '?since=' + eventsSince))`; Antwort → `eventsSince = seq`; `decide(...)` anwenden (`'silent'` → `openFile(currentFile)` nur wenn die neue Version (per `GET`) von `currentVersion` abweicht; `'ask'` → Hinweisleiste); bei `404` Schleife beenden (kein Watcher); Netzfehler → `await sleep(nextDelay(++failures))`. Start in `init()` nach dem ersten Laden.
- `web/static/app.css`: Stile `.external-change` (Leiste in der Warnfarbe des bestehenden `.error-text`-Schemas) und `#conflict-dialog` (vorhandene `.btn`-Klassen verwenden).

- [ ] **Step 4: Manuell prüfen** — `go run ./cmd/zza serve --templates-dir <tmp-Kopie von templates>`, Editor `http://127.0.0.1:8080/edit/sbb-105-v1` öffnen:
  - Datei `default.json` in einem Terminal ändern → Test-JSON und Vorschau aktualisieren sich in ≤ 1 s.
  - `template.yaml` im Editor ändern (nicht speichern), extern ändern → Hinweisleiste; Speichern → Konfliktdialog; „Überschreiben“ speichert.
  - Eigenes Speichern ohne externe Änderung → keine Leiste, kein Dialog, Cursor bleibt.
  Expected: alle drei Punkte wie beschrieben.

- [ ] **Step 5: Tests + Commit**

Run: `node --test 'web/static/test/*.test.mjs' && go test ./...` → PASS.
```bash
git add web/static/editor-sync.js web/static/test/editor-sync.test.mjs web/templates/edit-editor.html web/static/app.css
git commit -m "feat(editor): live reload on external changes and conflict dialog"
```

---

### Task 9: Doku, Client-Konfiguration, Abschluss

**Abhängigkeiten:** Tasks 1–8 (gemergt).

**Files:**
- Create: `.mcp.json`
- Modify: `docs/user-guide-templates.md`, `CLAUDE.md`, `build.sh`, `README.md`

**Interfaces:**
- Consumes: alle Tool-Namen und das Subcommand aus Tasks 1–8.
- Produces: nichts für Code.

- [ ] **Step 1: `.mcp.json`** im Repo-Root:

```json
{
  "mcpServers": {
    "zza": {
      "command": "go",
      "args": ["run", "./cmd/zza", "mcp", "--templates-dir", "./templates"]
    }
  }
}
```
Prüfen: `claude mcp list` im Repo-Root (nach Bestätigung des Projekt-Servers) zeigt `zza … ✓ Connected`. Kaltstart messen: `go clean -cache` ist **nicht** nötig; `time go run ./cmd/zza version` < 5 s nach einem `go build ./cmd/zza`.

- [ ] **Step 2: User-Guide** — neuer Abschnitt `## KI-Assistent (MCP)` am Ende von `docs/user-guide-templates.md` mit Unterabschnitten:
  - `### Einrichtung in Claude Desktop` — Pfad der Konfigurationsdatei (macOS `~/Library/Application Support/Claude/claude_desktop_config.json`, Windows `%APPDATA%\Claude\claude_desktop_config.json`), JSON-Schnipsel für macOS (`…/zza.app/Contents/MacOS/zza`, `"args": ["mcp"]`) und Windows (`C:\\…\\zza.exe`), Neustart von Claude Desktop, Hammer-/Werkzeug-Symbol prüfen, macOS-Quarantäne (`xattr -dr com.apple.quarantine /Pfad/zza.app`), optional `--templates-dir`.
  - `### Einrichtung in Claude Code` — `claude mcp add zza -- /Pfad/zza mcp`; im Repo: `.mcp.json` (Projekt-Server bestätigen), `MCP_TIMEOUT=60000 claude` beim ersten Start oder vorher `go build ./cmd/zza`; Hinweis, dass Änderungen in `./templates` (git) landen.
  - `### Erste Schritte` — drei Beispiel-Prompts (neues Template nach Beschreibung; Foto → Template mit Dateipfad; „lege Schrift X in Template Y“).
  - `### Schriften` — Ordner `fonts/` neben `templates/`, nur .ttf/.otf, Lizenzen.
  - `### Foto als Vorlage` — Pfad statt Anhang, Ablauf der Foto-Werkzeuge, Zwischendateien danach im Editor löschen.
  - `### Gleichzeitig mit dem Editor arbeiten` — Live-Aktualisierung, Hinweisleiste, Konfliktdialog.
  - `### Grenzen und Sicherheit` — nur 1,05"/240×240, kein Löschen per KI, Erlaubnisdialoge, `import_image` liest nur Bilder.

- [ ] **Step 3: `CLAUDE.md`** — Tabelle/Projektstruktur: `internal/mcp/` und `internal/watch/` als DESKTOP-ONLY, `docs/embed.go` + `docs/ai-design-guide.md`; Routen Desktop um `GET /edit/{template}/events` ergänzen; Desktop-Subcommands um `mcp`; Satz „Server-Build darf `internal/mcp`, `internal/watch`, fsnotify, oksvg nicht importieren“.

- [ ] **Step 4: `build.sh`** — im heredoc `README.txt` unter „Kommandozeile“ die Zeile `  zza mcp             — MCP-Server für Claude Desktop / Claude Code (siehe Anleitung)` ergänzen; sonst unverändert. `README.md`: ein Satz + Link auf den User-Guide-Abschnitt.

- [ ] **Step 5: Gesamtprüfung**

Run:
```bash
go test -race ./... && go vet ./... && node --test 'web/static/test/*.test.mjs' \
 && CGO_ENABLED=0 go build ./cmd/zza-server \
 && ! go list -deps ./cmd/zza-server | grep -E 'fsnotify|modelcontextprotocol|oksvg|rasterx|internal/mcp|internal/watch'
```
Expected: alles grün, letzte Zeile ohne Ausgabe. Danach manueller Testplan `docs/superpowers/plans/2026-11-15-mcp-server-manual-tests.md`.

- [ ] **Step 6: Commit**

```bash
git add .mcp.json docs/user-guide-templates.md CLAUDE.md build.sh README.md
git commit -m "docs: MCP setup for Claude Desktop and Claude Code"
```
