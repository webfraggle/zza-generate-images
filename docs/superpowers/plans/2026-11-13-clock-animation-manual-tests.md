# Frame-Animation + umuc-Uhr — Manueller Testplan

Alle Punkte vor einem Merge von `feature/clock-animation` → `develop` durchgehen. Bei einem fehlgeschlagenen Punkt nicht weitermachen.

**Stand:** Nichts davon ist gemerged, gepusht oder deployed. Alle Tests laufen lokal auf dem Branch `feature/clock-animation`.

## Vorbereitung

Referenz-GIF des Faltblatts **vor** dem Feature erzeugen (für Punkt 7):

```bash
mkdir -p /tmp/zza-manual
git checkout develop   # Arbeitsverzeichnis muss sauber sein
go run ./cmd/zza render -t faltblatt-105-v1 -i templates/faltblatt-105-v1/default.json -o /tmp/zza-manual/faltblatt-develop.gif --templates-dir templates
git checkout feature/clock-animation
```

Server lokal starten (Repo-Root, Branch `feature/clock-animation`):

```bash
mkdir -p /tmp/zza-manual
PORT=18080 TEMPLATES_DIR=./templates CACHE_DIR=/tmp/zza-manual/cache go run ./cmd/zza-server serve
```

Zweite Shell:

```bash
export B=http://localhost:18080
export D=templates/umuc-105-v1/default.json
```

Daten des Beispiels: U1 Mangfallplatz 10:15, U2 Messestadt Ost 10:18, U7 Neuperlach 10:26, Gleis 6, „Richtung Zentrum".

## 1. Standbild (PNG)

- [ ] `curl -s -D - -X POST $B/umuc-105-v1/render -H 'Content-Type: application/json' --data-binary @$D -o /tmp/zza-manual/umuc.png`. Erwartet: `200`, `Content-Type: image/png`, `X-Cache: BYPASS`, `Cache-Control: no-store`.
- [ ] Bild öffnen. Zwei Uhren (oben links, unten rechts) zeigen die **aktuelle Berliner Zeit**: Stundenzeiger korrekt zwischen zwei Stunden (z.B. 10:30 → genau zwischen 10 und 11), Minutenzeiger, dünner roter Sekundenzeiger.
- [ ] Zeiger drehen exakt um die Zifferblatt-Mitte, keine sichtbare Versetzung bei 6 Uhr bzw. 30 s.
- [ ] Spalte „Min." zeigt **5 / 8 / 16**, nicht „10:15".
- [ ] Badges U1, U2, U7 an gewohnter Stelle, „Gleis 6", „Richtung Zentrum", „Min.".
- [ ] Mit dem Original vergleichen (Legacy-Renderer bzw. Live-Instanz `https://gen.yuv.de/umuc-105-v1/preview`, Stand `v0.1.14`). Layout gleich, abgesehen von Zeigern und Minutenspalte.
- [ ] Zweiter Request wenige Sekunden später: `X-Cache: BYPASS`, der Sekundenzeiger steht anders. Die Uhr friert nicht mehr ein.

## 2. GIF (Frame-Modus)

- [ ] `curl -s -D - -w '%{time_total}\n' -X POST "$B/umuc-105-v1/render?format=gif" -H 'Content-Type: application/json' --data-binary @$D -o /tmp/zza-manual/umuc.gif`. Erwartet: `200`, `image/gif`, `X-Cache: BYPASS`.
- [ ] Renderzeit (Richtwert < 2 s): ______ s. Dateigröße (Richtwert ≤ 300 KB): ______ KB.
- [ ] Zusätzlich mit `-H 'X-Forwarded-Proto: http'`: **kein** 301, Antwort 200 + GIF.
- [ ] Struktur prüfen. Erwartet: `frames 60`, `loop 0`, alle Delays 100 (60 Frames, sofern keine identischen Folgeframes zusammengefasst wurden).

  ```bash
  mkdir -p /tmp/zza-manual/gifinfo && cat > /tmp/zza-manual/gifinfo/main.go <<'EOF'
  package main
  import ("fmt";"image/gif";"os")
  func main(){f,_:=os.Open(os.Args[1]);g,err:=gif.DecodeAll(f);if err!=nil{panic(err)};fmt.Println("frames",len(g.Image),"loop",g.LoopCount,"delays",g.Delay)}
  EOF
  go run /tmp/zza-manual/gifinfo/main.go /tmp/zza-manual/umuc.gif
  ```
- [ ] GIF im Browser (Datei per Drag & Drop) und in Vorschau/Quick Look öffnen. Der Sekundenzeiger springt jede Sekunde 6° weiter, in **beiden** Uhren. Stunde und Minute stehen still.
- [ ] Nach 60 s beginnt das GIF von vorn, **ohne Sprung**: Der Zeiger ist wieder an der Startposition, und es gibt keine 1-s-Pause am Ende.
- [ ] Startposition des Sekundenzeigers ≈ Sekunde des Requests (±1–2 s).

## 3. Grenzfälle der Minutenspalte

