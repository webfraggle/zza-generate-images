# Templates direkt auf dem ZZA rendern (ESP32-S3) — Design

Datum: 2026-11-13 · Status: **zurückgestellt** (für später vorgemerkt, nicht in Umsetzung)

Legende: **[C]** = von Christoph so entschieden · **[P]** = Entscheidung des Planers (darf Christoph kippen) · **[B]** = dem Umsetzer überlassen · **[?]** = offen, vor Umsetzung klären

## 1. Absicht und Warum

Heute rendert der Server (`POST /{template}/render`) das Bild, der ZZA lädt es als PNG und zeigt es an. Künftig soll der Ablauf so aussehen: Man bearbeitet und testet ein Template in der Desktop-App und überträgt es dann auf den ZZA. **Der ZZA rendert danach selbst**, aus den Zugdaten, die er ohnehin hat.

Gewinn:
- **Offline:** kein Server, keine Rundreise ins Netz. Gleiswechsel und Verspätung erscheinen sofort.
- **Keine Serverlast:** Render-Kosten und DoS-Fragen am öffentlichen Endpoint entfallen für Displays. Galerie und Editor bleiben.
- **Bessere Animation:** Split-Flap wird live auf dem Gerät gerendert. Pro Frame gehen nur die klappenden Module ans Display, ohne GIF-Download, GIF-Dekodierung oder Farbreduktion.
- **Neue Layouts ohne Firmware-Update.**

## 2. Ziele

1. **[C]** Zielhardware nur **LOLIN S3 Mini + 1.05" GC9A01 (240 × 240)**, Firmware-Umgebungen `ZZA_Universal_Display_Controller_105_QUAD/DUAL`. S2 Mini / 0.96" ist ausdrücklich nicht Ziel.
2. **[C]** Zeichensatz **Latin-1** (ISO 8859-1) für alle Texte und Klappen-Reels.
3. **[C]** Arbeitsteilung: Bearbeiten, Testen und Kompilieren auf dem Rechner (Desktop-App `cmd/zza`), auf dem ZZA **nur rendern**.
4. **[P]** Was die Editor-Vorschau im Geräte-Modus zeigt, zeigt der ZZA **pixelgleich** (siehe §6).
5. **[P]** Split-Flap-Animationen laufen auf dem Gerät live, deterministisch wie im Go-Renderer (gleicher Seed ⇒ gleicher Ablauf).

## 3. Nicht-Ziele

- Go-Code auf dem ESP32 (TinyGo unterstützt den S3 nicht brauchbar). Der Geräte-Renderer ist eigener C++-Code.
- YAML-Parser oder TrueType-Rasterizer auf dem Gerät.
- Bearbeiten von Templates auf dem Gerät oder in der Firmware-Web-UI.
- S2 Mini / 0.96" ST7735. **[C]**
- Zeichen außerhalb Latin-1 (z.B. `€`, kyrillisch, Emoji). **[C]**
- GIF-Wiedergabe auf dem Gerät (bleibt eigener Backlog-Punkt; dieses Feature macht sie für Templates überflüssig).

## 4. Anti-Ziele

- Bestehende Displays im Feld merken etwas: Firmware ohne installiertes Template verhält sich exakt wie heute (eingebaute Layouts, `imageUrl`).
- Vorschau und Gerät weichen stillschweigend voneinander ab.
- Ein Template mit Funktionen, die das Gerät nicht kann, wird übertragen und rendert dort „irgendwie“. Es muss **vorher** im Editor abgelehnt werden, und das Gerät lehnt es zusätzlich selbst ab.
- Unbegrenzter RAM- oder Rechenaufwand auf dem Gerät durch ein Template oder durch Zugdaten (z.B. sehr lange Texte, siehe den `scale_x`-Befund im Split-Flap-Review).
- Ein fehlerhaftes Template-Paket bringt den ZZA in eine Boot-Schleife.

## 5. Architektur **[P]**

```
Desktop-App (Go)                                   ZZA (ESP32-S3, C++)
─────────────────                                  ───────────────────
template.yaml + Bilder + OTF
        │  zza export --device
        ▼
ZZA-Paket (.zza)  ──── HTTP-Upload ────▶  LittleFS /tpl/<name>/
  template.json                                    │
  img/*.bin (RGB565 + Alpha)                       ▼
  font/*.bin (Glyphen, vorgerastert)       Interpreter: Ebenen → Sprite
  manifest.json (Formatversion, Hash)              │  Zugdaten aus Config
                                                   ▼
                                           TFT_eSPI → Display(s)
```

### 5.1 Paketformat „ZZA-Template v1“

- **`manifest.json`:**
  - `format: 1`, Templatename, Canvas (fest 240 × 240), Liste der Dateien mit Größe und SHA-256.
  - Liste der benutzten Funktionen (Layer-Typen, Filter, Funktionen im Evaluator).
