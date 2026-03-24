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

```bash
go build -o zza ./cmd/zza
```

## Tests

```bash
go test ./...
```
