# Faltblatt-Animation — Manueller Testplan

Alle Punkte vor einem Merge von `feature/splitflap-animation` → `develop` durchgehen. Bei einem fehlgeschlagenen Punkt nicht weitermachen.

**Stand:** Nichts davon ist gemerged, gepusht oder deployed. Alle Tests laufen lokal auf dem Branch `feature/splitflap-animation`. Auf `gen.yuv.de` ist die Animation noch nicht verfügbar.

## Vorbereitung

Server lokal starten (aus dem Repo-Root, Branch `feature/splitflap-animation`):

```bash
PORT=18080 TEMPLATES_DIR=./templates go run ./cmd/zza-server serve
```

Daten und Hilfsvariablen für die folgenden Tests (in einer zweiten Shell):

```bash
export B=http://localhost:18080
export D=templates/faltblatt-105-v1/default.json
mkdir -p /tmp/zza-manual
```

Daten des Beispiels: Zug `IC23`, `13:30`, Ziel `Frankfurt`, Via `Neuhof - Gelnhausen - Hanau`, Gleis `6`.

## 1. Standbild (PNG) gegen Referenz

- [ ] `curl -s -X POST $B/faltblatt-105-v1/render -H 'Content-Type: application/json' --data-binary @$D -o /tmp/zza-manual/still.png` → PNG, 240×240 (`file /tmp/zza-manual/still.png`).
- [ ] Bild neben `docs/superpowers/specs/assets/2026-11-13-faltblatt-referenz.png` legen. Gleiche Elemente an gleicher Position (±1 px), gleiche Größen: „Gleis 6", „Abfahrt 13 30", Zugart-Logo IC, Via, Zielzeile „FRANKFURT" auf Einzelklappen.
- [ ] Via zeigt „Neuhof" und „Gelnhausen"; „Hanau" entfällt.
- [ ] Rahmen/Scharniere (Overlay) liegen **über** den Texten, nicht darunter.
- [ ] Untere Bildhälfte ist wie in der Referenz umarrangiert (die drei `copy`-Layer).
- [ ] Die Kantenglättung darf abweichen; Pixelgleichheit ist nicht gefordert.

## 2. GIF per curl (http, ohne Redirect)

- [ ] `curl -s -D - -X POST "$B/faltblatt-105-v1/render?format=gif" -H 'Content-Type: application/json' --data-binary @$D -o /tmp/zza-manual/anim.gif` → `HTTP/1.1 200`, `Content-Type: image/gif`, `X-Cache: MISS`.
- [ ] Zusätzlich mit `-H 'X-Forwarded-Proto: http'`: **kein** 301, Antwort bleibt 200 + GIF (Render-Route ist vom HTTPS-Redirect ausgenommen, auch mit Query).
- [ ] `file /tmp/zza-manual/anim.gif` → `GIF image data … 240 x 240`.
- [ ] Dateigröße notieren (Richtwert ≤ 400 KB): ______ KB.
- [ ] Renderzeit des ersten Aufrufs notieren (`curl -w '%{time_total}'`, Richtwert < 1 s): ______ s.
- [ ] Ohne `format` und mit `format=png` kommt weiterhin ein PNG (`Content-Type: image/png`), Bytes identisch mit dem Ergebnis aus Test 1.
- [ ] `curl -s -o /dev/null -w '%{http_code}\n' -X POST "$B/faltblatt-105-v1/render?format=webp" -H 'Content-Type: application/json' --data-binary @$D` → `400` (Body nennt `png` und `gif`).
- [ ] Auch `format=GIF` (Großschreibung) und `format=` mit leerem Wert verhalten sich wie im Code vorgesehen: leer = PNG, `GIF` = 400.
- [ ] `GET $B/faltblatt-105-v1/preview?format=gif` → GIF; `…/preview?format=webp` → 400.

## 3. Cache

- [ ] Denselben GIF-Aufruf aus Test 2 direkt noch einmal absetzen → `X-Cache: HIT`, Bytes identisch zum ersten Ergebnis (`cmp`).
- [ ] Danach denselben Aufruf ohne `format` → PNG; Wechsel zwischen PNG und GIF überschreibt sich nicht (beide liefern bei Wiederholung `HIT`).
- [ ] Andere Daten (z.B. `zug1.nr` auf `RE5`) → `X-Cache: MISS`, anderes GIF.
- [ ] Datei `templates/faltblatt-105-v1/template.yaml` mit `touch` anfassen → nächster Aufruf wieder `MISS` (Cache-Schlüssel enthält die mod-time).

