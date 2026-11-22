# Dual-Build-Architektur: Server + Desktop-Exe

**Datum:** 2026-04-22
**Status:** Design abgeschlossen, bereit für Implementierungsplan

## Motivation

Der aktuelle Online-Server hat Editor-Funktionen mit E-Mail-basierter Authentifizierung, Admin-Bereich und SQLite-DB. Das bringt Pflichten in Sachen Rechte, Datenschutz und Sicherheit mit sich, die wir vermeiden wollen. Die Lösung: Editor-Funktionen komplett von der Online-Präsenz abziehen und nur noch in einer lokal laufenden Desktop-Exe anbieten.

## Zielbild

Zwei Build-Artefakte aus derselben Codebase:

| Build | Zweck | Auslieferung |
|---|---|---|
| `cmd/zza-server` | Preview + Render-Endpoint für Microcontroller | Docker-Image auf VM |
| `cmd/zza` | Editor + Preview + Render, lokal auf User-Rechner | ZIP mit Binary + Beispiel-Templates |

**Template-Fluss:** User erstellt Templates lokal mit der Desktop-Exe, schickt sie (manuell gezippt) dem Admin per E-Mail. Admin prüft und lädt per SCP/rsync auf den Server. Keine offene Upload-API, kein Server-seitiger Auth.

## Server-Build (`cmd/zza-server`)

