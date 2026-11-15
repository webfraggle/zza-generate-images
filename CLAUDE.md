# CLAUDE.md

Hinweise für Claude Code beim Arbeiten an diesem Repo.

## Architektur

Dual-Build aus einer Go-Codebase:

| Build | Zweck | Auslieferung |
|---|---|---|
| `cmd/zza-server` | Galerie + Vorschau + Render-Endpoint + ZIP-Download | Docker-Image auf VM (gen.yuv.de) |
| `cmd/zza` | Editor + Vorschau + Render + native GUI (Wails v2) | ZIPs für macOS arm64/intel + Windows x64 |

Server-Build ist CGO-frei, schlank, ohne Auth/DB/SMTP. Er darf `internal/mcp`, `internal/watch`, fsnotify, oksvg und das MCP-SDK nicht importieren (Prüfung: `go list -deps ./cmd/zza-server | grep -E 'fsnotify|modelcontextprotocol|oksvg|rasterx|internal/mcp|internal/watch'` muss leer sein). Desktop-Build bringt den Editor zurück, ohne Token-Flow — bindet auf `127.0.0.1`.

**Routen Server:** `GET /`, `GET /{template}`, `GET /{template}/preview`, `POST /{template}/render` (jeweils `?format=gif` für animiertes GIF, Standard PNG), `GET /{template}.zip`, `GET /health`.

**Routen Desktop (zusätzlich):** `GET /edit/{template}`, `GET/POST /edit/{template}/files|file/{name}|save|upload`, `DELETE /edit/{template}/file/{name}`, `GET /edit/{template}/events` (Long-Poll für externe Dateiänderungen, 25 s).

**Desktop-Subcommands:** `serve`, `render`, `version`, `mcp` (MCP-Server über stdio für Claude Desktop / Claude Code, 19 Tools; stdout gehört nur dem Protokoll, Logs über `log` nach stderr). Anleitung: `docs/user-guide-templates.md` → „KI-Assistent (MCP)"; Client-Config im Repo: `.mcp.json`.

**HTTPS-Redirect:** Alle Server-Routen außer `POST /{template}/render` werden bei `X-Forwarded-Proto: http` per 301 auf `https://` redirected. Render bleibt http für Microcontroller ohne TLS.

## Projektstruktur

```
cmd/
  zza-server/main.go     # schlanker Server (Cobra: serve, version)
  zza/main.go            # Desktop-Root (Cobra: serve, render, version, default=GUI)
  zza/wails.json         # Wails-v2-Config (muss neben main.go liegen)
internal/
  renderer/              # SHARED — YAML laden, PNG rendern
  server/                # SHARED — Router, Gallery, Detail, Render, ZIP, Cache
  gallery/               # SHARED
  config/                # SHARED — Env-Vars
  version/               # SHARED — Build-Version (per ldflags)
  cli/                   # DESKTOP-ONLY — Render-CLI
  editor/                # DESKTOP-ONLY — File-System-Editor (FSHandlers, files.go, starter)
  desktop/               # DESKTOP-ONLY — Wails-Bootstrap, Browser-Fallback, Templates-Pfad-Resolution
  mcp/                   # DESKTOP-ONLY — MCP-Server (Tools, Validierung, Foto-/SVG-Werkzeuge)
  watch/                 # DESKTOP-ONLY — fsnotify-Watcher für externe Änderungen
docs/embed.go            # embedded yaml-template-spec.md + ai-design-guide.md (für MCP)
web/                     # Frontend-Assets (HTML-Templates, CSS, JS) — embedded
templates/               # YAML-Templates
docs/                    # Specs, Pläne, User-Guide
legacy/                  # Alte PHP-Implementierung (Referenz)
VERSION                  # vX.Y.Z — Patch-Auto-Increment im build.sh
build.sh                 # Wails-Cross-Compile + Docker-Multi-Arch
```

## Konfiguration (Server-Build)

Env-Vars (nur Server liest sie): `PORT`, `TEMPLATES_DIR`, `CACHE_DIR`, `CACHE_MAX_AGE_HOURS`, `CACHE_MAX_SIZE_MB`. Desktop-Build hat keine Env-Vars, alles über CLI-Flags (`--templates-dir`, `--port`).

## Versionierung & Build

- `VERSION` im Root: `vX.Y.Z`. Patch wird beim `./build.sh` automatisch erhöht.
- Version landet via `-ldflags -X` in `internal/version.Version`.
- Docker-Image-Tag = Version + `latest`.
- HTML-Seiten zeigen die Version unten links.

## Workflow für neue Features

1. Spec in `docs/superpowers/specs/YYYY-MM-DD-<feature>-design.md`.
2. Plan in `docs/superpowers/plans/YYYY-MM-DD-<feature>.md` mit TDD-Tasks.
3. Manueller Testplan in `docs/superpowers/plans/YYYY-MM-DD-<feature>-manual-tests.md`.
4. Feature-Branch, Subagent-driven implementation mit Spec- und Code-Quality-Review pro Task.
5. Vor Merge: security-reviewer + code-reviewer auf den Branch-Diff.
6. Merge → develop → push.
7. Optional: `DOCKER_PUSH=1 ./build.sh` für Multi-Arch-Push, `gh release create` für Desktop-ZIPs.

## Deployment

VM `gen.yuv.de` per Docker Compose hinter Traefik (TLS via Let's Encrypt). Update-Befehl steht in `README.md`.

## Legacy-Referenz

`legacy/` enthält die alte PHP-Implementierung als Referenz für Rendering-Logik einzelner Themes, Default-Konfigurationen, Bildassets und Fonts.

`docs/legacy/` enthält die historischen Planungs-Dokumente vom Go-Rewrite (Phase 1–10, Security-Findings, Phasen-Workflow) — wurden vom Dual-Build-Refactor abgelöst.
