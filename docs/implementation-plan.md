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

## Phase 1 — Go-Projektgerüst & Renderer-Kern

**Ziel:** Go-Modul aufsetzen, YAML-Template laden, einfaches PNG rendern.

### Aufgaben
1. `go.mod` initialisieren (`github.com/webfraggle/zza-generate-images`)
2. Abhängigkeiten einbinden:
   - `github.com/golang/freetype` — TrueType Font Rendering
   - `golang.org/x/image` — Bildverarbeitung
   - `gopkg.in/yaml.v3` — YAML-Parsing
   - `github.com/mattn/go-sqlite3` — SQLite
3. YAML-Datenstruktur implementieren (`template.go`): `meta`, `fonts`, `layers`
4. Layer-Rendering implementieren (`renderer.go`):
   - `type: image` — PNG einlesen und platzieren
   - `type: rect` — Rechteck zeichnen
   - `type: text` — Text mit TrueType-Font rendern (inkl. `max_width`, `align`)
   - `type: copy` — Bereich des Canvas auf andere Position kopieren (für gespiegelte Displays)
5. Variablen-Interpolation (`evaluator.go`): `{{zug1.zeit}}` aus JSON ersetzen
6. Einfaches CLI: `zza render --template X --input X --output X`

### Agenten
- **implementer** schreibt den Code
- **security-reviewer** prüft evaluator.go (Injection-Risiko)
- **code-reviewer** prüft Gesamtstruktur

### Manueller Test (Phase 1)
> Beschreibung folgt am Ende der Phase vom **test-describer** Agenten.

---

## Phase 2 — Filter, Bedingungen, Zeit & Rotation

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

### Manueller Test (Phase 2)
> Beschreibung folgt am Ende der Phase.

---

## Phase 3 — HTTP-Server & Render-Endpunkt

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

### Manueller Test (Phase 3)
> Beschreibung folgt am Ende der Phase.

---

## Phase 4 — Template-Galerie & Ausprobiermodus

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

### Manueller Test (Phase 4)
> Beschreibung folgt am Ende der Phase.

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

### Manueller Test (Phase 5)
> Beschreibung folgt am Ende der Phase.

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

### Manueller Test (Phase 6)
> Beschreibung folgt am Ende der Phase.

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

### Manueller Test (Phase 7)
> Beschreibung folgt am Ende der Phase.

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
Phase 9 (Binaries + Docker) — am Ende
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
