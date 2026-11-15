# KI-Design-Leitfaden für Zugzielanzeiger-Templates

Dieser Leitfaden richtet sich an dich, die KI, die über den MCP-Server von `zza`
Templates baut. Er ergänzt die Syntax-Referenz (`get_template_spec`) um die Regeln,
nach denen ein Template auf dem echten Anzeiger gut aussieht. Halte dich an die
Zahlen; sie stammen aus den vorhandenen Templates. Wenn eine Nutzer-Vorgabe einer
Regel widerspricht, folge dem Nutzer und sage ihm, welche Regel du dafür brichst.

## 1. Zielgerät

- Unterstützt wird **nur das 1,05"-Display**. Jede Seite des Anzeigers zeigt
  **240×120 px**, das Template-Bild umfasst beide Seiten: Canvas **240×240**.
- Jeder Template-Name endet auf **`-105-v1`**, z. B. `berlin-sbahn-105-v1`
  (Kleinbuchstaben, Ziffern, Bindestriche).
- `create_template` legt Templates nur mit diesem Suffix an; `write_file` lehnt jedes
  `template.yaml` ab, dessen `meta.canvas` nicht 240×240 ist. Versuche keine andere
  Größe — sie wäre auf dem Gerät unbrauchbar.
- Ausgegeben wird 1:1 (ein Bildpixel = ein Displaypixel). Das Display ist klein
  (gut 2,5 cm breit) und wird aus einem bis drei Metern gelesen.

## 2. Vorder- und Rückseite

Der Anzeiger hängt am Bahnsteig und hat zwei Displays Rücken an Rücken. Das Bild
enthält beide:

| Bereich | Koordinaten | Seite |
|---|---|---|
| obere Hälfte | y 0–119 | Seite A (Vorderseite) |
| untere Hälfte | y 120–239 | Seite B (Rückseite) |

Jede Hälfte wird aufrecht angezeigt, so wie sie im Bild steht.

### Rand-Elemente sitzen außen

Elemente, die zu einem **Ende des Geräts** gehören — Gleisnummer, Uhr, Bahnhofs- oder
Gleis-Schild — sitzen **außen**: in der oberen Hälfte **links**, in der unteren Hälfte
**rechts**. Wer vor Seite A steht, sieht das Gleis links; wer auf der anderen Seite
steht, sieht Seite B mit dem Gleis rechts — beides ist dasselbe Geräte-Ende.

So machen es `umuc-105-v1` (Uhr + Gleis), `nederland-105-v1` (Gleis-Schild, Uhr auf
der Gegenseite) und `faltblatt-105-v1` (Gleis-Schild).

Daraus folgt: Die untere Hälfte wird **gespiegelt aufgebaut**, nicht 1:1 kopiert.
„Gespiegelt" heißt: die **Blöcke** tauschen die Seiten, ihr Inhalt bleibt lesbar
(nichts wird gespiegelt dargestellt). Vorgehen:

1. Obere Hälfte vollständig zeichnen (Hintergrund, Rand-Block, Inhaltsblock).
2. Am Ende mit **mehreren `copy`-Layern** die Blöcke vertauscht nach y 120 setzen:
   Inhaltsblock nach links, Rand-Block nach rechts.

Ein **einzelner `copy` der ganzen oberen Hälfte** ist **nur** für symmetrische
Layouts ohne Rand-Element richtig: `rhb-105-v1`, `sbb-105-v1`, `oebb-105-v1`.
Hat dein Layout eine Gleisnummer, eine Uhr oder ein Schild an einer Seite, ist ein
einzelner `copy` falsch.

### Vollständiges Minimal-Beispiel

Gleis-Feld außen (oben x 0–43, unten x 196–239), Zeit, Ziel, Via und Hinweis im
Inhaltsfeld, eine Schrift. Das Beispiel rendert so, wie es hier steht (die Schrift
muss per `add_font` im Template liegen):

