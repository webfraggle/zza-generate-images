# MCP-Server für den Desktop-Build — Design

Stand: 2026-11-15 · Grundlage: `docs/superpowers/plans/2026-11-13-mcp-server-plan.md` (Konzept)

## Ziel

Eine KI (Claude Desktop, Claude Code) baut per natürlicher Sprache und anhand von
Beispielen (Fotos echter Anzeiger, vorhandene Templates) neue Templates für den
Zugzielanzeiger. Dafür bekommt der Desktop-Build `cmd/zza` das Subcommand
`zza mcp`, einen lokalen MCP-Server über stdio. Die KI kann Templates lesen,
anlegen, Dateien schreiben, Bilder ablegen und bearbeiten, SVG rastern, Schriften
einbinden und vor allem **gerenderte Vorschauen als Bild sehen**, um iterativ
nachzubessern. Läuft gleichzeitig die GUI, erscheinen Änderungen der KI dort
sofort; gleichzeitiges Bearbeiten derselben Datei wird erkannt statt still
überschrieben.

Umfang: alle Bauschritte des Konzepts (1 Grundgerüst, 3 SVG, 4 Foto-Werkzeuge,
5 Parallelbetrieb, 6 Fonts) plus ein **KI-Design-Leitfaden**.

## Nicht-Ziele

- Kein Remote-Betrieb (Streamable HTTP, claude.ai-Connector) — nur stdio.
- Keine Änderung am Server-Build `cmd/zza-server`: keine neuen Routen, keine neuen
  Abhängigkeiten im Binary. Erlaubt sind nur zwei verhaltensneutrale Änderungen in
  gemeinsam genutztem Code: `renderer.ParseTemplate` (Refactoring von
  `LoadTemplate`) und der Font-Cache-Schlüssel (§ Parallelbetrieb).
- Kein Löschen von Templates oder Dateien per MCP. Zwischendateien (Fotos) räumt
  der Nutzer im Editor auf; der Leitfaden sagt der KI, sie am Ende zu nennen.
- Kein `type: svg`-Layer im Renderer (SVG wird einmalig zu PNG gerastert).
- Keine Systemschriften, kein Google-Fonts-Download.
- Keine Live-Aktualisierung von Galerie und Detailseite (nur der Editor).
- Keine `.mcpb`-Erweiterung (Ein-Klick-Installation) — später.

## Architektur

```
Claude Desktop / Claude Code
        │ stdio (JSON-RPC, newline-delimited)
        ▼
zza mcp ──► internal/mcp ──► internal/editor (Pfadschutz, atomares Schreiben, Versionen)
                         ├─► internal/renderer (ParseTemplate, Render, RenderAnimation, Quantize)
                         └─► docs (eingebettete Spec + Leitfaden)
              │ schreibt direkt in
              ▼
        templates/<name>/…  ◄── fsnotify ── internal/watch ──► GUI (zza / zza serve)
                                                               GET /edit/{t}/events (Long-Poll)
                                                               ETag / If-Match → 409
```

- `zza mcp` ist ein eigener Prozess. Er braucht die GUI nicht; beide arbeiten auf
  demselben Templates-Ordner.
- stdout gehört ausschließlich dem Protokoll. Alle Logs gehen nach stderr
  (`log`-Standardausgabe ist stderr; nichts darf `fmt.Print*` nach stdout schreiben).
- SDK: `github.com/modelcontextprotocol/go-sdk` **v1.8.0** (geprüft: `mcp.NewServer`,
  generisches `mcp.AddTool[In, Out]`, `mcp.StdioTransport`, `mcp.NewInMemoryTransports`,
  `mcp.ImageContent{Data, MIMEType}`, `ServerOptions.Instructions`; ein vom Handler
  zurückgegebener `error` wird zu `CallToolResult{IsError: true}` mit dem Fehlertext —
  genau das, was die KI zum Selbstkorrigieren braucht). Eingabeschemas werden aus
  Go-Structs abgeleitet: Felder ohne `omitempty` sind Pflicht, `jsonschema:"…"`-Tag
  ist die Beschreibung, falsche Typen werden vor dem Handler abgelehnt.

### Paketstruktur

| Pfad | Build | Inhalt |
|---|---|---|
| `cmd/zza/main.go` | Desktop | neues Subcommand `mcp` |
| `docs/embed.go` | Desktop | `package docs`, `//go:embed yaml-template-spec.md ai-design-guide.md` → `docs.TemplateSpec`, `docs.DesignGuide` (`string`) |
| `docs/ai-design-guide.md` | — | KI-Design-Leitfaden (Deutsch) |
| `internal/mcp/server.go` | Desktop | `Config`, `NewServer`, `Run`, Tool-Gruppen-Registry, `env` |
| `internal/mcp/instructions.go` | Desktop | Server-`instructions` (Kurzfassung des Leitfadens) |
| `internal/mcp/tools_read.go` | Desktop | `get_template_spec`, `get_design_guide`, `list_templates`, `read_template`, `read_file` |
| `internal/mcp/tools_write.go` | Desktop | `create_template`, `write_file`, `upload_image`, `import_image` |
| `internal/mcp/tools_preview.go` | Desktop | `render_preview` |
| `internal/mcp/images.go` | Desktop | gemeinsame Bild-Helfer (Laden mit Limits, PNG speichern, Bild-Ergebnis) |
| `internal/mcp/validate.go` | Desktop | Validierung von `template.yaml`/JSON + Warnungen |
| `internal/mcp/yamledit.go` | Desktop | `meta.*`-Felder per `yaml.Node` setzen |
| `internal/mcp/tools_svg.go` | Desktop | `svg_to_png` |
| `internal/mcp/tools_photo.go` | Desktop | Foto-Werkzeuge |
| `internal/mcp/tools_fonts.go` | Desktop | `list_fonts`, `add_font` |
| `internal/watch/` | Desktop | fsnotify-Watcher + Änderungs-Feed |
| `internal/editor/` | gemeinsam importiert, Desktop-Logik | `FileVersion`, `WriteTextFileIfMatch`, ETag/If-Match, `/events`-Handler (gegen Interface, ohne fsnotify) |
| `web/static/editor-sync.js` | — | reine Entscheidungslogik für den Editor (Node-testbar) |

