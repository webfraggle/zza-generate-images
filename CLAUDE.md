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
- `POST /{template}/render` — JSON → PNG
- `GET /{template}/edit` — Template-Editor (E-Mail-Auth)
- `GET /` — Template-Galerie mit Ausprobiermodus
- `GET /admin` — Superuser-Bereich (Token + TOTP)

**Projektstruktur (Ziel):**
```
cmd/zza/main.go          # Einstiegspunkt
internal/renderer/       # YAML laden, PNG rendern, Cache
internal/editor/         # Auth, Token, E-Mail, Datei-Upload
internal/admin/          # Superuser, TOTP
internal/gallery/        # Template-Galerie
internal/db/             # SQLite
internal/server/         # HTTP-Router, Middleware
web/                     # Frontend-Assets
templates/               # YAML-Templates (portiert aus legacy/)
legacy/                  # Alte PHP-Implementierung (nur Referenz)
```

## Agenten

| Agent | Datei | Wann einsetzen |
|---|---|---|
| `security-reviewer` | `~/.claude/agents/security-reviewer.md` | Nach jeder Phase — Pflicht |
| `code-reviewer` | `~/.claude/agents/code-reviewer.md` | Nach Security-Review — Pflicht |
| `template-porter` | `~/.claude/agents/template-porter.md` | Phase 8: PHP → YAML Portierung |

## Phasen-Workflow

Bei **jeder Phase** gilt: Implementierung → Security Review → Code Review → Commit → Manuelle Testbeschreibung → User-OK → Abschluss. Details: `docs/phase-workflow.md`.

## Konfiguration (Umgebungsvariablen)

`PORT`, `TEMPLATES_DIR`, `CACHE_DIR`, `CACHE_MAX_AGE_HOURS`, `CACHE_MAX_SIZE_MB`, `DB_PATH`, `SMTP_HOST`, `SMTP_PORT`, `SMTP_USER`, `SMTP_PASS`, `SMTP_FROM`, `EDIT_TOKEN_TTL_HOURS`, `ADMIN_TOKEN`, `TOTP_SECRET`, `BASE_URL`

## Legacy-Referenz

Die alte PHP-Implementierung unter `legacy/` dient als Referenz für:
- Rendering-Logik der einzelnen Themes
- Default-Konfigurationen (`default.json`) → neue YAML-Templates
- Bildassets und Fonts die übernommen werden
