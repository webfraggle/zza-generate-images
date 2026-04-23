# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Projekt-Status

Dieses Projekt befindet sich im **kompletten Rewrite** von PHP nach Go.
Die alte PHP-Implementierung liegt als Referenz unter `legacy/`.
Die neue Go-Implementierung wird auf Branch `develop` gebaut.

## Dokumentation

Alle relevanten Dokumente liegen unter `docs/`:

| Datei | Inhalt |
|---|---|
| `docs/requirements-collection.md` | Alle gesammelten Anforderungen und Entscheidungen |
| `docs/implementation-plan.md` | 9-Phasen-Implementierungsplan mit Agenten-Zuordnung |
| `docs/phase-workflow.md` | Pflicht-Ablauf bei jeder Phase (7 Schritte) |
| `docs/yaml-template-spec.md` | Spezifikation des YAML-Template-Formats |
| `docs/user-guide-templates.md` | User-Guide für Template-Erstellung |
| `docs/backlog.md` | Ideen und Wünsche für spätere Features |

## Architektur (Ziel)

Go-Server der YAML-Templates zu PNG-Bildern rendert. Modellbahn-Zugzielanzeiger schicken JSON → Server gibt PNG zurück.

**Stack:**
- Sprache: Go (single binary, cross-platform)
- Deployment: Docker Compose auf kleiner VM
- Lokale Nutzung: native Binaries für Windows + macOS
- Datenbank: SQLite
- Template-Format: YAML (flache Verzeichnisstruktur)
- Frontend: Vanilla JS + CodeMirror (kein Framework)

**URL-Struktur:**
- `POST /{template}/render` — JSON → PNG (einzige Route ohne HTTPS-Redirect, für Microcontroller)
- `GET /{template}` — Vorschau-Seite mit Meta-Info, Render-URL, PNG-Download
- `GET /{template}/edit` — Template-Editor (E-Mail-Auth)
- `GET /` — Template-Galerie mit Ausprobiermodus
- `GET /admin` — Superuser-Bereich (Token + TOTP)
- `GET /health` — Health-Check

**Projektstruktur:**
```
cmd/zza/main.go          # Einstiegspunkt (Server + CLI)
cmd/zza-desktop/main.go  # Desktop-CLI (nur render, kein Server/SQLite)
internal/renderer/       # YAML laden, PNG rendern
internal/editor/         # Auth, Token, E-Mail, Datei-Upload
internal/admin/          # Superuser, TOTP
internal/gallery/        # Template-Galerie
internal/db/             # SQLite
internal/server/         # HTTP-Router, Middleware, Cache
internal/config/         # Konfiguration (Umgebungsvariablen)
internal/cli/            # Geteilte CLI-Commands (render)
internal/version/        # Build-Version (per ldflags gesetzt)
web/                     # Frontend-Assets (HTML-Templates, CSS, JS)
templates/               # YAML-Templates (portiert aus legacy/)
legacy/                  # Alte PHP-Implementierung (nur Referenz)
VERSION                  # Versionsdatei (vX.Y.Z) — Patch auto-increment bei Build
build.sh                 # Cross-Compile + Docker Multi-Arch + Version-Management
```

## Agenten

| Agent | Datei | Wann einsetzen |
|---|---|---|
| `security-reviewer` | `~/.claude/agents/security-reviewer.md` | Nach jeder Phase — Pflicht |
| `code-reviewer` | `~/.claude/agents/code-reviewer.md` | Nach Security-Review — Pflicht |
| `template-porter` | `~/.claude/agents/template-porter.md` | Phase 8: PHP → YAML Portierung |

## Phasen-Workflow

Bei **jeder Phase** gilt: Implementierung → Security Review → Code Review → Commit → Manuelle Testbeschreibung → User-OK → Abschluss. Details: `docs/phase-workflow.md`.

## Versionierung

- `VERSION`-Datei im Root enthält die aktuelle Version (`vX.Y.Z`)
- Major.Minor wird manuell gepflegt, Patch wird bei jedem `./build.sh` automatisch erhöht
- Version wird per `-ldflags -X` in `internal/version.Version` kompiliert
- Docker-Image-Tag ist automatisch an die Version gekoppelt (plus `latest`)
- Alle HTML-Seiten zeigen die Version unten links

## HTTPS-Redirect

Alle Routen außer `POST /{template}/render` werden per 301 auf HTTPS umgeleitet (via `X-Forwarded-Proto` Header). Die Render-Route bleibt auf HTTP verfügbar, da sie von Microcontrollern ohne TLS-Support aufgerufen wird.

## Konfiguration (Umgebungsvariablen)

Nur im Server-Build relevant: `PORT`, `TEMPLATES_DIR`, `CACHE_DIR`, `CACHE_MAX_AGE_HOURS`, `CACHE_MAX_SIZE_MB`.

Der Desktop-Build (`cmd/zza`) liest keine Env-Vars; Konfiguration via CLI-Flags (`--templates-dir`, `--port`).

_Entfernt im Dual-Build-Refactor (2026-04-23):_ `DB_PATH`, `SMTP_*`, `EDIT_TOKEN_TTL_HOURS`, `ADMIN_TOKEN`, `TOTP_SECRET`, `BASE_URL`, `SECURE_COOKIES` — SMTP-Auth, SQLite-DB und Admin-Bereich wurden aus dem Server-Build gezogen; Editor läuft nur noch in der Desktop-App ohne Auth.

## Legacy-Referenz

Die alte PHP-Implementierung unter `legacy/` dient als Referenz für:
- Rendering-Logik der einzelnen Themes
- Default-Konfigurationen (`default.json`) → neue YAML-Templates
- Bildassets und Fonts die übernommen werden