**Server-Build bleibt schlank:** `internal/server` importiert `internal/editor`.
Deshalb kennt `internal/editor` nur das Interface `ChangeFeed`; die fsnotify-
Implementierung liegt in `internal/watch`, das nur `cmd/zza` importiert.
Prüfung: `go list -deps ./cmd/zza-server` enthält weder `fsnotify` noch
`modelcontextprotocol` noch `oksvg`; `CGO_ENABLED=0 go build ./cmd/zza-server` gelingt.

### Tool-Registrierung (parallel baubar)

`internal/mcp/server.go` enthält eine Registry:

```go
type toolGroup struct {
    name     string
    register func(s *mcp.Server, e *env)
}
var toolGroups []toolGroup
func registerToolGroup(name string, register func(*mcp.Server, *env))
```

Jede Datei `tools_<bereich>.go` registriert sich in **ihrer eigenen** `init()`-
Funktion mit genau einer Zeile `func init() { registerToolGroup("<bereich>", register<Bereich>Tools) }`.
`NewServer` ruft alle Gruppen nach Namen sortiert auf. **Abweichung vom
Vorschlag „eine Registrierungszeile in einer gemeinsamen Datei“:** Mehrere parallel
gebaute Branches, die je eine Zeile an dieselbe Stelle einer Liste einfügen,
erzeugen sichere Merge-Konflikte (benachbarte Zeilen). Mit der `init()`-Zeile in der
eigenen Datei berühren T4, T5 und T6 keine gemeinsame Go-Datei.

`env` (geteilter Zustand aller Tools):

```go
type env struct {
    templatesDir string     // absolut
    fontsDir     string     // absolut: <Elternordner von templatesDir>/fonts
    writeMu      sync.Mutex // serialisiert alle schreibenden Tool-Aufrufe dieses Prozesses
}
```

Lesende Tools sperren nicht. Jeder Render-Aufruf erzeugt einen frischen
`renderer.New(templatesDir)`, damit nie ein veralteter Font-Cache greift.

## CLI

`zza mcp [--templates-dir DIR]`

- Templates-Ordner wie bei `serve`: `desktop.ResolveTemplatesDir(override, os.Executable())`,
  dann `desktop.EnsureTemplatesDir`.
- Log nach stderr: `zza mcp: templates=<dir> fonts=<dir>`.
- Läuft bis stdin geschlossen wird bzw. SIGINT/SIGTERM (`signal.NotifyContext`).
- `Implementation{Name: "zza", Title: "Zugzielanzeiger", Version: version.Version}`.

## Gemeinsame Regeln aller Tools

- **Template-Namen:** `renderer.ValidateTemplateName`; alle Tools außer
  `create_template` verlangen ein existierendes Verzeichnis mit `template.yaml`
  (Fehler: `template "<n>" not found`).
- **Dateinamen:** Regeln aus `internal/editor` (`[a-zA-Z0-9._-]+`, kein Pfadanteil,
  Ziel innerhalb des Template-Ordners). Beim Schreiben zusätzlich
  (`editor.ValidateWriteName`): kein führender Punkt, keine Windows-Gerätenamen
  (`CON`, `PRN`, `AUX`, `NUL`, `COM1`–`COM9`, `LPT1`–`LPT9`, Groß-/Kleinschreibung
  egal, mit beliebiger Endung) → `invalid file name "<n>" (…)`. Vorhandene Ziele, die
  Symlinks oder keine regulären Dateien sind, werden nie überschrieben
  (`refusing to overwrite non-regular file`).
- **Atomar schreiben:** ein gemeinsamer Helfer `editor.AtomicWrite` für Editor-HTTP
  und alle MCP-Tools: `os.CreateTemp(dir, ".<name>.*.tmp")` (O_EXCL, folgt nie einem
  Symlink — ein vorab platziertes `<name>.tmp` wird nicht benutzt), schreiben,
  `chmod 0644`, schließen, `os.Rename`; ist das Ziel ein Symlink/keine reguläre
  Datei, wird abgelehnt; bei Fehler wird die Temp-Datei entfernt. Temp-Namen
  beginnen mit `.` und enden auf `.tmp`; Watcher und Kapazitätsprüfung ignorieren sie.
  `editor.InitTemplate` legt Starter-Dateien mit `O_CREATE|O_EXCL` an (folgt keinem
  hängenden Symlink).
- **Fehler** sind Tool-Fehler (Text, `IsError`), englisch, knapp und so formuliert,
  dass die KI weiß, was sie ändern muss. Nie Dateiinhalte im Fehlertext.
- **Bild-Ergebnisse** sind PNG (`image/png`), immer zusätzlich zu einem kurzen
  Text-Ergebnis mit Maßen.

### Limits

| Größe | Wert |
|---|---|
| `write_file` Inhalt | ≤ 256 KiB |
| Dateien pro Template (nur beim Anlegen neuer Dateien geprüft) | ≤ 64 |
| Gesamtgröße eines Template-Ordners nach einem Schreibvorgang | ≤ 30 MiB |
| `upload_image` dekodierte Bytes | ≤ 5 MiB (Base64-String ≤ 7 MiB Zeichen) |
| `upload_image` Bildmaße | ≤ 4096 × 4096 |
| `import_image` Quelldatei | ≤ 20 MiB, ≤ 8192 px je Seite |
| `import_image` gespeicherte Größe | längste Seite ≤ 2048 px (sonst verkleinert) |
| Foto-Werkzeuge Eingabe | ≤ 25 Megapixel, ≤ 8192 px je Seite, Datei ≤ 64 MiB |
| Foto-Werkzeuge Ausgabe | 1–2048 px je Seite (`quantize_colors`/`color_to_alpha` verkleinern größere Eingaben vorher und melden das in `note`) |
| `svg_to_png` SVG-Text | ≤ 256 KiB; Ausgabe 1–1024 px je Seite; ≤ 2000 Pfade (auch nach `<use>`-Expansion); XML-Tiefe ≤ 64 |
| `svg_to_png` Rechenbudget | Pfade × Breite × Höhe ≤ 2^26 (≈ 67 Mio., schlimmster Fall ca. 5 s); Rasterzeit ≤ 10 s (sonst `svg rendering timed out`, nichts geschrieben) |
| `render_preview` `scale` | 1–4 (Standard 2 für PNG, 1 für GIF) |
| `render_preview` `data` | ≤ 64 KiB (als JSON) |
| `render_preview` GIF-Kontaktbogen | ≤ 12 Frames, 4 Spalten, längste Seite ≤ 2048 px (sonst `scale` reduziert) |
| Bild-Ergebnisse der Foto-/Import-Werkzeuge | längste Seite ≤ 768 px (verkleinert, nur Anzeige) |
| `sample_colors` | ≤ 64 Punkte, `radius` 0–5 |
| `quantize_colors` | 2–64 Farben |
| Fonts | ≤ 50 MiB (wie `renderer.maxFontFileBytes`); `list_fonts` ≤ 200 Treffer |