```yaml
meta:
  name: "Beispiel 1.05\""
  description: "Gleis-Feld außen, Zeit und Ziel innen"
  version: "1.0"
  canvas:
    width: 240
    height: 240

fonts:
  - id: bold
    file: Roboto-Bold.ttf

layers:
  # ---------- Obere Hälfte = Seite A (y 0–119) ----------
  - type: rect                     # Hintergrund: dunkle Vollfarbe
    x: 0
    y: 0
    width: 240
    height: 120
    color: "#142a9b"

  - type: rect                     # Gleis-Feld außen = oben links (x 0–43)
    x: 0
    y: 0
    width: 44
    height: 120
    color: "#ffffff"

  - type: text
    value: "Gleis"
    x: 22                          # align center ohne width: x = Mitte
    y: 34
    font: bold
    size: 10
    color: "#142a9b"
    align: center

  - type: text
    value: "{{gleis}}"
    x: 22
    y: 46
    font: bold
    size: 30
    color: "#142a9b"
    align: center

  # Inhaltsfeld (x 44–239), Text beginnt 6 px hinter der Feldkante
  - type: text
    id: zeit                       # liefert zeit.end für die Verspätung
    advance: 6
    value: "{{zug1.zeit}}"
    x: 50
    y: 4
    font: bold
    size: 19
    color: "#ffffff"

  - type: text
    if: "greaterThan(zug1.abw, 0)"  # 0 oder leer = pünktlich, nichts zeigen
    value: "+{{zug1.abw}}'"
    x: "{{zeit.end}}"
    y: 4
    font: bold
    size: 19
    color: "#fff048"

  - type: text
    value: "{{zug1.vonnach}}"
    x: 50
    y: 27
    font: bold
    size: 19
    scale_x: 0.8                   # gestaucht: lange Ziele brauchen höchstens 2 Zeilen
    max_width: 186                 # 50 + 186 = 236 → 4 px Rand rechts
    color: "#ffffff"

  - type: text
    value: "{{zug1.via}}"
    x: 50
    y: 74
    font: bold
    size: 12
    scale_x: 0.8
    max_width: 186
    color: "#ffffff"
    fit_items:                     # einzeilig, Stationen, die nicht passen, entfallen
      split_by: " - "              # Trenner mit Leerzeichen: "Frankfurt-Stadion" bleibt ganz
      gap: 6

  - type: rect                     # Hinweis-Band, Platz für zwei Zeilen
    if: "not(isEmpty(zug1.hinweis))"
    x: 44
    y: 90
    width: 196
    height: 30
    color: "#ff0000"

  - type: text
    if: "not(isEmpty(zug1.hinweis))"
    value: "{{zug1.hinweis}}"
    x: 50
    y: 92
    font: bold
    size: 12
    scale_x: 0.7                   # gestaucht, damit lange Hinweise in 2 Zeilen passen
    width: 186                     # Box: Umbruch auf 186 px …
    height: 26                     # … und vertikal mittig im Band
    valign: middle
    line_height: 0.9
    color: "#ffffff"

  # ---------- Untere Hälfte = Seite B (y 120–239), gespiegelt ----------
  - type: rect                     # Hintergrund für Lücken (hier lückenlos, schadet nie)
    x: 0
    y: 120
    width: 240
    height: 120
    color: "#142a9b"

  - type: copy                     # Inhaltsfeld nach links unten: x 44–239 → x 0–195
    src_x: 44
    src_y: 0
    src_width: 196
    src_height: 120
    x: 0
    y: 120

  - type: copy                     # Gleis-Feld nach rechts unten: x 0–43 → x 196–239
    src_x: 0
    src_y: 0
    src_width: 44
    src_height: 120
    x: 196
    y: 120
```

Rechenprobe für die `copy`-Layer: Die Zielblöcke liegen lückenlos nebeneinander
(0 + 196 = 196, 196 + 44 = 240). Das Gleis-Feld liegt oben bei x 0, unten bei x 196 —
beide Male am Außenrand.

### So machen es die vorhandenen Templates

**umuc-105-v1** — oben Uhr + Gleis links (x 0–76), Abfahrtsliste rechts (x 120–239).
Der Hintergrund `bg.png` ist 240×240 und enthält beide Hälften schon gespiegelt
(Zifferblatt unten rechts); die Kopfzeile y 0–17 (MVG-Logo, Notruf) steht in `bg.png`
und wird nicht kopiert. Am Ende zwei `copy`-Layer:

```yaml
# Untere Hälfte: Abfahrtsliste nach links unten …
- type: copy
  src_x: 120
  src_y: 18
  src_width: 120
  src_height: 96
  x: 0
  y: 138

# … Uhr und Gleis nach rechts unten
- type: copy
  src_x: 0
  src_y: 18
  src_width: 77
  src_height: 102
  x: 162
  y: 138
```

Die Uhr (x 0–76) landet bei x 162–238, die Liste (x 120–239) bei x 0–119. Da die
Zeiger vor den `copy`-Layern gezeichnet werden, laufen sie auf beiden Seiten.

