# Implementierungsplan

> Jede Phase endet mit einem manuellen Test. Erst nach OK geht es weiter.

---

## Agenten

Folgende spezialisierte Agenten werden beim Implementieren eingesetzt:

| Agent | Aufgabe |
|---|---|
| **implementer** | Schreibt Go-Code nach Plan |
| **security-reviewer** | Prüft jeden PR auf Sicherheitslücken (OWASP, Path Traversal, Injection, etc.) — Senior/Lead Level |
| **code-reviewer** | Prüft Code-Qualität, Struktur, Go-Idiome, Fehlerbehandlung |
| **template-porter** | Portiert PHP-Themes aus `legacy/` in YAML-Templates |
| **test-describer** | Erstellt die manuelle Testbeschreibung am Ende jeder Phase |

---

## Projektstruktur (Ziel)

```
zza-generate-images/
├── cmd/
│   └── zza/
│       └── main.go              # Einstiegspunkt (Server + CLI)
├── internal/
│   ├── renderer/                # YAML-Template laden + PNG rendern
│   │   ├── renderer.go
│   │   ├── template.go          # YAML-Struktur (meta, fonts, layers)
│   │   ├── evaluator.go         # Variablen, Filter, if/elif/else
│   │   └── cache.go             # Datei-Cache + Cleanup
│   ├── editor/                  # Editor-Backend
│   │   ├── editor.go
│   │   ├── auth.go              # Token-Generierung, E-Mail-Versand
│   │   └── sanitize.go          # Dateinamen-Bereinigung
│   ├── admin/                   # Superuser-Bereich
│   │   ├── admin.go
│   │   └── totp.go
│   ├── gallery/                 # Template-Galerie
│   │   └── gallery.go
│   ├── db/                      # SQLite-Zugriff
│   │   └── db.go
│   └── server/                  # HTTP-Router + Middleware
│       └── server.go
├── web/                         # Frontend-Assets
│   ├── gallery/                 # Galerie-UI
│   ├── editor/                  # Editor-UI (Vanilla JS + CodeMirror)
│   └── static/
├── templates/                   # YAML-Templates (portiert aus legacy/)
│   ├── sbb-096-v1/
│   ├── oebb-096-v1/
│   └── ...
├── docker-compose.yml
├── Dockerfile
├── go.mod
└── go.sum
```

---

## Phase 1 — Go-Projektgerüst & Renderer-Kern ✅

**Ziel:** Go-Modul aufsetzen, YAML-Template laden, einfaches PNG rendern.

### Aufgaben
1. `go.mod` initialisieren (`github.com/webfraggle/zza-generate-images`)
2. Abhängigkeiten einbinden:
   - ~~`github.com/golang/freetype`~~ → **`golang.org/x/image/font/opentype`** (unterstützt OTF + TTF; freetype nur TTF)
   - `golang.org/x/image` — Bildverarbeitung + Skalierung (CatmullRom)
   - `gopkg.in/yaml.v3` — YAML-Parsing
   - `github.com/spf13/cobra` — CLI (statt manuell)
   - SQLite → **nicht in Phase 1** (erst ab Phase 5)
3. YAML-Datenstruktur implementieren (`template.go`): `meta`, `fonts`, `layers`, `StringOrCond`
4. Layer-Rendering implementieren (`renderer.go`):
   - `type: image` — PNG/JPG einlesen, optional skalieren (CatmullRom)
   - `type: rect` — Rechteck zeichnen
   - `type: text` — Text mit OTF/TTF-Font rendern (`max_width`, `align`, `valign`, `width`, `height`)
   - `type: copy` — Bereich des Canvas kopieren (für gespiegelte Displays)
5. Variablen-Interpolation (`evaluator.go`): `{{zug1.zeit}}` aus JSON ersetzen
6. Sicherheits-Limits: `maxCanvasDimension=16384`, `maxLayers=256`, `maxFontFileBytes=50MB`
7. Path-Traversal-Schutz: `sanitize.go` mit `ValidateTemplateName` + `SafeTemplatePath`
8. CLI: `zza render -t <template> -i <input.json> -o <output.png>`