## 4. Determinismus und GIF-Eigenschaften

- [ ] Cache-Verzeichnis leeren (Server neu starten mit anderem `CACHE_DIR`) und den Aufruf aus Test 2 erneut: die Datei ist **byte-identisch** mit dem ersten GIF (`cmp`). Gleiches Template + gleicher Request-Body ⇒ gleiches GIF.
- [ ] Anderer Body (z.B. Leerzeichen im JSON ergänzt) darf ein anderes Ablaufmuster ergeben; das ist erlaubt, das Standbild am Ende bleibt gleich.
- [ ] GIF in die Vorschau-App (macOS Vorschau/Quick Look) und in einen Browser laden: es spielt **einmal** und bleibt auf dem letzten Frame stehen, keine Schleife.
- [ ] Frame 0 ist bereits in Bewegung (kein ruhiges Startbild): obere Klappenhälften sind gestaucht/abgedunkelt.
- [ ] Die Klappen stoppen zu **unterschiedlichen** Zeiten (Zielzeile, Zugart, Via, Stunde, Minute nacheinander); „Gleis 6" und die Leerklappe oben rechts bewegen sich nicht.
- [ ] Letztes Frame sieht aus wie das PNG aus Test 1 (bis auf Palettierung). Optional: `ffmpeg -i anim.gif -vf "select=eq(n\,LAST)" …` bzw. letztes Frame exportieren und mit dem PNG vergleichen.
- [ ] Gesamtdauer liegt bei höchstens ca. 2,5 s plus mindestens 1 s Standzeit des letzten Frames (länger, wenn das letzte Frame dem vorletzten gleicht; Frames davor je 5 cs bei 20 fps).
- [ ] Optional, Firmware-Sicht: `gifsicle --info /tmp/zza-manual/anim.gif` bzw. `identify -verbose`: eine globale Farbtabelle (keine lokalen), Disposal „none/do not dispose", Frames nach dem ersten kleiner als 240×240 und mit transparenten Pixeln (unveränderte Pixel im Delta-Rechteck), Farbtabelle mit höchstens 255 echten Farben plus transparentem Eintrag, keine Loop-Extension.

## 5. Randdaten

Jeweils eine Kopie von `default.json` anpassen (`/tmp/zza-manual/edge-*.json`), GIF anfordern, Ergebnis ansehen. Erwartung überall: 200, GIF endet auf dem passenden Standbild, kein Absturz.

- [ ] **Via leer** (`"via": ""`): Via-Klappe blättert auf die Leerklappe, Standbild zeigt leere Tafel.
- [ ] **Zeit ohne Doppelpunkt** (`"zeit": "1330"`): Stunde zeigt `1330` (`before` liefert den ganzen Wert), Minute bleibt leer (`after` liefert `""`). Es gibt keinen Fehler.
- [ ] **Zeit leer** (`"zeit": ""`): beide Zeitklappen leer, kein Fehler.
- [ ] **Ziel länger als 13 Zeichen** (`"vonnach": "Frankfurt Flughafen Fernbahnhof"`): Zielzeile zeigt die ersten 13 Zeichen, der Rest wird abgeschnitten.
- [ ] **Ziel leer** (`"vonnach": ""`): alle 13 Klappen blättern auf Leer.
- [ ] **Kleinschreibung/Mischung** (`"nr": "ice 71"`, dann `"Ice 71"`, `"RB 12"`): Zugart zeigt jeweils das ICE-Logo bzw. RB-Logo (Erkennung unabhängig von Groß/Kleinschreibung, `ice` vor `ic`).
- [ ] **Unbekannte Zugart** (`"nr": "XYZ 1"`): Zugart-Klappe endet leer (kein passender Zweig), kein Fehler.
- [ ] **Umlaute/Sonderzeichen** im Ziel (`"Köln-Ü.Ä."`, kurz genug für 13 Zeichen): Zeichen aus dem `reel` (Ä Ö Ü - .) werden gezeichnet; Zeichen **außerhalb** des `reel` (z.B. `Ø` oder `ß`) erscheinen am Ende trotzdem so, wie sie sind.
- [ ] **Ungültiges JSON** im Body → 400 (`invalid JSON …`).

