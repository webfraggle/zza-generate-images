# MCP-Server für den Desktop-Editor — Plan

Stand: 2026-11-13 · Status: Idee / Konzept, noch nicht begonnen

## Ziel

Nutzer sollen eine KI (Claude o. ä.) ihre Template-Ideen umsetzen lassen können:
Foto einer echten Anzeige hochladen → „bau daraus ein Template“ → die KI legt
das Template an, schreibt YAML und Testdaten, erzeugt Grafiken, rendert eine
Vorschau, vergleicht sie mit dem Foto und bessert nach, bis es passt.

Dafür bekommt der Desktop-Build (`cmd/zza`) einen MCP-Server, über den die KI
auf Templates, Renderer und Bildwerkzeuge zugreift.

## Hintergrund: MCP in Kürze

- **Model Context Protocol** — offener Standard, über den eine KI-App
  (Claude Desktop, Claude Code, claude.ai, andere Clients) Werkzeuge eines
  fremden Programms aufruft.
- Der **MCP-Server** (hier: `zza`) meldet **Tools** an: Name, Beschreibung,
  JSON-Schema der Parameter. Das Modell entscheidet selbst, wann es welches
  Tool aufruft, und bekommt das Ergebnis zurück.
- Tool-Ergebnisse können **Text, JSON oder Bilder** sein. Bilder sind der
  Schlüssel: Die KI sieht die gerenderte Vorschau und kann sie mit dem Foto
  vergleichen.
- **Transport:**
  - **stdio** — die KI-App startet das Programm lokal als Unterprozess
    (Claude Desktop, Claude Code).
  - **Streamable HTTP** — der Server läuft unter einer URL im Internet
    (claude.ai „Custom Connector“, andere Webchats).
- **Go-SDK:** offizielles `github.com/modelcontextprotocol/go-sdk`, CGO-frei.

## Kernentscheidung: lokal statt Webchat (vorerst)

Ein Webchat läuft in der Cloud und kann den lokalen Rechner des Nutzers nicht
erreichen. Drei Varianten:

| Variante | Umsetzung | Aufwand | Bewertung |
|---|---|---|---|
| **A: lokal (stdio)** | `zza mcp` als Subcommand, in Claude Desktop eingetragen | klein | **Start hiermit** |
| B: remote (HTTP) | MCP-Endpoint auf gen.yuv.de | groß | braucht Login, Speicher pro Nutzer, Kontingente — widerspricht dem bewusst schlanken Server ohne Auth/DB |
| C: lokal + Tunnel | A über ngrok o. ä. erreichbar machen | mittel | für Endnutzer zu fummelig, öffnet den Rechner nach außen |

Variante B wird erst geprüft, wenn A sich in der Praxis bewährt hat.

## Phase 1 — MCP-Grundgerüst

### CLI

- Neues Subcommand im Desktop-Build: `zza mcp [--templates-dir DIR]`.
- Spricht MCP über stdin/stdout; Logausgaben ausschließlich nach stderr
  (stdout gehört dem Protokoll).
- Nur Desktop-Build (`internal/mcp`, DESKTOP-ONLY wie `internal/editor`).
  Der Server-Build bleibt unverändert.
- Einrichtung für Nutzer: kurzer Abschnitt im User-Guide mit dem
  Konfigurationsschnipsel für Claude Desktop (Pfad zur `zza`-Binary + `mcp`).
  Später ggf. als Claude-Desktop-Erweiterung (`.mcpb`) zum Ein-Klick-Install.

### Tools

| Tool | Zweck | baut auf |
|---|---|---|
| `get_template_spec()` | liefert `docs/yaml-template-spec.md` (embedded), damit die KI das Format kennt | — |
| `list_templates()` | Namen, Meta, Animation ja/nein | `internal/gallery` |
| `read_template(name)` | `template.yaml`, `default.json`, Dateiliste | `internal/editor` |
| `create_template(name)` | neues Template aus dem Starter | Editor-Starter |
| `write_file(template, name, content)` | YAML/JSON schreiben; YAML wird vor dem Speichern validiert, Fehler gehen als Text an die KI zurück | `internal/editor`, `renderer.LoadTemplate` |
| `upload_image(template, name, base64)` | PNG/JPG ablegen (Hintergrund, Icons, Nutzerfoto) | Editor-Upload |
| `render_preview(template, data?, format?)` | rendert und gibt das **PNG als Bild-Ergebnis** zurück; optional eigene Testdaten statt `default.json`; bei GIF ein Kontaktbogen aus einigen Frames | `internal/renderer` |

