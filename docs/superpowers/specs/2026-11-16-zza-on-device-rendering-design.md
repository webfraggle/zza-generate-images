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
8. **[C] Allgemeingültig, keine Sonderwege.** Das Gerät unterstützt das **Template-Format**, nicht die heute vorhandenen Templates. Nutzer (und die KI) bauen künftig Templates, die niemand vorhersieht. Daraus folgt:
   - Keine Regel, Einschränkung oder Optimierung darf sich an einzelnen Templates, deren Namen oder an „was die heutigen Templates nutzen“ orientieren.
   - Abgelehnt wird nur, was die Hardware grundsätzlich nicht kann (Canvas ≠ 240 × 240, Zeichen außerhalb Latin-1 in Konstanten, Überschreitung der Limits aus §8.3). Alles andere, was der Go-Renderer rendert, rendert auch das Gerät.
   - Optimierungen sind allgemeine Regeln mit klarer Gültigkeitsbedingung. Greift die Bedingung nicht, läuft der normale Pfad — langsamer, aber korrekt.
   - Laufzeit- und Speicherverhalten wird gegen **Worst-Case-Templates** an den Limits geprüft, nicht nur gegen die mitgelieferten.

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
  - **Nachtrag (Stufe 3, P3):** Der Kern bringt ein gepatchtes ArduinoJson 6.21.6 mit (`ZZA PATCH`: Float-Parsing über `strtod`, damit Datenzahlen wie im Go-Renderer gelesen werden; festgelegtes Layout) und legt es in einen **eigenen Namespace** `ArduinoJson::ArduinoJson6216_ZZA` (`ARDUINOJSON_VERSION_NAMESPACE` im Patch-Block). Eine zweite ArduinoJson-Kopie der Firmware (z. B. `lib_deps` für den Rest der Firmware) kollidiert dadurch nicht mehr beim Linken (ODR) und darf bleiben; die Kern-Quellen sehen ausschließlich den mitgelieferten Header (`#include "third_party/ArduinoJson.h"`), die Firmware-Kopie ist für ihren eigenen Code zuständig. Der Build bricht ohne den Namespace ab (`static_assert` unter `ZZA_TEST_HOOKS`);
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
  - Umsetzung (korrigiert in Stufe 3, siehe §15 P1): Das Gleis-Feld `template` ersetzt in den **Zugmodi** (Manuell, Intervall, Live, Client-Züge) das Online-Bild; die PNG-Modi bleiben unverändert, denn die Online-Bild-Logik liegt in den Zugmodi, nicht in `Mode_Png*`.
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
  - bei **dynamischem** `width`/`height` (Ausdruck) liegt das Original im Paket und der Kern skaliert zur Laufzeit (Catmull-Rom, Ergebnis pro Größe gecacht);
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
- Texte enthalten Zeichen außerhalb von Latin-1. Geprüft werden Konstanten; dynamische Zugdaten ersetzt das Gerät zur Laufzeit durch `?` **[P]**.
- Eine Funktion, ein Filter oder eine Variable fehlt im C++-Evaluator.
- Ein `match`-Muster ist ungültig (dieselbe Prüfung wie Go `regexp`).
- Ein `match`-Muster ist nicht konstant, d. h. das Musterargument ist kein in Anführungszeichen stehendes Literal (z. B. Muster aus `{{…}}`/Variable). Go erlaubt das, das Gerät kann zur Laufzeit nicht kompilieren; die einzige Format-Einschränkung dieser Art. Meldung: „Regex-Muster muss konstant sein“.
- Das kompilierte Programm eines konstanten `match`-Musters hat mehr als 2 000 Instruktionen oder mehr als 8 000 Runes insgesamt. Meldung: „Regex-Muster zu groß für das Gerät“.
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
  - Der Kern unterstützt die **volle RE2-Syntax, die Go `regexp` akzeptiert** (Klassen inkl. `[[:alpha:]]`, `\d \w \s \b`, Wiederholungen inkl. `{n,m}` und nicht-gierig, Gruppen inkl. benannt und nicht-fangend, Flags `(?i)` usw.), mit RE2-Semantik (Leftmost-First, Gruppen wie Go).
  - Eigene Engine, kein `std::regex` (groß, langsam, andere Semantik). Laufzeit **linear** in der Eingabelänge (Pike-VM wie RE2, kein Backtracking), weil Muster vom Nutzer und Eingaben aus den Zugdaten kommen.
  - Der Exporter kompiliert die Muster vorab mit Go `regexp/syntax` und legt sie als Programm ins Paket **[B]** oder das Gerät kompiliert selbst **[B]**; abgelehnt wird nur, was auch Go ablehnt.
  - Getestet gegen einen gemeinsamen Korpus (Muster, Eingabe, erwartete Gruppe), der in Go und C++ läuft, plus Fuzzing gegen Go `regexp`.
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
- **Funktionsabdeckung unabhängig von den mitgelieferten Templates [C]:** eine Sammlung von Test-Templates, die jede Funktion des Template-Formats einzeln und kombiniert nutzt (alle Layer-Typen, Filter, Bedingungen, Koordinaten-Ausdrücke, `loop`, `fit_items`, dynamische Bildgrößen, `rotate`, `scale_x`/`scale_y`, beide Animationsmodi), dazu zufällig erzeugte Templates (Generator auf Basis der Spec). C++-Kern und Go-Renderer werden auf allen verglichen (Schwellwert wie oben).
- **Worst-Case auf dem Gerät:** Templates an den Limits (max. Layer, 64 Module, 64 Reel-Einträge, 125 Frames, große Bilder, lange Texte) — geprüft werden Speicher (intern/PSRAM), Laufzeit, kein Absturz, sauberer Rückfall.
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

## 12. Nachtrag: Messwerte Stufe 1 (2026-11-16)

Details und Rohwerte: `spike/RESULTS.md` auf Branch `spike/zza-render` (Firmware-Spike: `spike/template-render`). Gemessen auf einem DUAL-Controller (S3 N16R8, 1,05").

**Entscheidung: Go [P].**

| | faltblatt (Split-Flap) | umuc (Uhr) |
|---|---|---|
| Paketgröße (roh) | 566 KB | 384 KB |
| Laden aus LittleFS | 1,4 s | 1,4 s |
| Standbild (render + 565 + push) | 88 ms | 93 ms |
| Animation pro Frame | live ≈ 70 ms (14 fps); **vorgerendert 18 ms → 20 fps** | 45 ms bei 1 s Takt |
| Interner Heap frei danach | 150 KB, stabil | 181 KB |
| Firmware-Größe | +49 KB | +49 KB |

Bildvergleich C++ ↔ Go-Renderer: Layout, Bilder, Bedingungen, Filter, Zeiger und Split-Flap-Ablauf (Seed) gleich; Abweichungen nur an Glyphenkanten (4–7 % der Pixel), weil Glyphen bei Subpixel 0 vorgerastert werden. Für Weg B unkritisch.

**Festlegungen für Stufe 2 aus den Messungen [P]:**

1. **Speicher:** Kerning, Glyphen und alle Tabellen als zusammenhängende Blöcke. Der Renderer bekommt eine Allokator-Schnittstelle, die Firmware gibt PSRAM (`heap_caps_malloc(MALLOC_CAP_SPIRAM)`). Viele kleine Allokationen (`std::map`-Knoten, kleine Leaves) haben im Spike den internen Heap geleert. Der interne Heap-Verbrauch wird Testkriterium (nach Laden und nach Animation ≥ 120 KB frei).
2. **Stack:** Der Renderer läuft in einem eigenen Task mit festem Stack (Loop-Task hat ~5 KB frei). Keine großen Objekte auf dem Stack.
3. **Rechnen:** kein `double` im Pixelpfad (die S3-FPU rechnet nur `float`; Zeigerdrehung 392 → 45 ms). Festkomma für Gewichte.
4. **Animation: live, wenn es reicht — sonst vorrendern [P]**, als allgemeine Regel für **jede** Animation (Split-Flap und Frame-Modus), entschieden nach **gemessenen Kosten**, nie nach Template-Art:
   - Der Adapter rendert die ersten Frames live und misst die Renderzeit pro Frame. Liegt sie mit Reserve unter dem Frame-Takt (Render + Push ≤ 70 % von `delay`), wird **live** gespielt — dann ist kein zusätzlicher Speicher belegt. Beispiel Uhr: 45 ms bei 1 000 ms Takt → live, belegt nichts.
   - Nur wenn live den Takt nicht schafft, wird **vorgerendert**: die geänderten Rechtecke je Display-Hälfte als fertiges RGB565 im PSRAM. Beispiel Split-Flap: 70 ms bei 50 ms Takt → vorrendern (gemessen: 1,7 s, 2,5 MB, danach 20 fps).
   - **Speicher als Leihgabe aus einem gemeinsamen Pool [P]:**
     - Jedes Gleis darf höchstens seinen **festen Anteil** am Pool leihen (DUAL: ganzer Pool; QUAD: je die Hälfte). Damit kann ein Gleis dem anderen nie Speicher nehmen — weder kurz noch dauerhaft.
     - **Endliche Animation** (Split-Flap, Frame-Modus ohne `loop`): Leihe nur für die Dauer des Abspielens, danach sofort zurück.
     - **Endlose Schleife** (Frame-Modus mit `loop`), die live zu langsam ist: Leihe bleibt bis zum nächsten Template- oder Zugwechsel bestehen.
     - Passt eine Animation nicht in den Anteil, läuft sie live — langsamer, die Frames dehnen sich, Ablauf und Bilder bleiben gleich.
   - Grenzwerte (70 %, Pool-Größe) **[B]**, festzulegen nach Messung auf DUAL und QUAD.
   - **Rückfall-Kette, immer vorhanden [C]:** Vorrendern ist nur eine Beschleunigung, nie Voraussetzung. Jede Stufe fällt auf die nächste zurück, ohne Fehler und ohne Absturz:
     1. **Live rendern**, wenn es den Takt schafft, sonst **vorrendern** und abspielen (Ziel-fps).
     2. **Live rendern** trotz zu langsamem Takt, wenn der Pool bzw. der faire Anteil nicht reicht (Ablauf und Endbild identisch; Frames verschieben sich, die Animation dauert länger).
     3. **Nur Endbild** — wenn auch für Live-Rendern der Speicher fehlt (z. B. Leaf-Cache passt nicht). Das Endbild ist immer das korrekte Standbild.
   - **Mehrere Gleise (QUAD, je eigenes Template) [P]:** Jedes Gleis leiht nur aus seinem Anteil (s. o.). Reicht der Anteil nicht, nimmt es sofort Stufe 2 (live) statt zu warten — die Anzeige eines Gleises wartet nie auf das andere. Bei einem Template- oder Zugwechsel gibt ein Gleis seine dauerhafte Leihe zurück und entscheidet neu. Warten auf freien Pool statt live **[B]**, nur wenn das in Messungen besser wirkt.
   - Alle drei Stufen werden getestet, auch erzwungen (Budget 0, Pool belegt, Speicher künstlich knapp), auf DUAL und QUAD mit zwei verschiedenen animierten Templates gleichzeitig.
   - Messwerte dazu: Der Zugwechsel ist nicht zeitkritisch (Idee Christoph). Gemessen: alle 33 Frames vorrendern 1,7 s, abgelegt als fertige RGB565-Teilrechtecke je Display-Hälfte im PSRAM (2,5 MB), danach Abspielen mit nur Push Ø 17,7 ms / max 22 ms → **20 fps, 0 verspätete Frames, 1,66 s Laufzeit** wie das GIF. Live-Rendern schaffte 14 fps. Dazu: statische Ebenen vor dem ersten `splitflap` einmal rendern (umgesetzt). PSRAM wird nach dem Abspielen freigegeben. **QUAD:** zwei Gleise gleichzeitig bräuchten ~5 MB → Vorrendern der Gleise nacheinander, oder bei knappem PSRAM Rückfall auf Live-Rendern **[B]**. Beispiel Uhr (Frame-Modus): 60 Frames mit kleinen geänderten Flächen (Zeiger) passen ins Budget und werden vorgerendert; eine Schleife mit großflächigen Änderungen fiele auf Live-Rendern zurück (im Spike 45 ms je Frame).
5a. **Statischer Vorspann [P]:** Ebenen vor dem ersten `splitflap` werden in einer Split-Flap-Animation einmal gerendert und pro Frame zurückkopiert. Gültigkeitsbedingung: Im Split-Flap-Modus ist `now` eingefroren und `anim.*` konstant, diese Ebenen ändern sich also nie. Setzt der erste `splitflap` eine `if`-Kette fort, entfällt die Optimierung.
5. **Paketgröße / LittleFS:** Partition 3,375 MB, belegt ~0,9 MB → nur 2–3 weitere Templates in Rohform. v1 braucht: Bilder RLE-komprimiert (oder PNG), Glyphen auf die möglichen Zeichen beschränken wo statisch bestimmbar (Reels, konstante Texte), sonst voller Latin-1-Satz. Kerning bleibt drin (≈ 1 800 Paare je Schrift, sichtbar). Eine größere LittleFS-Partition ist eine Option für später (App-Partitionen haben 6,25 MB, Firmware braucht 1,2 MB), aber nur mit einmaligem USB-Flash.
6. **Ladezeit** 1,4 s: zulässig für Template-Wechsel; beim Booten wird zuerst das eingebaute Layout gezeigt, falls das Laden länger dauert **[B]**.

## 13. Nachtrag: Ergebnisse Stufe 2 und Eingaben für Stufe 3 (2026-11-17)

**Server- und ZZA-Vorschau (R12, R14).** Der Kern setzt Glyphen auf ganze Pixel (keine Subpixel-Phasen, vierfacher Glyphenspeicher wäre sonst nötig). Die Server-Vorschau (Go) rastert dagegen je Glyphe in ihrer Subpixel-Phase. Beide Vorschauen können deshalb an Glyphenkanten abweichen, besonders bei vielfach übereinander gezeichnetem Text an derselben Stelle (die Kanten summieren sich bei ganzen Pixeln anders). Maßgeblich ist die **ZZA-Vorschau**: Sie ist bitgleich zum Gerät. Der Test-Vergleich Go ↔ C++ nutzt ein allgemeines Maß (3×3-Weichzeichnung plus Nachbarschaftsvergleich plus 8×8-Blockmittel), keine Ausnahmen je Template.

**Worst-Case-Speicher (`MemPeak`, Task 8, Generator-Fälle am Limit):**

| Fall | MemPeak |
|---|---|
| flap (Split-Flap, Limit) | 3 161 KB |
| frames (125 Frames) | 1 404 KB |
| images (viele/große Bilder) | 2 487 KB |
| layers (Layer-Limit, Skalierungs-Cache) | 1 381 KB |
| text (Textlimits) | 4 165 KB (3,97 MB schon nach dem Laden) |

**Eingaben für Stufe 3 (Firmware):**
- Render-Task braucht ca. **10 KB Stack** (Rekursion im Evaluator und in Layern); auf dem Gerät messen.
- ArduinoJson-Verschachtelungstiefe: der Kern lässt für `t.json` **80** zu. Ob der Gerätepuffer dieselbe Tiefe verträgt, muss auf dem Gerät gemessen werden.
- newlib-`strtod`/`printf` auf dem ESP32-S3 müssen **korrekt runden** (Float-Parsing und Zahlenformatierung entsprechen sonst nicht dem Go-Verhalten); Float-printf muss aktiviert sein.
- `zza_anim_end` gibt den Animationsspeicher frei. Die Leihe aus dem Pool (§12.4) hängt davon ab: nach dem Abspielen aufrufen, bei Schleifen beim nächsten Wechsel.
- Animations-Strategie (live messen, vorrendern, Rückfall-Kette) und Adapter siehe §12.4 und `docs/backlog.md` (Stufe 3).

**Nachträge aus dem Abschluss-Review Stufe 2 (2026-11-18).**

- **Rechenaufwand je Aufruf (Anti-Ziel „unbegrenzter Rechenaufwand“).** Jeder Aufruf von `zza_render`, `zza_anim_begin` und `zza_anim_frame` zählt seine dominanten Operationen (geschriebene/geblendete Pixel inkl. Leaves und Flip-Zeichnung, Skalierungs-Taps, gesetzte Glyphen, Regex-VM-Thread-Schritte, Evaluator-Bytes) und bricht beim Überschreiten mit `ZZA_ERR_TEMPLATE` „Rechenaufwand zu hoch“ ab; der Renderer bleibt benutzbar. API: `zza_set_work_limit(r, ops)` (0 = Standard, bleibt über `zza_load` erhalten), `zza_work_used(r)`. Gemessen (`TestWorkLimitMargin`, je Template mit default/leeren/langen Daten; `TestGeneratedNear`, 300 erzeugte Templates), größter Wert eines Aufrufs:

  | Quelle | größter Aufruf |
  |---|---|
  | worst-layers (Standbild) | 23 684 966 |
  | worst-images (Standbild) | 2 510 915 |
  | worst-flap (`anim_begin`) | 2 327 437 |
  | worst-frames (Frame) | 1 199 012 |
  | ausgelieferte Templates (max. faltblatt, `anim_begin`) | 272 668 |
  | cov-* (max. cov-image, lange Daten) | 1 413 931 |
  | erzeugte Templates (300) | 254 358 |

  Standard `ZZA_DEFAULT_WORK_LIMIT` = **100 000 000** (4,2 × worst-layers). Das Angriffs-Paket aus dem Security-Review (240×240-Glyphe mit Vorschub 0, 256 Text-Layer à 4096 Zeichen, vorher 136 s je Render) endet auf dem Mac nach 0,38 s. Wie lange 10⁸ Operationen auf dem ESP32-S3 dauern, ist in Stufe 3 zu messen; ggf. setzt die Firmware ein kleineres Limit.
- **R16 erledigt (Stufe 3):** Jede Textsuche des Evaluators (`replace` zweimal, `before`, `after`, `strip`, `stripAll`, `stripBetween`, `contains`, `startsWith`, `endsWith`) belastet das Rechenbudget zusätzlich zu den Bytes der Filterwerte mit der Länge des durchsuchten Bereichs. `split_by` (Loop) und der Interpolations-Scan waren bereits gezählt (`TestWorkLimitSubstringSearch`).
- **Interpolation höchstens 1 MiB je Feld** (`kMaxInterpolateBytes`, 4 × die 256-KiB-Datengrenze); darüber `ZZA_ERR_TEMPLATE`. **Abweichung von Go** (dort keine Grenze); legitime Templates erreichen sie nicht (Text wird ohnehin auf 4096 Zeichen gekürzt).
- **Lade-Plausibilität im Kern** wie im Exporter: Regex-Programme ≤ 2 000 Instruktionen und ≤ 8 000 Rune-Werte; Bilder ≤ 6 291 456 Pixel (= 8 × Paketgröße / 4 Byte); Glyphen-Masken ≤ 1024 Pixel je Seite. Der Exporter lehnt Templates mit größeren Glyphen ab (Schriftgröße/Skalierung zu groß) und prüft Bildmaße per `image.DecodeConfig` **vor** dem Dekodieren.
- **Exporter:** YAML-Aliase werden mit Zykluserkennung und höchstens 200 000 Knoten aufgelöst (größtes Test-Template ca. 1 900); die Latin-1-Prüfung umfasst auch Konstanten in `{{…}}`, die gezeichnet werden (`prefix`, `suffix`, Ersatztext von `replace`, `format`-Muster), und die Konstanten von `loop`-/`splitflap`-Werten (nicht Bedingungen und `match`-Muster).
- **ArduinoJson-Layout festgelegt** (ZZA PATCH): 4-Byte-Slot-Offsets auch auf 32 Bit (sonst zerstört ein Container mit > 32 767 Elementen die Verkettung), dazu `USE_LONG_LONG`, `USE_DOUBLE`, String-Deduplizierung, `DECODE_UNICODE`, NaN/Infinity/Kommentare aus, Alignment an. **Eingabe Stufe 3:** ein JSON-Slot wird auf dem ESP32-S3 dadurch 24 statt 16 Byte groß; der Pool von `t.json` und Zugdaten wächst entsprechend (Worst-Case-Speicher auf dem Gerät neu messen).
- **C++-Exceptions sind Pflicht** (`#error` ohne `__cpp_exceptions`); die `zza_test_*`-Funktionen gibt es nur mit `ZZA_TEST_HOOKS`.
- **Lange Zahl-Strings** (z. B. `{{a | mul(2)}}` mit 200 KB Ziffern) laufen wie lange Zahlen in den Zugdaten über die beschränkte kanonische Form (`decnum.h`) vor `strtod`: beschränkter Aufwand, Rundung wie Go.
- **ZZA-Vorschau (Desktop):** höchstens 2 gleichzeitige Renders (wie die GIF-Renders des Servers), sonst nach 15 s Wartezeit 503.

## 14. Nachtrag: Messwerte Stufe 3 (2026-11-20)

Gemessen auf dem QUAD-Controller (S3 N16R8, 1,05"), Release-Build, LittleFS aus `data/`, danach einmal mit DUAL-Release auf demselben Gerät. Werkzeug: Firmware `e2e/scripts/tpl-measure.ts` (installiert jedes Paket, schaltet Zugwechsel, liest `/templateStatus`). Die `worst-*`-Templates laufen dort als Geräte-Variante: dieselbe YAML, die Felder auf `config.json`-Pfade umgeschrieben (Host-Render bitgleich zum Original); lange Texte sind auf 1 300 Zeichen gekürzt (Body-Grenze 3 072 Byte von `/api/a/display/N`). Zeiten je Zugwechsel, mehrere Läufe.

**Freier Speicher ohne Template:** PSRAM QUAD 7 224 315 B, DUAL 7 715 027 B (Canvas je Gleis schon belegt).

| Template (Paket) | Laden (davon Lesen) | Bild | Modus, Ablauf | Kern mem / peak |
|---|---|---|---|---|
| faltblatt (303 KB, 14 Dateien) | 0,7–2,5 s (0,55–2,3 s) | Frame 0: 41 ms, Frame 1: 48–57 ms bei 50 ms Takt | vorgerendert in 1,3–2,1 s, Leihe 0,95–1,49 MB; Animation gesamt 3,3–4,2 s (33 Frames) | 578 KB / 1,25 MB |
| nederland (149 KB) | 0,42–0,66 s | 49–58 ms je Frame | live, 1 s Takt (1,1 fps gemessen) | 488 KB / 488 KB |
| oebb (293 KB) | 0,55–0,64 s | 42–62 ms | live | 657 KB / 658 KB |
| rhb (166 KB) | 0,43–0,51 s | Standbild 28–37 ms | still | 309 KB / 484 KB |
| sbb (290 KB, 18 Dateien) | 1,24–1,32 s (1,07–1,15 s) | Standbild 30–36 ms | still | 498 KB / 673 KB |
| umuc (112 KB, 17 Dateien) | 0,83–0,91 s | 69–78 ms | live | 581 KB / 581 KB |
| worst-flap | 0,2–1,0 s | anim_begin 0,8–1,8 s | QUAD: nur Endbild (Bedarf 3,16 MB > Budget); DUAL: liveSlow, 8,2 fps (Vorrender-Bedarf 5,4 MB) | 3,16 MB (DUAL) |
| worst-frames (125 Frames, Schleife) | 0,3–1,2 s | 0,30–0,38 s je Frame | Vorrendern bis der Anteil voll ist, dann liveSlow (Bedarf 14,4 MB) | 1,35 MB / 1,41 MB |
| worst-images (1,4 MB) | 1,5 s | Standbild 1,03 s | QUAD: eingebautes Layout „zu wenig Speicher beim Rendern“; DUAL: still | 2,30 MB / 2,49 MB |
| worst-layers | 0,5–0,7 s | Standbild 7,3–8,0 s | still | 1,12 MB / 1,30 MB |
| worst-text (1,96 MB, 22 Dateien) | DUAL 3,0 s | Standbild 13–28 ms | QUAD: eingebautes Layout „zu wenig Speicher beim Laden“; DUAL: still | 3,96 MB / 4,14 MB |

**QUAD, beide Gleise animiert:** faltblatt + umuc: A vorgerendert (Leihe 1,25–1,39 MB), B live mit 1 s Takt. faltblatt + faltblatt: beide vorgerendert, das Vorrendern läuft verschränkt (2,7–2,8 s je Gleis, Animation gesamt ≈ 5 s), Leihe 1,25–1,55 MB je Gleis, PSRAM-Minimum 1,95 MB. Kein Neustart, keine Panic in allen Läufen; jede Rückfallstufe (vorrendern → liveSlow → Endbild → eingebautes Layout) kam vor.

**Vorrender-Bedarf faltblatt** (Host, Kern und `FrameStore`-Größen des Geräts, 270 zufällige Züge): bis 1,64 MB, 95 %-Quantil 1,40–1,43 MB; Kern-Spitze 1,21–1,27 MB. Mit den Startwerten (Anteil 1,25 MiB) fiel faltblatt bei manchen Zügen auf liveSlow (6 fps, 5,5 s statt 1,7 s).

**Rechenrate:** worst-layers 23,7 Mio. Operationen in 7,8 s → **3,0–3,1 Mio. Operationen/s** (Mischung aus Pixeln, Taps, Glyphen). `anim_begin` von Split-Flaps ist langsamer je Operation (faltblatt 0,27 Mio. in 0,22 s, worst-flap 0,95 Mio. in 0,8 s ≈ 1,1–1,2 Mio./s). Der Kern-Standard 10⁸ entspräche 33 s (Mischung) bis ≈ 85 s. **Befund:** 5 s × Rate = 15 Mio. liegt unter 2 × größtem legitimen Aufruf (47,4 Mio.); die Firmware-Grenze folgt deshalb der 2×-Regel, nicht der 5-s-Regel.

**Verschachtelung:** Paket mit 16 Block-Ebenen, darin ein `splitflap` mit wieder 16 Block-Ebenen, `t.json` auf Tiefe 80 aufgefüllt: lädt und animiert live (13–14 fps); Loop-Stack frei (Hochwasser) **7,9 KB** von 20 KB, also ≈ 12,1 KB belegt (alle anderen Templates ≥ 15,2 KB frei). Tiefe 81: „Paket fehlerhaft: t.json: invalid JSON: TooDeep“, kein Absturz. Zugdaten-Tiefe am `jsoncheck`-Limit ist auf dem Gerät nicht erreichbar (der Kern bekommt `config.json` mit fester Tiefe 3); der Deserializer ist derselbe wie für `t.json` und mit Tiefe 80 abgedeckt.

**Interner Heap:** nach Laden und nach Animation (abgefragt) QUAD ≥ 137 KB, DUAL ≥ 159 KB. Das Minimum seit dem Start (`heap_caps_get_minimum_free_size`) erreichte auf dem QUAD **107 KB** während der Läufe mit zwei Gleisen (ein Gleis: 121 KB; DUAL 143 KB) — Dauer unbekannt, Ursache nicht eingegrenzt (gleichzeitige HTTP-Anfragen und zwei Ladevorgänge).

**Festgelegt [P]** (Firmware `src/ProjectSetup.h`):

| Wert | QUAD | DUAL | Begründung |
|---|---|---|---|
| Kern-Budget je Gleis | 1,5 MiB (war 2,5) | 4,5 MiB (war 4) | QUAD: größter Bedarf eines ausgelieferten Templates 1,27 MB (+ 23 %); DUAL: worst-text (4,14 MB Spitze) lädt |
| Vorrender-Anteil je Gleis | 1,75 MiB (war 1,25) | 2 MiB (war 3) | faltblatt bis 1,64 MB; QUAD: 2 × (1,5 + 1,75) MiB = 6,8 MB von 7,22 MB frei, Rest 0,4 MB für den Rest der Firmware; DUAL: 6,8 MB von 7,7 MB |
| `TPL_WORK_LIMIT` | 48 000 000 | 48 000 000 | 2 × worst-layers; ≈ 16 s bei 3 Mio./s |
| `TPL_LOOP_STACK` | 20 KB | 20 KB | tiefstes Paket braucht ≈ 12,1 KB, Reserve 7,9 KB ≥ 4 KB |

Folge: Auf dem QUAD fallen Templates mit 1,5–2,5 MiB Kern-Bedarf (z. B. worst-images) jetzt auf das eingebaute Layout zurück, die vorher liefen; dafür spielt faltblatt immer vorgerendert. Ein Paket, das beim Laden mehr PSRAM braucht als frei ist (Dateien liegen beim Laden zusätzlich im PSRAM), scheitert sauber mit „zu wenig Speicher beim Laden“.

**Befund und Fix (Firmware):** Stellte man auf dem QUAD ein Gleis auf ein Template um, während das andere Gleis sein Paket lud, erschien das Template nie (Status blieb `off`): der Durchlauf hatte „kein Template“ schon entschieden und verbrauchte das Update-Flag für das eingebaute Layout. Behoben, Test API-TPL-015.

## 15. Nachtrag Stufe 3: Entscheidungen, die den Entwurf geändert haben (2026-11-20)

Bei der Umsetzung der Firmware-Integration wurden folgende Punkte anders entschieden als oben beschrieben; sie gelten.

- **P1 Bildquelle:** Das Gleis-Feld `template` (Name oder leer, in `config.json`) ersetzt das Online-Bild in den **Zugmodi** (Manuell, Intervall, Live, Client-Züge). `imageUrl` bleibt gespeichert und wird ignoriert; Bildmodi und Client-Bild bleiben unverändert (die Online-Bild-Logik liegt in den Zugmodi, nicht in `Mode_Png*`; §6.3 entsprechend korrigiert).
- **P2 Kern im Loop-Task:** Kein eigener Render-Task. Der Kern läuft im Loop-Task (Core 1) mit 20 KB Stack (`SET_LOOP_TASK_STACK_SIZE`), damit Sprites und SPI in einem Task bleiben. Je Durchlauf höchstens ein Frame; nur das Laden blockiert (0,3–2,5 s). Messung: tiefstes Paket braucht ≈ 12,1 KB, Reserve 7,9 KB.
- **P3 Eigener ArduinoJson-Namespace** (ersetzt R4, „einzige Kopie“): Die Firmware behält ArduinoJson in `lib_deps` (Mobadi-Regel), der Kern trägt seine Kopie in einem eigenen Namespace, beide stehen ohne ODR-Konflikt nebeneinander.
- **P4 Upload und Commit:** `POST /templateBegin?name=` leert/erstellt `/tpl_<name>.new`, damit ein Wiederholungsversuch nicht an Resten scheitert. `POST /templateCommit` ist ein **asynchroner Auftrag (R4-1)**: Antwort 202, Prüfen und Umbenennen läuft im `webServerTask` unter dem Dateisystem-Mutex (Watchdog-Risiko im HTTP-Handler), das Ergebnis steht in `GET /templateStatus` (`job: {name, state: idle|running|ok|error, error}`); ein zweiter Commit währenddessen gibt 409 `busy`. Das Aufräumen alter Ordner (`.old`, `.del`) geschieht ebenfalls im `webServerTask` in Häppchen, Reste räumt der Start auf. Die Web-UI fragt den Status ab.
- **P6 Namenslänge:** Auf dem Gerät `[a-z0-9-]`, 1–32 Zeichen (LittleFS-Pfadlänge: `/tpl_<32>.new/<12>` < 64). Der Generator lässt 64 zu; die Web-UI lehnt längere Namen vor dem Upload mit Klartext ab.
- **Datenquelle:** Der Kern bekommt `Config::configJson()` (RAM-Stand), nicht die Datei. Nach dem Speichern ist das byteidentisch mit `config.json`; der RAM-Stand ist neuer, wenn MQTT oder der Client-Modus Daten setzen, bevor gespeichert wurde.
- **R6-1 „Zugwechsel“:** Jede Änderung des JSON, das das Template bekommt, löst Animation und eine neue Zufallszeit aus; identische Daten tun nichts. Ein echtes Faltblatt klappt immer, wenn sich der angezeigte Inhalt ändert (auch bei Live-Aktualisierung mit geänderter Abweichung). Kosten: gelegentliches erneutes Klappen nach einem Einstellungs-Speichern.
- **R5-1 Startschutz:** Die Startmarke (vor `zza_load` gesetzt, danach gelöscht) zählt nur nach einem Panic-/Watchdog-Reset, nicht nach normalem Neustart oder Stromausfall. Entscheidungsfunktion rein und auf dem Host getestet; der echte Absturzpfad nur manuell (MAN-TPL-003).
- **Backup:** Installierte Templates (`tpl_<name>`) gehören zum Komplett-Backup und kommen nur mit „Alles ersetzen“ zurück; „Hinzufügen“ überspringt sie (sonst Mischstand aus alten und neuen Dateien).
- **R7-1 Werte aus der Messung (§14):** QUAD-Budget 1,5 MiB / Anteil 1,75 MiB, Arbeitsgrenze 48 Mio. nach der 2×-Regel. Folge: Templates der worst-images-Klasse zeigen auf dem QUAD das eingebaute Layout, pathologische Templates können die Schleife bis ≈ 16 s blockieren. Beides ist eine offene Entscheidung für den Nutzer.