### Abweichungen vom ursprünglichen Plan
- Font-Library: `opentype` statt `freetype` — Legacy-Themes verwenden `.otf`, freetype unterstützt nur `.ttf`
- SQLite nicht in Phase 1 — erst in Phase 5 benötigt
- `StringOrCond`-Typ hinzugefügt: YAML-Felder können einfacher String oder `if/then/else`-Map sein
- Sicherheits-Ressourcenlimits und Path-Traversal-Schutz bereits in Phase 1 eingebaut (Security Review)

### Manueller Test (Phase 1)
Abgeschlossen ✅ — `go run ./cmd/zza render -t sbb-096-v1 -i templates/sbb-096-v1/default.json -o /tmp/out.png`

---

## Phase 2 — Filter, Bedingungen, Zeit & Rotation ✅

**Ziel:** Vollständiger Evaluator mit Filtern, if/elif/else, Zeitvariablen, Mathe-Filtern und Bild-Rotation.

### Aufgaben
1. Filter-Pipeline (`evaluator.go`):
   - `strip('x')`, `stripAll('x')`, `stripBetween('a','b')`
   - `upper`, `lower`, `trim`
   - `prefix('x')`, `suffix('x')`
   - Verkettung: `{{wert | strip('*') | upper}}`
2. Bedingungslogik:
   - Layer-Ebene: `if:` blendet ganzen Layer ein/aus
   - Eigenschafts-Ebene: `if/then/elif/then/else` für Farben, Werte etc.
3. Bedingungsfunktionen: `startsWith`, `endsWith`, `contains`, `isEmpty`, `equals`, `greaterThan`, `not`
4. Leere Felder: werden leer dargestellt, kein Fehler
5. **Systemvariablen Zeit** (`evaluator.go`):
   - `{{now}}` → aktuelle Uhrzeit als `HH:MM`
   - `{{now.hour}}`, `{{now.hour12}}`, `{{now.minute}}`, `{{now.second}}`
   - `{{now.day}}`, `{{now.month}}`, `{{now.year}}`, `{{now.weekday}}`
   - Filter `format('HH:mm')` für individuelle Formatierung
6. **Mathe-Filter** (`evaluator.go`):
   - `mul(x)`, `div(x)`, `add(x)`, `sub(x)`, `round`
   - Eingabe und Ausgabe als String — Konvertierung intern
   - Typischer Einsatz: `{{now.minute | mul(6)}}` → Winkel für Uhrzeiger
7. **Bild-Rotation** (`renderer.go`):
   - Neues Feld `rotate` auf `type: image` — Winkel in Grad
   - `pivot_x`, `pivot_y` — Drehmittelpunkt (Standard: Bildmitte)
   - `rotate` kann Variable/Ausdruck sein: `"{{now.minute | mul(6)}}"`
   - Rotation via `golang.org/x/image/draw` mit affiner Transformation

### Agenten
- **implementer**
- **security-reviewer** — besonderes Augenmerk auf Template-Injection
- **code-reviewer**