- **`template.json`:**
  - Die YAML als JSON. Ausdrücke `{{…}}` bleiben als Strings, weil sie zur Laufzeit gegen die Zugdaten ausgewertet werden.
  - Konstante Felder sind aufgelöst.
  - Schriftverweise zeigen auf vorgerasterte Varianten.
- **Bilder:** RGB565 plus 8-Bit-Alpha, unkomprimiert oder RLE **[B]**. Alternativ PNG behalten, weil PNGdec in der Firmware schon vorhanden ist **[B]**. Entscheidung nach Messung im Prototyp (Flash-Platz gegen Dekodierzeit).
- **Schriften:** Pro Kombination aus Schrift, `size` und `scale_x` eine Glyphentabelle.
  - Inhalt: alle Latin-1-Zeichen als 8-Bit-Alpha-Masken, dazu Advance, Bearing und Kerning-Paare.
  - `scale_x` ist eingerechnet; das Gerät skaliert nie.
- Paket als **eine Datei** (z.B. unkomprimiertes `tar` oder eigenes Containerformat **[B]**), damit der Upload atomar ist.

### 5.2 Funktionsumfang v1

Unterstützt werden alle Funktionen, die die heutigen Templates nutzen:

- **Ebenen:** `image`, `rect`, `text`, `copy`, `loop`, `if`/`elif`/`else` auf Layer-Ebene und als Feldbedingung, `splitflap`.
- **Text:** `align`, `valign` (inkl. `baseline`), `max_width`, Umbruch, `line_height`, `fit_items`.
- **Ausdrücke:** der vollständige Evaluator (Pfade, Arithmetik in `IntOrExpr`, Vergleichs- und String-Funktionen).
- **Filter:** alle Filter (`upper`, `lower`, `before`, `after`, …).

Nicht in v1: `canvas.colors`, weil das Gerät RGB565 ohne Palette rendert.

Der Exporter lehnt ein Template mit einer nicht unterstützten Funktion mit klarer Meldung ab („nicht ZZA-kompatibel: …“). Jede spätere Erweiterung erhöht die Formatversion; die Firmware lehnt Pakete mit höherer Version ab.

### 5.3 Geräte-Renderer

- Eigene C++-Bibliothek **ohne Abhängigkeit von Arduino oder TFT_eSPI im Kern**. Sie rendert in einen RGB565-Puffer.
- Ein dünner Adapter verbindet sie mit `DisplayBuffer`/`TFT_eSPI`.
- Damit läuft die Bibliothek auch auf dem Mac (PlatformIO `native`) und ist testbar (§7).
- Bestandteile:
  - JSON laden (ArduinoJson),
  - Evaluator und Filter,
  - Ebenen-Interpreter,
  - Glyphen-Blitter mit Alpha,
  - Bild-Blitter mit Alpha,
  - `copy` innerhalb des Puffers,
  - Split-Flap-Zeitplan und Klappenbild (`composeFlip`, `shade`).
- **Limits wie im Go-Renderer**, hart erzwungen:
  - max. Layer, Rekursionstiefe, 64 Module, 64 Reel-Einträge, `flip_frames` 1–6, fps 5–25, Dauer 0,5–5 s.
  - Neu: Textlänge nach Interpolation begrenzt, Glyphen außerhalb der Leinwand werden übersprungen.
  - Mit diesen Limits ist der RAM-Bedarf vorab bekannt.

### 5.4 Animation auf dem Gerät

- **Ablauf:** Bei einem Zugwechsel berechnet das Gerät den Zeitplan wie `RenderAnimation`. Der Seed kommt aus Templatename und Zugdaten; der kanonische Byte-Strom dafür wird in v1 festgelegt **[B]**.
- **Klappenbilder:** Pro Modul und Reel-Wert wird das Blatt einmal gerendert und im PSRAM gecacht, wie `cachedLeaf`.
- **Frames:** Pro Frame werden nur die betroffenen Modul-Rechtecke neu zusammengesetzt und als Teilbereich ans Display geschickt.
- **QUAD:** Alle Displays hängen an einem SPI-Bus. Wechseln mehrere gleichzeitig, laufen ihre Animationen nacheinander oder mit reduzierter fps **[B]**.

### 5.5 Übertragung

- **Desktop-App:** Knopf „An ZZA senden“ mit IP-Eingabe; die zuletzt benutzte IP wird gemerkt. Die App ruft den Upload-Endpoint der Firmware auf.
- **Firmware:**
  - Neuer Endpoint zum Hochladen (z.B. `POST /api/template`, Name noch offen) im vorhandenen ESPAsyncWebServer.
  - Das Paket wird in ein temporäres Verzeichnis geschrieben und Hashes und Formatversion werden geprüft. Erst danach ersetzt es per Umbenennen das bisherige Template.
  - Bei einem Fehler bleibt das alte Template aktiv.
- **Zuordnung:** In der Firmware-Konfiguration wählt man pro Gleis (A/B, bei QUAD je Display) „eingebautes Layout“ oder „Template <name>“.
- **Fehler beim Rendern:** Das Gerät fällt auf das eingebaute Layout zurück und zeigt den Fehler im Info-Display bzw. in der Web-UI.

## 6. Gleichheit Vorschau ↔ Gerät **[P]**

Der Go-Renderer bekommt einen **Geräte-Modus**. Er rendert aus demselben ZZA-Paket wie das Gerät: vorgerasterte Glyphen statt OTF, RGB565-Rundung, keine Catmull-Rom-Skalierung. Die Editor-Vorschau bietet „Server“ und „ZZA“ als Umschalter.

Pixelgleich heißt: Geräte-Modus in Go und C++-Renderer liefern für dieselben Daten **bitgleiche RGB565-Puffer**. Das wird automatisch geprüft (§7).

## 7. Tests **[P]**

- **Referenzbilder:**
  - Go erzeugt für jedes kompatible Template plus eine Reihe von Datensätzen Referenzpuffer im Geräte-Modus.
  - Die C++-Bibliothek rendert dieselben Fälle in einer PlatformIO-`native`-Umgebung auf dem Mac.
  - Der Test vergleicht beide bitweise, Standbild und ausgewählte Animationsframes.
- **Evaluator:** gemeinsame Testfälle als Datei (Ausdruck, Daten, erwartetes Ergebnis), von Go- und C++-Tests gelesen.
- **Robustheit:** Fuzzing des Paket-Parsers und des Evaluators auf dem Host, Limits-Tests (lange Texte, maximale Module).
- **Gerät:** E2E-Test im Firmware-Repo (`e2e/`):
  - Paket hochladen, Gleis zuordnen, Zugdaten setzen und Screenshot (`/screenshotA`) mit der Referenz vergleichen.
  - Defektes Paket hochladen ⇒ altes Template bleibt aktiv.

## 8. Ressourcen (zu messen im Prototyp)

| Größe | Annahme | Prüfen |
|---|---|---|
| Bildpuffer 240 × 240 RGB565 | ≈ 113 KB, existiert schon | – |
| Klappen-Cache Faltblatt | wenige 10 KB im PSRAM | messen |
| Paket `faltblatt-105-v1` | Schrift ist der größte Teil (OTF heute 72 KB) | messen |
| Freier LittleFS-Platz neben OTA-Partitionen | unbekannt, `data/` belegt schon 972 KB | Partitionstabelle der `lolin_s3`-Umgebung und echter Flash der Boards prüfen |
| Standbild-Renderzeit | zehn bis wenige hundert ms | messen |
| Animation | SPI-gebunden; volles Bild bei 40 MHz ≈ 23 ms, Teil-Updates deutlich weniger | messen, auch QUAD |

## 9. Stufen (Plan für später)

1. **Prototyp und Messung:**
   - `faltblatt-105-v1` als Standbild auf dem S3, handgebautes Paket, minimaler C++-Interpreter (nur die von diesem Template benutzten Ebenen).
   - Renderzeit, RAM und Flash messen.
   - **Go/No-Go-Entscheidung.**
2. **Format v1 und Exporter:**
   - Paketformat und Funktionsprüfung, `zza export --device` in der Desktop-App.
   - Geräte-Modus im Go-Renderer, Vorschau-Umschalter im Editor.
3. **Geräte-Renderer vollständig:**
   - Alle Funktionen aus §5.2 außer Animation, `native`-Tests mit Referenzbildern.
   - Upload-Endpoint, Gleis-Zuordnung, Rückfall auf eingebautes Layout.
4. **Split-Flap live auf dem Gerät:** Teil-Updates, QUAD-Verhalten.
5. **Doku:** Template-Spec um „ZZA-kompatibel“ ergänzen, User-Guide „Template auf den ZZA übertragen“, Firmware-API-Doku (`docs/API.md`, `openapi.yaml` im Firmware-Repo).

Jede Stufe folgt dem üblichen Ablauf (Spec-Detail, Plan mit TDD-Tasks, manueller Testplan, Reviews).

## 10. Offen **[?]**

- Sollen Templates die eingebauten Layouts (`DisplayV5x2`) auf Dauer **ersetzen** oder eine **Zusatzoption** bleiben? Das beeinflusst, ob v1 Wagenreihung und die übrigen Funktionen der eingebauten Layouts abdecken muss.
- Ein Template pro Gerät oder mehrere gleichzeitig (pro Gleis verschieden)?
- Wie kommen Templates aufs Gerät, wenn keine Desktop-App im Spiel ist, z.B. aus der Galerie auf gen.yuv.de („auf meinen ZZA laden“)? Das hätte CORS- und Sicherheitsfolgen für die Firmware.
- Name und Absicherung des Upload-Endpoints (heute ist die Firmware-API im LAN offen).