## Tools

Parameter in `snake_case`. „opt.“ = optional. Rückgabe „JSON“ heißt: Text-Content
mit JSON (vom SDK aus dem `Out`-Typ erzeugt) bzw. Text + Bild.

### Lesen (T1)

| Tool | Parameter | Ergebnis | Fehler |
|---|---|---|---|
| `get_template_spec` | — | Text: `docs/yaml-template-spec.md` | — |
| `get_design_guide` | — | Text: `docs/ai-design-guide.md` | — |
| `list_templates` | — | JSON `{templates: [{name, title, description, width, height, animated, has_default, error?}]}`; alle Ordner mit gültigem Namen und `template.yaml`, sortiert; nicht ladbare Templates erscheinen mit `error` (damit die KI ihr eigenes kaputtes Template findet) | — |
| `read_template` | `template` | JSON `{template, template_yaml, default_json, files: [{name, size, editable}], versions: {"template.yaml": v, "default.json": v}}`; fehlt `default.json`, ist `default_json` leer und die Version fehlt | not found |
| `read_file` | `template`, `name` | JSON `{name, content, version}` für `.yaml`/`.json` | not found; `only .yaml and .json files can be read as text` |

`version` = `editor.FileVersion(content)`: die ersten 16 Hex-Zeichen von SHA-256.
Dieselbe Funktion liefert die ETags des Editors.

### Schreiben und Vorschau (T2)

**`create_template(name, title?, description?)`**
- `name` muss `renderer.ValidateTemplateName` erfüllen **und** auf `-105-v1` enden
  (Fehler: `template names must end in "-105-v1" (only 1.05" displays are supported)`).
- Existiert der Ordner: `template "<n>" already exists — use read_template / write_file`.
- Legt per `editor.InitTemplate` Starter-`template.yaml` + `default.json` an; setzt
  `meta.name` (Standard: `name`) und `meta.description` per `yaml.Node`.
- Ergebnis: JSON `{template, files, versions}` + Vorschau-Bild (Scale 2).

**`write_file(template, name, content, expected_version?)`**
- Nur `.yaml`/`.json`, ≤ 256 KiB.
- `expected_version` gesetzt und ≠ aktueller Version (oder Datei fehlt):
  `conflict: <name> was changed since you read it (current version <v>). Call read_file/read_template again and re-apply your change.`
  Nichts wird geschrieben.
- `.json`: muss ein JSON-Objekt sein (`invalid JSON: …`).
- `template.yaml`: `renderer.ParseTemplate(content, templateDir)`; danach
  `meta.canvas` muss **240 × 240** sein; danach Probe-Render (`Render`, Standbild) mit
  `default.json` (fehlt/ungültig → `{}`). Jeder Fehler → nichts wird geschrieben,
  Fehlertext geht an die KI (`template.yaml rejected: <fehler>`).
- Andere `.yaml`-Dateien: nur YAML-Syntax prüfen.
- `default.json`: nach dem Schreiben Probe-Render des Templates; ein Render-Fehler ist
  hier nur eine **Warnung** (sonst könnte ein kaputtes `template.yaml` das Reparieren
  der Testdaten blockieren).
- Warnungen (blockieren nicht), nur für `template.yaml`:
  - `bottom half (y 120-239) is empty — the back side of the display shows nothing; rebuild it with copy layers (see get_design_guide)` — wenn alle Pixel der unteren Hälfte im Probe-Render vollständig transparent sind.
  - `text layer <pfad> uses size <s> px — below 10 px is hard to read on the 1.05" display` — für jeden Text-Layer (rekursiv) mit `0 < size < 10`; `<pfad>` = `layers[i]` bzw. `layers[i].layers[j]`.
  - `layer <pfad> draws static text "<value>" that looks like train data — use {{zug1.…}} placeholders` — wenn ein Text-Layer ohne `{{` einen Wert hat, der wie eine Uhrzeit (`^\d{1,2}:\d{2}$`) aussieht.
- Ergebnis: JSON `{name, version, warnings}`.

**Entscheidung Canvas:** Ablehnen statt warnen. Das Gerät zeigt ausschließlich
240×240 (zwei 240×120-Seiten); jedes andere Format wäre auf dem Anzeiger unbrauchbar,
und eine Warnung würde die KI erfahrungsgemäß übergehen. Die GUI bleibt frei
(Experten, Tests mit anderen Formaten); nur der MCP-Weg erzwingt 240×240.
Wird 2026 ein weiteres Display unterstützt, wird die Regel zu einer Liste erlaubter
Formate je Namens-Suffix.

**`upload_image(template, name, data_base64)`**
- `name` per `editor.SanitizeFilename`; Endung `.png`, `.jpg`, `.jpeg`.
- Base64 (Standard oder URL-Alphabet, Data-URL-Präfix `data:image/…;base64,` wird
  entfernt) ≤ 5 MiB dekodiert; muss sich als PNG/JPEG dekodieren lassen, Format passend
  zur Endung (`file content is jpeg but name ends in .png`), Maße ≤ 4096².
- Überschreiben erlaubt (`replaced: true`).
- Ergebnis: JSON `{name, width, height, bytes, replaced}`.

**`import_image(template, path, name?)`**
- Kopiert eine **lokale** Bilddatei des Nutzers in das Template. Grund: Ein Modell
  kann die Bytes eines Chat-Anhangs nicht zuverlässig als Base64 wiedergeben; der Nutzer
  nennt stattdessen den Pfad (oder zieht die Datei ins Terminal/den Chat, der den Pfad
  zeigt).
