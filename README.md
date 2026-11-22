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
  -t sbb-105-v1 \
  -i templates/sbb-105-v1/default.json \
  -o /tmp/out.png
open /tmp/out.png
```

Dasselbe als animiertes GIF (Faltblatt-Animation; das Format folgt der Endung von `-o` oder `--format png|gif`):

```bash
go run ./cmd/zza render -t faltblatt-105-v1 -i templates/faltblatt-105-v1/default.json -o /tmp/out.gif
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

KI-Assistent: `zza mcp` startet einen MCP-Server, mit dem Claude Desktop oder Claude Code Templates baut — Einrichtung siehe [`docs/user-guide-templates.md`](docs/user-guide-templates.md#ki-assistent-mcp) (im Repo ist `.mcp.json` bereits konfiguriert).

## API

```bash
# PNG (Standard)
curl -X POST http://localhost:18080/sbb-105-v1/render -H 'Content-Type: application/json' --data-binary @templates/sbb-105-v1/default.json -o out.png

# Animiertes GIF (Faltblatt-Animation)
curl -X POST 'http://localhost:18080/faltblatt-105-v1/render?format=gif' -H 'Content-Type: application/json' --data-binary @templates/faltblatt-105-v1/default.json -o out.gif

# Uhr mit laufendem Sekundenzeiger (Frame-Animation, 60 Frames, Endlosschleife)
curl -X POST 'http://localhost:18080/umuc-105-v1/render?format=gif' -H 'Content-Type: application/json' --data-binary @templates/umuc-105-v1/default.json -o umuc.gif
```

- `POST /{template}/render` und `GET /{template}/preview` akzeptieren `?format=png|gif`. Ohne Angabe oder mit `png` kommt ein PNG wie bisher; jeder andere Wert ergibt 400.
- `format=gif` liefert `Content-Type: image/gif`. Faltblatt-Templates (`splitflap`) spielen einmal ab und enden mit einem Halte-Frame auf dem Standbild (identisch zum PNG). Templates mit `meta.animation.frames` (Frame-Animation, z.B. `umuc-105-v1`) ergeben `frames` Frames à `frame_delay` Sekunden, ohne Halte-Frame und mit `loop: true` als Endlosschleife. Templates ohne Animation ergeben ein GIF mit einem Frame.
- Gleiches Template und exakt gleicher Request-Body (Bytes) ergeben ein bytegleiches GIF (Ausnahme: Templates mit `{{now.*}}`). PNG und GIF werden getrennt gecacht (`X-Cache: HIT|MISS`).
- Zeitabhängige Templates (`now` oder `format(…)` in einem Ausdruck) werden nicht gecacht: `X-Cache: BYPASS` und `Cache-Control: no-store`, für PNG und GIF. Nur `GET /{template}/preview?format=gif` hält das letzte GIF 10 Sekunden im Speicher (`X-Cache: PREVIEW`), damit Galerie-Aufrufe nicht jedes Mal neu rendern. `now` läuft in `meta.timezone` (Standard `Europe/Berlin`).
- Es laufen höchstens 2 GIF-Renders gleichzeitig, weitere warten bis zu 15 s und bekommen danach `503` mit `Retry-After`. Template-Fehler (z.B. Limits überschritten) ergeben 422.
- `POST …/render` bleibt auch mit `?format=gif` vom HTTPS-Redirect ausgenommen (Microcontroller ohne TLS).
- Format des GIFs und Template-Syntax: `docs/yaml-template-spec.md` (`type: splitflap`, „Frame-Animation", „GIF-Ausgabe").

## ZZA-Export und -Vorschau

Templates lassen sich für den Controller (ZZA) exportieren, der sie mit einem eigenen C++-Renderer (`device/zzarender/`) selbst zeichnet:

```bash
go run ./cmd/zza export -t faltblatt-105-v1 -o /tmp/faltblatt-zza.zip
```

Der Export prüft die Kompatibilität (Canvas 240 × 240, Latin-1-Konstanten, konstante Regex-Muster, Paket ≤ 3 MB; siehe „ZZA-kompatibel“ in `docs/yaml-template-spec.md`). Im Editor schaltet **Server / ZZA** die Vorschau auf den Geräte-Renderer um. Das ZIP lädst du in der Web-UI der Firmware hoch.

Dasselbe Paket gibt es auch online, ohne Desktop-App: auf der Detailseite eines Templates über **Für ZZA exportieren** oder direkt unter `GET /{template}/zza.zip` (z. B. `https://gen.yuv.de/umuc-105-v1/zza.zip`). Der Server liefert immer das Paket des aktuellen Exporters; auf dem Controller muss dafür die neueste Firmware laufen. Ein nicht kompatibles Template ergibt 422 mit den Gründen, der Knopf ist dann ausgegraut. Die Pakete werden im Ordner `zza/` des `CACHE_DIR` gecacht, mit eigener Grenze `ZZA_CACHE_MAX_SIZE_MB` (Standard 50); ein geändertes Template wird neu exportiert.

Der C++-Kern wird per cgo in die Desktop-Builds eingebunden. Der Server-Build bleibt CGO-frei. Auf einem Mac mit defekten Command Line Tools (Fehler `'cstdint' file not found`) setzt `build.sh` für die macOS-Builds automatisch `CGO_CXXFLAGS`; für `go test`/`go build` von Hand siehe `device/README.md`.

## Tests

```bash
go test ./...
```

Mit cgo-Kern auf so einem Mac:

```bash
export CGO_CXXFLAGS="-nostdinc++ -isystem $(xcrun --show-sdk-path)/usr/include/c++/v1"
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

Nur die Desktop-ZIPs, ohne Docker-Image: `DESKTOP_ONLY=1 ./build.sh` (zählt die Version ebenfalls hoch).

Ohne Schalter im Terminal gestartet, fragt `./build.sh` zuerst, was gebaut werden soll (Desktop + lokales Image, nur Desktop, mit Push, mit Deployment). Mit einem gesetzten Schalter (`DESKTOP_ONLY`, `DOCKER_PUSH`, `DEPLOY`, auch `=0`) oder ohne Terminal entfällt die Frage.

Voraussetzungen für die Desktop-Builds:
- [Wails CLI](https://wails.io/docs/gettingstarted/installation) (`go install github.com/wailsapp/wails/v2/cmd/wails@v2.9.3`)
- Xcode Command Line Tools (`xcode-select --install`)
- Für Windows-Cross-Compile vom Mac: `brew install mingw-w64` (kompiliert auch den C++-Kern, `x86_64-w64-mingw32-g++`)

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

`DOCKER_PUSH=1` und `DEPLOY=1` lassen zuerst `go vet ./...` und `go test ./...` laufen (gut eine Minute). Schlägt etwas fehl, bricht das Skript ab, bevor `VERSION` hochgezählt oder irgendetwas gebaut wird. Ein einfaches `./build.sh` läuft ohne Tests.

Vor dem Push prüft `scripts/scan-image.sh` das Image mit Trivy auf bekannte Sicherheitslücken (alpine-Pakete und Go-Binary). Das Skript listet alle bekannten Lücken auf und bricht nur ab, wenn es für eine Lücke der Stufe HIGH oder CRITICAL bereits einen Fix gibt; dann wird nichts gepusht und nicht deployt. Meist hilft ein neuer Lauf (das Image zieht bei jedem Build die aktuellen alpine-Pakete), sonst ein neueres Basis-Image oder Go-Modul. Trivy läuft als Container, es muss nichts installiert werden. `SKIP_SCAN=1 DOCKER_PUSH=1 ./build.sh` pusht ohne Scan.

## Deployment auf der VM (gen.yuv.de)

Voraussetzung: Traefik läuft bereits (via modellbahn-api).

### Deployment per build.sh

```bash
DEPLOY=1 ./build.sh
```

baut wie `DOCKER_PUSH=1 ./build.sh` (Desktop-ZIPs + Multi-Arch-Image mit Versions-Tag und `latest`), überträgt danach `docker-compose.ionos.yml` als `~/zza-generate-images/docker-compose.yml` auf die VM (nur wenn sie sich geändert hat), führt dort `docker compose pull && docker compose up -d` aus, löscht die abgelösten Images (`docker image prune -f`) und wartet, bis `https://gen.yuv.de/health` die neue Version meldet (`{"status":"ok","version":"vX.Y.Z"}`). Schlägt ein Build-Schritt fehl, wird nicht deployt. SSH-Login, laufendes Docker und die Tests werden vor dem Hochzählen von `VERSION` geprüft.

Hat sich die Compose-Datei geändert, zeigt das Skript die Unterschiede (Server → lokal), prüft die neue Datei auf der VM mit `docker compose config` und ersetzt die alte erst danach; die alte bleibt als `docker-compose.yml.bak` liegen. Ist die neue Datei ungültig oder klappt die Übertragung nicht, bricht der Deploy ab und auf der VM bleibt alles beim Alten. Andere Dateien überträgt das Skript nicht, insbesondere nie die `.env` und nie `templates/`.

Die Anmeldung läuft über deinen SSH-Schlüssel, es wird kein Passwort hinterlegt. Ziel ist der Alias `modellbahn-vm` aus `~/.ssh/config` – derselbe wie bei modellbahn-api (Einrichtung dort im README, Abschnitt „Deployment per build.sh“). Anderer Alias: `DEPLOY_SSH=… DEPLOY=1 ./build.sh`.

Manuell geht das Update weiterhin so:

```bash
scp docker-compose.ionos.yml modellbahn-vm:zza-generate-images/docker-compose.yml
ssh modellbahn-vm "cd ~/zza-generate-images && docker compose pull && docker compose up -d"
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
| `docs/superpowers/specs/2026-11-13-splitflap-animation-design.md` | Design-Spec der Faltblatt-Animation (GIF) |
| `docs/superpowers/plans/2026-11-13-splitflap-animation-manual-tests.md` | Manueller Test-Plan der Faltblatt-Animation |
| `docs/superpowers/specs/2026-11-13-clock-animation-design.md` | Design-Spec Frame-Animation, Zeitzone, Filter-Argumente, Uhr `umuc-105-v1` |
| `docs/superpowers/plans/2026-11-13-clock-animation-manual-tests.md` | Manueller Test-Plan der Frame-Animation |
| `docs/yaml-template-spec.md` | YAML-Template-Format (inkl. `splitflap`, Frame-Animation, GIF-Ausgabe) |
| `docs/user-guide-templates.md` | User-Guide Template-Erstellung |
| `CLAUDE.md` | Hinweise für Claude Code beim Arbeiten am Repo |
