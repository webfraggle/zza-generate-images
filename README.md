# zza-generate-images

Go-Server der YAML-Templates zu PNG-Bildern rendert. Modellbahn-Zugzielanzeiger schicken JSON → Server gibt PNG zurück.

## Testen (CLI)

Immer `go run` verwenden — nie eine alte Binary ausführen:

```bash
go run ./cmd/zza render \
  -t sbb-096-v1 \
  -i templates/sbb-096-v1/default.json \
  -o /tmp/out.png
```

Ausgabe öffnen:

```bash
open /tmp/out.png        # macOS
```

Anderes Template oder andere Input-Daten:

```bash
go run ./cmd/zza render -t <template-name> -i <input.json> -o /tmp/out.png
```

## Build

Alle Binaries und Docker-Image lokal bauen:

```bash
./build.sh
```

Erzeugt in `dist/`:
- `zza-desktop-macos-arm64` — macOS Apple Silicon
- `zza-desktop-macos-x64` — macOS Intel
- `zza-desktop.exe` — Windows

Und lädt ein lokales Docker-Image `ghcr.io/webfraggle/zza-generate-images:latest` für die aktuelle Architektur.

## Tests

```bash
go test ./...
```

## Lokal mit Docker

```bash
cp .env.example .env
# .env anpassen (mindestens BASE_URL setzen)

docker compose up
```

Server läuft auf http://localhost:8080.

## Release: Image auf ghcr.io publizieren

**1. GitHub Personal Access Token erstellen** (falls abgelaufen):

→ github.com → Settings → Developer settings → Personal access tokens → Tokens (classic) → Generate new token

Benötigte Scopes: `write:packages`, `read:packages`

**2. Login:**

```bash
echo "DEIN_TOKEN" | docker login ghcr.io -u webfraggle --password-stdin
```

**3. Multi-Arch Image bauen und pushen** (`linux/arm64` + `linux/amd64`):

```bash
DOCKER_PUSH=1 ./build.sh
```

Mit versioniertem Tag:

```bash
DOCKER_PUSH=1 IMAGE_TAG=v1.0.0 ./build.sh
```

## Deployment auf IONOS (gen.yuv.de)

Voraussetzung: Traefik läuft bereits auf dem Server (via modellbahn-api).

```bash
cp .env.example .env
# .env mit Produktionswerten befüllen

docker compose -f docker-compose.ionos.yml up -d
```

Templates und SQLite-Datenbank bleiben bei Updates erhalten:
- `./templates/` — Bind Mount auf dem Host
- `db_data` — Docker Named Volume

Update auf neue Image-Version:

```bash
docker compose -f docker-compose.ionos.yml pull
docker compose -f docker-compose.ionos.yml up -d
```