- `path` muss absolut sein; `~/` am Anfang wird zum Home-Verzeichnis aufgelöst.
- Zuerst die Endung `.png`/`.jpg`/`.jpeg` (Groß-/Kleinschreibung egal), erst danach
  wird die Datei angefasst. `os.Lstat`: reguläre Datei, **kein Symlink**; ≤ 20 MiB;
  muss als PNG/JPEG dekodierbar sein; ≤ 8192 px je Seite und ≤ 25 Mio. Pixel
  (`maxToolInputPixels`, geprüft vor dem Dekodieren).
- Längste Seite > 2048 px → auf 2048 verkleinert (`CatmullRom`), gespeichert als PNG
  bzw. JPEG (Qualität 90) je nach Zielendung; sonst Byte-Kopie.
- `name` opt., Standard `SanitizeFilename(filepath.Base(path))`, Endung muss erlaubt sein.
- EXIF-Orientierung wird nicht ausgewertet (Nicht-Ziel; der Leitfaden sagt, ein
  gedrehtes Foto per `perspective_correct` gerade zu ziehen).
- Ergebnis: JSON `{name, width, height, scaled_from?: {width, height}}` + Bild
  (längste Seite ≤ 768) + Hinweis, dass Koordinaten für Foto-Werkzeuge sich auf
  `width`×`height` beziehen.
- „Fehlt“, „keine reguläre Datei/Symlink“ und „nicht lesbar“ ergeben denselben Text
  `cannot read image file` (kein Orakel für die Existenz beliebiger Pfade);
  Dateiinhalte erscheinen nie im Fehlertext.

**Sicherheitsabwägung `import_image`:** Das Tool liest beliebige Pfade, die der
Nutzer lesen kann — eine KI könnte damit fremde Bilder in den Chat holen. Gegenmaßnahmen:
(1) nur dekodierbare PNG/JPEG — Schlüssel, Dokumente, Konfigurationsdateien scheitern
beim Dekodieren, und weder Inhalt noch Teilinhalt landet im Fehlertext; (2) keine
Symlinks, nur reguläre Dateien (keine Geräte/FIFOs); (3) Größenlimits; (4) Ziel nur
innerhalb des Templates; (5) Claude Desktop und Claude Code fragen vor jedem Tool-Aufruf
um Erlaubnis, der Pfad steht sichtbar im Aufruf; (6) nur lokal, kein Netz. Das
Restrisiko (KI liest ein privates Foto, das der Nutzer nicht meinte) entspricht dem
eines Dateiauswahldialogs und wird akzeptiert.

**`render_preview(template, data?, format?, scale?)`**
- `data` opt.: JSON-Objekt statt `default.json` (fehlt `default.json` → `{}`).
- `format`: `png` (Standard) oder `gif`.
- `png`: `Render`; wenn `meta.canvas.colors > 0`, `renderer.Quantize` wie der Server;
  Vergrößerung um `scale` (Standard 2) mit Nearest Neighbour.
- `gif`: `RenderAnimation` mit `renderer.AnimationSeed(template, dataJSON)`; ist das
  Template nicht animiert, wie `png` plus Hinweis `template is not animated`. Sonst
  Kontaktbogen: ≤ 12 Frames gleichmäßig verteilt inkl. erstem und letztem, 4 Spalten,
  2 px weißer Abstand, `scale` Standard 1. Der Kontaktbogen-Code lebt in
  `internal/mcp` (die vorhandene Funktion in `contactsheet_test.go` ist Test-Code).
- Ergebnis: Text `{"width":240,"height":240,"scale":2,"frames":1,"render_ms":12}` + Bild.
- Fehler: Render-Fehler wörtlich (`render failed: …`).

### SVG (T4)

**`svg_to_png(template, name, svg, width, height, antialias?)`**
- `name` endet auf `.png`; `antialias` Standard `true`.
- Vorprüfung (Groß-/Kleinschreibung egal) lehnt ab: `<!DOCTYPE`, `<!ENTITY`, `<text`,
  `<tspan`, `<image`, `<foreignObject`, `<script`, `<style`, `@import`, jedes
  `href`/`xlink:href`, dessen Wert nicht mit `#` beginnt, jedes `url(` nicht gefolgt
  von `#`. Fehler nennen die Regel, z. B.
  `svg must not contain text — put text in a text layer of template.yaml`.
- Struktur-Vorprüfung vor oksvg (`checkSVGStructure`, `encoding/xml` mit derselben
  Zeichensatz-Behandlung wie oksvg): oksvg expandiert `<use>` rekursiv schon beim
  Parsen — eine Selbstreferenz sprengt den Stack (fataler Absturz, nicht per
  `recover` abfangbar), verschachtelte `<use>` wachsen exponentiell, bevor das
  Pfadlimit greift. Die Prüfung bildet oksvgs Definitionsläufe in `<defs>` nach
  (ein Element mit `id` beginnt einen Lauf, alle folgenden Elemente gehören dazu),
  zählt Formen je `id` per DFS mit Memo und Sättigung und lehnt ab:
  Zyklus → `svg contains a recursive <use> reference`; unbekannte id →
  `svg <use> references unknown id "<id>"`; mehr als 2000 Formen nach Expansion →
  `svg has too many shapes after <use> expansion (max 2000)`; Verschachtelung > 64 →
  `svg is nested too deeply (max 64 levels)`; nicht lesbares XML → `svg parse error: …`.
- Parsen mit `oksvg.ReadIconStream(…, oksvg.StrictErrorMode)` (lehnt nicht
  unterstützte Elemente ab), ≤ 2000 Pfade.
- Rendern: `icon.SetTarget(0, 0, width, height)` (ViewBox wird auf die Zielgröße
  gestreckt).
  - `antialias: true`: `icon.Draw` mit `rasterx.NewScannerGV` auf transparentem NRGBA.
  - `antialias: false`: jeder Pfad einzeln mit Antialiasing in eine leere Ebene
    gezeichnet; jedes Pixel mit Alpha ≥ 128 wird mit Alpha 255 und seiner Farbe ins
    Ergebnis geschrieben, alle anderen bleiben unberührt. Ergebnis: harte Kanten auch
    zwischen zwei Formen. Halbtransparenz geht dabei verloren (steht in der Beschreibung).
- Ergebnis: JSON `{name, width, height}` + Bild (bis 240 px je Seite ×2 Nearest
  Neighbour, sonst 1:1).
