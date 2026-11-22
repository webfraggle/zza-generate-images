# ZZA-Paket online herunterladen — Manueller Testplan

Spec: `docs/superpowers/specs/2026-11-22-zza-online-export-design.md`. Alle Punkte vor einem Merge von
`feature/zza-online-export` durchgehen.

**Stand:** Nichts davon ist gemerged oder deployed.

## Vorbereitung

Server lokal: `PORT=18080 TEMPLATES_DIR=./templates CACHE_DIR=/tmp/zza-manual/cache go run ./cmd/zza-server serve`;
`export B=http://localhost:18080`.

## Server (Browser)

1. `$B/umuc-105-v1` öffnen: Knopf „Für ZZA exportieren“ ist da und aktiv. Klick lädt `umuc-105-v1-zza.zip`.
2. Das ZIP in der Weboberfläche eines Anzeigers mit Firmware 3.3.19-experimental unter „Templates“ einspielen
   und einem Gleis zuweisen: das Template wird angezeigt.
3. `curl -sI $B/umuc-105-v1/zza.zip` zweimal: erst `X-Cache: MISS`, dann `HIT`. In `/tmp/zza-manual/cache/zza/`
   liegt eine `.zzapkg`.
4. `templates/umuc-105-v1/template.yaml` speichern (Änderungszeit neu), erneut abrufen: `X-Cache: MISS`; im
   Cache-Ordner liegt weiterhin nur eine Datei für das Template.
5. `curl -s -o /dev/null -w '%{http_code}\n' $B/gibt-es-nicht/zza.zip` → 404.
6. Ein Template mit Canvas ≠ 240 × 240 in den Templates-Ordner legen: Detailseite zeigt den Knopf ausgegraut,
   der Tooltip nennt den Grund; `curl $B/<name>/zza.zip` → 422 mit den Gründen.
7. „Template-Source herunterladen“ liefert weiterhin `<template>.zip`.

## Desktop-App

8. App starten, Detailseite eines Templates: „Für ZZA exportieren“ öffnet den Speichern-Dialog, die Datei ist
   nicht leer.
9. Im Editor: „Für ZZA exportieren“ funktioniert wie bisher.

## Nach dem Deploy

10. `https://gen.yuv.de/umuc-105-v1/zza.zip` lädt das Paket; `http://…` leitet auf `https` um.
11. `ssh modellbahn-vm "docker compose -f zza-generate-images/docker-compose.yml exec zza-server ls /data/cache/zza"`
    zeigt die gecachten Pakete.