### Routen
- `GET /` — Galerie (Template-Liste mit Thumbnails)
- `GET /{template}` — Preview-Seite mit Meta-Info, Live-JSON-Editor, Render-URL, PNG-Download, **ZIP-Download-Button**. **Kein Edit-Button.**
- `POST /{template}/render` — JSON → PNG, die einzige Route ohne HTTPS-Redirect (Microcontroller ohne TLS)
- `GET /{template}.zip` — streamt das Template-Verzeichnis als ZIP (Endnutzer kann's lokal im Editor öffnen und anpassen)
- `GET /health` — Health-Check

### Config (Umgebungsvariablen)
- **Bleiben:** `PORT`, `TEMPLATES_DIR`, `CACHE_DIR`, `CACHE_MAX_AGE_HOURS`, `CACHE_MAX_SIZE_MB`, `BASE_URL`
- **Ersatzlos raus:** `DB_PATH`, `SMTP_HOST`, `SMTP_PORT`, `SMTP_USER`, `SMTP_PASS`, `SMTP_FROM`, `SECURE_COOKIES`, `EDIT_TOKEN_TTL_HOURS`, `ADMIN_TOKEN`, `TOTP_SECRET`

### Entfernte Infrastruktur
- SQLite-Datenbank komplett weg — die gespeicherten `templates`- und `edit_tokens`-Tabellen sind rein Editor-Kram.
- E-Mail-Versand (Edit-Link-Request) weg.
- Admin-Bereich (`/admin`, Token + TOTP) weg.
- HTTPS-Redirect-Middleware bleibt — alle Routen außer `/render` werden auf HTTPS umgeleitet.

## Desktop-Build (`cmd/zza`)

### Subcommands (Cobra-CLI)
```
zza                         # Default: Wails-Fenster + lokaler Webserver
zza serve [--port N]        # nur Webserver, kein Fenster
zza render <tmpl> <json> <out.png>   # unveränderter Render-CLI-Modus
zza version
```

### Routen (zusätzlich zum Server-Set)
- `GET /edit/{template}` — YAML-Editor + Node-Editor + Live-Preview. **Kein Auth-Check.**
- `POST /edit/{template}/save` — speichert `template.yaml` zurück aufs Dateisystem, invalidiert Render-Cache
- Weitere Editor-Endpunkte (Asset-Upload, Template anlegen, etc.) — alle ohne Token-Flow
- Preview-Seite zeigt **Edit-Button** (zusätzlich zum ZIP-Download-Button), der auf `/edit/{template}` verlinkt.

### HTTPS-Redirect
Die HTTPS-Redirect-Middleware aus `internal/server/` wird im Desktop-Build nicht angewickelt — Desktop läuft auf `127.0.0.1` ohne Reverse-Proxy, ein Redirect-Pfad ergibt dort keinen Sinn.

### Umsetzungsdetail: gemeinsames Preview-Template
Das HTML-Template für die Preview-Seite kriegt eine Bool-Variable `EditorEnabled`. Server-Build setzt `false`, Desktop-Build `true`. Ein einziges Template, kein Duplikat.

### Templates-Ordner-Resolution (portable)
- **Windows:** `<exe-dir>/templates/`
- **macOS als `.app`-Bundle:** `<Bundle>.app/../templates/` (Geschwister-Ordner neben dem App-Bundle, nicht innerhalb)
- **macOS nackte Binary:** `<exe-dir>/templates/`
- **Ordner fehlt:** beim ersten Start anlegen und mit Default-Starter-Template befüllen
- **Override:** `--templates-dir` Flag

### Wails-Integration (v2, stable)
1. Lokalen HTTP-Server auf Port 0 starten (OS wählt freien Port)
2. Wails-Fenster öffnet `http://127.0.0.1:<port>/`
3. Frontend ist unverändert unser bestehendes Vanilla-JS + HTML
4. **Fallback:** Wenn Wails-Init scheitert (Windows 10 ohne WebView2, exotisches Linux) → öffnen des Default-Browsers via `open` (macOS) / `start` (Windows)
5. Server läuft solange Fenster offen ist bzw. Ctrl+C im Terminal

## Code-Struktur nach Umbau

```
cmd/
  zza-server/main.go      # Server-Binary
  zza/main.go             # Desktop-Binary (Flagship)

internal/
  renderer/               # SHARED
  config/                 # SHARED (Server + Desktop lesen unterschiedliche Keys)
  version/                # SHARED
  cli/                    # DESKTOP-ONLY (Render-CLI)
  server/                 # SHARED (Router, Gallery-Handler, Preview-Handler,
                          #         Render-Handler, HTTPS-Redirect, Cache)
  gallery/                # SHARED
  editor/                 # DESKTOP-ONLY (YAML-Editor, Template-CRUD;
                          #               Auth/Token/E-Mail-Code ausgebaut)
  desktop/                # DESKTOP-ONLY (Wails-Bootstrap, Browser-Fallback,
                          #               Templates-Ordner-Resolution)

  admin/                  # GELÖSCHT
  db/                     # GELÖSCHT
```

**Trennung via Go-Imports, keine Build-Tags.** Server-Main importiert nur server+gallery+renderer+config+version; Desktop-Main importiert alles inklusive editor+desktop+cli. Keine Dead-Code-Branches, kein Feature-Flag-Mechanismus nötig.

## Datenflüsse

### Render (beide Builds, identisch)
```
Client → POST /{template}/render + JSON
      → Template aus TEMPLATES_DIR laden
      → YAML parsen, gegen JSON rendern
      → Cache-Hit? PNG zurück
      → sonst: rendern, cachen, PNG zurück
```

### Editor (nur Desktop)
```
Browser → GET /edit/{template}
       → template.yaml + default.json vom Dateisystem lesen
       → YAML-Editor + Node-Editor + Live-Preview rendern
POST /edit/{template}/save
       → YAML validieren (Parse-Check)
       → template.yaml zurück schreiben
       → Render-Cache für dieses Template invalidieren
```

### Template-Sharing (außerhalb der App)
User zippt `templates/mein-template/` manuell (oder nutzt den ZIP-Download-Button auf der Preview-Seite als Startpunkt) und schickt's per E-Mail an Admin. Admin entpackt nach Prüfung in Server-`TEMPLATES_DIR` und deployt per Docker-Compose-Reload.

### ZIP-Download (beide Builds)
Route `GET /{template}.zip` streamt das komplette Template-Verzeichnis als ZIP (`template.yaml` + `default.json` + alle Assets/Fonts). Kein Cache nötig — die ZIP wird on-the-fly gestreamt; Request-Volumen ist niedrig. Implementierung in `internal/server/` als Handler, shared von beiden Builds.

## Fehlerbehandlung

- **Templates-Ordner fehlt (Desktop)** → einmalig anlegen + Default-Template einkopieren; nicht abstürzen
- **Port belegt (Desktop)** → Port 0 → OS wählt freien; kein Fehler möglich
- **WebView2 fehlt (Windows 10)** → Wails-Init wirft Fehler, wird gefangen; Default-Browser öffnen. Hinweistext im Terminal: "Kein Webview verfügbar, öffne Browser."
- **Invalid YAML beim Save** → HTTP 400 mit Fehlermeldung; Datei wird NICHT überschrieben
- **Server-Build bekommt Editor-Request** → Route existiert nicht, 404. Kein Code-Pfad, der versehentlich Editor-Logik ausführt.

## Testing-Strategie

- Renderer-Unit-Tests bleiben (shared, keine Änderung)
- Server-Integrationstests: neu dass `/edit/*` und `/admin/*` im Server-Build 404 liefern
- Desktop-Integrationstests: lokaler Server startet, lädt Template, Render funktioniert (headless, ohne Wails-Fenster)
- Node-Editor-Tests (JS) bleiben unverändert
- Wails-Fenster-Tests: verzichten; manueller Smoke-Test auf macOS + Windows reicht

## Plattformen & Distribution

### Build-Matrix
- Windows x64
- macOS Intel (x86_64)
- macOS Apple Silicon (arm64)

### Signierung
Kein Code-Signing im MVP (Null-Budget-Option). User klicken die Erst-Start-Warndialoge durch. README erklärt das.

### Release-Artefakte
- `zza-windows-x64.zip`
- `zza-macos-intel.zip`
- `zza-macos-arm64.zip`
- `zza-server:<version>` Docker-Image (wie heute)

**ZIP-Inhalt (Desktop):**
- Binary (bzw. `.app`-Bundle auf macOS)
- **Kompletter `templates/`-Ordner** (alle kuratierten Templates aus dem Repo, nicht nur `default/`)
- `README.txt` mit:
  - Erst-Start-Anleitung auf Windows ("Weitere Informationen → Trotzdem ausführen")
  - Erst-Start-Anleitung auf macOS (Rechtsklick → Öffnen)
  - Hinweis auf Templates-Ordner neben dem Binary

**Auto-Update:** Explizit YAGNI — User lädt neuere Version von GitHub Releases.

### Build-Skript
`build.sh` wird erweitert:
- Bisher: Docker-Image für Server (unverändert bleiben)
- Neu: `wails build -platform windows/amd64`, `darwin/amd64`, `darwin/arm64`
- Windows-Cross-Compile vom Mac mit `mingw-gcc`; macOS-Builds benötigen macOS-Host (gegeben, da Entwicklung auf macOS)
- Version via ldflags wie bisher

## Umsetzungs-Reihenfolge (Big-Bang auf Branch `feature/dual-build`)

1. **Code umziehen** — `cmd/zza` → `cmd/zza-server`; alter `cmd/zza-desktop` wegwerfen; neuer `cmd/zza` mit Cobra-CLI (`serve`, `render`, `version`, Default=GUI)
2. **Server entschlacken** — editor/admin/db-Imports aus `internal/server/` und `cmd/zza-server` raus; `internal/admin/` und `internal/db/` Verzeichnisse löschen; Config-Keys raus; Preview-Template `EditorEnabled=false`
3. **Editor auth-frei machen** — Token/E-Mail-Flow aus `internal/editor/` rausreißen; Handler laufen ohne Middleware; `/edit/{template}` lädt direkt vom Dateisystem
4. **Wails integrieren** — `internal/desktop/` mit Wails-Bootstrap, Browser-Fallback, Templates-Ordner-Resolution; Preview-Template `EditorEnabled=true` im Desktop-Build
5. **Build-Skripte** — `build.sh` erweitert um Wails-Builds; Release-ZIPs bauen
6. **Manuelle Tests** — Server-Preview/Render-Test; Desktop-GUI-Start auf macOS; Windows-Test (Parallels/VM); Portable-ZIP entpacken + starten
7. **Deploy** — neuer Server-Build auf VM (alte SQLite-DB kann dort ignoriert/später gelöscht werden); GitHub-Release mit Desktop-Artefakten

## Ersatzlos gelöscht

- `internal/admin/`, `internal/db/`
- `zza.db` aus Repo und vom Server
- E-Mail-HTML-Templates in `web/templates/email-*.html`
- Admin-/Token-Handler in `internal/server/`
- SMTP/DB/Admin-Umgebungsvariablen aus Config
