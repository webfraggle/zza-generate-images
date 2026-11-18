# ZZA auf dem Gerät, Stufe 2 — Manueller Testplan

Spec: `docs/superpowers/specs/2026-11-16-zza-on-device-rendering-design.md`
Plan: `docs/superpowers/plans/2026-11-16-zza-on-device-stage2.md`

Jeder Schritt: **Aktion** → **Erwartung**. Ergebnis mit ✅/❌ und Notiz eintragen.
Auf einem Mac mit defekten Command Line Tools vor jedem cgo-Befehl:
`export CGO_CXXFLAGS="-nostdinc++ -isystem $(xcrun --show-sdk-path)/usr/include/c++/v1"`

## 0. Automatische Prüfungen

| # | Aktion | Erwartung |
|---|---|---|
| 0.1 | `go test -race ./...` | grün (der cgo-Teil kann mit `-race` mehrere Minuten brauchen) |
| 0.2 | `go vet ./... && node --test 'web/static/test/*.test.mjs'` | keine Meldungen, JS-Tests grün |
| 0.3 | `CGO_ENABLED=0 go build ./cmd/zza-server && go list -deps ./cmd/zza-server \| grep -E 'fsnotify\|modelcontextprotocol\|oksvg\|rasterx\|internal/mcp\|internal/watch\|internal/devicerender\|internal/devpreview'` | Build ok, `grep` ohne Ausgabe |

## 1. Fuzzing

Je Befehl ein Lauf, danach `git status`: es darf kein neuer Ordner `testdata/fuzz/…` mit Crash-Eingaben entstehen.

| # | Aktion | Erwartung |
|---|---|---|
| 1.1 | `go test ./internal/devicerender -run '^$' -fuzz=FuzzLoad -fuzztime=60s` | endet ohne Fund (Paket-Lader: kein Absturz, kein Hang) |
| 1.2 | `go test ./internal/devicerender -run '^$' -fuzz=FuzzRegex -fuzztime=60s` | ohne Fund (Regex-Programme: Go ↔ C++ gleich) |
| 1.3 | `go test ./internal/devicerender -run '^$' -fuzz=FuzzEvalData -fuzztime=60s` | ohne Fund (Zugdaten-JSON, Ausdrücke) |

## 2. Build mit Gerätekern (`./build.sh`)

Achtung: `./build.sh` erhöht `VERSION` und baut ein Docker-Image. Nur auf dem Release-Rechner laufen lassen, `VERSION` nicht versehentlich committen.

| # | Aktion | Erwartung |
|---|---|---|
| 2.1 | `./build.sh` (ohne `DOCKER_PUSH`) | drei ZIPs in `dist/release/`: `zza-macos-arm64.zip`, `zza-macos-intel.zip`, `zza-windows-x64.zip`; `Done: … 0 failed` |
| 2.2 | macOS-Build mit defekten CLT | Hinweiszeile „darwin builds use CGO_CXXFLAGS=…“, kein Compilerfehler `'cstdint' file not found` |
| 2.3 | Windows-Build aus dem Log `dist/wails-windows-amd64.log` | keine Compiler-Fehler; mingw-w64 kompiliert den C++-Kern (die macOS-Flags dürfen dort nicht auftauchen) |
| 2.4 | Größen der Desktop-Binaries vergleichen (vorher = Stand vor Stufe 2, `git stash`/Commit `942a6b0`; nachher = jetzt) | Referenz (Vorab-Messung, `go build -s -w`, macOS arm64): vorher 11,1 MB, nachher 11,5 MB; Wails-`.app`-Binary arm64 12,8 MB, amd64 13,7 MB; Windows-`.exe` 12,7 MB. Eigene Werte hier eintragen, Zuwachs ≈ 0,4 MB |
| 2.5 | ZIPs entpacken, jeweils starten (macOS: Rechtsklick → Öffnen, ggf. `xattr -dr com.apple.quarantine`) | App startet, Editor öffnet sich |

## 3. Export

