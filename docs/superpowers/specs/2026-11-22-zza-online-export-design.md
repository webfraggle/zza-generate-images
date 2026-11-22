# ZZA-Paket online herunterladen – Entwurf

Stand 2026-11-22. Branch `feature/zza-online-export` (von `feature/zza-on-device-rendering`).

## Wozu

Das Paket für den Controller („Für ZZA exportieren“, `zza export`) gab es bisher nur aus der Desktop-App. Wer nur
ein fertiges Template auf seinen Anzeiger bringen will, soll es auch auf `gen.yuv.de` herunterladen können, ohne
die App zu installieren.

Der Nutzer ist dafür verantwortlich, dass auf seinem Controller die neueste Firmware läuft (Entscheidung
2026-11-22). Der Server liefert deshalb immer nur das Paket des aktuellen Exporters; eine Auswahl nach
Formatnummer gibt es nicht.

## Route

`GET /{template}/zza.zip` – im Server- und im Desktop-Build.

- Antwort: das Paket, bytegleich mit `zza export` (`Content-Type: application/zip`,
  `Content-Disposition: attachment; filename="<template>-zza.zip"`, `Cache-Control: no-store`,
  `X-Cache: HIT|MISS`).
- 400 ungültiger Template-Name, 404 Template gibt es nicht, 422 `nicht ZZA-kompatibel:` + Gründe (wie im
  Editor), 503 mit `Retry-After: 5`, wenn kein Export-Platz frei wird, 500 sonst (Ursache nur im Log).
- `/zza.zip` allein bleibt das Source-ZIP eines Templates namens `zza`.
- Die Route wird wie `/{template}.zip` vor dem Mux verteilt. `/{template}/zza.zip` kann nicht mit dem
  Source-ZIP kollidieren, weil ein Template-Name kein `/` enthält (`/foo-zza.zip` wäre mehrdeutig gewesen).
- `http` wird wie alle Routen außer `/render` auf `https` umgeleitet.

Die Desktop-Route `GET /edit/{template}/zza-export` bleibt für den Editor bestehen.

## Detailseite

Der Knopf „Für ZZA exportieren“ steht jetzt in beiden Builds. Ist das Template nicht ZZA-kompatibel, ist er
ausgegraut und nennt die Gründe im Tooltip. Lässt sich die Kompatibilität beim Laden der Seite gerade nicht
feststellen (alle Export-Plätze belegt), wird der Knopf angeboten und ein Klick meldet das Ergebnis.

## Last und Cache

Die Route ist öffentlich und ohne Anmeldung. Gemessen (M1, 2026-11-22): ein Export braucht 20–85 ms, ein Paket ist
110–300 KB groß, alle acht Templates zusammen 1,8 MB.

- **Cache:** eigener Ordner `zza/` im `CACHE_DIR`, eigene Grenze `ZZA_CACHE_MAX_SIZE_MB` (Standard 50),
  Höchstalter wie der Bild-Cache (`CACHE_MAX_AGE_HOURS`). Aufgeräumt wird im selben Takt wie der Bild-Cache
  (alle 15 min); dazwischen kann der Ordner die Grenze kurz überschreiten.
- **Eine Datei je Template:** der Dateiname hängt nur vom Template-Namen ab (`<hash>.zzapkg`). Die Datei
  beginnt mit einem Stempel (64 Hex-Zeichen) über Name/Größe/Änderungszeit aller Dateien des Templates,
  Paketformat und Server-Version; danach folgt das ZIP. Passt der Stempel nicht mehr (Template geändert, neuer
  Deploy), wird neu exportiert und die Datei überschrieben. Der Cache hält so nie mehr als ein Paket je Template
  – auch in der Desktop-App, die keinen Aufräum-Takt hat. Eingaben des Aufrufers gehen nicht in den Namen ein.
- **Nicht kompatible Templates:** das Ergebnis (Gründe) wird je Template im Speicher gemerkt, solange der
  Stempel passt. Wiederholte Aufrufe kosten keinen Export und keinen Export-Platz.
- **Gleichzeitigkeit:** höchstens 2 Exporte zugleich, Wartezeit 10 s, danach 503.
- Der Bild-Cache zählt und löscht nur noch Dateien direkt in `CACHE_DIR`, keine Unterordner.

### Gemeinsames Volume mit modellbahn-api?

Nein. Technisch ginge ein externes Volume für beide Compose-Projekte, aber:

- Beide Caches räumen ihren Ordner selbst auf und kennen nur ihre eigene Grenze. Im selben Ordner würden sie
  sich gegenseitig Dateien löschen oder die Größe falsch zählen; getrennte Unterordner im selben Volume bringen
  nichts außer der Kopplung.
- Die beiden Stacks werden getrennt deployt. Ein gemeinsames externes Volume muss von Hand angelegt werden und
  macht jeden Stack vom anderen abhängig (Rechte, `docker volume prune`).
- zza-server hat schon ein eigenes Volume (`cache_data` unter `/data/cache`). Der neue Ordner liegt darin, die
  Compose-Datei braucht kein neues Volume.

## Abgrenzung

- Keine ZZA-Vorschau auf dem Server (braucht den cgo-Gerätekern, bleibt Desktop).
- Der Server-Build bleibt CGO-frei: `internal/devexport` ist reines Go.
- Die Templates auf der VM liegen als eigener Ordner und werden vom Deploy nicht aktualisiert.