### Abweichungen vom Plan (Phase 2)
- `elif` wurde **nicht implementiert** — `StringOrCond` unterstützt nur `if/then/else` (einstufig). Wird in einer eigenen Aufgabe nachgezogen (siehe „elif-Erweiterung" nach Phase 6).

### Manueller Test (Phase 2)

6 manuelle Testfälle bestanden (2026-03-24):
1. `{{zug1.hinweis | strip('*')}}` — Präfix entfernen
2. `if: not(isEmpty(zug1.hinweis))` — Layer-Bedingung
3. `color: {if/then/else}` — bedingte Farbe
4. `{{now | format('HH:mm:ss')}}` — Zeitformatierung
5. `{{now.minute | mul(6)}}` — Mathe-Filter für Winkelberechnung
6. `rotate: "{{now.minute | mul(6)}}"` — Bild-Rotation mit Pivot

---

## Phase 3 — HTTP-Server & Render-Endpunkt ✅

**Ziel:** Go-HTTP-Server, Render-Route, Datei-Cache mit Cleanup.

### Aufgaben
1. HTTP-Router aufsetzen (`server.go`)
2. Route `POST /{template}/render` — JSON entgegennehmen, PNG zurückgeben
3. CORS-Middleware
4. Datei-Cache (`cache.go`):
   - SHA1-Hash des JSON als Dateiname
   - Cache-Hit: direkt ausliefern
   - Cleanup-Goroutine: läuft periodisch
     - Löscht Dateien älter als X (konfiguierbar)
     - Löscht älteste Dateien wenn Gesamtgröße > X MB (konfigurierbar)
5. Konfiguration via Umgebungsvariablen:
   - `CACHE_MAX_AGE_HOURS`
   - `CACHE_MAX_SIZE_MB`
   - `TEMPLATES_DIR`
   - `PORT`
6. Dockerfile + docker-compose.yml (Grundversion)

### Agenten
- **implementer**
- **security-reviewer** — Path Traversal in Template-Namen, Cache-Pfaden
- **code-reviewer**

### Abweichungen vom Plan (Phase 3)
- Cache-Key: SHA-256 statt SHA-1 (sicherer, kein Mehraufwand)
- Cache-Key inkludiert Template-Name (verhindert Cross-Template-Kollisionen)
- `RWMutex` statt `Mutex` im Cache (Get = RLock, Set/cleanup = Lock)
- `GET /health` Endpunkt ergänzt (nicht im Plan, aber nützlich)
- Port-Validierung in `config.ValidatePort` (1–65535)
- Non-root User im Dockerfile (`zza:1000`)
- `Content-Length` Header in PNG-Responses
- Path-Traversal via `../../` wird von Go's ServeMux bereinigt → 404 (nicht 400); sicher

### Manueller Test (Phase 3)

8 manuelle Testfälle bestanden (2026-03-24):
1. Server startet, Health-Check OK
2. POST /sbb-096-v1/render → 200, image/png, X-Cache: MISS
3. Zweiter gleicher Request → X-Cache: HIT
4. Ungültiger Template-Name (Grossbuchstaben) → 400
5. Path-Traversal `../../etc/passwd` → 404 (ServeMux bereinigt Pfad, kein Dateizugriff)
6. Unbekanntes Template → 404
7. Ungültiger JSON-Body → 400
8. CORS Preflight OPTIONS → 204, Access-Control-Allow-Origin: *

---

## Phase 4 — Template-Galerie & Ausprobiermodus ✅

**Ziel:** Öffentliche Web-UI zum Durchsuchen und Ausprobieren von Templates.

### Aufgaben
1. Route `GET /` — Galerie-Übersicht
   - Alle Templates aus `templates/` einlesen
   - Vorschaubild generieren (aus `default.json` des Templates)
   - Name + Beschreibung aus `meta` anzeigen
2. Route `GET /{template}` — Template-Detailseite mit Ausprobiermodus
   - Formular vorbelegt mit `default.json`
   - Live-Preview: Formular → `POST /{template}/render` → PNG anzeigen
3. `default.json` pro Template (flach im Verzeichnis)
4. Vanilla JS Frontend für Galerie + Ausprobiermodus

### Agenten
- **implementer**
- **security-reviewer**
- **code-reviewer**

### Abweichungen vom Plan (Phase 4)
- Static-Handler als Pre-Mux-Check implementiert (nicht via ServeMux-Route) — Go 1.22 ServeMux meldet Konflikt zwischen `GET /static/` und `GET /{template}/preview`
- `renderAndServe` als gemeinsame Pipeline für Preview- und Render-Handler (DRY)

### Manueller Test (Phase 4)

7 manuelle Testfälle bestanden (2026-03-24):
1. Galerie unter `/` zeigt Template-Karten mit Vorschaubild
2. Static-Assets (`/static/app.css`) → 200
3. Detail-Seite öffnet mit default.json vorausgefüllt
4. Live-Preview aktualisiert sich bei JSON-Änderung (debounced)
5. Ungültiges JSON zeigt Fehlermeldung
6. `GET /sbb-096-v1/preview` → 200, gültiges PNG
7. Unbekanntes Template → 404

---

## Phase 5 — Editor-Backend (Auth, Token, E-Mail)

**Ziel:** E-Mail-basierte Authentifizierung für Template-Editing.

### Aufgaben
1. SQLite-Schema (`db.go`):
   - `templates` — id, name, email_hash, created_at
   - `edit_tokens` — token, template_id, expires_at, used
2. Route `GET /{template}/edit` — Einstieg Editor
   - Wenn Template neu: Formular Name + E-Mail eingeben
   - Wenn Template existiert: E-Mail-Eingabe → Token versenden
3. Token-Generierung (`auth.go`):
   - Kryptografisch sicherer Zufallstoken (32 Byte, hex-kodiert)
   - Gültigkeitsdauer: konfigurierbar (Standard: 24h)
   - Token ist an Template-ID gebunden — serverseitig geprüft
   - E-Mail wird als Hash gespeichert (nicht im Klartext)
4. E-Mail-Versand via SMTP:
   - Konfiguration: `SMTP_HOST`, `SMTP_PORT`, `SMTP_USER`, `SMTP_PASS`, `SMTP_FROM`
5. Rate Limiting: max. X Token-Anfragen pro E-Mail/Stunde
6. Route `GET /edit/{token}` — Token validieren → Editor öffnen
7. Route `POST /edit/{token}/save` — Änderungen speichern

### Agenten
- **implementer**
- **security-reviewer** — Token-Sicherheit, Rate Limiting, E-Mail-Hash, Path Traversal beim Speichern
- **code-reviewer**

### Status: ✅ Abgeschlossen

**Implementiert:**
- `internal/db/db.go` — SQLite-Schema mit `templates` und `edit_tokens`, `SetMaxOpenConns(1)`
- `internal/editor/auth.go` — `HashEmail` (HMAC-SHA256), `GenerateToken` (32 Byte / 64-Hex), `RequestToken`, `ValidateToken`
- `internal/editor/mailer.go` — SMTP-Versand mit optionalem Auth-Skip
- `internal/editor/auth_test.go` — 9 Tests (deterministisch, Rate-Limit, Expiry, etc.)
- `internal/server/editor_handlers.go` — `GET /{template}/edit`, `POST /{template}/edit`, `GET /edit/{token}`, `POST /edit/{token}/save` (501 Stub)
- `cmd/zza/main.go` — DB-Öffnung, ephemere HMAC-Warnung, `RegisterEditorRoutes`

**Security Review:** BEDINGT OK
- **M1 (akzeptiert):** Rate-Limit ist per-Template, nicht per-IP — akzeptables Risiko für Intranet-Deployment
- **M2 (behoben):** Dev-Log zeigt vollständigen Token mit explizitem `[DEV]`-Prefix und `//nolint`-Kommentar

**Code Review:** APPROVED WITH MINOR COMMENTS — keine blockers, Tech-Debt in Phase 6 addressieren

**Nachträgliche Änderung:** E-Mail-Adressen werden im Klartext gespeichert (statt HMAC-Hash) — HMAC verursachte permanente Lockouts nach Neustarts ohne persistenten Secret. E-Mail-Vergleich ist case-insensitiv.

**User-OK:** 2026-03-25 — inkl. SMTP-Versand manuell getestet ✅

### Manueller Test (Phase 5)

**Voraussetzungen:** Server läuft ohne SMTP-Config (`SMTP_HOST` nicht gesetzt)

```sh
# 1. Erstes Edit-Request — Formular anzeigen (neues Template)
curl -s http://localhost:8080/default/edit | grep -o "<title>[^<]*"

# 2. Token anfordern — Dev-Log beobachten
curl -s -X POST http://localhost:8080/default/edit -d "email=test@example.com" -L

# 3. Dev-Log-Ausgabe enthält: [DEV] edit link for "default": http://localhost:8080/edit/<token>
#    → Token aus Log kopieren und aufrufen:
# curl -s http://localhost:8080/edit/<token> | grep "authentifiziert"

# 4. Falsches Token → 401
curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/edit/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa

# 5. Gleiche E-Mail → neues Token (kein Fehler)
curl -s -X POST http://localhost:8080/default/edit -d "email=test@example.com" -o /dev/null -w "%{http_code}"

# 6. Falsche E-Mail → Fehlermeldung im HTML
curl -s -X POST http://localhost:8080/default/edit -d "email=wrong@example.com" | grep "nicht als Besitzer"

# 7. Rate-Limit: 3 Requests in Folge → 4. Anfrage zeigt Fehler
for i in 1 2 3 4; do curl -s -X POST http://localhost:8080/default/edit -d "email=test@example.com" | grep -o "Zu viele\|gültig und du"; done
```

---

## Phase 6 — Editor-Frontend

**Ziel:** Web-UI für den Template-Editor.

### Aufgaben
1. Editor-Layout (Vanilla JS):
   - Linke Spalte: Dateiliste (Assets des Templates)
   - Mitte: YAML-Editor (CodeMirror mit YAML-Syntax-Highlighting)
   - Rechte Spalte oben: Zug-JSON Testfeld
   - Rechte Spalte unten: PNG-Preview
2. Datei-Upload für Assets (Bilder, Fonts)
   - Erlaubte Typen: `.png`, `.jpg`, `.ttf`, `.otf`
   - Dateinamen werden automatisch sanitized
   - Max. Dateigröße: konfigurierbar
3. Datei löschen (nur eigene Template-Dateien)
4. `default.json` editierbar im Editor
5. Auto-Preview: bei Änderung im YAML oder JSON → neue Render-Anfrage

### Agenten
- **implementer**
- **security-reviewer** — File Upload, Dateitype-Whitelist, Dateinamen-Sanitizing, Token-Prüfung bei jedem Request
- **code-reviewer**

### Status: ✅ Abgeschlossen

**Implementiert:**
- `internal/editor/files.go` — ListFiles, ReadTextFile, WriteTextFile, UploadFile, DeleteFile mit Path-Traversal-Schutz, Typ-Whitelist (.yaml/.json editierbar, .png/.jpg/.ttf/.otf uploadbar), atomischem Write (temp+rename), 10 MiB Upload-Limit
- `internal/editor/starter/template.yaml` + `default.json` — Starter-Dateien als go:embed, direkt im Quellcode editierbar
- `editor_handlers.go` — GET /files, GET /file/{name}, POST /save, POST /upload, DELETE /file/{name}, alle mit requireToken
- `server.go` — template.yaml Mod-Time im PNG-Cache-Key für automatische Cache-Invalidierung
- `edit-editor.html` — 3-Spalten-Layout (Dateiliste | CodeMirror 6 YAML/JSON | Test-JSON + Preview), Tab-Indent via DOM-Event, Cmd+S/Ctrl+S Speichern, Blob-URL-Revocation
- Render-Fehler (YAML-Parse, fehlende Fonts etc.) werden direkt im Preview-Bereich angezeigt

**Fixes während Entwicklung:**
- CodeMirror duplicate-state Fehler: EditorState-Import entfernt, EditorView direkt mit doc+extensions
- `codemirror@6.0.1` exportiert keymap/indentWithTab nicht — Tab via DOM keydown Handler gelöst
- Preview aktualisiert sich nach jedem Save (nicht nur default.json)
- default.json und template.yaml sind schreibgeschützt (nicht löschbar)

**Security Review:** BEDINGT OK — alle Findings behoben (Upload-Fehler 500 statt 400, loadFiles-Fehler mit User-Feedback, Blob-URL Revocation)
**Code Review:** APPROVED WITH MINOR COMMENTS

**User-OK:** 2026-03-25 ✅

### Manueller Test (Phase 6)

1. Editor-Link öffnen → 3-Spalten-Layout, template.yaml öffnet sich automatisch
2. Dateiliste — alle Template-Dateien aufgelistet, default.json und template.yaml ohne Löschen-Button
3. Datei öffnen — default.json klicken → öffnet im Editor
4. Preview — default.json wird automatisch geladen, Preview-Bild erscheint rechts
5. YAML editieren & Cmd+S → "Gespeichert ✓", Preview aktualisiert sich
6. Datei hochladen — .png/.jpg/.ttf/.otf via + Button
7. Datei löschen — hochgeladene Datei mit × löschen
8. YAML-Fehler einbauen → Fehlermeldung erscheint in roter Box im Preview-Bereich
9. Falsches Token → 401

---

## elif-Erweiterung — Mehrstufige Bedingungen im Evaluator

**Ziel:** `if/elif/else` in YAML-Eigenschafts-Werten vollständig unterstützen (war in Phase 2 geplant, aber nicht implementiert).

### Hintergrund

`StringOrCond` in `internal/renderer/template.go` unterstützt derzeit nur einstufiges `if/then/else`. Für komplexere Templates (z. B. Farb-Auswahl mit mehr als zwei Zuständen) wird eine `elif`-Kette benötigt:

```yaml
color:
  if:   equals(status, 'delayed')
  then: '#FF0000'
  elif: equals(status, 'cancelled')
  then: '#888888'
  else: '#FFFFFF'
```

### Aufgaben

1. **`internal/renderer/template.go`** — `condMap` auf Slice umstellen:
   ```go
   type condMap struct {
     branches []condBranch  // if/elif-Paare
     els      string
   }
   type condBranch struct {
     ifExpr string
     then   string
   }
   ```
2. **`UnmarshalYAML`** — wiederholte `elif`/`then`-Schlüssel als Kette parsen (YAML-Reihenfolge via `yaml.Node` erhalten)
3. **`Resolve()`** — Branches der Reihe nach auswerten, erstes `true` gewinnt
4. **Tests** — `elif`-Ketten mit 0, 1 und N `elif`-Zweigen

### Agenten
- **implementer**
- **security-reviewer** — Template-Injection durch neue Ausdruck-Zweige
- **code-reviewer**

### Status: ✅ Abgeschlossen

**Implementiert:**
- `condBranch`-Typ + `condMap.branches []condBranch` in `template.go`
- `UnmarshalYAML` parst `if`/`elif`/`then`/`else` via `yaml.Node.Content` (duplicate-key-sicher)
- `Resolve()` iteriert Branches, erstes `true` gewinnt
- DoS-Schutz: `maxCondBranches = 50`
- Validierung: `then` ohne `if`/`elif` und `if`/`elif` ohne `then` geben Fehler zurück
- 11 neue Tests (inkl. 3 Fehlertests)

**Security Review:** APPROVED WITH MINOR COMMENTS — alle Findings behoben
**Code Review:** APPROVED WITH MINOR COMMENTS — alle Findings behoben

### Manueller Test (elif-Erweiterung)

3 manuelle Testfälle bestanden (2026-03-26):
1. `if`-Branch (`status=delayed`) → roter Hintergrund
2. `elif`-Branch (`status=cancelled`) → grauer Hintergrund
3. `else`-Branch (`status=on-time`) → grüner Hintergrund

---

## loop-Erweiterung — `type: loop` + `split_by` im Renderer

**Ziel:** Via-Stationen (und ähnliche Listen) als gesplitteten String iterieren und pro Element Sub-Layer rendern.

### Aufgaben
1. `Layer`-Struct: neues Feld `Layers []Layer` (Sub-Layer), `SplitBy string`, `StepY int`, `MaxItems int`, `Var string`
2. Renderer (`render.go`): `type: loop` auswerten — String splitten, pro Element Sub-Layer mit angepasstem Y-Offset und Loop-Variable im Scope rendern
3. Evaluator: Loop-Variablen (`item`, `loop.index`, `loop.y`) in den Scope aufnehmen
4. Spec: `type: loop` ist bereits dokumentiert (`yaml-template-spec.md`)
5. Tests: leerer String, ein Element, N Elemente, `max_items`-Limit

### Agenten
- **implementer**
- **security-reviewer**
- **code-reviewer**

### Status: Implementierung ausstehend

### Manueller Test (loop-Erweiterung)
> Beschreibung folgt am Ende der Erweiterung.

---

## Phase 7 — Superuser-Bereich

**Ziel:** Admin-Zugang mit Token + TOTP.

### Aufgaben
1. TOTP-Setup (`totp.go`):
   - Beim ersten Start: TOTP-Secret generieren, QR-Code ausgeben (Terminal oder Setup-Route)
   - Secret wird in Umgebungsvariable / `.env` gespeichert
2. Admin-Auth-Flow:
   - `POST /admin/login` — Admin-Token + TOTP-Code prüfen
   - Session via kurzlebigem Cookie (kein dauerhafter State)
3. Admin-Routen (nur mit gültiger Session):
   - `GET /admin` — Übersicht aller Templates
   - `GET /admin/{template}` — Template öffnen (wie Editor, aber ohne Token-Flow)
   - `DELETE /admin/{template}` — Template löschen
   - `GET /admin/cache` — Cache-Status, manuelles Leeren
4. Umgebungsvariablen: `ADMIN_TOKEN`, `TOTP_SECRET`

### Agenten
- **implementer**
- **security-reviewer** — TOTP-Implementierung, Session-Sicherheit, Brute-Force-Schutz
- **code-reviewer**

### Status: ✅ Abgeschlossen — User-OK 2026-03-26

**Implementiert:**
- `internal/admin/totp.go` — RFC 6238 TOTP, Replay-Guard, `GenerateSecret`, `OTPAuthURL`
- `internal/admin/session.go` — `SessionStore` (8h TTL), `LoginLimiter` (5/15min), `TOTPReplayGuard`
- `internal/server/admin_handlers.go` — alle Admin-Handler, `requireSession`, constant-time Token-Vergleich
- `internal/server/cache.go` — `Stats()` + `Flush()`
- 4 Admin-HTML-Templates (login, overview, cache, editor)
- `ADMIN_TOKEN`, `TOTP_SECRET`, `SECURE_COOKIES` Env-Vars
- `zza totp-setup` CLI-Command

**Security Review:** CHANGES REQUIRED → alle Findings behoben
**Code Review:** APPROVED WITH MINOR COMMENTS → alle Findings behoben

### Manueller Test (Phase 7)

8 manuelle Testfälle bestanden (2026-03-26):

1. **Admin deaktiviert ohne Env-Vars** — Server startet, `/admin` antwortet nicht (404)
2. **totp-setup** — `zza totp-setup` gibt Secret + otpauth://-URL aus; QR-Code in Authenticator-App scannen
3. **Login-Seite** — GET `/admin/login` → Formular mit Token- und TOTP-Feld
4. **Login mit falschem Token** — POST → Fehlermeldung „Falscher Token oder TOTP-Code."
5. **Login mit korrekten Credentials** — POST → Redirect auf `/admin`, Session-Cookie gesetzt
6. **Übersicht** — `/admin` zeigt Template-Liste mit Edit-Links
7. **Template-Editor** — `/admin/{name}` öffnet CodeMirror-Editor (wie User-Editor)
8. **Rate-Limiting** — 5 Fehlversuche → 429; Note: In-Memory-Limiter, Server-Neustart für Test nötig

**User-OK:** 2026-03-26 ✅

---

## Phase 8 — Template-Portierung (legacy → YAML)

**Ziel:** Alle 14 bestehenden PHP-Themes als YAML-Templates neu erstellen.

### Aufgaben
Pro Theme (Reihenfolge nach Komplexität):
1. `sbb-096-v1`
2. `sbb-105-v1`
3. `oebb-096-v1`
4. `oebb-105-v1`
5. `rhb-096-v1`
6. `rhb-105-v1`
7. `umuc-096-v1`
8. `umuc-105-v1`
9. `nederland-096-v1`
10. `nederland-105-v1`
11. `faltblatt`
12. `faltblatt-105-v1`
13. `streamdeck-v1`
14. `instafollower`

Vorgehen pro Theme:
- PHP-Logik aus `legacy/` analysieren
- YAML-Template schreiben
- Assets (PNG, Fonts) aus `legacy/` übernehmen
- Mit `default.json` aus `legacy/` testen

### Agenten
- **template-porter** portiert je Theme
- **implementer** ergänzt fehlende Renderer-Features falls nötig
- **security-reviewer** prüft ob neue Template-Features Risiken einführen

### Manueller Test (Phase 8)
> Visueller Vergleich: jedes neue YAML-Template gegen das alte PHP-Rendering.

---

## Phase 9 — Cross-Platform Binaries & Docker Finalisierung

**Ziel:** Release-Build für alle Plattformen, Docker Compose produktionsreif.

### Aufgaben
1. Build-Script / Makefile:
   - `make build-linux` → Docker-Image
   - `make build-windows` → `zza.exe`
   - `make build-macos` → `zza` (arm64 + amd64)
2. Docker Compose finalisieren:
   - Volume-Mounts für `templates/`, Cache, SQLite
   - `.env`-Datei für alle Konfigurationsvariablen
   - Ressourcen-Limits (CPU, RAM)
3. GitHub Actions CI: Build + Security-Scan bei jedem Push auf `develop`

### Agenten
- **implementer**
- **security-reviewer** — Docker-Config, exposed Ports, Volume-Permissions

### Manueller Test (Phase 9)
> Beschreibung folgt am Ende der Phase.

---

## Phase 10 — Frontend-Design & UX-Optimierungen

**Ziel:** _(noch zu planen — nach MVP-Launch)_

### Status: Planung ausstehend

---

## Reihenfolge & Abhängigkeiten

```
Phase 1 (Renderer-Kern)
  └── Phase 2 (Filter + Bedingungen)
        └── Phase 3 (HTTP-Server + Cache)
              ├── Phase 4 (Galerie + Ausprobiermodus)
              ├── Phase 5 (Editor-Backend)
              │     └── Phase 6 (Editor-Frontend)
              └── Phase 7 (Superuser)
Phase 8 (Template-Portierung) — parallel ab Phase 3 möglich
Phase 9 (Binaries + Docker) — MVP-Launch
Phase 10 (Frontend-Design & UX) — nach MVP-Launch, Planung ausstehend
```

---

## Konfigurationsübersicht (Umgebungsvariablen)

| Variable | Beschreibung | Standard |
|---|---|---|
| `PORT` | HTTP-Port | `8080` |
| `TEMPLATES_DIR` | Pfad zum Templates-Verzeichnis | `./templates` |
| `CACHE_DIR` | Pfad zum Cache-Verzeichnis | `./cache` |
| `CACHE_MAX_AGE_HOURS` | Max. Alter von Cache-Dateien | `24` |
| `CACHE_MAX_SIZE_MB` | Max. Gesamtgröße Cache | `500` |
| `DB_PATH` | Pfad zur SQLite-Datei | `./data/zza.db` |
| `SMTP_HOST` | SMTP-Server | — |
| `SMTP_PORT` | SMTP-Port | `587` |
| `SMTP_USER` | SMTP-Benutzername | — |
| `SMTP_PASS` | SMTP-Passwort | — |
| `SMTP_FROM` | Absender-Adresse | — |
| `EDIT_TOKEN_TTL_HOURS` | Gültigkeit Editier-Links | `24` |
| `RATE_LIMIT_EMAIL_PER_HOUR` | Max. Token-Anfragen pro E-Mail/h | `5` |
| `UPLOAD_MAX_SIZE_MB` | Max. Dateigröße Upload | `10` |
| `ADMIN_TOKEN` | Langer Admin-Token | — |
| `TOTP_SECRET` | TOTP-Secret (Base32) | — |
| `BASE_URL` | Öffentliche URL (für E-Mail-Links) | — |
