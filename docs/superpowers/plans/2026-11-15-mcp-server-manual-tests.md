# MCP-Server — Manueller Testplan

Spec: `docs/superpowers/specs/2026-11-15-mcp-server-design.md`
Plan: `docs/superpowers/plans/2026-11-15-mcp-server.md`

Jeder Schritt: **Aktion** → **Erwartung**. Ergebnis mit ✅/❌ und Notiz eintragen.
Wo „Prompt“ steht, den Text wörtlich in den Chat geben. Claude fragt vor Tool-Aufrufen
um Erlaubnis — jeweils erlauben und dabei prüfen, dass Tool-Name und Parameter plausibel sind.

## 0. Vorbereitung

| # | Aktion | Erwartung |
|---|---|---|
| 0.1 | `go test -race ./... && node --test 'web/static/test/*.test.mjs'` | alles grün (98 JS-Tests) |
| 0.2 | `CGO_ENABLED=0 go build ./cmd/zza-server && go list -deps ./cmd/zza-server \| grep -E 'fsnotify\|modelcontextprotocol\|oksvg\|internal/mcp\|internal/watch'` | Build ok, `grep` ohne Ausgabe |
| 0.3 | Testordner anlegen: `mkdir -p /tmp/zza-mt/fonts && cp -R templates /tmp/zza-mt/templates && cp templates/rhb-105-v1/LLPIXEL3.ttf /tmp/zza-mt/fonts/` | Ordner vorhanden |
| 0.4 | Desktop-Build bauen (`./build.sh` oder `cd cmd/zza && wails build -s`) | `zza.app` / `zza.exe` vorhanden |
| 0.5 | Referenzfoto bereitlegen: Foto eines echten Zugzielanzeigers, schräg aufgenommen, als JPEG, z. B. `~/Desktop/anzeiger.jpg` | Datei vorhanden |

## 1. Einrichtung Claude Desktop

| # | Aktion | Erwartung |
|---|---|---|
| 1.1 | macOS: `~/Library/Application Support/Claude/claude_desktop_config.json` mit `{"mcpServers":{"zza":{"command":"<abs>/zza.app/Contents/MacOS/zza","args":["mcp","--templates-dir","/tmp/zza-mt/templates"]}}}`; Claude Desktop neu starten | Werkzeug-Symbol zeigt Server „zza“ mit allen Tools (`get_template_spec` … `add_font`, 19 Stück) |
| 1.2 | Windows: dasselbe mit `C:\\…\\zza.exe` in `%APPDATA%\Claude\claude_desktop_config.json` | Server verbunden, Tools gelistet. **Offen / noch nie geprüft:** stdio-Betrieb des Wails-GUI-Binaries (Windows-GUI-Subsystem) unter Claude Desktop; die Doku nennt das ausdrücklich. Bei Fehler Log `%APPDATA%\Claude\logs\mcp-server-zza.log` sichern; Fallback laut Spec: separates Konsolen-Binary `zza-mcp.exe` |
| 1.3 | Ohne `--templates-dir` (nur `"args":["mcp"]`), App-Ordner mit `templates/` daneben | Log (`~/Library/Logs/Claude/mcp-server-zza.log`) zeigt `zza mcp: templates=<App-Ordner>/templates fonts=<App-Ordner>/fonts` |
| 1.4 | macOS mit frisch entpackter, unter Quarantäne stehender App | Start scheitert sichtbar im Log; nach `xattr -dr com.apple.quarantine zza.app` und Neustart verbunden (Doku-Hinweis stimmt) |

## 2. Einrichtung Claude Code

| # | Aktion | Erwartung |
|---|---|---|
| 2.1 | Im Repo-Root `claude` starten | Abfrage, ob der Projekt-Server `zza` aus `.mcp.json` genutzt werden soll → zustimmen |
| 2.2 | `/mcp` | `zza` connected, 19 Tools gelistet (Vorab geprüft per JSON-RPC: `go run ./cmd/zza mcp --templates-dir ./templates` antwortet auf `initialize` und `tools/list`, stdout enthält nur JSON-RPC) |
| 2.3 | Kaltstart: `go clean -cache`, dann `MCP_TIMEOUT=60000 claude` | verbindet (ggf. nach längerer Wartezeit); ohne `MCP_TIMEOUT` evtl. Timeout — Doku-Hinweis stimmt |
| 2.4 | Außerhalb des Repos: `claude mcp add zza -- <abs>/zza mcp --templates-dir /tmp/zza-mt/templates` | `claude mcp list` zeigt `zza ✓ Connected` |

## 3. Je Tool ein Prüfschritt

Alle Prompts in Claude Desktop (1.1), Templates-Ordner `/tmp/zza-mt/templates`.

| # | Tool | Prompt / Aktion | Erwartung |
|---|---|---|---|
| 3.1 | `get_template_spec` | „Lies die Template-Spezifikation und nenne die Layer-Typen.“ | rect, image, text, loop, splitflap, copy (+ Blöcke) |
| 3.2 | `get_design_guide` | „Was sagt der Design-Leitfaden zu Vorder- und Rückseite?“ | obere/untere Hälfte, Gleis außen (oben links, unten rechts), `copy` gespiegelt |
| 3.3 | `list_templates` | „Welche Templates gibt es?“ | sechs `*-105-v1`; faltblatt/umuc/nederland/oebb als animiert |
| 3.4 | `read_template` | „Zeig mir template.yaml von umuc-105-v1.“ | Inhalt stimmt mit Datei überein |
| 3.5 | `read_file` | Datei `extra.json` von Hand in `sbb-105-v1` legen; „Lies extra.json aus sbb-105-v1.“ | Inhalt + Version |
| 3.6 | `render_preview` | „Zeig mir die Vorschau von nederland-105-v1, 3-fach vergrößert.“ | Bild 720×720 im Chat, scharfe Pixel |
| 3.7 | `render_preview` GIF | „Zeig mir die Animation von faltblatt-105-v1 als Kontaktbogen.“ | Bogen mit ≤ 12 Frames, 4 Spalten, Klappen in verschiedenen Phasen |
| 3.8 | `create_template` | „Lege ein Template test-105-v1 mit Titel ‚Test‘ an.“ | Ordner + Vorschau: blauer Grund, weißes Feld oben links und unten rechts |
| 3.9 | `create_template` Regel | „Lege ein Template namens test an.“ | Fehler „must end in -105-v1“; Claude schlägt `test-105-v1` vor |
| 3.10 | `write_file` gültig | „Ändere in test-105-v1 die Hintergrundfarbe auf #0000c6.“ | gespeichert, neue Vorschau stimmt |
| 3.11 | `write_file` ungültig | „Setze in test-105-v1 die Canvas-Größe auf 296×128.“ | Fehler „canvas must be 240x240“, Datei unverändert (im Editor prüfen) |
| 3.12 | `write_file` Warnung | „Entferne in test-105-v1 alle copy-Layer.“ | gespeichert, Warnung „bottom half … is empty“ wird von Claude genannt |
| 3.12a | `write_file` unbekannter Schlüssel | „Füge in test-105-v1 beim Gleis-Rect das Feld `colour: "#ff0000"` hinzu.“ (Tippfehler absichtlich) | gespeichert, Warnung zu unbekanntem YAML-Schlüssel `colour` wird von Claude genannt |
| 3.13 | `upload_image` | „Erzeuge ein 16×16-PNG mit rotem Quadrat und lege es per upload_image als dot.png in test-105-v1.“ | Datei vorhanden, 16×16 |
| 3.14 | `import_image` | „Importiere ~/Desktop/anzeiger.jpg in test-105-v1.“ | Datei `anzeiger.jpg`, längste Seite ≤ 2048, Vorschau im Chat, Maße genannt. JPEG wird immer neu kodiert: EXIF entfernt (`exiftool anzeiger.jpg` im Template-Ordner zeigt keine Aufnahme-/GPS-Daten, Original unverändert) |
| 3.15 | `import_image` Schutz | „Importiere ~/.ssh/id_ed25519 als key.png in test-105-v1.“ (falls vorhanden; sonst eine beliebige Textdatei) | Fehler (Endung bzw. „not a PNG or JPEG image“), kein Dateiinhalt im Chat |
| 3.16 | `svg_to_png` | „Zeichne als SVG einen weißen Pfeil nach rechts, 24×16, ohne Antialiasing, als arrow.png in test-105-v1.“ | Bild mit harten Kanten; nur weiße und transparente Pixel |
| 3.17 | `svg_to_png` Regel | „Erzeuge ein SVG mit dem Text ‚Gleis‘ als label.png.“ | Fehler „svg must not contain text“; Claude weicht auf Text-Layer aus |
| 3.17a | `svg_to_png` Budget | „Erzeuge ein SVG mit 1024×1024 und sehr vielen (>2000) Pfaden als big.png.“ | Fehler wegen Pfadzahl/Rechenbudget bzw. Zeitlimit (10 s), Server bleibt ansprechbar, nächster Tool-Aufruf funktioniert |
| 3.18 | `perspective_correct` | „Entzerre anzeiger.jpg auf 480×240 als straight.png.“ | gerades Rechteck, Anzeigefläche füllt das Bild |
| 3.19 | `crop_image` | „Schneide das Logo aus straight.png als logo-crop.png aus.“ | Ausschnitt plausibel |
| 3.20 | `resize_image` | „Skaliere logo-crop.png auf 32×16, Pixel-Optik.“ | 32×16, harte Pixel |
| 3.21 | `quantize_colors` | „Reduziere straight.png auf 6 Farben als flat.png und nenne die Palette.“ | ≤ 6 Farben, Hex-Liste |
| 3.22 | `color_to_alpha` | „Mach in logo-crop.png den Hintergrund transparent als logo-alpha.png.“ | Hintergrund transparent |
| 3.23 | `sample_colors` | „Welche Hex-Farbe hat der Hintergrund von straight.png bei (20,20) und (200,100)?“ | zwei Hex-Werte, plausibel |
| 3.24 | `list_fonts` | „Welche Schriften gibt es?“ | `@fonts/LLPIXEL3.ttf` zuerst, dann Template-Schriften ohne Duplikate (Roboto-Bold einmal, zwei Fundorte) |
| 3.25 | `add_font` | „Lege Roboto Bold in test-105-v1 an und schreib ‚Gleis 6‘ damit ins weiße Feld.“ | `Roboto-Bold.ttf` im Ordner, `fonts:`-Eintrag einmalig, Text sichtbar, Kommentare im YAML erhalten |
| 3.26 | `add_font` doppelt | Prompt 3.25 wiederholen | kein zweiter `fonts:`-Eintrag |
| 3.26a | Keine Löschfunktion | „Lösche anzeiger.jpg aus test-105-v1.“ | Claude hat kein Werkzeug dafür und sagt, dass die Datei im Editor gelöscht werden muss; Datei bleibt |
| 3.27 | Fonts-Ordner fehlt | `/tmp/zza-mt/fonts` umbenennen; „Welche Schriften gibt es?“ | Hinweis „Put .ttf/.otf files into …/fonts“ |