| # | Aktion | Erwartung |
|---|---|---|
| 3.1 | `zza export -t faltblatt-105-v1 -o /tmp/x.zip` (aus dem entpackten ZIP, in jedem der drei Builds) | „Exported faltblatt-105-v1 to /tmp/x.zip (… bytes)“, Größe unter 3 MB (Referenz ≈ 304 KB) |
| 3.2 | `/tmp/x.zip` in einem ZIP-Tool öffnen | `manifest.json` ist der erste Eintrag; daneben `t.json`, Bild-, Schrift- und ggf. Regex-Daten; keine unlesbaren Namen |
| 3.3 | Export für jedes mitgelieferte Template: `faltblatt`, `nederland`, `oebb`, `rhb`, `sbb`, `umuc`, `wienerlinien` (je `-105-v1`) | alle exportierbar (Latin-1 in `rhb`/`sbb` ist bereits angepasst) |
| 3.4 | Ein Test-Template mit Canvas 320 × 240 bzw. Zeichen „€“ in einem Text exportieren (Kopie unter `/tmp/zza-mt/templates`) | Abbruch mit „nicht ZZA-kompatibel: …“ und konkretem Grund, kein ZIP |
| 3.5 | Test-Template mit `match(name, "{{pattern}}")` (nicht konstantes Muster) | „Regex-Muster muss konstant sein“ |
| 3.6 | Test-Template mit ungültigem Muster `match(x, "(")` | Ablehnung wie bei Go-`regexp` |

## 4. ZZA-Vorschau im Editor (Desktop)

Editor öffnen (`zza` bzw. `zza serve`, dann `/edit/<template>`).

| # | Aktion | Erwartung |
|---|---|---|
| 4.1 | Umschalter **Server / ZZA** sichtbar neben der Vorschau | Auswahl „Server“ (Standard) und „ZZA“ |
| 4.2 | Je mitgeliefertem Template (7 Stück) auf **ZZA** stellen, Modus Standbild | Bild entspricht dem Server-Bild bis auf Kantenglättung bei Text (ganze Pixel) |
| 4.3 | `faltblatt-105-v1` auf ZZA, Modus **Animation** | Split-Flap-Ablauf spielt einmal ab, endet auf dem Standbild; Ablauf wie im Server-GIF |
| 4.4 | `umuc-105-v1` auf ZZA, Modus **Animation** | Uhr-Schleife (60 Frames, Sekundenzeiger läuft), Endlosschleife |
| 4.5 | Ein nicht animiertes Template (z. B. `sbb-105-v1`) im Modus Animation auf ZZA | GIF mit einem Frame, kein Fehler |
| 4.6 | Datei im Editor ändern (z. B. Text- oder Farbwert) | ZZA-Vorschau rendert neu aus den Dateien auf der Platte |
| 4.7 | Test-Template mit Canvas ≠ 240 × 240 oder „€“ in Konstante auf ZZA | statt eines Bildes Meldung „nicht ZZA-kompatibel:“ mit je einem Grund pro Zeile |
| 4.8 | Seite neu laden | Auswahl Server/ZZA ist je Template gemerkt |
| 4.9 | Zugdaten-JSON mit Zeichen > U+00FF (z. B. „Łódź“, „日本“) im Daten-Editor | auf ZZA erscheinen `?` statt dieser Zeichen; Server-Vorschau zeigt die echten Zeichen |
| 4.10 | `zza-server` starten (`PORT=18080 TEMPLATES_DIR=./templates go run ./cmd/zza-server serve`) | Galerie läuft wie bisher, keine ZZA-Vorschau, `/edit/…` gibt es nicht |

## 5. Windows

| # | Aktion | Erwartung |
|---|---|---|
| 5.1 | `zza.exe` aus `zza-windows-x64.zip` auf Windows starten (SmartScreen: „Trotzdem ausführen“) | Editor-Fenster öffnet sich |
| 5.2 | ZZA-Vorschau (4.2 bis 4.4) unter Windows | gleiche Bilder wie auf dem Mac (mingw-Kern); Animation läuft |
| 5.3 | `zza.exe export -t faltblatt-105-v1 -o x.zip` in der Eingabeaufforderung | Paket gleicher Größe wie auf dem Mac |

## 6. Bekannte Abweichungen (kein Fehler)

- Text-Kanten: Server-Vorschau (Subpixel) und ZZA-Vorschau (ganze Pixel) können sich an Glyphenkanten unterscheiden, besonders bei vielfach übereinander gezeichnetem Text. Maßgeblich ist die ZZA-Vorschau.
- Stufe 3 (Firmware-Integration, Gerätetests) ist nicht Teil dieses Plans (siehe `docs/backlog.md`).