- Bibliothek: `github.com/srwiley/oksvg` + `github.com/srwiley/rasterx`
  (CGO-frei, nur Desktop-Build). Geprüft: `StrictErrorMode` meldet
  `Cannot process svg element text` bzw. `… image`.

### Foto-Werkzeuge (T5)

Gemeinsam: `src` ist eine `.png`/`.jpg`/`.jpeg` im Template; `dst` endet auf `.png`,
darf nicht gleich `src` sein (`dst must differ from src — originals are kept`);
Eingabe-Limits siehe oben; Ergebnis JSON `{name, width, height}` + Bild (≤ 768 px).

| Tool | Parameter | Verhalten |
|---|---|---|
| `perspective_correct` | `template, src, dst, corners: [{x,y}×4], width, height` | Ecken in Quellpixeln, Reihenfolge oben-links, oben-rechts, unten-rechts, unten-links. Homographie Ziel→Quelle, bilineare Abtastung, Pixel außerhalb der Quelle transparent. Fehler bei ≠ 4 Ecken oder degeneriertem Viereck (`corners do not form a valid quadrilateral`). |
| `crop_image` | `template, src, dst, x, y, width, height` | Rechteck muss vollständig im Bild liegen (`crop rectangle exceeds image bounds 0,0–<w>,<h>`). |
| `resize_image` | `template, src, dst, width, height, mode` | `mode`: `nearest` (Pixel-Optik) oder `smooth` (CatmullRom, Standard). Keine Seitenverhältnis-Automatik. |
| `quantize_colors` | `template, src, dst, colors` | `renderer.Quantize` (Median-Cut) auf 2–64 Farben; Ergebnis zusätzlich `palette: ["#rrggbb", …]`. |
| `color_to_alpha` | `template, src, dst, color, tolerance` | `color` `#rrggbb`; Pixel mit `max(|Δr|,|Δg|,|Δb|) ≤ tolerance` (0–255, Standard 24) werden voll transparent. |
| `sample_colors` | `template, src, points: [{x,y}], radius` | Mittelwert über das Quadrat (2·radius+1)², am Rand beschnitten; Ergebnis `{colors: [{x, y, hex}]}`, kein Bild, schreibt nichts. Punkt außerhalb → Fehler mit Bildmaßen. |

Reines Go (`golang.org/x/image/draw` ist bereits Abhängigkeit), keine neue Bibliothek.

### Fonts (T6)

**Quellen** (entschieden):
1. **Templates:** jede `.ttf`/`.otf` (Endung case-insensitive) direkt in einem
   Template-Ordner unter `templatesDir`. Gleiche Datei (gleicher SHA-256) in mehreren
   Templates wird einmal gelistet, mit allen Fundorten.
2. **Fonts-Ordner:** `<Elternordner von templatesDir>/fonts/` — der Geschwisterordner
   von `templates/`. Nur Dateien direkt darin (keine Unterordner), keine Symlinks.
   Der Ordner wird nicht automatisch angelegt.

Schlüssel einer Schrift (`key`): Fonts-Ordner `@fonts/<datei>`, Template
`<template>/<datei>`. `@` ist in Template-Namen unzulässig → eindeutig.

**`list_fonts(query?)`**
- Ergebnis JSON `{fonts_dir, fonts_dir_exists, fonts: [{key, file, family, style, source, found_in, bytes, license?}], skipped: [{key, reason}], truncated?, note?}`.
- `fonts_dir` ist nur der Ordnername mit `/` (Standard `fonts/`, liegt neben dem
  Templates-Ordner) — kein absoluter Pfad (Nutzername, Ordnerstruktur).
- `family`/`style` aus der Name-Tabelle (`sfnt` NameID 1/2, Typographic 16/17 bevorzugt),
  `license` aus NameID 13 (≤ 200 Zeichen), `source` = `fonts-dir` | `template`.
- `query`: Teilstring, case-insensitive, über `family`, `style`, `file`.
- Unlesbare/zu große Dateien landen in `skipped`. Sortierung: `fonts-dir` zuerst,
  dann `family`, `style`. ≤ 200 Einträge.
- Ist der Fonts-Ordner leer oder fehlt er, enthält `note` den Hinweis mit dem
  **absoluten** Pfad (der Nutzer muss wissen, wohin):
  `Put .ttf/.otf files into <absoluter Pfad> to make more fonts available.`
  Bei mehr als 200 Treffern `truncated: true` und in `note`
  `Results are limited to 200 entries; narrow the list with query.`

**`add_font(template, font, id?)`**
- `font` = `key` aus `list_fonts`. Quelle wird erneut validiert (Schlüsselform,
  Dateiname, regulär, kein Symlink, ≤ 50 MiB, `opentype.Parse` erfolgreich).
- Kopiert die Datei unter ihrem (sanitisierten) Namen ins Template. Existiert dort
  schon eine Datei gleichen Namens: identischer Inhalt → wiederverwenden; sonst Fehler
  `<file> already exists in <template> with different content`.
- Trägt unter `fonts:` in `template.yaml` `{id, file}` ein, per `yaml.Node`
  (Kommentare bleiben erhalten, Einrückung 2):
  - Eintrag mit gleicher `file` vorhanden → nichts doppelt anlegen, vorhandene `id` zurück.
  - `id` opt.; Standard: Dateiname ohne Endung, kleingeschrieben, jede Folge von
    Zeichen außerhalb `[a-z0-9]` → `-`, Ränder getrimmt (`NimbusSanL-Bol.otf` → `nimbussanl-bol`).
  - `id` schon für andere Datei vergeben → Fehler `font id "<id>" is already used for <file>`.
  - Kein `fonts:`-Schlüssel → wird vor `layers:` eingefügt.
- Das geänderte YAML wird mit `renderer.ParseTemplate` geprüft und atomar geschrieben.
- Lizenz: Ergebnis enthält `license_note: "Font licences are the user's responsibility; check before sharing the template."`.
- Ergebnis JSON `{id, file, added, copied, license_note}`.

## Parallelbetrieb mit der GUI (T7, T8)

### Watcher (`internal/watch`)

- `fsnotify` auf `templatesDir` und jedem direkten Unterordner; neue Unterordner werden
  bei `Create` nachgemeldet und beobachtet.