Mit eigenem JSON (Kopie von `default.json`, Felder ändern), jeweils PNG:

- [ ] `zug1.zeit: "23:58"`, `zug2.zeit: "00:04"`, `zug3.zeit: "00:10"` → 5 / 11 / 17.
- [ ] `zug3.zeit: ""` → Zeile 3 ohne Minutenwert, kein Fehler. Mit `zug3.vonnach: ""` und `zug3.nr: ""` ist Zeile 3 komplett leer.
- [ ] `zug2.zeit: "ca. 10"` → Zeile 2 ohne Minutenwert.
- [ ] `zug1.zeit: ""` → alle Minutenwerte leer (keine Fantasiewerte wie 618).
- [ ] `zug2.nr: "U9"` → statt Badge der Text „U9". `zug3.nr: "Bus 54"` → Text.

## 4. Zeitzone

- [ ] `TZ=UTC` setzen und den Server neu starten (`TZ=UTC PORT=18080 … serve`). PNG rendern: Die Uhr zeigt trotzdem die Berliner Zeit.
- [ ] Testkopie eines Templates mit `meta.timezone: "Mars/Olympus"` (in einem temporären `TEMPLATES_DIR`) → Render `404`/Fehler. Im Log steht `invalid meta.timezone`.
- [ ] Dieselbe Kopie mit `meta.timezone: "America/New_York"` → Uhr 6 Stunden zurück (bzw. 5 während der Umstellungswochen).

## 5. Cache-Verhalten

- [ ] `sbb-096-v1` zweimal rendern → `MISS`, dann `HIT` (unverändert).
- [ ] `nederland-105-v1` zweimal → beide `BYPASS`. Die Uhr läuft dort jetzt auch weiter.
- [ ] `ls -R /tmp/zza-manual/cache` enthält keine Einträge für `umuc-105-v1`/`nederland-105-v1`. Den Cache-Key kennt man nicht, daher reicht: Anzahl Dateien vor und nach den umuc-Requests ist gleich.
- [ ] Last: `for i in 1 2 3 4 5; do curl -s -o /dev/null -w '%{http_code}\n' -X POST "$B/umuc-105-v1/render?format=gif" -H 'Content-Type: application/json' --data-binary @$D & done; wait` ergibt nur 200 (oder 503 mit `Retry-After`, falls > 15 s Wartezeit). Der Server bleibt reaktionsfähig (`curl $B/health`).

## 6. Browser: Detailseite und Editor

- [ ] Galerie `$B/` lädt, Vorschaubild `umuc-105-v1` zeigt die aktuelle Zeit.
- [ ] Detailseite `$B/umuc-105-v1`: Buttons „Animation abspielen" und „GIF herunterladen" sind vorhanden. „Animation abspielen" zeigt das laufende GIF in beiden Hälften. Erneuter Klick startet neu mit aktueller Zeit. Eine JSON-Änderung schaltet zurück aufs Standbild. Der GIF-Download speichert `umuc-105-v1.gif`.
- [ ] Detailseite `sbb-096-v1`: weiterhin **keine** Animations-Buttons.
- [ ] Desktop-App (`go run ./cmd/zza` bzw. `serve`): Editor `/edit/umuc-105-v1`, Vorschau-Modus „Animation", GIF läuft in Schleife, „Erneut abspielen" startet neu.
- [ ] Im Editor auf den Node-Tab wechseln und zurück, ohne etwas zu ändern: YAML unverändert.
- [ ] Node-Tab: ein Text-Node mit der Minutenformel zeigt die Filter-Chips `minutes`, `sub(zug1.zeit | minutes)`, `mod(1440)`, `add(5)`. Die Live-Vorschau zeigt für Zug 2 `8`. Eine Kleinigkeit ändern (z.B. `add(5)` → `add(6)`) und zurück zu YAML: Die Formel ist intakt (`sub(zug1.zeit | minutes)` nicht zerschnitten).

## 7. Regression

- [ ] `go run ./cmd/zza render -t faltblatt-105-v1 -i templates/faltblatt-105-v1/default.json -o /tmp/zza-manual/faltblatt-branch.gif --templates-dir templates`, dann `cmp /tmp/zza-manual/faltblatt-develop.gif /tmp/zza-manual/faltblatt-branch.gif` → keine Ausgabe (bytegleich).
- [ ] Faltblatt-GIF spielt weiterhin **einmal** und bleibt auf dem Standbild stehen.
- [ ] PNG-Render von `rhb-105-v1` und `sbb-105-v1` sieht unverändert aus (Sichtvergleich mit `develop`).
- [ ] `umuc-096-v1` ist unverändert (`git diff develop -- templates/umuc-096-v1` leer).
- [ ] `CGO_ENABLED=0 go build ./cmd/zza-server` und `GOOS=windows GOARCH=amd64 go build ./cmd/zza-server` bauen.

## 8. Abschluss

- [ ] `go test ./...`, `go vet ./...`, `node --test web/static/test/*.mjs` grün.
- [ ] Ergebnisse (Zeiten, Größen, Auffälligkeiten) hier notieren:
