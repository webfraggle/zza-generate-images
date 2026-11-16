# Templates direkt auf dem ZZA rendern — Design

Datum: 2026-11-16 · Status: **Entwurf zur Freigabe** · Löst `2026-11-13-esp32-on-device-rendering-design.md` ab.

Legende: **[C]** = von Christoph entschieden · **[P]** = Entscheidung des Planers (darf Christoph kippen) · **[B]** = dem Umsetzer überlassen · **[?]** = offen, vor der jeweiligen Stufe klären

Zwei Repos:
- **Generator:** `zza-generate-images` (dieses Repo, Go)
- **Firmware:** `Zugzielanzeiger-GIT` (PlatformIO, C++)

## 1. Absicht und Warum

Templates lassen sich inzwischen schnell und gut bauen: in der Desktop-App, im Editor und mit dem KI-Assistenten (MCP). Damit wächst der Bedarf, sie **direkt auf dem ZZA zu rendern**. Der Ablauf soll so aussehen:

1. Template in der Desktop-App bauen und testen.
2. Als Paket exportieren und auf den ZZA übertragen.
3. Der ZZA rendert das Bild selbst aus den Zugdaten, die er ohnehin hat.

Was das bringt:
- Der ZZA arbeitet offline, ohne gen.yuv.de und ohne Rundreise ins Netz.
- Die Serverlast entfällt.
- Split-Flap läuft live auf dem Display.
- Neue Layouts brauchen kein Firmware-Update.

## 2. Ziele

1. **[C]** Templates sind eine **Zusatzoption**. Die eingebauten Layouts (`DisplayV5x2`) bleiben unverändert. Die GIF-Wiedergabe in der Firmware bleibt ebenfalls erhalten.
2. **[C]** Zielhardware:
   - Controller: ESP32-S3 **N16R8** (16 MB Flash, 8 MB PSRAM), PlatformIO-Board `lolin_s3`.
   - Displays: 1,05"-Panels mit GC9A01-Treiber, je **240 × 120** sichtbar.
   - Umgebungen: `ZZA_Universal_Display_Controller_105_DUAL_*` und `…_QUAD_*`.
   - **DUAL** = ein Gleis (Außenbahnsteig) mit zwei Displays, Vorder- und Rückseite (D1/D2).
   - **QUAD** = zwei Gleise (Mittelbahnsteig, Gleis A und B) mit je zwei Displays, also vier Displays.
   - **Template-Canvas 240 × 240 = ein Gleis** **[C]**. Die beiden Hälften zu je 240 × 120 gehen auf Vorder- und Rückseite, genau wie heute beim PNG/GIF in `DisplayPng`: obere Hälfte in `sprite2` (D2), untere in `sprite1` (D1).
3. **[C]** Funktionsumfang v1 = **was das Template-Format heute kann** (`docs/yaml-template-spec.md`), nicht mehr. Die Wagenreihung (`reihung`) aus den Zugdaten wird für Templates nicht ausgewertet.
4. **[C]** **Beide Animationen** gehören in v1:
   - Split-Flap läuft live, einmal pro Zugwechsel.
   - Frame-Modus (Uhren) spielt als Schleife ab, wie das heutige GIF.
5. **[C]** Zeichensatz **Latin-1** (ISO 8859-1).
6. **[C]** Gleichheit nach **Weg B** (§5): Es gibt **einen** C++-Renderer. Das Gerät nutzt ihn, und die Desktop-App bindet ihn per cgo für eine „ZZA-Vorschau“ ein. Die ZZA-Vorschau ist damit pixelgleich mit dem Gerät.
7. **[C]** Der C++-Code wird **zwischen den Repos kopiert**. Ein eigenes Bibliotheks-Repo gibt es vorerst nicht.

## 3. Nicht-Ziele

- S2 Mini und 0,96"-ST7735.
- Zeichen außerhalb von Latin-1.
- YAML-Parser oder TrueType-Rasterizer auf dem Gerät.
- Bearbeiten von Templates auf dem Gerät oder in der Firmware-Web-UI.
- **Echte Uhrzeit in v1.** Für Tests liefert das Gerät eine zufällige Uhrzeit **[C]**. NTP bzw. RTC ist eine spätere Stufe **[C]**.
- **Live-Uhr in v1.** Uhren laufen als Schleife über die `frames` des Frame-Modus **[C]**.
- Direkter Upload von gen.yuv.de oder aus der Desktop-App auf den ZZA, also ohne den Umweg „ZIP herunterladen, in der Firmware-Web-UI hochladen“.
- Ein Geräte-Modus im Go-Renderer. Er entfällt durch Weg B.
- Pixelgleichheit zwischen **Server-Vorschau** (Go) und Gerät. Pixelgleich zum Gerät ist nur die ZZA-Vorschau.

## 4. Anti-Ziele

- Displays ohne installiertes Template verhalten sich anders als heute.
- ZZA-Vorschau und Gerät weichen stillschweigend voneinander ab.
- Ein Template mit Funktionen, die das Gerät nicht kann, wird übertragen und rendert dort „irgendwie“.
  - Der Exporter muss es ablehnen.
  - Das Gerät lehnt es zusätzlich selbst ab.
- Ein Template oder Zugdaten lösen unbegrenzten RAM- oder Rechenaufwand aus, etwa durch sehr lange Texte, `scale_x` oder viele Module.
- Ein defektes Paket bringt den ZZA in eine Boot-Schleife oder legt das bisherige Template lahm.
- Der Server-Build (`cmd/zza-server`) bekommt cgo oder C++.

## 5. Ansätze

| | Weg | Vorteil | Nachteil |
|---|---|---|---|
| A | Go-Renderer bekommt einen Geräte-Modus; C++-Renderer separat; Referenztests vergleichen bitweise | Desktop bleibt reines Go | Geräte-Logik doppelt implementiert und gepflegt |
| **B** | **Ein C++-Renderer; die Desktop-App nutzt ihn per cgo für die ZZA-Vorschau** | Nur eine Implementierung; Vorschau = Gerät per Konstruktion | C++ im Desktop-Build; Code in zwei Repos |
| C | Gerät rendert ohne Gleichheitsversprechen | Am billigsten | Bricht das Anti-Ziel „stillschweigende Abweichung“ |

**Gewählt: B [C].** Warum:
- Neue Template-Funktionen werden nur einmal für das Gerät gebaut.
- Die Vorschau, auf die man sich beim Bauen verlässt, *ist* der Geräte-Code.
- Der Desktop-Build nutzt wegen Wails ohnehin schon cgo, und mingw für Windows ist im `build.sh` vorhanden.

**Code-Weitergabe: Kopie zwischen den Repos [C].**

## 6. Architektur

```
Generator-Repo (Go + C++)                         Firmware-Repo (C++)
───────────────────────────                       ───────────────────
templates/<name>/ (YAML, PNG, OTF/TTF)
      │ internal/devexport (Go, CGO-frei)
      ▼
<name>-zza.zip ── Browser (Firmware-Web-UI) ──▶  LittleFS /tpl_<name>/
                  entpackt, lädt Datei für Datei
      │                                                   │
      ▼                                                   ▼
device/zzarender/  ═══ Kopie (tools/sync) ═══▶  lib/zzarender/
 (C++17-Kern, ohne Arduino)                      + src/DisplayTemplate.* (Adapter)
      │ internal/devicerender (cgo, nur Desktop)          │
      ▼                                                   ▼
ZZA-Vorschau im Editor, Tests                     DisplayBuffer → TFT_eSPI
```

### 6.1 Wo der Code liegt [P]

- **Master-Kopie** des C++-Kerns: `device/zzarender/` im Generator-Repo. Begründung:
  - Hier laufen die Tests gegen den Go-Renderer und die Templates.
  - Desktop-Vorschau und Exporter liegen im selben Repo.
- **Kopie in der Firmware:** `lib/zzarender/`. Sie wird **nie direkt bearbeitet**.
  - Script: `tools/sync-zzarender.sh <pfad-zum-firmware-repo>`.
  - Es kopiert den Kern und schreibt `lib/zzarender/SOURCE.txt` mit Commit-Hash und Datum.
  - Ein Check-Modus (`--check`) meldet, wenn die Kopie abweicht.
- **Abhängigkeiten des Kerns:**
  - nur C++17-Standardbibliothek und ArduinoJson (header-only, läuft auch auf dem Host);
  - die ArduinoJson-Version ist in beiden Repos gleich gepinnt;
  - kein Arduino, kein TFT_eSPI, kein FreeRTOS.
- **Ausgabe des Kerns:** Er rendert die ganze Canvas (240 × 240) und liefert sie als RGB565, entweder in einen vom Aufrufer gestellten Puffer oder zeilenweise über einen Callback **[B]**. Die Aufteilung auf die zwei 240 × 120-Displays macht der Adapter, nicht der Kern. Speicher holt er über eine Allokator-Schnittstelle, damit die Firmware PSRAM nutzen kann.

### 6.2 Generator-Repo

- **`internal/devexport`** (Go, CGO-frei, nur von `cmd/zza` importiert):
  - prüft ein Template auf ZZA-Kompatibilität (§7.3);
  - schreibt das Paket (§7).
- **`internal/devicerender`** (cgo, nur Desktop):
  - dünner Wrapper um `device/zzarender`;
  - API etwa `Render(pkg []byte, data []byte, now Time) ([]uint16, error)` und eine Animations-Variante, die Frames liefert **[B]**.
- **CLI:** `zza export -t <name> -o <name>-zza.zip`.
- **Editor (Desktop):**
  - Umschalter „Server / ZZA“ für die Vorschau. Die ZZA-Vorschau exportiert intern und rendert über `devicerender`. Ist das Template nicht kompatibel, zeigt sie die Gründe statt eines Bildes.
  - Knopf „Für ZZA exportieren“: lädt die ZIP herunter.
  - Übertragen wird über die Web-UI des Controllers (§6.3) **[C]**. Ein direktes „An ZZA senden“ aus der App gibt es in v1 nicht **[P]**.
- **Galerie (Server), optional in Stufe 4 [P]:** `GET /{template}-zza.zip`. `devexport` ist reines Go und CGO-frei, deshalb darf der Server es nutzen. Damit lassen sich Templates von gen.yuv.de auf den ZZA bringen, ohne CORS, denn der Browser lädt erst herunter und dann hoch. Dabei ist zu prüfen, dass die Server-Prüfung in `CLAUDE.md` weiter greift: kein cgo, kein `devicerender`.
- **MCP:** Ein Tool `check_device_compat` bzw. die Kompatibilität im bestehenden `validate_template` ist erwünscht, aber nicht in Stufe 1 **[P]**.
- **Server-Build:**
  - bleibt CGO-frei;
  - die Prüfung in `CLAUDE.md` (`go list -deps ./cmd/zza-server | grep …`) wird um `devicerender` erweitert.
- **`build.sh`:**
  - Für Windows wird zusätzlich `CXX=x86_64-w64-mingw32-g++` benötigt; das ist schon gesetzt.
  - macOS arm64/amd64 bauen mit clang.
  - **Zu prüfen in Stufe 2:** Der Cross-Build von macOS-arm64 nach amd64 mit C++.

### 6.3 Firmware-Repo

- **`lib/zzarender/`:** die Kopie (s. o.).
- **`src/DisplayTemplate.{h,cpp}`:** der Adapter.
  - Er rendert pro Gleis über den Kern und verteilt die Canvas wie `DisplayPng`: Zeilen 0–119 nach `sprite2` (D2), Zeilen 120–239 nach `sprite1` (D1). Danach ruft er `pushDual()` auf.
  - Damit funktionieren Screenshot (`/screenshotA`/`B`) und Client-Modus ohne Änderung.
  - Animationen: Ein geändertes Rechteck wird an der Grenze y = 120 geteilt. Jeder Teil geht als Teilbereich nur an das betroffene Display.
- **Datenquelle:** derselbe JSON-String, den der PNG-Modus heute an den Server schickt (`Config::getConfigString()`, siehe `PngDownloader.cpp`). Damit sehen Template, `default.json` und Gerät dieselbe Datenform.
- **Mehrere Templates, Auswahl pro Gleis [C]:**
  - Auf dem Gerät können mehrere Templates liegen.
  - Jedes Gleis wählt sein eigenes, wie heute `imageUrl`. DUAL hat ein Gleis, QUAD hat zwei (A und B) mit unabhängig wählbaren Templates. Ein Template füllt immer beide Displays seines Gleises.
  - Umsetzung **[P]**: Die PNG-Modi (`Mode_PngManual`, `Mode_PngInterval`) bekommen eine zweite Bildquelle, „lokales Template `<name>`“ statt `imageUrl`. Diese Modi tun heute schon genau das, nämlich ein Bild aus der Gleis-Config erzeugen. Neue Modi wären die Alternative **[B]** nach Blick in den Code.
- **Ablage:** `/tpl_<name>/<datei>`, also flach mit genau einer Ordnerebene, wie die übrigen eigenen Ordner (`GleisA`, `GleisAimg*`). Die Dateinamen im Paket hält der Exporter kurz, damit die LittleFS-Namenslänge passt **[B]**: prüfen.
- **Übertragung: Browser entpackt, Firmware nimmt Einzeldateien [C].**
  - Neue Seite bzw. neuer Abschnitt „Templates“ in der Firmware-Web-UI (`data/app`).
  - Sie nutzt die vorhandenen Bausteine aus `app-komplett-backup.js`: `zipEntries` liest ungepackt und deflate, dazu `sha256.js`. Gemeinsame Teile werden in ein eigenes Script verschoben **[B]**.
  - Ablauf:
    1. ZIP wählen.
    2. Der Browser liest `manifest.json` und prüft Formatversion, Canvas sowie Größe und SHA-256 jeder Datei. Den Platz prüft er gegen `/userFiles` (`total`/`used`).
    3. Er lädt jede Datei mit dem **vorhandenen** `POST /userFile?p=/tpl_<name>.new/<datei>` hoch.
    4. `POST /templateCommit?name=<name>` prüft auf dem Gerät noch einmal Manifest und Hashes. Erst dann ersetzt es per Umbenennen `/tpl_<name>/`; vorher wird der alte Ordner nach `/tpl_<name>.old/` verschoben und nach Erfolg gelöscht.
    5. Die Seite zeigt das Ergebnis und die Liste der installierten Templates.
  - Bei einem Fehler bleibt das alte Template aktiv. Übrig gebliebene `.new`/`.old`-Ordner räumt die Firmware beim Booten auf.
  - **Änderungen an der Firmware-API:**
    - `isUserDir` (Firmware **und** Browser-Kopie) erlaubt zusätzlich `tpl_<name>` und `tpl_<name>.new`, mit `<name>` nach den Regeln von `ValidateTemplateName`.
    - Neu: `POST /templateCommit?name=` und `POST /templateDelete?name=`.
    - Die Liste der Templates liefert das vorhandene `/userFiles`.
  - Während `templateCommit` darf kein Gleis das Template lesen: kurzes Sperren im Adapter, kein `restoreBegin`, die Anzeige läuft weiter.
- **Komplett-Backup:**
  - Weil `tpl_*` als eigener Ordner gilt, landen Templates automatisch in der Sicherung.
  - Zu klären in Stufe 3 **[B]**: Wie behandeln „Alles ersetzen“, „Nur Züge …“ und `wipeUserData` die Template-Ordner? Vorschlag: „Alles ersetzen“ ersetzt sie mit, die anderen Modi lassen sie unverändert.
- **Absicherung:** wie die übrige Firmware-API, also LAN-offen **[P]**. Eine Härtung folgt gemeinsam mit der restlichen API.
- **Fehler beim Laden oder Rendern:**
  - Das Gleis fällt auf das eingebaute Layout zurück.
  - Die Fehlermeldung erscheint in Web-UI und Info-Display.
  - Ein beim Booten defektes Paket wird markiert und nicht erneut geladen (keine Boot-Schleife).
- **Zeit (`now`):**
  - Der Adapter übergibt dem Kern eine Zeit.
  - In v1 ist das eine **zufällige Uhrzeit** pro Start bzw. Zugwechsel **[C]**.
  - Der Kern kennt keine Zeitquelle; NTP/RTC wird später nur im Adapter ergänzt.

## 7. Paketformat „ZZA-Template v1“

Das Format ist **Implementierungsdetail des Kerns**, da nur C++ es liest. Festgelegt sind diese Punkte:

### 7.1 Inhalt

- **Eine ZIP-Datei** `<name>-zza.zip` **[C]**.
  - Flach, ohne Unterordner. Ein Oberordner, den Finder oder Explorer beim Neu-Zippen dazupacken, wird toleriert wie beim Backup.
  - Ungepackt (Methode 0) **[P]**: Die Bilddaten sind ohnehin roh, und der Browser braucht dann kein `DecompressionStream`. Deflate ist erlaubt, falls RLE wegfällt **[B]**.
  - Atomar ist die Übertragung durch den Commit-Schritt (§6.3), nicht durch die Datei.
- **`manifest.json`:**
  - `format: 1`, Templatename, Canvas 240 × 240;
  - Dateiliste mit Größe und SHA-256;
  - benutzte Funktionen (Layer-Typen, Filter, Variablen wie `now`/`anim`).
- **`template.json`:**
  - die Layer-Struktur des YAML als JSON;
  - Ausdrücke `{{…}}` bleiben Strings, weil das Gerät sie zur Laufzeit gegen die Zugdaten auswertet;
  - `meta.animation` und `timezone` werden übernommen.
- **Bilder:**
  - **vorskaliert**, wenn `width`/`height` konstant sind; die Skalierung macht der Exporter mit demselben Catmull-Rom wie der Go-Renderer;
  - Ablage als RGBA8888 **[P]** (siehe §8.1), roh oder RLE **[B]**;
  - Speicher ist mit 16 MB Flash unkritisch; PNG-Dekodierung auf dem Gerät entfällt.
- **Schriften:**
  - pro Kombination aus (Schrift, `size`, `scale_x`, `scale_y`) eine Glyphentabelle für die Latin-1-Zeichen 0x20–0xFF;
  - Glyphen als 8-Bit-Alpha-Masken, mit Advance, Bearing, Ascent/Descent und Kerning-Paaren;
  - gerastert in Go mit `golang.org/x/image/font/opentype` (wie heute);
  - `scale_x`/`scale_y` sind eingerechnet, das Gerät skaliert nie;
  - alle diese Felder sind laut Template-Spec Konstanten, daher geht das vorab.

### 7.2 Versionierung

- Jede spätere Erweiterung erhöht `format`.
- Die Firmware lehnt Pakete mit unbekannter Version ab.

### 7.3 Kompatibilitätsprüfung (Exporter)

Der Exporter lehnt ab, mit konkreter Meldung „nicht ZZA-kompatibel: …“, wenn eine dieser Bedingungen zutrifft:
- Die Canvas ist nicht 240 × 240.
- `image`-Layer haben **dynamische** `width`/`height`. Die heutigen Templates nutzen das nicht.
- Texte enthalten Zeichen außerhalb von Latin-1. Geprüft werden Konstanten; dynamische Zugdaten ersetzt das Gerät zur Laufzeit durch `?` **[P]**.
- Eine Funktion, ein Filter oder eine Variable fehlt im C++-Evaluator.
- Ein `match`-Muster liegt außerhalb der Regex-Teilmenge (§8.1).
- Ein Ressourcenlimit (§8.3) wird überschritten.

`canvas.colors` wirkt nur auf die GIF-Ausgabe und wird ignoriert.

## 8. C++-Kern (`zzarender`)

### 8.1 Bestandteile

Jeder Bestandteil ist eine Portierung der Go-Logik unter `internal/renderer/`:

| Teil | Go-Vorlage |
|---|---|
| Paket laden, Hashes prüfen, JSON parsen | — |
| Evaluator: Interpolation, Pfade, Filter, Bedingungen | `evaluator.go` |
| Koordinaten-Ausdrücke | `expr.go` |
| `if`/`elif`/`else`, `loop`, `copy`, `rect` | `renderer.go` |
| `image` inkl. `rotate` (bilinear) | `renderImage`, `rotateImageCW` |
| `text`: `align`, `valign` inkl. `baseline`, `max_width`, Umbruch, `line_height`, `fit_items`, `id`/`advance` | `renderText`, `wrapText`, `layoutFitItems` |
| `splitflap`: Module planen, Blatt-Schnappschuss, Leaf-Cache | `splitflap.go` |
| Split-Flap-Animation: Seed, `flipCount`, `flipSequence`, `composeFlip`, `shade` | `animation.go` |
| Frame-Modus: `anim.frame`, `anim.frames`, `anim.time` | `animation.go`, `renderFrameMode` |

**Festlegungen für die Portierung [P]:**

- **Reguläre Ausdrücke (`match`):**
  - Die Templates nutzen `match` 32-mal, mit einfachen Mustern: Zeichenklassen, `+`, `*`, `^`, Gruppen, Alternativen. Beispiele: `[0-9]+`, `([a-zA-Z]+)`, `^(ICE|TGV|…)`.
  - Der Kern bekommt eine **kleine eigene Regex-Engine** für diese Teilmenge mit Leftmost-First-Semantik wie RE2. Er nutzt nicht `std::regex`, das auf dem ESP32 groß und langsam ist und andere Semantik hat.
  - Die Teilmenge: Literale, `.`, `[…]`, `[^…]`, `^`, `$`, `*`, `+`, `?`, `(…)`, `|`, Escapes `\d \w \s`.
  - Die Muster sind Konstanten im Template. Der Exporter prüft sie mit `regexp/syntax` gegen die Teilmenge und lehnt andere ab.
- **Farbtiefe intern:**
  - Der Kern zeichnet intern in **8 Bit pro Kanal** (RGB888-Leinwand, ≈ 173 KB im PSRAM) und wandelt erst am Ende nach RGB565.
  - So bleiben Alpha-Mischen, `copy` und Split-Flap-Schnappschüsse nah am Go-Renderer.
  - Bilder werden deshalb als **RGBA8888** abgelegt, nicht als RGB565+A8 wie in §7.1 zunächst gedacht. Flash ist dafür ausreichend.
- **SHA-256:** Wird für Paket-Hashes und den Animations-Seed gebraucht.
  - Der Kern bringt eine portable Implementierung mit.
  - Die Firmware darf sie per Adapter durch mbedTLS ersetzen.
- **Zahlenformat:** Arithmetik-Filter (`add`, `mul`, `div`, `round`, `minutes` …) liefern Strings. Deren Formatierung, insbesondere von Kommazahlen, muss **exakt** der Go-Ausgabe entsprechen (`doMath`). Die gemeinsame Evaluator-Testdatei (§9) deckt das ab.

### 8.2 Animation auf dem Gerät

- **Split-Flap:**
  - Bei einem Zugwechsel berechnet der Kern den Zeitplan wie `RenderAnimation`.
  - Seed wie `AnimationSeed(templateName, body)`, wobei `body` der Config-JSON-String ist.
  - Pro Frame rendert er nur die betroffenen Modul-Rechtecke neu und meldet sie dem Adapter für Teil-Updates.
  - Die Blätter (Leaves) liegen im PSRAM-Cache.
- **Frame-Modus (Uhren):**
  - Der Kern rendert Frame `i` auf Anfrage, mit `anim.frame = i` und eingefrorener Startzeit.
  - Der Adapter taktet mit `frame_delay` und springt nach dem letzten Frame auf 0, wenn `loop` gesetzt ist.
  - Frames werden **nicht vorab gespeichert** (60 × 113 KB wären unnötig). Ob statische Ebenen als Hintergrund gecacht werden, entscheidet die Messung **[B]**.
- **QUAD:**
  - Beide Gleise können gleichzeitig animieren, z. B. Split-Flap auf A und eine Uhr auf B.
  - Alle vier Displays hängen an einem SPI-Bus.
  - Gleichzeitige Animationen laufen verschränkt, nacheinander oder mit reduzierter Bildrate **[B]**, entschieden nach Messung.
  - Jedes Gleis hat seine eigene Kern-Instanz mit eigener Canvas und eigenem Leaf-Cache; der Speicher wird also doppelt gebraucht.

### 8.3 Limits

- Hart erzwungen wie im Go-Renderer: Layer-Anzahl, Rekursionstiefe, 64 Module, 64 Reel-Einträge, `flip_frames` 1–6, fps 5–25, Dauer 0,5–5 s, `frames` 2–125.
- **Neu:**
  - Textlänge nach Interpolation ist begrenzt (Wert **[B]**, Orientierung `capText`).
  - Glyphen außerhalb der Leinwand werden übersprungen.
  - Größe von Paket und Leaf-Cache ist begrenzt.
- Damit ist der Speicherbedarf vorab bekannt.

## 9. Tests

- **Kern auf dem Host:** Go-Tests in `internal/devicerender` rendern alle kompatiblen Templates in `templates/` mit mehreren Datensätzen über den C++-Kern.
  - Sie prüfen Fehlerfreiheit.
  - Sie prüfen Nähe zum Go-Renderer per Schwellwert, weil die Server-Vorschau nicht pixelgleich sein muss.
  - Sie prüfen Gleichheit mit eingecheckten Golden-Bildern (RGB565), damit Regressionen auffallen.
- **Evaluator:**
  - Eine gemeinsame Testfall-Datei (Ausdruck, Daten, erwartetes Ergebnis) wird vom Go-Evaluator **und** vom C++-Evaluator geprüft.
  - So bleibt die Semantik gleich, auch wenn die Server-Vorschau anders rastert.
- **Robustheit:**
  - Fuzzing von Paket-Parser und Evaluator auf dem Host (`go test -fuzz` über cgo oder libFuzzer **[B]**).
  - Limit-Tests: lange Texte, maximale Module, defekte Pakete.
- **Gleichheit Vorschau = Gerät:**
  - Im Firmware-E2E (`e2e/`): Paket hochladen, Gleis zuordnen, Zugdaten setzen, `/screenshotA` holen und mit dem Golden-Bild aus der ZZA-Vorschau vergleichen.
  - Ein defektes Paket hochladen; das alte Template muss aktiv bleiben.
- **Sync:** `tools/sync-zzarender.sh --check` läuft in den Firmware-Tests mit und meldet eine veraltete Kopie.

## 10. Stufen

Jede Stufe folgt dem üblichen Ablauf: Spec-Detail, Plan mit TDD-Tasks, manueller Testplan, Reviews. Die Ausnahme ist Stufe 1.

1. **Prototyp (Wegwerf, ohne Plan und Reviews) [C]**
   - **Firmware:** Branch `spike/template-render` von `develop`. Die laufende Arbeit auf `feature/animated-gif` wird nicht angefasst.
   - **Paket:** von Hand bzw. mit einem Wegwerf-Script gebaut, fest im LittleFS oder in die Firmware eingebaut.
   - **Templates:** `faltblatt-105-v1` mit Split-Flap-Animation und eine Uhr mit `rotate` und Frame-Schleife, z. B. `umuc-105-v1`, mit zufälliger Uhrzeit.
   - **Messen:**
     - Renderzeit Standbild;
     - Frame-Zeit Split-Flap (Teil-Updates) und Uhr;
     - RAM/PSRAM;
     - Flash und Paketgröße;
     - LittleFS-Platz laut Partitionstabelle der `lolin_s3`-Umgebungen;
     - ob PSRAM in den Umgebungen aktiv ist.
   - **Ergebnis:** Go/No-Go und Messwerte, die als Nachtrag in diese Spec kommen. Der Code wird verworfen.
2. **Kern, Export und ZZA-Vorschau** (Generator-Repo):
   - `device/zzarender` mit allen Funktionen aus §8.1, inkl. Animationen;
   - `internal/devexport`, `internal/devicerender`;
   - `zza export`, Editor-Umschalter;
   - Tests nach §9;
   - `build.sh` mit C++ für alle drei Ziele.
3. **Firmware-Integration** (Firmware-Repo):
   - Sync-Script, Adapter;
   - Templates-Seite in der Web-UI;
   - `isUserDir`-Erweiterung, `templateCommit`/`templateDelete`, Verhalten im Komplett-Backup;
   - Auswahl pro Gleis, Rückfall;
   - Animation inkl. QUAD-Verhalten;
   - E2E-Tests;
   - API-Doku (`docs/API.md`, `openapi.yaml`).
4. **Export-Knopf und Doku:**
   - „Für ZZA exportieren“ im Editor;
   - optional der ZIP-Download in der Galerie;
   - User-Guide „Template auf den ZZA übertragen“;
   - Template-Spec um „ZZA-kompatibel“ ergänzen;
   - MCP-Kompatibilitätsprüfung.
5. **Später, eigener Entwurf:** NTP/RTC und echte Uhrzeit, Live-Uhr.

## 11. Offen

- **[?]** Wie viele Templates sollen gleichzeitig auf dem Gerät liegen dürfen? Das hängt von der Messung in Stufe 1 ab (Paketgröße gegen LittleFS-Platz). Begrenzt wird über den Platz, nicht über eine feste Anzahl **[P]**.