- Ordner-Ereignisse auf der Wurzel zählen nur als `Create|Remove|Rename` und nur für
  gültige Template-Namen (Windows meldet `Write` auf dem Ordner bei jeder Dateiänderung darin).
- Ignoriert: Namen auf `.tmp`, Namen mit führendem `.`, Namen außerhalb
  `[a-zA-Z0-9._-]+`, reine `Chmod`-Ereignisse (wichtig: `Server.InvalidateTemplateCache`
  macht `os.Chtimes` auf `template.yaml` — sonst Endlosschleife).
- Entprellen: 200 ms nach dem letzten Ereignis je Template wird ein Sammel-Ereignis
  veröffentlicht `{seq, template, files[]}`; `seq` ist ein prozessweiter, streng
  steigender Zähler. Ein Ordner-Ereignis (Template angelegt/gelöscht) erzeugt
  `files: ["*"]`.
- Ring der letzten 256 Ereignisse für Nachzügler.

### Transport: Long-Poll statt SSE (Abweichung, begründet)

Die Desktop-GUI läuft im Wails-AssetServer. In Wails v2.9.3 puffert der Windows-
`responseWriter` (`pkg/assetserver/webview/responsewriter_windows.go`) die gesamte
Antwort in einem `bytes.Buffer` und gibt sie erst bei Handler-Ende an WebView2; der
vorgeschaltete `contentTypeSniffer` implementiert kein `http.Flusher`. **SSE käme unter
Windows nie an.** Long-Poll funktioniert mit gepufferten und gestreamten Antworten
gleichermaßen (Wails macOS/Windows, Browser-Fallback, `zza serve`).

`GET /edit/{template}/events?since=<seq>`
- `since` fehlt → sofort `{"seq": <aktuell>, "files": []}` (Startpunkt).
- Gibt es Ereignisse für dieses Template mit `seq > since` → sofort
  `{"seq": <aktueller globaler Zähler>, "files": [vereinigt, sortiert]}`.
- `since` älter als der Ring, nach einem fsnotify-Überlauf (`ErrEventOverflow`) oder
  größer als der aktuelle Zähler (App-Neustart) → sofort `{"seq": <aktuell>, "files": ["*"], "reset": true}`.
- Sonst warten bis zu **25 s** (oder Request-Ende) → danach `{"seq": <aktueller globaler Zähler>, "files": []}`
  (`seq` ist in allen Antworten der aktuelle globale Zähler).
- Antwort `Content-Type: application/json`, `Cache-Control: no-store`.
- Ohne Watcher (`FSHandlers.Feed == nil`): `404`.
- Ungültiges `since` → `400`.

### Versionen, ETag, If-Match (Editor-HTTP)

- `GET /edit/{t}/file/{name}` setzt `ETag: "<FileVersion>"`.
- `POST /edit/{t}/save`: Header `If-Match` (mit oder ohne Anführungszeichen) gesetzt
  und ≠ aktuelle Version bzw. Datei fehlt → **409** mit JSON
  `{"error":"conflict","current_version":"<v>"}` (`""` wenn die Datei fehlt), nichts
  wird geschrieben. `If-Match: *` oder fehlender Header → schreiben wie bisher.
  Erfolg: weiterhin `204`, zusätzlich `ETag: "<neue Version>"`.
- Prüfung und Schreiben laufen in `editor.WriteTextFileIfMatch` unter einem
  prozessweiten Mutex. Zwischen zwei Prozessen (GUI und `zza mcp`) bleibt ein
  Zeitfenster von Mikrosekunden zwischen Prüfen und Umbenennen — akzeptiert.

### Caches

- Render-Cache des Servers: Der Schlüssel enthält bereits Name, Größe und mtime jeder
  Datei des Template-Ordners → externe Änderungen invalidieren automatisch. Der Watcher
  ruft **nicht** `InvalidateTemplateCache` auf (würde selbst Ereignisse auslösen).
- GIF-Vorschau-Mikrocache (`previewGIFs`, 10 s): neue Methode
  `Server.ForgetPreview(name string)`; der Desktop-Build ruft sie bei jedem Watcher-
  Ereignis auf.
- Font-Cache des Renderers: Schlüssel bisher nur absoluter Pfad → eine ersetzte
  Schriftdatei bliebe bis zum Neustart alt. Neu: Schlüssel `absPath + "\x00" + size +
  "\x00" + mtimeUnixNano` (der `os.Stat` passiert vor dem Cache-Lookup). Gilt auch
  für den Server-Build; dort ändert sich nur, dass ein Deploy mit geänderten Fonts ohne
  Neustart greift.

### Editor-Frontend (T8)

- Merkt sich je offener Datei die Version (ETag) und den geladenen Inhalt;
  „ungespeichert“ = aktueller Inhalt ≠ geladener Inhalt.
- Speichern sendet `If-Match`. Bei `409` erscheint ein Dialog:
  „Die Datei wurde außerhalb des Editors geändert (z. B. von der KI).“ mit
  **Neu laden** (verwirft eigene Änderungen), **Überschreiben** (speichert ohne
  `If-Match`) und **Abbrechen**.
- Long-Poll-Schleife ab Seitenstart. Bei Ereignis:
  - Dateiliste neu laden;
  - offene Datei betroffen (oder `*`): ohne ungespeicherte Änderungen still neu laden,
    sonst Hinweisleiste „Datei wurde extern geändert — Neu laden / Ignorieren“;
  - `default.json` betroffen und Test-JSON-Feld seit dem Laden unverändert → Feld neu füllen;
  - Vorschau neu rendern.
- Fehler/Netzabbruch: 3 s warten, neu verbinden (mit `since` des letzten Erfolgs).
- Entscheidungslogik als reines Modul `web/static/editor-sync.js`, getestet mit
  `node --test`.
- Echo der eigenen Speicherung: Nach dem Speichern kennt der Editor die neue Version;
  ein Ereignis für dieselbe Datei führt zum Neuladen nur, wenn die per `GET` gelesene
  Version abweicht (keine sichtbare Wirkung beim eigenen Speichern).

## KI-Design-Leitfaden (`docs/ai-design-guide.md`)