### Tool-Beschreibungen

Die Beschreibungen sind der wichtigste Hebel für die Qualität — sie sind das
„Handbuch“ der KI. Darin festhalten:

- Arbeitsweise: erst Spec lesen, dann Template anlegen, nach jeder Änderung
  `render_preview` und mit der Vorlage vergleichen.
- Leinwand 240×240, obere/untere Hälfte typischerweise per `copy` gespiegelt.
- Dynamische Inhalte (Zeit, Ziel, Via …) immer als Text-Layer mit
  `{{zug1.…}}`, nie in Bilder einbrennen.
- Bekannte Felder von `zug1`/`zug2`/`zug3` und Beispielwerte.

### Sicherheit (lokal)

- Schreiben nur im Templates-Ordner; Pfad-Schutz aus `internal/editor`
  wiederverwenden (kein `..`, keine absoluten Pfade, keine Symlinks).
- Größenlimits: Upload ≤ 5 MB, Dateien pro Template begrenzt.
- Template-Namen nach den bestehenden Regeln validieren.
- Kein Löschen ganzer Templates über MCP in Phase 1 (nur einzelne Dateien
  überschreiben) — Fehlgriffe der KI bleiben reparierbar.

## Phase 2 — SVG → PNG

KIs schreiben SVG zuverlässig: es ist Text, gezielt änderbar und gut für
Logos, Badges, Piktogramme, Pfeile, Linien, Zifferblätter und Punktraster.

### Tool

`svg_to_png(template, name, svg, width, height, antialias = true)`

- Rastert einmal und speichert ein normales PNG im Template-Ordner.
- Gibt das Ergebnis zusätzlich als Bild zurück, damit die KI es prüfen kann.
- **Bewusst kein `type: svg`-Layer** im Renderer: Templates bleiben
  deterministisch, der öffentliche Server muss nie SVG parsen. Ein SVG-Layer
  (z. B. für dynamisch eingefärbte Grafiken) kann später kommen.

### Bibliothek (CGO-frei)

| Option | Bewertung |
|---|---|
| `oksvg` + `rasterx` | klein, Pfade/Formen/Verläufe; kaum Text/Filter — reicht für Logos. **Erste Wahl.** |
| `tdewolff/canvas` | vollständiger, deutlich größer — falls `oksvg` nicht reicht |
| `resvg` | beste Qualität, aber nur über CGO oder WASM — vorerst nicht |

### Regeln

- **Kein Text im SVG.** Schrift bleibt in Text-Layern mit echten Fonts;
  eingebrannter Text wäre nicht mehr per JSON änderbar. Steht in der
  Tool-Beschreibung.
- **`antialias: false`** für harte Kanten — wichtig für Pixel- und
  Punktmatrix-Optik auf 240×240.
- **Sicherheit:** keine externen Referenzen nachladen (`<image href>`,
  `xlink:href`, `@import`), keine Entities; Größe des SVG-Texts und
  Pixelzahl begrenzen (z. B. ≤ 256 KB, ≤ 1024×1024).

## Phase 3 — Foto-Werkzeuge

Freies „Malen“ per Tool-Aufruf (Linie für Linie) ist für eine KI mühsam und
fehleranfällig — dafür gibt es SVG. Sinnvoll sind dagegen **feste, deterministische
Werkzeuge**, um das hochgeladene Foto nutzbar zu machen:

| Tool | Zweck |
|---|---|
| `perspective_correct(src, dst, corners[4], width, height)` | schräg fotografierte Anzeige auf ein gerades Rechteck entzerren |
| `crop_image(src, dst, x, y, w, h)` | Ausschnitt, z. B. Original-Logo oder Hintergrundfläche |
| `resize_image(src, dst, w, h, mode)` | auf Zielgröße bringen (`nearest` für Pixel-Optik, sonst glatt) |
| `quantize_colors(src, dst, n)` | auf wenige Farben reduzieren, entfernt Fotorauschen (Quantisierung existiert im Repo schon für GIFs) |
| `color_to_alpha(src, dst, color, tolerance)` | Hintergrundfarbe transparent machen, z. B. für Icons |
| `sample_colors(src, points[])` | Farbwerte an Punkten auslesen — für exakte Hex-Farben im YAML |

Alle Werkzeuge schreiben eine neue Datei (Original bleibt erhalten) und geben
das Ergebnis als Bild zurück.

## Parallelbetrieb mit der GUI

Ziel: Der Nutzer hat die Desktop-GUI offen und sagt der KI z. B. „lege Font
XY in Template Z“ — die Änderung erscheint direkt im Editor.

### Grundprinzip

- Claude Desktop startet `zza mcp` als **eigenen Prozess** (stdio, kein Port)
  — kein Konflikt mit der laufenden GUI.
- Beide arbeiten **direkt auf demselben Templates-Ordner**. Die GUI muss
  dafür nicht laufen; der MCP-Server funktioniert auch allein.
- Verworfene Alternative: `zza mcp` spricht die HTTP-API der laufenden GUI an
  (nur ein Schreiber, einfachere Live-Updates). Nachteil: ohne offene GUI
  geht nichts. Direkter Dateizugriff + Watcher + Konfliktprüfung ist robuster.

### Live-Aktualisierung im Editor

- File-Watcher in der GUI (`fsnotify`) auf dem Templates-Ordner, Ereignisse
  kurz entprellen (z. B. 200 ms), da Schreibvorgänge mehrere Events auslösen.
- Meldung an die offene Editor-Seite per **Server-Sent Events**
  (`GET /edit/{template}/events`).
- Editor reagiert:
  - Dateiliste neu laden (neue Font/Bilder erscheinen sofort),
  - Vorschau neu rendern,
  - offene Datei: ohne ungespeicherte Änderungen still neu laden, sonst
    Hinweis anzeigen (siehe Konflikte).
- Galerie und Detailseite: optional ebenfalls neu laden.
- Falls Templates, Fonts oder Vorschauen im Desktop-Build zwischengespeichert
  werden (zu prüfen: `renderer`-Face-Cache, `server`-Render-Cache), müssen
  Dateiänderungen diese Einträge verwerfen — sonst zeigt die Vorschau den
  alten Stand.

### Konflikte beim gleichzeitigen Bearbeiten

Riskanter Fall: Datei im Editor geöffnet und ungespeichert geändert, die KI
schreibt dieselbe Datei, der Nutzer speichert → Änderung der KI wäre still
überschrieben.

- Editor merkt sich beim Laden die Version der Datei (Prüfsumme oder
  Änderungszeit, als `ETag`).
- Speichern schickt sie mit (`If-Match`); der Server lehnt mit `409` ab,
  wenn sich die Datei inzwischen geändert hat.
- Editor fragt dann: „Datei wurde extern geändert — neu laden oder
  überschreiben?“
- Gleiches Prinzip optional auch für `write_file` im MCP-Server: Die KI gibt
  die gelesene Version mit und bekommt bei Konflikt eine klare Fehlermeldung,
  statt Nutzeränderungen zu überschreiben.
- Atomares Schreiben (temporäre Datei + Umbenennen), damit Watcher und
  Renderer nie halb geschriebene Dateien sehen.

## Fonts

„Lege Font XY in Template Z“ scheitert nicht am Kopieren, sondern an der
Frage, woher die Font-Datei kommt.

### Quellen

| Quelle | Umsetzung | Bewertung |
|---|---|---|
| **Mitgelieferte Auswahl** | kuratierte freie Fonts in der Desktop-App (Roboto, Nimbus Sans, Pixel-Fonts wie 04B_03/LLPIXEL3 — Lizenzen prüfen) | einfach, lizenzsicher — **Start hiermit** |
| Systemschriften | Suche in den Font-Ordnern von macOS/Windows (`/Library/Fonts`, `~/Library/Fonts`, `C:\Windows\Fonts`) | praktisch; Lizenz liegt beim Nutzer; nur lesend, nur aus diesen Ordnern, Hinweis zur Lizenz im Tool-Ergebnis |
| Google Fonts | Download bei Bedarf | große Auswahl, OFL-Lizenz unproblematisch; braucht Internet; nur von fest hinterlegter Quelle, Größenlimit |