**faltblatt-105-v1** — erst kopiert ein `copy` die ganze obere Hälfte (x 0–239,
y 0–119) nach y 120; er liefert Rahmen und Hintergrund überall dort, wo die
Block-Kopien nicht hinkommen. Darüber setzen zwei `copy` die vertauschten Blöcke:
das Gleis-Schild (x 3–51, y 10–119) nach rechts außen auf x 186, y 130, und die
Klappen (x 52–234, y 10–115) nach links auf x 3, y 130.

**nederland-105-v1** — oben Gleis-Schild links, Tafel in der Mitte, Uhr rechts; unten
drei `copy`: Gleis → x 205, Uhr → x 0, Tafel → x 59. Zusätzlich der
„Parken-und-Zurückkopieren"-Trick, mit dem `copy` Text an der Tafelkante abschneidet:
Vor dem ersten Text wird der Bereich rechts der Tafel (x 173–239) unberührt in die
noch leere untere Hälfte geparkt; nach dem Text wird er zurückkopiert und überdeckt
alles, was über die Tafel hinausläuft. Die Parkfläche wird später von den
End-`copy`-Layern überschrieben.

```yaml
- type: copy                 # parken (vor den Text-Layern)
  src_x: 173
  src_y: 0
  src_width: 67
  src_height: 120
  x: 59
  y: 120
# … Text-Layer der Tafel …
- type: copy                 # zurück (nach den Text-Layern): schneidet Überlauf ab
  src_x: 59
  src_y: 120
  src_width: 67
  src_height: 120
  x: 173
  y: 0
```

**rhb-105-v1** (symmetrisch, ohne Rand-Element) — ein einziger `copy` der ganzen
oberen Hälfte nach y 120 genügt.

### Regeln für `copy`

- Die Spiegel-`copy`-Layer für die untere Hälfte stehen **am Ende** der Layer-Liste.
  Ein `copy` kopiert, was zu diesem Zeitpunkt auf dem Canvas steht — alles davor,
  auch animierte Zeiger und Klappen in jedem Frame. Was danach gezeichnet wird,
  erscheint nur einmal. Einzige Ausnahme ist der nederland-Trick (Parken und
  Zurückkopieren), dessen zwei `copy` mitten in der Liste stehen.
- Quell- und Zielblöcke dürfen sich nicht überlappen (Quelle in y 0–119, Ziel in
  y 120–239). Mehrere Zielblöcke dürfen sich ebenfalls nicht gegenseitig überdecken,
  außer bewusst wie bei faltblatt (erst ganze Hälfte, dann Blöcke darüber).
- Lücken in der unteren Hälfte, die kein `copy` füllt, brauchen Hintergrund:
  ein 240×240-`bg.png` oder ein `rect` (y 120, Höhe 120) **vor** den `copy`-Layern.
  Sonst bleiben sie transparent.
- Prüfe nach dem Spiegeln mit `render_preview`: Steht das Gleis oben links **und**
  unten rechts? Ist jeder Block vollständig (nichts abgeschnitten, nichts doppelt)?
- Tippfehler sind still: ein falsch geschriebener Schlüssel (`src_widht`, `iff`)
  ist für den Renderer kein Fehler, der Wert wird einfach ignoriert. `write_file`
  meldet unbekannte Schlüssel als Warnung mit Zeilennummer. Wirkt eine Änderung im
  `render_preview` nicht, prüfe zuerst die Schreibweise gegen `get_template_spec`.

## 3. Anmutung

Echte Zugzielanzeiger sind Informationstafeln, keine Bilder. Baue sie so:

- **Vollflächen und klare Farbfelder**: flächige Hintergründe, scharf begrenzte
  Rechtecke für Gleis-Feld, Hinweis-Band, Fußzeile.
- **2–5 kräftige Farben, hoher Kontrast**: weiße oder gelbe Schrift auf dunklem Blau
  oder Schwarz, dunkle Schrift auf Weiß.
- **keine Verläufe**, keine Schatten, kein Fotorealismus, keine Transparenzeffekte,
  keine Glanzlichter, keine abgerundeten Deko-Elemente ohne Vorbild.
- Hintergrund schwarz oder eine dunkle Vollfarbe. Fotos nur als **Vorlage** zum
  Abnehmen von Maßen und Farben, nie als Hintergrund im fertigen Template.
- Schrift muss aus Lesedistanz lesbar sein: lieber weniger Information groß als viel
  Information klein.

Belegte Paletten aus den vorhandenen Templates:

| Vorbild | Farben |
|---|---|
| SBB (`sbb-105-v1`), U-Bahn München (`umuc-105-v1`) | Blau `#142a9b`, Weiß `#ffffff`, Gelb `#fff048`, Rot `#ff0000` |
| ÖBB (`oebb-105-v1`) | Blau `#0000c6`, Weiß `#ffffff`, Gelb `#fbef52` |
| NS (`nederland-105-v1`) | Blau `#024ddc` / `#0415a1`, Weiß `#ffffff`, Grau `#e6ebef` |
| RhB (`rhb-105-v1`, LED-Optik) | Gelb `#ffde00` auf Schwarz `#000000` |

Hat der Nutzer ein Foto oder eine Vorlage, nimm die Farben von dort (`sample_colors`,
Abschnitt 11) statt aus dieser Tabelle.

## 4. Schrift

- `size` ist die Schriftgröße in **Pixeln** (72 dpi): `size: 19` ergibt rund 19 px
  Zeilenhöhe. `size` ist eine feste Zahl, kein Ausdruck.
- `y` ist die **Oberkante** der Textzeile (mit `valign: baseline` die Grundlinie).
  Plane pro Textzeile etwa `size × 1,2` px Höhe.
- Belegte Größen:

| Element | Größe | Beispiele |
|---|---|---|
| Hauptzeile / Ziel | 19–23 px | nederland 19.5, sbb 21.07, oebb 22.67 |
| Zeit / Zug-Nr. | 15–23 px | nederland 15, oebb 16, sbb 21.33, oebb Linie 22 |
| Via / Folgezeilen / Hinweis | 12–16 px | nederland 15 und 12, sbb 13.33, oebb 15.2, Hinweis 14.67 |
| Listenzeilen (3 Züge) | 12 px | umuc |
| Kopf- / Kleintext | 10 px | umuc „Min.", Kopfzeile |
| Gleisnummer groß | 17–30 px (faltblatt 58.67) | umuc 17, nederland 21 |
| LED-Optik (rhb) | LLPIXEL3 40.89–58.67 mit `scale_x: 0.5`, 04B_03 17.78 | rhb |

- **Minimum 10 px.** Kleinere Schrift ist auf dem Display nicht lesbar; `write_file`
  warnt bei `size` < 10.
- Pixel-Fonts haben eine andere Größenskala. Die Zeilenhöhe misst du per
  `render_preview`, die `size`-Regel gilt hier nicht.
- Fette Grotesk (Roboto Bold, Nimbus Sans Bold) für Ziel, Zeit, Gleis; Regular oder
  Condensed (Roboto Condensed, Nimbus Sans Regular) für Listen und Via.
- LED-/Punktmatrix-Optik: Pixel-Fonts wie `LLPIXEL3.ttf` mit `scale_x: 0.5`, für
  Kleintext `04B_03__.TTF` (rhb).
- Lange Texte, in dieser Reihenfolge:
  1. `max_width` (oder `width`) setzen — Text bricht um; plane die zweite Zeile ein
     (Hinweis-Band im Minimal-Beispiel: 30 px hoch für zwei Zeilen);
  2. `scale_x` 0.5–0.9 zum Stauchen (0.8 ist kaum sichtbar, 0.5 ist LED-Optik);
  3. `fit_items` für Via-Listen mit `split_by: " - "` (der Renderer trimmt die Teile):
     einzeilig, Stationen, die nicht passen, entfallen. Die Bestands-Templates nutzen
     `split_by: "-"` und zerlegen deshalb Namen mit Bindestrich („Frankfurt-Stadion"
     → „Frankfurt" „Stadion") — übernimm das nicht.
  4. Nur als letztes Mittel `fit_items` mit `split_by: " "` für Fließtext (Hinweis):
     überzählige Wörter fallen dann **ersatzlos weg**, der Satz wird abgeschnitten.
- Text darf nie über das Feld oder die Seitenkante hinauslaufen. `max_width` =
  Feldbreite − linker Textabstand − rechter Rand.

## 5. Raster und Ränder

- Außenrand 2–5 px: Kein Text näher als 2 px an der Kante einer Seite.
- Textanfang ≥ 3 px vom Feldrand (im Beispiel 6 px hinter dem Gleis-Feld).
- Zeilenraster 20–26 px. umuc: Listenzeilen bei y 37 / 63 / 89 (26 px Abstand).
- Fußband 16 px hoch (nederland: y 95–110, `rect` + weiße 12-px-Schrift).
- Rand-Block (Gleis, Uhr) 35–77 px breit: nederland 35 px, faltblatt 49 px,
  umuc 77 px, Minimal-Beispiel 44 px.
- Rechne Blockgrenzen so, dass die `copy`-Layer glatt aufgehen
  (Rand-Block + Inhaltsblock = 240).