Deutsch, eingebettet, per `get_design_guide` abrufbar. Zielgruppe ist die KI, nicht
der Nutzer: Imperativ, konkrete Zahlen, Beispiele aus den vorhandenen Templates.
Gliederung und **Pflichtaussagen** (der volle Text entsteht in T3):

1. **Zielgerät** — Nur 1,05"-Displays: je Seite 240×120 px, Canvas 240×240,
   Template-Name `<name>-105-v1`. `create_template`/`write_file` lehnen andere Größen ab.
2. **Vorder- und Rückseite** — Obere Hälfte y 0–119 = Seite A, untere Hälfte y 120–239
   = Seite B; der Anzeiger hängt am Bahnsteig und wird von beiden Seiten gelesen.
   Elemente, die zu einem Ende des Geräts gehören (Gleisnummer, Uhr, Bahnhofs-/Gleis-
   Schild), sitzen **außen**: in der oberen Hälfte links, in der unteren Hälfte rechts
   (so machen es `umuc-105-v1`, `nederland-105-v1`, `faltblatt-105-v1`). Dafür wird
   die untere Hälfte **gespiegelt aufgebaut**, nicht 1:1 kopiert: erst die obere Hälfte
   vollständig zeichnen, dann mit mehreren `copy`-Layern die Blöcke vertauscht nach
   y 120 setzen. Ein einzelner `copy` der ganzen oberen Hälfte ist **nur** für
   symmetrische Layouts ohne Rand-Element richtig (`rhb-105-v1`, `sbb-105-v1`,
   `oebb-105-v1`). Konkrete Beispiele mit Koordinaten:
   - umuc: oben Uhr+Gleis links (x 0–76), Liste rechts (x 120–239); unten
     `copy src 120,18 120×96 → 0,138` und `copy src 0,18 77×102 → 162,138`;
     Hintergrund `bg.png` ist 240×240 und enthält beide Hälften schon gespiegelt.
   - nederland: Gleis-Schild links, Tafel Mitte, Uhr rechts; unten drei `copy`
     (Gleis → x 205, Uhr → x 0, Tafel → x 59), plus der „Parken-und-Zurückkopieren“-
     Trick, mit dem `copy` Text an der Tafelkante abschneidet.
   - faltblatt: erst ganze obere Hälfte kopieren, dann zwei `copy` mit vertauschten
     Blöcken darüber (Gleis-Schild → x 186, Klappen → x 3).
   - Regeln: `copy` steht **am Ende** (kopiert alles davor, auch animierte Zeiger in
     jedem Frame); Quell- und Zielblöcke dürfen sich nicht überlappen; Lücken in der
     unteren Hälfte brauchen Hintergrund (240×240-`bg.png` oder `rect` vor den `copy`).
3. **Anmutung** — Vollflächen, klare Farbfelder, 2–5 kräftige Farben, hoher Kontrast;
   keine Verläufe, Schatten, Fotorealismus, Transparenzeffekte. Belegte Paletten:
   Blau `#142a9b` + Weiß + Gelb `#fff048` + Rot `#ff0000` (SBB, U-Bahn München);
   Blau `#0000c6` + Weiß + Gelb `#fbef52` (ÖBB); Blau `#024ddc`/`#0415a1` + Weiß + Grau
   `#e6ebef` (NS); Gelb `#ffde00` auf Schwarz (RhB, LED-Optik). Hintergrund schwarz oder
   eine dunkle Vollfarbe; Fotos nur als Vorlage, nie als Hintergrund.
4. **Schrift** — `size` = Pixelhöhe (72 dpi). Belegte Größen: Hauptzeile/Ziel 19–23 px,
   Zeit/Zug-Nr. 15–23 px, Via/Folgezeilen 12–16 px, Kopf-/Kleintext 10 px.
   **Minimum 10 px.** Fette Grotesk (Roboto Bold, Nimbus Sans Bold) für Hauptinhalte,
   Regular/Condensed für Listen; Pixel-Fonts (LLPIXEL3, 04B_03) mit `scale_x: 0.5` für
   LED-Optik. Lange Texte: `max_width` + Umbruch, `scale_x` 0.5–0.9 zum Stauchen,
   `fit_items` für Via-Listen.
5. **Raster und Ränder** — Außenrand 2–5 px; Zeilenraster 20–26 px (umuc: Zeilen bei
   y 37/63/89); Fußband 16 px (nederland y 95–110); Textanfang ≥ 3 px vom Feldrand.
6. **Dynamische Inhalte** — Alles, was sich je Zug ändert, ist ein Text-Layer mit
   `{{…}}`, nie Teil eines Bildes. Feldliste mit Beispielwerten aus den `default.json`:
   `gleis` (6), `abschnitt` ("A-F"), `folgezuege`, `zugN.zeit` ("15:54"),
   `zugN.vonnach` ("Sennhof-Kyburg"), `zugN.nr` ("IC23", "U1", "Sprinter"),
   `zugN.via` ("Othmarsingen - Mägenwil - …", Trenner " - "), `zugN.abw` (10; 0/leer =
   pünktlich), `zugN.hinweis` ("Abweichende Wagenreihung"), `fusszeile`, `abschnitte`,
   `reihung`; Systemvariablen `now`, `anim`. Optionale Elemente mit
   `if: "not(isEmpty(…))"`. Zuggattungs-Logos: `file` ist nicht interpolierbar →
   `if`/`elif`-Kette (umuc U1–U8).
7. **Bilder** — Statische Teile (Rahmen, Logos, Piktogramme, Hintergrund) als PNG:
   bevorzugt `svg_to_png` (ohne Text, `antialias: false` für Pixel-/Punktmatrix-Optik),
   Fotos nur über die Foto-Werkzeuge. Hintergrund 240×120 (wird mitkopiert) oder 240×240.
8. **Arbeitsweise** — (1) `get_template_spec` + `get_design_guide` lesen;
   (2) `list_templates`, 1–2 passende Beispiele mit `read_template` + `render_preview`
   ansehen; (3) `create_template`; (4) obere Hälfte bauen, nach **jeder** Änderung
   `render_preview` und mit der Vorlage vergleichen; (5) untere Hälfte gespiegelt;
   (6) Testdatensätze; (7) Abschluss-Checkliste; (8) dem Nutzer Name, Vorschau und
   übrig gebliebene Zwischendateien nennen.