### Tools

- `list_fonts(query?)` — verfügbare Fonts mit Quelle, Schnitten (Regular,
  Bold, Italic …) und Lizenz.
- `add_font(template, font, style?, id?)` — kopiert die Datei ins Template
  **und** trägt sie unter `fonts:` in `template.yaml` ein (`id`, `file`);
  vorhandene Einträge werden nicht doppelt angelegt. Ein Schritt statt zwei,
  weniger Fehlerquellen für die KI.
- Nur `.ttf`/`.otf`; Datei vor dem Ablegen mit der Font-Bibliothek laden
  (`opentype`), damit kaputte Dateien sofort auffallen.

## Typischer Ablauf „Foto → Template“

1. Nutzer lädt das Foto in den Chat und bittet um ein Template.
2. KI: `get_template_spec`, `create_template("meine-anzeige")`.
3. Foto per `upload_image` ablegen, mit `perspective_correct` entzerren.
4. Farben mit `sample_colors` bestimmen; Logos per `crop_image` übernehmen
   oder als SVG nachbauen (`svg_to_png`).
5. Hintergrund als SVG/Rechtecke, alle Texte als Text-Layer, `default.json`
   mit den Werten vom Foto.
6. `render_preview` → mit Foto vergleichen → nachjustieren, mehrere Runden.
7. Nutzer prüft im Editor/Vorschau und speichert bzw. exportiert das ZIP.

## Erwartbare Grenzen

- **Gut:** Aufbau erkennen (Zeit, Ziel, Via, Gattung, Hinweis), Farben,
  grobe Positionen, YAML schreiben, iterativ angleichen.
- **Mittel:** pixelgenaue Lage — Fotos sind schräg, unscharf, mit
  Spiegelungen. Ergebnis: „sieht aus wie“, nicht „identisch“.
- **Schwach:** fotorealistische Bildassets; die KI baut nach statt zu kopieren.
- **Schriften:** Die KI kann Originalschriften oft benennen, darf sie aber
  nicht mitliefern — siehe Abschnitt „Fonts“ (freie Alternativen).
- **Kosten/Kontext:** Jede Vorschau ist ein Bild im Chatverlauf; viele
  Runden füllen den Kontext. Vorschauen klein halten (240×240 reicht).

## Offene Fragen

- Welche KI-Clients sollen offiziell unterstützt werden (nur Claude Desktop
  oder auch andere MCP-Clients)?
- Font-Auswahl: welche freien Schriften, Lizenzen prüfen.
- Systemschriften und Google Fonts als Quelle zulassen, oder nur die
  mitgelieferte Auswahl?
- Wann ist Variante B (Webchat/remote) wirtschaftlich und rechtlich sinnvoll
  (Login, Speicher, Missbrauchsschutz, Datenschutz für hochgeladene Fotos)?

## Reihenfolge und grobe Aufwände

| Schritt | Inhalt | Aufwand (grob) |
|---|---|---|
| 1 | MCP-Grundgerüst + 7 Tools, stdio, User-Guide-Abschnitt | 1–2 Tage |
| 2 | Praxistest mit Claude Desktop an 2–3 echten Fotos | ½ Tag |
| 3 | `svg_to_png` mit `oksvg` | ½–1 Tag |
| 4 | Foto-Werkzeuge | 1–2 Tage |
| 5 | Parallelbetrieb: File-Watcher, SSE, Konfliktprüfung im Editor | 1–1½ Tage |
| 6 | Fonts: mitgelieferte Auswahl, `list_fonts`, `add_font` | ½–1 Tag |
| 7 | Entscheidung über Variante B (remote) | — |

Nach Schritt 2 entscheiden, ob die Qualität die weiteren Schritte rechtfertigt.