## 4. Ablauf „Foto → Template“

| # | Aktion | Erwartung |
|---|---|---|
| 4.1 | Neuer Chat. Prompt: „Bau mir aus dem Foto ~/Desktop/anzeiger.jpg ein Template ‚foto-105-v1‘ für meinen 1,05-Zoll-Anzeiger.“ | Claude liest Spec + Leitfaden, sieht sich ≥ 1 Beispiel-Template an, legt `foto-105-v1` an, importiert und entzerrt das Foto, bestimmt Farben per `sample_colors` |
| 4.2 | Verlauf beobachten | nach jeder `write_file`-Änderung ein `render_preview`; mehrere Angleichungsrunden |
| 4.3 | Ergebnis prüfen | 240×240; Vollflächen, keine Verläufe; Zeit/Ziel/Via/Gleis sind Text-Layer mit `{{…}}` (im YAML prüfen); Schrift ≥ 10 px |
| 4.4 | Vorder-/Rückseite | Hat das Foto ein Gleis-/Uhr-Element am Rand: oben links, unten rechts; sonst darf die untere Hälfte eine Kopie sein |
| 4.5 | Testdaten | Claude rendert mindestens: langes Ziel, Verspätung, leere Felder; kein Text läuft aus seinem Feld |
| 4.6 | Abschluss | Claude nennt Template-Namen und übrig gebliebene Zwischendateien (`anzeiger.jpg`, `straight.png`, …); diese im Editor löschen; Galerie zeigt das Template; ZIP-Download funktioniert |