9. **Testdatensätze** — Pflicht vor „fertig“: `render_preview` mit (a) `default.json`,
   (b) langem Ziel (`"Frankfurt (Main) Flughafen Fernbahnhof"`) und langer Via,
   (c) Verspätung (`abw: 25`) und Hinweis, (d) leerem Hinweis, leerem Via, leeren
   `zug2`/`zug3`. Die JSON-Beispiele stehen vollständig im Leitfaden.
10. **Fonts** — `list_fonts` → `add_font`; nur vorhandene Quellen; Lizenzhinweis.
11. **Foto → Template** — `import_image` → `perspective_correct` auf 240×120 (bzw.
    das Seitenverhältnis des Fotos) → `sample_colors` für exakte Hex-Werte → Logos per
    `crop_image` oder als SVG nachbauen → Texte als Text-Layer → vergleichen.
12. **Abschluss-Checkliste** — 240×240; obere und untere Hälfte gefüllt; Gleis außen
    (falls vorhanden); keine Schrift < 10 px; alle Zugdaten als Platzhalter; vier
    Testdatensätze ohne Überlauf; `default.json` mit realistischen Werten.

Die **Server-`instructions`** (englisch, ≤ 1500 Zeichen) fassen 1, 2, 3, 6 und 8 zusammen
und verweisen auf `get_design_guide`. Wortlaut (fest):

```
zza builds images for a model-railway departure display (Zugzielanzeiger).
Before creating or changing a template, call get_template_spec and get_design_guide and read them fully.

Hard rules:
- Only 1.05" displays: canvas 240x240, template names end in "-105-v1".
- The image holds both sides of the display: top half (y 0-119) = side A, bottom half (y 120-239) = side B.
  Elements that belong to one end of the device (platform number, clock) sit on the OUTSIDE:
  left in the top half, right in the bottom half. Build the bottom half with several copy layers
  that swap the blocks; a single copy of the whole top half is only right for symmetric layouts.
- Flat look: solid colour areas, 2-5 strong colours, no gradients, shadows or photos; text >= 10 px.
- Everything that changes per train is a text layer with {{zug1.…}} placeholders, never baked into an image.

Workflow: look at 1-2 similar templates (list_templates, read_template, render_preview),
create_template, then after EVERY change call render_preview and compare with the reference.
Before finishing, test with long destination, delay and empty fields as described in the guide.
```

Tool-Beschreibungen sind englisch und enthalten die für das Tool relevanten Regeln
(z. B. `svg_to_png`: „no text“, `write_file`: Canvas-Regel, `render_preview`: „call after
every change“). Sie werden in dem Task geschrieben, der das Tool einführt, und stehen
wörtlich im Plan.

### Starter-Template

`internal/editor/starter/template.yaml` zeigt künftig den gespiegelten Aufbau (ohne
Schrift, da der Starter keine Font-Datei mitbringt): dunkler Vollflächen-Hintergrund,
ein farbiges Gleis-Feld links (x 0–43), ein Inhaltsfeld rechts; die untere Hälfte
entsteht aus zwei `copy`-Layern (Inhalt → x 0, Gleis-Feld → x 196). Kommentare
erklären: „Text-Layer brauchen eine Schrift — per `fonts:` eintragen (Desktop-KI:
`add_font`)“. Die GUI nutzt denselben Starter; das ist gewollt.

## Clients und Einrichtung (T9)

- **Claude Desktop** (`claude_desktop_config.json`):
  macOS `{"mcpServers":{"zza":{"command":"/Pfad/zza.app/Contents/MacOS/zza","args":["mcp"]}}}`,
  Windows `{"command":"C:\\Pfad\\zza.exe","args":["mcp"]}`. Templates-Ordner wird wie
  bei der GUI neben der App gefunden; `--templates-dir` optional.
- **Claude Code**: `claude mcp add zza -- /Pfad/zza mcp` (User-Scope) oder die
  Projektdatei `.mcp.json` im Repo-Root:
  `{"mcpServers":{"zza":{"command":"go","args":["run","./cmd/zza","mcp","--templates-dir","./templates"]}}}`.
  Geprüft: `go run ./cmd/zza` kompiliert trotz Wails (CGO, macOS) und startet aus dem
  Build-Cache in ≈ 0,5 s; der erste Kaltstart kann länger als das Standard-Timeout
  dauern → User-Guide nennt `MCP_TIMEOUT=60000` bzw. vorher einmal `go build ./cmd/zza`.
  Änderungen landen in `./templates` des Repos (git-Arbeitskopie).
- User-Guide-Abschnitt in `docs/user-guide-templates.md`: Einrichtung beider Clients,
  erste Schritte (Beispiel-Prompts), Fonts-Ordner, Foto-Ablauf, Parallelbetrieb,
  Grenzen, Sicherheit/Erlaubnis-Dialoge, macOS-Quarantäne (`xattr -dr com.apple.quarantine zza.app`).
- `CLAUDE.md`: Paket `internal/mcp`, `internal/watch` (DESKTOP-ONLY), Subcommand,
  `/edit/{t}/events`, `docs/ai-design-guide.md`.
- `build.sh`: README.txt im ZIP um `zza mcp` ergänzen; sonst keine Änderung (Wails baut
  denselben Main-Code).

## Risiken

- **Windows-GUI-Binary als stdio-Server:** Wails baut `zza.exe` als GUI-Subsystem.
  Von Claude Desktop mit umgeleiteten Pipes gestartet, sollten stdin/stdout funktionieren
  (`STARTF_USESTDHANDLES`); muss im manuellen Test bestätigt werden. Fallback: separates
  Konsolen-Binary `zza-mcp.exe` im ZIP (nur dann).
- **Long-Poll unter Wails:** Ein 25-s-Request blockiert nur seine eigene Goroutine; die
  Wails-Webview begrenzt parallele Requests nicht nennenswert. Manueller Test.
- **SDK-Entwicklung:** v1.8.0 ist fest gepinnt; Protokoll-Version wird vom SDK
  ausgehandelt.
- **oksvg** ist seit 2022 unverändert; reicht für Formen/Pfade/Verläufe. Bei Bedarf
  später `tdewolff/canvas`.
- **Qualität der KI-Ergebnisse** hängt vom Leitfaden ab; Praxistest (Konzept Schritt 2)
  nach T3.