## 6. Dynamische Inhalte

Alles, was sich je Zug ändert, ist ein **Text-Layer mit Platzhalter** (`{{…}}`), nie
Teil eines Bildes. Feste Beschriftungen („Gleis", „Min.") dürfen fester Text sein.

| Platzhalter | Bedeutung | Beispielwerte aus `default.json` |
|---|---|---|
| `{{gleis}}` | Gleisnummer | `6` |
| `{{abschnitt}}` | Gleisabschnitt | `"A-F"` |
| `{{folgezuege}}` | Beschriftung Folgezüge | `"Folgezüge"` |
| `{{zug1.zeit}}` | Abfahrtszeit | `"15:54"` |
| `{{zug1.vonnach}}` | Ziel | `"Sennhof-Kyburg"`, `"Den Haag"` |
| `{{zug1.nr}}` | Zugnummer / Linie | `"IC23"`, `"U1"`, `"Sprinter"` |
| `{{zug1.via}}` | Unterwegshalte, Trenner `" - "` | `"Othmarsingen - Mägenwil - Mellingen - Zürich HB - Winterthur"` |
| `{{zug1.abw}}` | Verspätung in Minuten; `0` oder leer = pünktlich | `10` |
| `{{zug1.hinweis}}` | Hinweistext | `"Abweichende Wagenreihung"` |
| `{{zug1.fusszeile}}`, `{{zug1.abschnitte}}`, `{{zug1.reihung}}` | Zusatzfelder | meist leer |

`zug2` und `zug3` haben dieselben Felder (Folgezüge). Systemvariablen: `{{now}}`
(Uhrzeit, z. B. für eine Digitaluhr), `{{now.hour}}`, `{{now.minute}}` …, und
`{{anim.frame}}` für Animationen (siehe Spec).

Regeln:

- Optionale Elemente blendest du mit `if: "not(isEmpty(…))"` ein; leere Felder
  sonst einfach leer lassen (kein Fehler):

```yaml
- type: rect
  if: "not(isEmpty(zug1.hinweis))"
  x: 44
  y: 90
  width: 196
  height: 30
  color: "#ff0000"
```

- Verspätung nur zeigen, wenn sie größer 0 ist; Farbe wechseln statt Element
  verstecken geht auch:

```yaml
- type: text
  value: "{{zug1.zeit}}"
  x: 50
  y: 4
  font: bold
  size: 19
  color:
    if: "greaterThan(zug1.abw, 0)"
    then: "#fff048"
    else: "#ffffff"
```

- Elemente, die hinter einem Text variabler Länge stehen, hängst du per `id` und
  `advance` an (`x: "{{zeit.end}}"`, siehe Minimal-Beispiel).
- Zuggattungs-Logos: `file` ist **nicht interpolierbar** (`file: "{{zug1.nr}}.png"`
  geht nicht). Baue eine `if`/`elif`-Kette, mit Text-Fallback im `else` (umuc U1–U8):

```yaml
- type: image
  if: "equals(zug1.nr, 'U1')"
  file: U1.png
  x: 120
  y: 36
- type: image
  elif: "equals(zug1.nr, 'U2')"
  file: U2.png
  x: 120
  y: 36
- type: text
  else: true
  value: "{{zug1.nr}}"
  x: 121
  y: 37
  font: regular
  size: 12
  color: "#ffffff"
```

- Schreibe nie Beispieldaten fest ins Template (kein `value: "15:54"`); `write_file`
  warnt bei festen Uhrzeiten.
- Lege in `default.json` realistische Werte passend zum Vorbild an (Bahnhofsnamen,
  Linien, Zeiten der Region), mit allen Feldern aus Abschnitt 9.

## 7. Bilder

- Statische Teile — Rahmen, Logos, Piktogramme, Hintergrund — sind PNG-Dateien im
  Template und werden mit `type: image` gezeichnet.
- Bevorzugt erzeugst du sie mit
  `svg_to_png(template, name, svg, width, height, antialias?)`: einfache Formen
  (`rect`, `circle`, `path`, `polygon`), **kein Text** (Text gehört in einen
  Text-Layer), keine eingebetteten Bilder, keine `<style>`-Blöcke, keine externen
  Verweise. `antialias: false` für harte Kanten (Pixel-/Punktmatrix-Optik).
  `width`/`height` 1–1024 px; die `viewBox` wird auf diese Größe gestreckt.
  Grenzen: höchstens 2000 Formen und ein Rechenbudget (Formen × Breite × Höhe ≤ 2^26)
  — bei „svg too complex" Formen zusammenfassen oder kleiner rendern. Ein SVG ohne
  zeichenbare Form wird abgelehnt. Der Dateiname kann bereinigt werden: verwende im
  `image`-Layer den `name` aus dem Ergebnis.