## 6. Template ohne splitflap, Regression

- [ ] `curl -s -X POST "$B/sbb-096-v1/render?format=gif" -H 'Content-Type: application/json' --data-binary @templates/sbb-096-v1/default.json -o /tmp/zza-manual/sbb.gif` → GIF mit **einem** Frame (`gifsicle --info` bzw. `identify sbb.gif | wc -l` = 1).
- [ ] Display-Regression: PNG-Render **aller anderen** Templates ist bitgleich wie vor der Änderung. Vergleich mit `develop`: auf `develop` und auf dem Branch je `go run ./cmd/zza render -t <name> -i templates/<name>/default.json -o /tmp/zza-manual/<label>-<name>.png --templates-dir ./templates` für jedes Template außer `faltblatt-105-v1`, danach `cmp` je Paar. `<label>` ist `develop` bzw. `branch` (kein Branch-Name mit Schrägstrich). Erwartung: keine Unterschiede.
- [ ] `POST /{template}/render` ohne Query liefert für alle Templates `image/png`, `X-Cache` MISS/HIT wie gewohnt.
- [ ] `GET /`, `GET /{template}`, `GET /{template}.zip`, `GET /health` unverändert (siehe Dual-Build-Testplan).

## 7. Limits und Fehler

- [ ] Kopie von `faltblatt-105-v1` unter anderem Namen (in einem temporären `TEMPLATES_DIR`) mit `meta.animation.fps: 60` → GIF-Anfrage liefert 422, Meldung nennt den erlaubten Bereich 5–25.
- [ ] Dieselbe Kopie mit `duration: 10` → 422.
- [ ] `cells: 13` ohne `split: chars` → 422 (`cells > 1 requires split: chars`).
- [ ] `cells: 65` → 422 (Limit 64 Module).
- [ ] Zwei überlappende Module → 422 (`overlaps another splitflap module`).
- [ ] `reel` mit mehr als 64 Einträgen → 422.
- [ ] `loop`/`copy` als Kind-Layer eines `splitflap` → 422.
- [ ] Bei Fehlern in den `splitflap`-Layern (`cells`, Überlappung, `reel`, Kind-Layer) schlägt auch der PNG-Render (ohne `format`) mit 422 fehl. Bei Fehlern in `meta.animation` (`fps`, `duration`) dagegen **nicht**: `meta.animation` wird nur beim GIF-Render gelesen, der PNG-Render bleibt 200.
- [ ] Nebenläufigkeit: 6 GIF-Anfragen gleichzeitig mit jeweils anderem Body (`for i in 1..6 … & wait`) → alle liefern 200, es laufen höchstens 2 Renders parallel (Server-Last beobachten), keine Fehler.

## 8. Web: Detailseite (Server und Desktop)

Browser auf `$B/faltblatt-105-v1`.

