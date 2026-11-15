# KI-Design-Leitfaden für Zugzielanzeiger-Templates

> Erste Fassung (Gerüst): Pflichtaussagen als Stichpunkte. Der Fließtext folgt in einem späteren Schritt.

## 1. Zielgerät

- Nur 1,05"-Displays: je Seite 240×120 px, Canvas 240×240, Template-Name `<name>-105-v1`.
- `create_template`/`write_file` lehnen andere Größen ab.

## 2. Vorder- und Rückseite

- Obere Hälfte y 0–119 = Seite A, untere Hälfte y 120–239 = Seite B; der Anzeiger hängt am Bahnsteig und wird von beiden Seiten gelesen.
- Elemente, die zu einem Ende des Geräts gehören (Gleisnummer, Uhr, Bahnhofs-/Gleis-Schild), sitzen **außen**: in der oberen Hälfte links, in der unteren Hälfte rechts (so machen es `umuc-105-v1`, `nederland-105-v1`, `faltblatt-105-v1`).
- Die untere Hälfte wird **gespiegelt aufgebaut**, nicht 1:1 kopiert: erst die obere Hälfte vollständig zeichnen, dann mit mehreren `copy`-Layern die Blöcke vertauscht nach y 120 setzen.
- Ein einzelner `copy` der ganzen oberen Hälfte ist **nur** für symmetrische Layouts ohne Rand-Element richtig (`rhb-105-v1`, `sbb-105-v1`, `oebb-105-v1`).
- umuc: oben Uhr+Gleis links (x 0–76), Liste rechts (x 120–239); unten `copy src 120,18 120×96 → 0,138` und `copy src 0,18 77×102 → 162,138`; Hintergrund `bg.png` ist 240×240 und enthält beide Hälften schon gespiegelt.
- nederland: Gleis-Schild links, Tafel Mitte, Uhr rechts; unten drei `copy` (Gleis → x 205, Uhr → x 0, Tafel → x 59), plus der „Parken-und-Zurückkopieren“-Trick, mit dem `copy` Text an der Tafelkante abschneidet.
- faltblatt: erst ganze obere Hälfte kopieren, dann zwei `copy` mit vertauschten Blöcken darüber (Gleis-Schild → x 186, Klappen → x 3).
- `copy` steht **am Ende** (kopiert alles davor, auch animierte Zeiger in jedem Frame).
- Quell- und Zielblöcke dürfen sich nicht überlappen.
- Lücken in der unteren Hälfte brauchen Hintergrund (240×240-`bg.png` oder `rect` vor den `copy`).

## 3. Anmutung

- Vollflächen, klare Farbfelder, 2–5 kräftige Farben, hoher Kontrast; keine Verläufe, Schatten, Fotorealismus, Transparenzeffekte.
- Belegte Paletten: Blau `#142a9b` + Weiß + Gelb `#fff048` + Rot `#ff0000` (SBB, U-Bahn München); Blau `#0000c6` + Weiß + Gelb `#fbef52` (ÖBB); Blau `#024ddc`/`#0415a1` + Weiß + Grau `#e6ebef` (NS); Gelb `#ffde00` auf Schwarz (RhB, LED-Optik).
- Hintergrund schwarz oder eine dunkle Vollfarbe; Fotos nur als Vorlage, nie als Hintergrund.

## 4. Schrift

- `size` = Pixelhöhe (72 dpi).
- Belegte Größen: Hauptzeile/Ziel 19–23 px, Zeit/Zug-Nr. 15–23 px, Via/Folgezeilen 12–16 px, Kopf-/Kleintext 10 px.
- **Minimum 10 px.**
- Fette Grotesk (Roboto Bold, Nimbus Sans Bold) für Hauptinhalte, Regular/Condensed für Listen.
- Pixel-Fonts (LLPIXEL3, 04B_03) mit `scale_x: 0.5` für LED-Optik.
- Lange Texte: `max_width` + Umbruch, `scale_x` 0.5–0.9 zum Stauchen, `fit_items` für Via-Listen.

## 5. Raster und Ränder

- Außenrand 2–5 px.
- Zeilenraster 20–26 px (umuc: Zeilen bei y 37/63/89).
- Fußband 16 px (nederland y 95–111).
- Textanfang ≥ 3 px vom Feldrand.

## 6. Dynamische Inhalte

- Alles, was sich je Zug ändert, ist ein Text-Layer mit `{{…}}`, nie Teil eines Bildes.
- Feldliste mit Beispielwerten aus den `default.json`: `gleis` (6), `abschnitt` ("A-F"), `folgezuege`, `zugN.zeit` ("15:54"), `zugN.vonnach` ("Sennhof-Kyburg"), `zugN.nr` ("IC23", "U1", "Sprinter"), `zugN.via` ("Othmarsingen - Mägenwil - …", Trenner " - "), `zugN.abw` (10; 0/leer = pünktlich), `zugN.hinweis` ("Abweichende Wagenreihung"), `fusszeile`, `abschnitte`, `reihung`.
- Systemvariablen `now`, `anim`.
- Optionale Elemente mit `if: "not(isEmpty(…))"`.
- Zuggattungs-Logos: `file` ist nicht interpolierbar → `if`/`elif`-Kette (umuc U1–U8).

## 7. Bilder

- Statische Teile (Rahmen, Logos, Piktogramme, Hintergrund) als PNG: bevorzugt `svg_to_png` (ohne Text, `antialias: false` für Pixel-/Punktmatrix-Optik), Fotos nur über die Foto-Werkzeuge.
- Hintergrund 240×120 (wird mitkopiert) oder 240×240.

## 8. Arbeitsweise

1. `get_template_spec` + `get_design_guide` lesen.
2. `list_templates`, 1–2 passende Beispiele mit `read_template` + `render_preview` ansehen.
3. `create_template`.
4. Obere Hälfte bauen, nach **jeder** Änderung `render_preview` und mit der Vorlage vergleichen.
5. Untere Hälfte gespiegelt.
6. Testdatensätze.
7. Abschluss-Checkliste.
8. Dem Nutzer Name, Vorschau und übrig gebliebene Zwischendateien nennen.

## 9. Testdatensätze

- Pflicht vor „fertig“: `render_preview` mit (a) `default.json`, (b) langem Ziel (`"Frankfurt (Main) Flughafen Fernbahnhof"`) und langer Via, (c) Verspätung (`abw: 25`) und Hinweis, (d) leerem Hinweis, leerem Via, leeren `zug2`/`zug3`.
- Die JSON-Beispiele stehen vollständig im Leitfaden (folgt).

## 10. Fonts

- `list_fonts` → `add_font`; nur vorhandene Quellen; Lizenzhinweis.

## 11. Foto → Template

- `import_image` → `perspective_correct` auf 240×120 (bzw. das Seitenverhältnis des Fotos) → `sample_colors` für exakte Hex-Werte → Logos per `crop_image` oder als SVG nachbauen → Texte als Text-Layer → vergleichen.

## 12. Abschluss-Checkliste

- 240×240; obere und untere Hälfte gefüllt; Gleis außen (falls vorhanden); keine Schrift < 10 px; alle Zugdaten als Platzhalter; vier Testdatensätze ohne Überlauf; `default.json` mit realistischen Werten.