- Erzeuge PNGs in **Zielgröße** (ein Logo 20×14 px als `width: 20, height: 14`),
  nicht groß und dann skaliert.
- Fotos verarbeitest du nur über die Foto-Werkzeuge (Abschnitt 11).
- Hintergrundbild: entweder **240×120** (nur obere Hälfte; die `copy`-Layer nehmen
  es mit nach unten) oder **240×240** mit beiden Hälften, die untere schon gespiegelt
  (umuc). Für flächige Hintergründe genügt ein `rect` — dafür braucht es kein Bild.
- Dateinamen: Kleinbuchstaben, Ziffern, `-`, `_`, `.`; keine Unterordner.

## 8. Arbeitsweise

1. `get_template_spec` und `get_design_guide` vollständig lesen.
2. `list_templates`, dann 1–2 passende Beispiele mit `read_template` und
   `render_preview` ansehen (Gleis außen: umuc, nederland, faltblatt; symmetrisch:
   sbb, oebb, rhb).
3. `create_template(name, title, description)` — der Starter zeigt schon das
   gespiegelte Gerüst (Gleis-Feld oben links, unten rechts).
4. Schriften holen (`list_fonts`, `add_font`, Abschnitt 10), Bilder erzeugen.
5. **Obere Hälfte bauen.** Nach **jeder** Änderung `write_file` (mit
   `expected_version` aus `read_template`/dem letzten `write_file`), Warnungen lesen,
   dann `render_preview` und das Bild mit der Vorlage vergleichen. Lehnt `write_file`
   ab, wurde nichts gespeichert — Fehlertext lesen, korrigieren, erneut schreiben.
   Meldet es `conflict`, hat der Nutzer im Editor geändert: neu lesen, Änderung neu
   anwenden. Achtung, Tippfehler sind still: unbekannte YAML-Schlüssel erzeugen im
   Renderer keinen Fehler; `write_file` meldet sie nur als Warnung
   (`line 42: unknown key "src_widht"`), bei abgelehnter Datei zusätzlich im
   Fehlertext. Wirkt eine Änderung im `render_preview`
   nicht, prüfe zuerst die Schreibweise gegen `get_template_spec`.
6. **Untere Hälfte gespiegelt** mit `copy`-Layern (Abschnitt 2), dann `render_preview`.
7. `default.json` mit realistischen Werten schreiben; die vier Testdatensätze
   (Abschnitt 9) durchspielen und das Layout nachbessern, bis nichts überläuft.
8. Abschluss-Checkliste (Abschnitt 12) abarbeiten.
9. Dem Nutzer Template-Namen und letzte Vorschau zeigen und übrig gebliebene
   Zwischendateien nennen (Fotos, Ausschnitte), die er löschen kann.

## 9. Testdatensätze

