# zza-generate-images

Go-Server der YAML-Templates zu PNG-Bildern rendert. Modellbahn-Zugzielanzeiger schicken JSON → Server gibt PNG zurück.

## Architektur — Dual Build

Aus derselben Codebase entstehen zwei Binaries:

| Build | Zweck | Auslieferung |
|---|---|---|
| `cmd/zza-server` | Galerie + Vorschau + Render-Endpoint | Docker-Image auf VM |
| `cmd/zza` | Editor + Vorschau + Render + GUI (Wails) | ZIPs für macOS + Windows |

Templates erstellt der Admin lokal mit der Desktop-App, schickt sie (manuell gezippt) per E-Mail, und sie werden per SCP/rsync auf den Server kopiert. Keine Upload-API, kein Server-seitiges Auth.

## Entwicklung lokal

Render-CLI ohne Server starten — schnellster Smoke-Test:

```bash
go run ./cmd/zza render \
  -t sbb-096-v1 \
  -i templates/sbb-096-v1/default.json \
  -o /tmp/out.png
open /tmp/out.png
```

Server lokal starten:

```bash
PORT=18080 TEMPLATES_DIR=./templates go run ./cmd/zza-server serve
# → http://localhost:18080
```

Desktop-App lokal starten (öffnet Wails-Fenster):

```bash
go run ./cmd/zza
# oder ohne Fenster, nur Server:
go run ./cmd/zza serve --port 18081
```

## Tests

```bash
go test ./...
```

## Build

`./build.sh` baut alles:

- Server-Docker-Image für die aktuelle Architektur (lädt nach lokalem Docker)
- Desktop-Release-ZIPs in `dist/release/`:
  - `zza-macos-arm64.zip` — Apple Silicon
  - `zza-macos-intel.zip` — Intel Mac
  - `zza-windows-x64.zip` — Windows
- Patch-Version in `VERSION` wird automatisch hochgezählt

Voraussetzungen für die Desktop-Builds:
- [Wails CLI](https://wails.io/docs/gettingstarted/installation) (`go install github.com/wailsapp/wails/v2/cmd/wails@v2.9.3`)
- Xcode Command Line Tools (`xcode-select --install`)
- Für Windows-Cross-Compile vom Mac: `brew install mingw-w64`

Jedes Desktop-ZIP enthält die Binary/`.app`, den vollständigen `templates/`-Ordner und ein deutschsprachiges `README.txt` mit Erst-Start-Anleitung.

## Lokal mit Docker

```bash
cp .env.example .env       # optional anpassen
docker compose up
# → http://localhost:8080
```

## Release: Image auf ghcr.io publizieren

GitHub Personal Access Token mit `write:packages`+`read:packages` erstellen, dann:

```bash
echo "DEIN_TOKEN" | docker login ghcr.io -u webfraggle --password-stdin
DOCKER_PUSH=1 ./build.sh
```

Pusht ein Multi-Arch-Image (`linux/arm64`+`linux/amd64`) mit Versions-Tag (aus `VERSION`) und `latest`.

## Deployment auf der VM (gen.yuv.de)

Voraussetzung: Traefik läuft bereits (via modellbahn-api).

Update auf neue Image-Version:

```bash
ssh -l root -p 2847 87.106.149.199 \
  "cd ~/zza-generate-images && docker compose pull && docker compose up -d"
```

Templates liegen als Bind Mount unter `./templates/` und bleiben bei Updates erhalten. Cache liegt im Named Volume `cache_data` und kann jederzeit weggeworfen werden.

## Desktop-App verteilen

Nach `./build.sh` die ZIPs aus `dist/release/` als GitHub-Release hochladen — keine Code-Signatur (User klickt Erst-Start-Warnungen weg, README.txt im ZIP erklärt das).

## Dokumentation

| Datei | Inhalt |
|---|---|
| `docs/superpowers/specs/2026-04-22-dual-build-architecture-design.md` | Design-Spec für die Trennung Server/Desktop |
| `docs/superpowers/plans/2026-04-22-dual-build-architecture.md` | Implementierungsplan mit allen Tasks |
| `docs/superpowers/plans/2026-04-22-dual-build-architecture-manual-tests.md` | Manueller Test-Plan vor jedem Release |
| `docs/yaml-template-spec.md` | YAML-Template-Format |
| `docs/user-guide-templates.md` | User-Guide Template-Erstellung |
| `CLAUDE.md` | Hinweise für Claude Code beim Arbeiten am Repo |