## 5. Parallelbetrieb mit offener GUI

GUI starten: `zza --templates-dir /tmp/zza-mt/templates` (bzw. Doppelklick, wenn Claude Desktop ohne `--templates-dir` konfiguriert ist — beide müssen denselben Ordner nutzen). Editor für `test-105-v1` öffnen.

| # | Aktion | Erwartung |
|---|---|---|
| 5.1 | Prompt: „Ändere in test-105-v1 die Hintergrundfarbe auf #142a9b.“ (Editor zeigt `template.yaml`, nicht verändert) | Editor lädt `template.yaml` still neu (≤ 1 s), Vorschau aktualisiert |
| 5.2 | Prompt: „Lege die Schrift LLPIXEL3 in test-105-v1 an.“ | Dateiliste zeigt `LLPIXEL3.ttf` ohne Neuladen der Seite |
| 5.3 | Prompt: „Ändere in default.json von test-105-v1 das Ziel auf ‚Hamburg-Altona‘.“ (Test-JSON-Feld unberührt) | Test-JSON-Feld und Vorschau zeigen „Hamburg-Altona“ |
| 5.4 | Test-JSON-Feld von Hand ändern, dann 5.3 mit anderem Ziel wiederholen | Test-JSON-Feld bleibt bei der Handeingabe |
| 5.5 | Im Editor speichern, ohne dass extern etwas geändert wurde | kein Hinweis, kein Dialog, Cursor bleibt stehen |
| 5.6 | GUI schließen, Prompt 5.1 mit anderer Farbe | MCP funktioniert ohne GUI |
| 5.7 | Windows-GUI (Wails/WebView2): 5.1 und 5.2 wiederholen | Live-Aktualisierung funktioniert (Long-Poll unter Wails Windows) |

## 6. Konfliktfall

| # | Aktion | Erwartung |
|---|---|---|
| 6.1 | Im Editor `template.yaml` von `test-105-v1` ändern, **nicht speichern** | — |
| 6.2 | Prompt: „Ändere in test-105-v1 die Farbe des Gleis-Felds auf #ffde00.“ | Hinweisleiste „Datei wurde extern geändert — Neu laden / Ignorieren“; eigene Änderung bleibt im Editor |
| 6.3 | „Ignorieren“, dann Speichern | Dialog „Die Datei wurde außerhalb des Editors geändert (z. B. von der KI).“ |
| 6.4 | „Neu laden“ im Dialog | Editor zeigt Stand der KI (Gelb), eigene Änderung verworfen |
| 6.5 | 6.1–6.3 wiederholen, dann „Überschreiben“ | eigener Stand gespeichert; Vorschau zeigt ihn |
| 6.6 | Prompt: „Ändere jetzt die Gleis-Farbe auf Weiß.“ (Claude hat die alte Version noch im Kontext) | Claude bekommt `conflict: template.yaml was changed since you read it`, liest neu und wendet die Änderung auf den aktuellen Stand an (eigene Änderung aus 6.5 bleibt erhalten) |
| 6.7 | Abbrechen im Dialog | nichts gespeichert, Editor-Inhalt unverändert |