Pflicht vor „fertig": `render_preview` mit jedem der vier Datensätze. Prüfe in jedem
Bild beide Hälften: kein Text über Feld- oder Seitenkante, nichts überdeckt, leere
Felder hinterlassen keine Reste (leere Bänder, verwaiste „+'"-Zeichen).

**(a) Standarddaten** — `render_preview` **ohne** `data`: es gilt das `default.json`
des Templates. `create_template` legt es mit allen Feldern an (dieselben Felder wie
in (b)–(d)); ersetze die Werte durch realistische Daten passend zum Vorbild, lösche
keine Felder.

(b)–(d) übergibst du jeweils vollständig als `data`-Parameter:

**(b) Lange Texte** — langes Ziel, lange Via, langer Hinweis, zweistelliges Gleis,
volle Folgezüge:

```json
{
    "folgezuege": "Folgezüge",
    "gleis": 12,
    "abschnitt": "A-G",
    "zug1": {
        "vonnach": "Frankfurt (Main) Flughafen Fernbahnhof",
        "nr": "ICE 1234",
        "zeit": "23:59",
        "via": "Frankfurt-Stadion - Kelsterbach - Rüsselsheim - Bischofsheim - Mainz-Kastel - Wiesbaden Hbf",
        "abw": 0,
        "hinweis": "Bitte beachten Sie die geänderte Wagenreihung: Wagen 1 bis 5 halten in Abschnitt A bis C",
        "fusszeile": "",
        "abschnitte": "",
        "reihung": ""
    },
    "zug2": {
        "vonnach": "Garmisch-Partenkirchen",
        "nr": "RB 59466",
        "zeit": "00:12",
        "via": "Pasing - Starnberg - Tutzing - Weilheim - Murnau",
        "abw": 0,
        "hinweis": "",
        "fusszeile": "",
        "abschnitte": "",
        "reihung": ""
    },
    "zug3": {
        "vonnach": "Berchtesgaden Hbf",
        "nr": "RE 5",
        "zeit": "00:25",
        "via": "Rosenheim - Traunstein - Freilassing - Bad Reichenhall",
        "abw": 0,
        "hinweis": "",
        "fusszeile": "",
        "abschnitte": "",
        "reihung": ""
    }
}
```

**(c) Verspätung und Hinweis**:

```json
{
    "folgezuege": "Folgezüge",
    "gleis": 6,
    "abschnitt": "A-F",
    "zug1": {
        "vonnach": "Sennhof-Kyburg",
        "nr": "IC23",
        "zeit": "15:54",
        "via": "Othmarsingen - Mägenwil - Mellingen - Zürich HB - Winterthur",
        "abw": 25,
        "hinweis": "Heute ohne Halt in Mellingen",
        "fusszeile": "",
        "abschnitte": "",
        "reihung": ""
    },
    "zug2": {
        "vonnach": "Winterthur",
        "nr": "S12",
        "zeit": "16:02",
        "via": "Effretikon",
        "abw": 5,
        "hinweis": "",
        "fusszeile": "",
        "abschnitte": "",
        "reihung": ""
    },
    "zug3": {
        "vonnach": "",
        "nr": "",
        "zeit": "",
        "via": "",
        "abw": "",
        "hinweis": "",
        "fusszeile": "",
        "abschnitte": "",
        "reihung": ""
    }
}
```

**(d) Leere Felder** — kein Hinweis, keine Via, pünktlich, keine Folgezüge:

```json
{
    "folgezuege": "Folgezüge",
    "gleis": 6,
    "abschnitt": "A-F",
    "zug1": {
        "vonnach": "Sennhof-Kyburg",
        "nr": "IC23",
        "zeit": "15:54",
        "via": "",
        "abw": 0,
        "hinweis": "",
        "fusszeile": "",
        "abschnitte": "",
        "reihung": ""
    },
    "zug2": {
        "vonnach": "",
        "nr": "",
        "zeit": "",
        "via": "",
        "abw": "",
        "hinweis": "",
        "fusszeile": "",
        "abschnitte": "",
        "reihung": ""
    },
    "zug3": {
        "vonnach": "",
        "nr": "",
        "zeit": "",
        "via": "",
        "abw": "",
        "hinweis": "",
        "fusszeile": "",
        "abschnitte": "",
        "reihung": ""
    }
}
```

Läuft bei (b) etwas über: `max_width` setzen, `scale_x` verringern, Via mit
`fit_items` einzeilig machen, Zeilen enger setzen — nicht die Testdaten kürzen.
Bei animierten Templates zusätzlich `render_preview` mit `format: "gif"`.

## 10. Fonts

1. `list_fonts(query?)` zeigt alle verfügbaren Schriften: aus den vorhandenen
   Templates und aus dem Fonts-Ordner (`fonts_dir` im Ergebnis, ein Ordner neben
   dem Templates-Ordner; fehlt er, nennt `note` den vollen Pfad). Höchstens 200
   Einträge; ist `truncated: true`, mit `query` (Teil von Familie, Stil oder
   Dateiname, z. B. `"bold"`) eingrenzen.
2. `add_font(template, font, id?)` mit dem `key` aus `list_fonts` kopiert die Datei
   ins Template und trägt sie unter `fonts:` ein. Das Ergebnis nennt die `id`, die du
   in Text-Layern als `font:` verwendest.
3. Nur Schriften aus `list_fonts` verwenden — keine Dateinamen erfinden, keine
   Schriften aus dem Netz laden. Passt keine, sag dem Nutzer, dass er `.ttf`/`.otf`-
   Dateien in den `fonts_dir` legen kann, und nimm bis dahin die nächstbeste.
4. Lizenz: `add_font` liefert einen Lizenzhinweis. Gib ihn an den Nutzer weiter,
   besonders wenn er das Template weitergeben will.

Bewährte Schriften in den Templates: `Roboto-Bold.ttf`, `RobotoCondensed-Regular.ttf`
(umuc), `NimbusSanL-Bol.otf`, `NimbusSanL-Reg.otf` (nederland, sbb, faltblatt),
`LLPIXEL3.ttf`, `04B_03__.TTF` (rhb).

## 11. Foto → Template

Der Nutzer hat ein Foto eines echten Anzeigers und möchte es nachbauen. Reihenfolge:

1. **Foto holen**: `import_image(template, path, name?)` mit dem absoluten Pfad, den
   der Nutzer nennt (frag danach; Chat-Anhänge nicht als Base64 hochladen). Das
   Ergebnis nennt die gespeicherte Größe (längste Seite höchstens 2048 px, größere
   Fotos werden verkleinert, JPEG wird immer neu kodiert); alle Koordinaten der
   folgenden Werkzeuge beziehen sich auf diese Größe, nicht auf das Original.
2. **Entzerren**: `perspective_correct(template, src, dst, corners, width, height)`.
   `corners` sind die vier Ecken **einer** Displayseite im Foto, in Quellpixeln,
   im Uhrzeigersinn:
   **oben-links, oben-rechts, unten-rechts, unten-links**, z. B.
   `[{"x":412,"y":288},{"x":1630,"y":301},{"x":1622,"y":905},{"x":405,"y":890}]`.
   Ziel `width: 240, height: 120` (eine Seite in Displaygröße; hat die Tafel auf dem
   Foto ein anderes Seitenverhältnis, nimm dieses und skaliere später). Ein schief
   oder gedreht aufgenommenes Foto wird dabei gleich mit gerade gezogen.
3. **Farben messen**: `sample_colors(template, src, points, radius)` auf dem
   entzerrten Bild, `radius: 2`, ein Punkt mitten in jede Farbfläche. Übernimm die
   Hex-Werte, runde nicht auf „schöne" Farben. Jeder Wert hat auch `alpha`: Werte
   unter 255 liegen am Rand oder außerhalb des Fotos — Punkt weiter nach innen setzen.
4. **Logos**: mit `crop_image(template, src, dst, x, y, width, height)` ausschneiden
   (bei Bedarf `color_to_alpha(template, src, dst, color, tolerance)` für
   transparenten Hintergrund, `quantize_colors(template, src, dst, colors)` für
   saubere Flächen — beide verkleinern auf höchstens 2048 px —,
   `resize_image(template, src, dst, width, height, mode: "nearest")` für
   Pixel-Optik) — oder,
   besser bei einfachen Formen, als SVG mit `svg_to_png` nachbauen.
5. **Texte** als Text-Layer mit Platzhaltern nachbauen: Positionen und Größen aus dem
   entzerrten Bild abmessen (240×120 = Displaypixel), Schrift per `list_fonts`
   möglichst ähnlich wählen.
6. **Vergleichen**: `render_preview` neben das entzerrte Bild legen, Abweichungen
   korrigieren. Dann die untere Hälfte gespiegelt bauen (Abschnitt 2).
7. Das Foto wird nie als Hintergrund verwendet. Nenne dem Nutzer am Ende die
   Zwischendateien (Foto, entzerrtes Bild, Ausschnitte), die er löschen kann.

## 12. Abschluss-Checkliste

Erst wenn alle Punkte erfüllt sind, ist das Template fertig:

- [ ] Canvas 240×240, Name endet auf `-105-v1`.
- [ ] Obere Hälfte (y 0–119) und untere Hälfte (y 120–239) sind beide vollständig
      gefüllt, keine transparenten Lücken.
- [ ] Gleis, Uhr oder Schild (falls vorhanden) sitzen außen: oben links, unten rechts.
      Ein einzelner `copy` der ganzen Hälfte nur bei symmetrischem Layout.
- [ ] `copy`-Layer stehen am Ende; animierte Teile erscheinen auf beiden Seiten.
- [ ] Vollflächen, 2–5 kräftige Farben, keine Verläufe, Schatten oder Fotos.
- [ ] Keine Schrift kleiner als 10 px; Text mindestens 2 px vom Seitenrand.
- [ ] Alle Zugdaten sind Platzhalter (`{{zug1.…}}`, `{{gleis}}`), nichts fest
      eingetragen, nichts in Bildern.
- [ ] Alle vier Testdatensätze (a)–(d) mit `render_preview` geprüft: kein Überlauf,
      keine Reste leerer Felder.
- [ ] `default.json` enthält realistische Werte passend zum Vorbild und alle Felder.
- [ ] `write_file` meldet keine Warnungen mehr (oder du kannst jede begründen).
- [ ] Dem Nutzer Name, letzte Vorschau und übrig gebliebene Zwischendateien genannt.