- [ ] Standardansicht zeigt das **Standbild** (PNG), nicht das GIF.
- [ ] Button „Animation abspielen" ist vorhanden, außerdem „GIF herunterladen" neben „PNG herunterladen".
- [ ] Klick auf „Animation abspielen": das GIF läuft einmal ab (in der gleichen Zwei-Hälften-Darstellung wie das Standbild) und bleibt auf dem Standbild stehen.
- [ ] Erneuter Klick startet die Animation von vorn (kein Hängen auf dem letzten Frame).
- [ ] GIF-Download liefert eine `.gif`-Datei, die sich abspielen lässt. Das Standbild am Ende entspricht Test 1; das Ablaufmuster kann von Test 2 abweichen, weil die Seite den JSON-Body neu serialisiert (`JSON.stringify(JSON.parse(…))`) und der Seed vom Body abhängt.
- [ ] PNG-Download liefert weiterhin das PNG.
- [ ] JSON im Textfeld ändern (z.B. Ziel auf „Hanau"): die Ansicht wechselt zurück auf das **Standbild** mit den neuen Daten; erneutes „Animation abspielen" spielt die Animation für die neuen Daten.
- [ ] Detailseite eines Templates **ohne** splitflap (`/sbb-096-v1`): weder „Animation abspielen" noch GIF-Download sind sichtbar.
- [ ] Galerie `/`: zeigt für alle Templates, auch `faltblatt-105-v1`, das statische PNG (keine Animation).
- [ ] Version unten links wird wie bisher angezeigt.

## 9. Desktop-Build: Editor

`go run ./cmd/zza serve --port 18081 --templates-dir ./templates` (oder `go run ./cmd/zza` für das Wails-Fenster) und `http://127.0.0.1:18081/edit/faltblatt-105-v1`.

- [ ] Alle Punkte aus Abschnitt 8 gelten auch auf der Desktop-Detailseite (inkl. „Bearbeiten"-Button).
- [ ] Editor-Vorschau hat einen Umschalter **Standbild | Animation**; Standard ist Standbild.
- [ ] Umschalten auf „Animation" spielt die Animation einmal ab, der Button **„Erneut abspielen"** startet sie wieder.
- [ ] YAML ändern und speichern (z.B. `flip_frames` oder `max_flips` ändern): Vorschau aktualisiert sich, im Animationsmodus mit der geänderten Animation.
- [ ] YAML mit Fehler speichern (z.B. `cells: 65`): Fehlermeldung wird angezeigt, kein Absturz; Datei ggf. wie beim Dual-Build-Test (400 bei Syntaxfehler).
- [ ] Editor eines Templates ohne splitflap (z.B. `/edit/sbb-096-v1`) zeigt keinen Umschalter und keinen „Erneut abspielen"-Button. Ob der Umschalter erscheint, wird beim Laden der Seite bestimmt: ein erst im Editor ergänzter `splitflap`-Layer zeigt ihn erst nach Neuladen der Seite.
- [ ] „Erneut abspielen" ist nur im Animationsmodus sichtbar.
- [ ] In der Wails-App: Fenster schließen beendet den Prozess sauber; GIF-Download funktioniert über den nativen Download.

## 10. Desktop-CLI

- [ ] `go run ./cmd/zza render -t faltblatt-105-v1 -i templates/faltblatt-105-v1/default.json -o /tmp/zza-manual/cli.gif --templates-dir ./templates` → `Rendered … to …`, Datei ist ein GIF.
- [ ] Dasselbe mit Endung `.GIF` (Großbuchstaben) → ebenfalls GIF.
- [ ] `… -o /tmp/zza-manual/cli.png` → PNG, bitgleich mit dem Server-Ergebnis aus Test 1.
- [ ] `… -o /tmp/zza-manual/cli.dat --format gif` → GIF; `--format png` mit `-o x.gif` → PNG (`--format` gewinnt über die Endung).
- [ ] `--format webp` → Fehler `unknown format "webp" (use png or gif)`, keine Ausgabedatei.
- [ ] Das CLI-GIF ist byte-identisch mit dem Server-GIF aus Test 2, wenn die JSON-Datei byte-identisch als Body gesendet wird (gleicher Seed).
- [ ] GIF für ein Template ohne splitflap (`sbb-096-v1`) → GIF mit einem Frame.

## 11. Regression und Build

- [ ] `go test ./...` grün.
- [ ] `go vet ./...` ohne Befund.
- [ ] `CGO_ENABLED=0 go build ./cmd/zza-server` erfolgreich.
- [ ] `go list -deps ./cmd/zza-server | grep -i wails` → leer.
- [ ] `git status`: keine ungewollten Änderungen; `git log develop..HEAD` enthält nur die Commits dieses Features.

## Nicht Teil dieses Plans

- Wiedergabe auf dem ESP32 (Firmware, läuft unabhängig; Backlog: „Firmware: GIF/MJPEG-Wiedergabe").
- Deployment auf `gen.yuv.de` und Release-ZIPs — erst nach Merge und Freigabe durch Christoph.
- Vergleich mit dem Legacy-Generator unter `https://zza.yuv.de/i/` für das Standbild: optional, zusätzlich zu Test 1. Der Abgleich Standbild gegen Legacy-Ausgabe derselben Daten sollte gleiche Elemente an gleicher Position (±1 px) zeigen.
