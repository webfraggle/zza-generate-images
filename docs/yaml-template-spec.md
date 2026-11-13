# YAML Template Specification

---

## Dateistruktur eines Templates

Alle Dateien liegen **flach** im Template-Verzeichnis — keine Unterordner.

```
templates/
└── sbb-096-v1/
    ├── template.yaml       ← Hauptdatei
    ├── background.png
    ├── logo-sbb.png
    ├── Roboto-Regular.ttf
    └── Roboto-Bold.ttf
```

---

## Aufbau der `template.yaml`

Eine Template-Datei hat drei Abschnitte:

1. `meta` — Metadaten des Templates
2. `fonts` — Schriftarten-Definitionen (wiederverwendbar)
3. `layers` — Zeichenobjekte von unten nach oben (wie Photoshop-Ebenen)

---

## 1. meta

```yaml
meta:
  name: "SBB 0.96\""
  description: "Schweizer Bundesbahnen, 0.96 Zoll Display"
  author: "dein-name"
  version: "1.0"
  display: "0.96 Zoll OLED"
  instructions: |
    Dieses Template erwartet zug1 und zug2 mit zeit, vonnach, nr und via.
    Optional: gleis für die Gleisanzeige.
  canvas:
    width: 160    # Breite in Pixeln
    height: 80    # Höhe in Pixeln
    colors: 32    # optional — Farbpalette reduzieren (2–256); weglassen = volle Farbe
  timezone: "Europe/Berlin"  # optional — IANA-Zeitzone für {{now.*}}; Standard Europe/Berlin
  animation:      # optional — nur relevant für GIF-Ausgabe
    fps: 20       # Splitflap-Modus: Frames pro Sekunde; Standard 20, erlaubt 5–25
    duration: 2.5 # Splitflap-Modus: maximale Laufzeit in Sekunden; Standard 2.5, erlaubt 0.5–5
    # Frame-Modus (statt fps/duration): frames, frame_delay, loop — siehe unten
```

- `display` (optional) — Bezeichnung des Ziel-Displays, z.B. `"0.96 Zoll OLED"`. Wird auf der Vorschau-Seite angezeigt.
- `instructions` (optional) — Freitext-Anleitung für Nutzer des Templates. Wird auf der Vorschau-Seite angezeigt. Mehrzeilig möglich mit YAML Block-Scalar (`|`).
- `canvas.colors` (optional) — Reduziert die Ausgabe auf eine Indexed-PNG mit maximal N Farben (2–256) via Median-Cut-Algorithmus. Verringert die Dateigröße bei einfachen Grafiken erheblich. Weglassen oder `0` = volle 32-Bit-Farbe. Bei GIF-Ausgabe ist es die Zahl der echten Farben der globalen Palette (höchstens 255); der transparente Eintrag kommt zusätzlich dazu, siehe [GIF-Ausgabe](#gif-ausgabe).
- `timezone` (optional) — IANA-Zeitzone (`Europe/Berlin`, `UTC`, `America/New_York`, …), in der `{{now.*}}` und `format(…)` ausgewertet werden. Standard: `Europe/Berlin` (nicht die Zeitzone des Servers). Ungültige Namen sind ein Template-Fehler (`invalid meta.timezone`): höchstens 64 Zeichen, nur Buchstaben, Ziffern, `_`, `+`, `-` und `/` als Trenner, `Local` ist nicht erlaubt. Die Zeitzonen-Datenbank ist in die Binärdatei eingebettet.
- `animation` (optional) — Einstellungen für GIF-Ausgabe. Es gibt zwei Modi, die sich gegenseitig ausschließen:

| Feld | Typ | Modus | Standard | Erlaubt |
|---|---|---|---|---|
| `fps` | int | Splitflap | 20 | 5–25 |
| `duration` | float (s) | Splitflap | 2.5 | 0.5–5 |
| `frames` | int | Frame | 0 = aus | 2–125 |
| `frame_delay` | float (s) | Frame | 1 | 0.02–10, Vielfaches von 0.01 |
| `loop` | bool | beide | `false` | GIF endlos wiederholen; `false` = einmal abspielen |

  - **Frame-Modus** (`frames` gesetzt): Das Template wird `frames`-mal gerendert, jedes Frame dauert `frame_delay` Sekunden, siehe [Frame-Animation](#frame-animation). Gilt als Template-Fehler (HTTP 422 beim Rendern), wenn `frames` außerhalb 2–125 liegt, wenn zusätzlich `fps` oder `duration` gesetzt sind, wenn das Template einen `splitflap`-Layer enthält, wenn `frame_delay` außerhalb 0.02–10 liegt oder kein Vielfaches von 0.01 ist oder wenn `frames × frame_delay` 120 s überschreitet. Außerdem müssen `frames × Breite × Höhe` höchstens 32.000.000 Pixel ergeben.
  - **Splitflap-Modus** (`frames` nicht gesetzt, Template enthält `splitflap`): `fps` 5–25 und `duration` 0.5–5, siehe [`type: splitflap`](#type-splitflap). Die **maximal möglichen** Bilder (`duration × fps`, höchstens 125) × Breite × Höhe dürfen höchstens 32.000.000 Pixel ergeben. Geprüft wird diese Obergrenze vor dem Rendern, nicht die tatsächliche Frame-Zahl. Ein gesetztes `frame_delay` ohne `frames` ist ein Fehler.
  - **Nicht animiert** (weder `frames` noch `splitflap`): `?format=gif` liefert ein GIF mit einem Frame, `loop` wird ignoriert. Werte außerhalb von `fps`/`duration` sind trotzdem ein Template-Fehler, `frame_delay` ohne `frames` ebenfalls.

---

## 2. fonts

Schriftarten werden einmal definiert und dann per `id` referenziert.

```yaml
fonts:
  - id: regular
    file: Roboto-Regular.ttf

  - id: bold
    file: Roboto-Bold.ttf
```

---

## 3. layers

Die Layer werden **von oben nach unten** gerendert — der erste Layer liegt ganz unten,
der letzte ganz oben. Jeder Layer hat einen `type`.

### Verfügbare Typen

| Typ      | Beschreibung         |
|----------|----------------------|
| `image`  | Bilddatei einfügen   |
| `rect`   | Gefülltes Rechteck   |
| `text`   | Text ausgeben        |
| `copy`   | Bereich kopieren (z.B. obere Hälfte auf untere Hälfte spiegeln) |
| `loop`   | Sub-Layer über gesplitteten String iterieren |
| `splitflap` | Faltblatt-Klappen (Einzelzeichen-Raster oder ganze Tafeln), animierbar als GIF |

---

### Koordinaten-Ausdrücke

Die Felder `x`, `y`, `width`, `height` und `max_width` akzeptieren neben ganzzahligen Werten auch **arithmetische Ausdrücke** in `{{...}}`-Syntax:

```yaml
x: "{{i * 20 + 10}}"      # Abstand 20, Start bei 10
y: "{{loop.index * 12}}"  # 12px pro Element
width: "{{12 * 3}}"       # auch konstante Ausdrücke erlaubt
```

**Unterstützte Operatoren:** `+`, `-`, `*`, `/` (Ganzzahldivision), Klammern

**Verfügbare Variablen in Ausdrücken:**

| Variable       | Beschreibung                              | Nur in        |
|----------------|-------------------------------------------|---------------|
| `i`            | Loop-Index, 0-basiert (Kurzform)          | `type: loop`  |
| `loop.index`   | Loop-Index, 0-basiert (Langform)          | `type: loop`  |
| `loop.y`       | Absolutes Y des aktuellen Elements        | `type: loop`  |

Außerhalb eines Loops sind `i` und `loop.*` nicht definiert → Fehler beim Rendern.

> **`size` ist kein Ausdrucksfeld.** Die Schriftgröße ist eine feste Zahl (Kommazahlen erlaubt, z.B. `21.33`); `{{…}}` wird dort nicht ausgewertet.

---

### type: image

```yaml
- type: image
  file: background.png
  x: 0
  y: 0
  width: 160    # optional — Standard: Originalgröße
  height: 80    # optional — Standard: Originalgröße
  rotate: 0     # optional — Drehwinkel in Grad (Uhrzeigersinn)
```

**Rotation** wird verwendet für analoge Uhren. Der Winkel kann über einen Ausdruck berechnet werden und darf größer als 360° sein.

> **Hinweis:** Wenn `rotate` gesetzt ist, sind `x` und `y` die **Mittelpunkt-Koordinaten** des Bildes auf dem Canvas (nicht die obere linke Ecke). Das Bild wird um seine eigene Mitte gedreht und dann so platziert, dass sein Mittelpunkt auf `(x, y)` liegt. Die Drehachse liegt für jeden Winkel exakt auf `x`/`y` (kein Zittern bei dünnen Zeigern).

```yaml
# Minutenzeiger: 360° / 60 Minuten = 6° pro Minute
# x/y = Mittelpunkt des Zeigers auf dem Canvas
- type: image
  file: clock-minutes.png
  x: 120   # Mittelpunkt X auf dem Canvas
  y: 120   # Mittelpunkt Y auf dem Canvas
  rotate: "{{now.minute | mul(6)}}"

# Stundenzeiger: 360° / 12 Stunden = 30° pro Stunde
- type: image
  file: clock-hour.png
  x: 120
  y: 120
  rotate: "{{now.hour12 | mul(30) | add(now.minute | div(2))}}"   # inkl. Minutenanteil

# Sekundenzeiger (läuft im GIF, siehe Frame-Animation)
- type: image
  file: clock-seconds.png
  x: 120
  y: 120
  rotate: "{{now.second | add(anim.frame) | mul(6)}}"
```

---

### type: rect

```yaml
- type: rect
  x: 0
  y: 60
  width: 160
  height: 20
  color: "#FFFF00"
```

---

### type: text

```yaml
- type: text
  value: "{{zug1.zeit}}"
  x: 5
  y: 10
  font: bold          # Referenz auf fonts[].id
  size: 14            # Schriftgröße in Punkten
  color: "#FFFFFF"
  align: left         # left | center | right
  valign: top         # top | middle | bottom — benötigt height; baseline — y ist die Grundlinie
  width: 80           # optional — Boxbreite für Ausrichtung + Zeilenumbruch
  height: 20          # optional — Boxhöhe für vertikale Ausrichtung
  max_width: 80       # optional — nur Zeilenumbruch (alternativ zu width)
  line_height: 1.5    # optional — Zeilenabstand als Multiplikator (Standard: 1.0)
  scale_x: 0.5        # optional — horizontale Skalierung der Schrift (Standard: 1)
  fit_items:          # optional — einzeilige Element-Einpassung (Elemente entfallen nur mit max_width)
    split_by: "-"
    gap: 2
```

**`valign: baseline`** — `y` ist die Grundlinie der ersten Zeile (so rechnet das Legacy-PHP), nicht die Oberkante. `height` wird dabei nicht gebraucht.

**`scale_x`** — Skaliert die Schrift horizontal (`0.5` = halbe Breite, `1` oder weglassen = unverändert). Erlaubt sind `0.05` bis `8`; andere Werte (auch negative, `.inf`, `.nan`) lehnt das Laden des Templates ab. Text, der skaliert weit über die Leinwand hinausragt, wird nur im sichtbaren Bereich gezeichnet. Ausrichtung, `width`, `max_width` und Zeilenumbruch rechnen mit der skalierten Breite. Das Legacy-Theme staucht fast alle Texte mit `xFactor 0.5`; das entspricht `scale_x: 0.5`.

**`fit_items`** — Setzt den Wert einzeilig und linksbündig in Elemente: Der Wert wird an `split_by` geteilt (Elemente werden getrimmt, leere entfallen) und die Elemente werden mit `gap` Pixeln Abstand nebeneinander gesetzt. Ein Element wird nur gezeichnet, wenn `xpos + Breite < max_width` gilt; `xpos` rückt auch für ausgelassene Elemente um `Breite + gap` weiter — wie die Via-Schleife im Legacy-PHP. Ohne `max_width` (oder mit `0`) wird nichts weggelassen. Zeilenumbruch und `align` gelten in diesem Modus nicht.

**Portierung von Legacy-PHP-Themes:** Schriftgröße = Legacy-Wert × 16/9 (z.B. Legacy 12 → `size: 21.33`); Legacy-`y` eines Textes ist die Grundlinie (`valign: baseline`); Legacy-`xFactor 0.5` ist `scale_x: 0.5`.

---

### type: loop

Iteriert über einen gesplitteten String und rendert Sub-Layer für jedes Element. Typisch für Via-Stationen bei SBB (Icon + Text pro Station).

```yaml
- type: loop
  value: "{{zug1.via}}"   # String der gesplittet wird
  split_by: "|"            # Trennzeichen
  var: "item"              # Name der Loop-Variable (in Sub-Layern als {{item}} verfügbar)
  max_items: 6             # Sicherheits-Limit (DoS-Schutz); Standard: 20, Max: 200
  layers:                  # Sub-Layer — alle Koordinaten absolut, {{i}} für Berechnungen
    - type: image
      file: via-dot.png
      x: 45
      y: "{{i * 12 + 30}}" # i=0 → y=30, i=1 → y=42, i=2 → y=54, …
    - type: text
      value: "{{item}}"
      x: 55
      y: "{{i * 12 + 30}}"
      font: regular
      size: 9
      color: "#888888"
      max_width: 100
```

**Automatische Loop-Variablen** (in Sub-Layern verfügbar):

| Variable        | Beschreibung                              |
|-----------------|-------------------------------------------|
| `{{item}}`      | Aktuelles Element (Name via `var`)        |
| `{{i}}`         | Aktueller Index, 0-basiert (Kurzform)     |
| `{{loop.index}}`| Aktueller Index, 0-basiert (Langform)     |

Alle Koordinaten in Sub-Layern sind **absolut** — kein implizites Y-Offset.
Positionierung über Ausdrücke: `y: "{{i * 12 + 30}}"`.

**Leere Elemente** nach dem Split werden übersprungen.
**Wenn `value` leer ist**, wird der Loop nicht ausgeführt (kein Fehler).

---

### type: splitflap

Eine oder mehrere **Klappen** (Module) einer mechanischen Faltblattanzeige. Ein Modul ist ein Rechteck mit horizontaler Teilung (Scharnier); sein Inhalt wird durch Kind-Layer gezeichnet, die den aktuellen Wert über eine Variable bekommen (ähnlich `loop`). Im PNG-Render und im letzten GIF-Frame zeigt jedes Modul seinen Zielwert — ein `splitflap`-Layer ist also auch ohne Animation nutzbar (z.B. als Einzelzeichen-Raster).

```yaml
# Zielzeile: 13 Einzelklappen
- type: splitflap
  x: 54              # linke obere Ecke des ersten Moduls
  y: 82
  width: 11          # Modulgröße = ganzes Blatt
  height: 30
  split_y: 14        # Scharnier, relativ zum Modul (Standard: height / 2)
  cells: 13          # Module nebeneinander (Standard 1, max. 64)
  pitch: 14          # Abstand Modul zu Modul in x (Standard: width)
  split: chars       # ein Zeichen (Rune) pro Modul; weglassen = ganzer Wert in einem Modul
  value: "{{zug1.vonnach | upper}}"
  reel: " ABCDEFGHIJKLMNOPQRSTUVWXYZÄÖÜ-."   # Vorrat, durch den ein Modul blättert
  var: ch            # Variable für die Kind-Layer
  flip_frames: 2     # Animation: Frames pro Umklappen (1–6, Standard 2)
  min_flips: 8       # Animation: mindestens so viele Umklappschritte (Standard 4)
  max_flips: 16      # Animation: höchstens so viele (Standard: so viele, wie in duration passen)
  layers:            # Kind-Layer; Koordinaten relativ zur linken oberen Ecke des Moduls
    - type: text
      value: "{{ch}}"
      x: 5
      y: 23
      valign: baseline
      align: center
      scale_x: 0.5
      font: bold
      size: 21.33
      color: "#3a3c3b"

# Zugart: eine ganze Tafel, blättert durch die anderen Zugarten
- type: splitflap
  x: 53
  y: 48
  width: 90
  height: 28
  split_y: 15
  value: "{{zug1.nr | lower}}"
  reel: ["rb", "re", "ic", "ice", ""]
  var: nr
  flip_frames: 3
  max_flips: 8
  layers:
    - if: "startsWith(nr, 'ice')"      # ice vor ic prüfen
      layers:
        - type: image
          file: ice.png
          x: 2
          y: 5
    - elif: "startsWith(nr, 'ic')"
      layers:
        - type: image
          file: ic.png
          x: 2
          y: 5
    # … rb, re
```

Beide Beispiele stammen aus `templates/faltblatt-105-v1/template.yaml` (dort stehen alle Layer vollständig).

| Feld | Beschreibung |
|---|---|
| `x`, `y`, `width`, `height` | Rechteck des **ersten** Moduls (Ausdrücke erlaubt). Das Modul deckt das ganze Blatt ab, wie es im Hintergrundbild gezeichnet ist. `width`/`height` müssen positiv sein; das Modul muss komplett im Canvas liegen |
| `split_y` | Scharnierlinie relativ zum Modul, `1` bis `height-1`; Standard `height / 2`. Auf den Scharnierstrich des Overlays legen |
| `cells` | Anzahl Module nebeneinander; Standard 1, max. 64 |
| `pitch` | x-Abstand von Modul zu Modul; Standard `width` |
| `split` | `chars` = ein Zeichen pro Modul; fehlt = ganzer Wert in einem Modul. **`cells > 1` verlangt `split: chars`**. Überzählige Zeichen werden abgeschnitten, fehlende Module bekommen den Wert `""` |
| `value` | Zielwert (Variablen, Filter und Bedingungen wie bei `text`) |
| `reel` | Vorrat, durch den ein Modul beim Animieren blättert: String (ein Eintrag pro Zeichen) oder YAML-Liste (ein Eintrag pro Listenelement, z.B. `["00", "01", …]`). Max. 64 Einträge pro Layer. Ohne `reel` blättert das Modul durch leere Klappen |
| `var` | Name der Variable, unter der die Kind-Layer den aktuellen Wert sehen (in Werten **und** Bedingungen) |
| `flip_frames` | Animation: Frames pro Umklappen, 1–6, Standard 2 |
| `min_flips`, `max_flips` | Animation: Spanne der Umklappschritte pro Modul (siehe unten) |
| `layers` | Kind-Layer: `text`, `image`, `rect` und Block-Nodes (`if`/`elif`/`else`). **Nicht** erlaubt: `loop`, `copy`, verschachtelte `splitflap` (auch nicht in nicht-genommenen Zweigen) |

**Regeln**

- **Blatt-Hintergrund = Leinwand-Schnappschuss.** An der Stelle des Layers in der Zeichenreihenfolge wird der Leinwandinhalt unter dem Modulrechteck **einmal pro Wert** als Blatt-Hintergrund genommen; jeder Wert wird darauf gezeichnet. Layer, die **vor** dem `splitflap`-Layer stehen, bilden also das Blatt (z.B. das Hintergrundbild mit den leeren Klappen). Ein Blatt ist deckend; beim Umklappen scheint nichts vom Nachbarwert durch.
- Ein `splitflap` darf deshalb nicht auf etwas liegen, das sich selbst pro Frame ändert (z.B. auf einem anderen `splitflap`-Modul oder einem `now`-abhängigen Layer). Module dürfen sich außerdem **nicht überlappen** (Template-Fehler).
- **Overlays** (Scharniere, Rahmen, Schattierung) gehören **nach** die `splitflap`-Layer. `copy`-Layer danach folgen der Animation automatisch.
- Kind-Koordinaten sind relativ zum Modul; gezeichnet wird auf das Modulrechteck geclippt. Feste Beschriftung auf einer Klappe (z.B. „Abfahrt" neben der Stunde) ist ein konstanter Kind-Layer und klappt mit.
- `splitflap` darf in Block-Nodes stehen, nicht in `loop`.
- Der Zielwert muss **nicht** im `reel` stehen; er wird am Ende trotzdem angezeigt. Bei `split: chars` werden Zeichen außerhalb des `reel` genau so gezeichnet, wie sie sind.
- Limits: max. 64 Module pro Template (Summe aller `splitflap`-Layer), max. 64 `reel`-Einträge pro Layer, max. 125 Frames pro Animation, `meta.animation.fps` 5–25, `meta.animation.duration` 0.5–5 s, maximal mögliche Frames (`duration × fps`, höchstens 125) × Breite × Höhe ≤ 32.000.000 Pixel (Obergrenze, vorab geprüft). Überschreitungen sind Template-Fehler (HTTP 422 beim Rendern).

**Animationsmodell (nur GIF)**

- Ein Umklappen von Wert A auf Wert B dauert `flip_frames` Frames: erst fällt die obere Hälfte (obere Hälfte von A, gestaucht und abgedunkelt, über der oberen Hälfte von B), dann klappt die untere Hälfte von B nach unten über A. Solange ein Modul läuft, folgt Umklappen auf Umklappen ohne Ruheframe.
- Jedes Modul durchläuft `k` Schritte entlang seines `reel` (zyklisch) und endet auf dem Zielwert. Liegt der Zielwert im `reel`, wird die Startposition so gewählt, dass der Lauf dort ankommt; sonst folgt er als letzter Schritt auf `reel`-Einträge. `k` wird pro Modul pseudozufällig aus `[min_flips, max_flips]` gezogen; die Grenzen werden auf das begrenzt, was in `duration` passt. Die Module stoppen deshalb zu unterschiedlichen Zeiten.
- **Deterministisch:** Der Zufall ist ein Hash aus Template-Name, Request-Body, Layer- und Modulnummer. Gleiches Template + gleicher Request-Body ⇒ **bytegleiches GIF**. Der Zeitpunkt für `{{now.*}}` wird einmal pro Animation eingefroren; die Zusage gilt daher nur für Templates ohne `now`.
- **Start und Ende:** Frame 0 zeigt bereits alle Module in Bewegung, denn das Display zeigt vorher das Standbild des alten Zugs. Das letzte Frame ist das Standbild und entspricht dem PNG-Render derselben Daten (bis auf die GIF-Palettierung). Das GIF spielt **einmal** (keine Loop-Extension) und bleibt auf dem letzten Frame stehen.
- Alle übrigen Layer (auch `copy`) werden pro Frame normal gerendert.
- Ein Template ohne `splitflap`-Layer und ohne `meta.animation.frames` ergibt bei `?format=gif` ein GIF mit einem einzigen Frame.

#### GIF-Ausgabe

Für Firmware-Autoren: Das GIF hat **eine globale Farbtabelle** (keine lokalen Tabellen) mit `min(canvas.colors, 255)` echten Farben (Standard 255) **plus** einem zusätzlichen transparenten Index (`canvas.colors: 32` ergibt also 33 Einträge). Frame 0 ist vollständig; alle weiteren Frames sind **Delta-Frames** über das Rechteck der geänderten Pixel, mit Disposal „do not dispose" (Frames werden aufeinander komponiert). Unveränderte Pixel innerhalb eines Delta-Rechtecks sind transparent. Aufeinanderfolgende identische Frames werden zu einem zusammengefasst (Verzögerung addiert). Verzögerung: jedes Frame vor dem letzten dauert `round(100/fps)` Hundertstelsekunden (bei 20 fps 5 cs), das letzte Frame 100 cs (1 s). Ist das letzte Frame identisch mit dem vorletzten, werden beide zusammengefasst und die Standzeit ist entsprechend länger als 1 s.

**Delay und Loop je Modus:**

| | Splitflap | Frame-Modus |
|---|---|---|
| Delay je Frame | `round(100/fps)` cs | `frame_delay` (in cs) |
| letztes Frame | 100 cs (Halte-Frame) | wie alle anderen (kein Halten) |
| Wiederholung | `loop: true` = endlos, sonst einmal | `loop: true` = endlos, sonst einmal |

Palette und Delta-Frames sind in beiden Modi gleich. Im Frame-Modus werden aufeinanderfolgende identische Frames ebenfalls zusammengefasst (Verzögerung addiert).

**Cache:** Templates, deren Ausgabe von der Zeit abhängt (irgendein `{{…}}` enthält `now` oder den Filter `format(…)`, auch in `if`/`elif`), werden vom Server nicht gecacht. Die Antwort trägt `X-Cache: BYPASS` und `Cache-Control: no-store`. Das gilt für PNG und GIF.

---

## Frame-Animation

Jedes Template kann ohne `splitflap` als Frame-Animation gerendert werden, z.B. für einen laufenden Sekundenzeiger. Aktiviert wird sie durch `meta.animation.frames` (Felder und Grenzen: siehe [meta](#1-meta)).

```yaml
meta:
  timezone: "Europe/Berlin"
  animation:
    frames: 60        # eine Minute …
    frame_delay: 1    # … in Sekundenschritten
    loop: true        # danach von vorn
```

**Ablauf:** Bei `?format=gif` wird die komplette Layer-Liste `frames`-mal durchlaufen, jedes Frame mit eigener `anim.frame`. `copy`-Layer übernehmen den Zustand des jeweiligen Frames. Es gibt keinen zusätzlichen Halte-Frame am Ende. Gleiche aufeinanderfolgende Frames werden zusammengefasst (Verzögerung addiert).

**`now` ist eingefroren:** Die Uhrzeit wird einmal pro Render gelesen und gilt für alle Frames. Stunden- und Minutenzeiger stehen im GIF also still, nur Ausdrücke mit `anim.*` ändern sich. Der Sekundenzeiger beginnt bei der aktuellen Sekunde (`now.second`) und läuft pro Frame weiter.

**PNG = Frame 0:** `?format=png` (und Vorschau) rendert das Template mit `anim.frame = 0`, also pixelgleich zu Frame 0 des GIFs.

### Animationsvariablen

| Variable | GIF im Frame-Modus | PNG, Splitflap-GIF, nicht animiert |
|---|---|---|
| `anim.frame` | 0 … `frames`−1 | 0 |
| `anim.frames` | `frames` | `frames` im Frame-Modus (PNG), sonst 1 |
| `anim.time` | `frame × frame_delay` in Sekunden (`0`, `1`, `0.3`, …) | 0 |

`anim.*` gibt es in allen interpolierten Feldern (`value`, `color`, `rotate`, …), in Bedingungen und in `loop`-Layern. `anim.frame` und `anim.frames` sind zusätzlich in Koordinaten-Ausdrücken nutzbar (`x: "{{anim.frame * 2}}"`). `anim` ist reserviert wie `now`: ein gleichnamiges JSON-Feld ist nicht erreichbar.

### Beispiel: laufender Sekundenzeiger

```yaml
meta:
  canvas: { width: 240, height: 240 }
  animation:
    frames: 60
    frame_delay: 1
    loop: true

layers:
  - type: image
    file: zifferblatt.png
    x: 0
    y: 0
  - type: image          # Drehpunkt = Bildmitte, bei 0° nach oben
    file: clock-seconds.png
    x: 120
    y: 120
    rotate: "{{now.second | add(anim.frame) | mul(6)}}"
```

Nach 60 Frames steht der Zeiger wieder auf der Startposition, die Endlosschleife springt nicht.

### type: copy

Kopiert einen rechteckigen Bereich des Canvas auf eine andere Position. Wird verwendet um z.B. die obere Displayhälfte auf die untere zu spiegeln (typisch für Zugzielanzeiger mit zwei identischen Zeilen).

```yaml
- type: copy
  # Quellbereich
  src_x: 0
  src_y: 0
  src_width: 160
  src_height: 80
  # Zielposition
  x: 0
  y: 81
```

---

## Variablen

Variablen aus dem JSON-Input werden mit `{{` und `}}` eingebettet.

### Verfügbare Variablen (aus der Zug-JSON)

```
{{zug1.zeit}}        Abfahrtszeit       z.B. "15:53"
{{zug1.vonnach}}     Ziel               z.B. "Neulengbach"
{{zug1.nr}}          Zugnummer          z.B. "S1", "IC 123"
{{zug1.via}}         Via-Text           z.B. "Wien Hütteldorf"
{{zug1.abw}}         Verspätung (Min.)  z.B. 5
{{zug1.hinweis}}     Hinweistext        z.B. "*Hält nicht in ..."
{{zug2.zeit}}        ... (analog für zug2, zug3)
{{gleis}}            Gleisnummer        z.B. "3"
```

### Systemvariablen (automatisch befüllt)

```
{{now}}              Aktuelle Uhrzeit   z.B. "15:51"
{{now.hour}}         Stunde 0–23        z.B. 15
{{now.hour12}}       Stunde 1–12        z.B. 3
{{now.minute}}       Minute 0–59        z.B. 51
{{now.second}}       Sekunde 0–59       z.B. 7
{{now.day}}          Tag 1–31           z.B. 24
{{now.month}}        Monat 1–12         z.B. 3
{{now.year}}         Jahr               z.B. 2026
{{now.weekday}}      Wochentag          z.B. "Dienstag"
```

`now` wird in der Zeitzone `meta.timezone` ausgewertet (Standard `Europe/Berlin`) und ist pro Render (bzw. pro GIF) eingefroren.

```
{{anim.frame}}       Frame-Nummer       0 … frames−1 (sonst 0)
{{anim.frames}}      Anzahl Frames      z.B. 60 (sonst 1)
{{anim.time}}        Sekunden seit Start  z.B. 12 (sonst 0)
```

Datum/Zeit lässt sich auch formatieren:
```yaml
value: "{{now | format('HH:mm')}}"     # → "15:51"
value: "{{now | format('dd.MM.yyyy')}}" # → "24.03.2026"
```

Format-Tokens: `HH` (Stunde 00–23), `hh` (Stunde 01–12), `mm` (Minute), `ss` (Sekunde), `dd` (Tag), `MM` (Monat), `yyyy` (Jahr), `EE` (Wochentag kurz), `EEEE` (Wochentag lang)

### Text-Filter

Filter werden mit `|` angehängt und transformieren den Wert vor der Ausgabe.
**Filter sind kombinierbar** — sie werden von links nach rechts ausgeführt.

```yaml
value: "{{zug1.hinweis | strip('*')}}"              # Führendes * entfernen
value: "{{zug1.hinweis | stripBetween('{', '}')}}"  # Alles zwischen { } entfernen (inkl. Klammern)
value: "{{zug1.vonnach | upper}}"                   # Großbuchstaben
value: "{{zug1.abw | prefix('+')}}"                 # Präfix anhängen, z.B. "+5"
value: "{{zug1.hinweis | strip('*') | upper}}"      # Kombination: erst strip, dann upper
```

#### Verfügbare Filter

| Filter                      | Beschreibung                                              |
|-----------------------------|-----------------------------------------------------------|
| `strip('x')`                | Entfernt das Zeichen `x` am Anfang des Textes             |
| `stripAll('x')`             | Entfernt alle Vorkommen des Zeichens `x` im Text          |
| `stripBetween('a', 'b')`    | Entfernt alles zwischen Zeichen `a` und `b` (inkl. beider)|
| `upper`                     | Wandelt Text in Großbuchstaben um                         |
| `lower`                     | Wandelt Text in Kleinbuchstaben um                        |
| `prefix('x')`               | Setzt Zeichen/Text `x` vor den Wert                       |
| `suffix('x')`               | Hängt Zeichen/Text `x` an den Wert an                    |
| `trim`                      | Entfernt führende und nachfolgende Leerzeichen            |
| `format('pattern')`         | Datum/Zeit formatieren (nur für `now`-Variablen)          |
| `mul(x)`                    | Multipliziert Zahlenwert mit x                            |
| `div(x)`                    | Dividiert Zahlenwert durch x                              |
| `add(x)`                    | Addiert x zum Zahlenwert                                  |
| `sub(x)`                    | Subtrahiert x vom Zahlenwert                              |
| `mod(x)`                    | Modulo, Vorzeichen des Teilers (`-3 \| mod(1440)` → 1437); `mod(0)` → 0 |
| `minutes`                   | `"H:MM"`/`"HH:MM"` → Minuten seit Mitternacht (`10:15` → 615); sonst `""` |
| `round`                     | Rundet auf ganze Zahl                                     |
| `before('x')`               | Teil vor dem ersten Vorkommen von `x`; ohne `x` im Wert der ganze Wert |
| `after('x')`                | Teil nach dem ersten Vorkommen von `x`; ohne `x` im Wert leer (`""`)  |

**`before` / `after`** teilen einen Wert an einem Trenner, z.B. Stunde und Minute aus `13:30`:
```yaml
value: "{{zug1.zeit | before(':')}}"   # 13:30 → 13
value: "{{zug1.zeit | after(':')}}"    # 13:30 → 30
```
Enthält der Wert den Trenner nicht (z.B. `1330`), liefert `before` den ganzen Wert und `after` einen leeren String.

**Grammatik:** `{{ operand | filter | filter(arg, …) }}`. `|` und `,` trennen nur auf oberster Ebene, also außerhalb von Anführungszeichen und Klammern (`stripAll('|')` ist damit möglich). Leerzeichen um `|` sind optional.

**Argumente der Mathe-Filter** (`add`, `sub`, `mul`, `div`, `mod`):

| Argument | Bedeutung |
|---|---|
| `'6'`, `"6"` oder `6`, `-0.5` | Literal |
| alles andere, z.B. `anim.frame` oder `zug1.zeit \| minutes` | wird als Variable bzw. Pipeline ausgewertet (höchstens 4 Ebenen tief); ist das Ergebnis keine Zahl, ist das Gesamtergebnis `""` |

Ist der Wert links keine Zahl, bleibt er unverändert; ein leerer Wert (z.B. eine leere Zeit) pflanzt sich so durch die ganze Kette fort und ergibt eine leere Ausgabe. Text-Filter (`strip`, `prefix`, …) akzeptieren nur Literale als Argument.

**Mathe-Filter Beispiele** (typisch für analoge Uhren):
```yaml
rotate: "{{now.minute | mul(6)}}"                                    # Minutenzeiger: 0–354°
rotate: "{{now.hour12 | mul(30) | add(now.minute | div(2))}}"        # Stundenzeiger mit Minutenanteil (10:15 → 307.5°)
rotate: "{{now.second | add(anim.frame) | mul(6)}}"                  # Sekundenzeiger, läuft im GIF
value:  "{{now.hour | mul(30) | round}}"                             # Winkel als Zahl ausgeben
# Minuten bis zur Abfahrt relativ zu Zug 1 (+5 wie im Original); über Mitternacht positiv:
value:  "{{zug2.zeit | minutes | sub(zug1.zeit | minutes) | mod(1440) | add(5)}}"   # 10:15/10:18 → 8
```

**`minutes`:** Nach dem Entfernen von Leerraum muss der Wert mit `Ziffern:Ziffern` beginnen (ein- oder zweistellig, Rest egal): `10:15 Uhr` → 615, `10:5` → 605. Keine Bereichsprüfung. Passt das Muster nicht (`""`, `ca. 10`, `1015`), ist das Ergebnis `""`.

---

## Bedingungen (if / elif / else)

### Ganze Layer ein-/ausblenden

Ein Layer wird nur gerendert wenn die Bedingung wahr ist:

```yaml
- type: rect
  if: "startsWith(zug1.hinweis, '*')"
  x: 0
  y: 60
  width: 160
  height: 20
  color: "#FFFF00"
```

### Layer-Kette: if / elif / else

Mehrere Layer können zu einer exklusiven Kette zusammengefasst werden. Sobald eine Bedingung zutrifft, werden alle nachfolgenden `elif`/`else`-Layer übersprungen.

```yaml
# Nur eines der folgenden Icons wird gerendert:
- type: image
  if: "startsWith(zug1.nr, 'ICN')"
  file: icn.png
  x: 48
  y: 11
- type: image
  elif: "startsWith(zug1.nr, 'IC')"
  file: ic.png
  x: 48
  y: 11
- type: image
  elif: "startsWith(zug1.nr, 'EC')"
  file: ec.png
  x: 48
  y: 11
- type: text
  else: true
  value: "{{zug1.nr}}"
  x: 49
  y: 11
  font: regular
  size: 7
  color: "#ffffff"
  align: left
```

- **`elif: "bedingung"`** — setzt die Kette fort; wird nur ausgewertet wenn kein vorheriges `if`/`elif` wahr war
- **`else: true`** — Fallback; wird gerendert wenn kein `if`/`elif` in der Kette zutraf
- Ein `elif`/`else` ohne vorausgehendes `if` ist ein Fehler
- Jeder Layer ohne `if`/`elif`/`else` beendet die aktuelle Kette

### Eigenschaften bedingt setzen

Eine Eigenschaft kann je nach Bedingung unterschiedliche Werte haben:

```yaml
- type: text
  value: "{{zug1.hinweis | strip('*')}}"
  x: 5
  y: 62
  font: regular
  size: 10
  color:
    if: "startsWith(zug1.hinweis, '*')"
    then: "#000000"
    else: "#FFFFFF"
```

### Eigenschaften: if / elif / else

```yaml
  color:
    if: "startsWith(zug1.hinweis, '*')"
    then: "#000000"
    elif: "isEmpty(zug1.hinweis)"
    then: "#888888"
    else: "#FFFFFF"
```

### Block-Level if/elif/else

Mehrere Layer können unter einer gemeinsamen Bedingung gruppiert werden.
Ein Block-Eintrag hat **kein `type:`**, dafür eine eigene `layers:`-Liste.
Er steht wie ein normaler Layer innerhalb der äußeren `layers:`-Liste des Templates.

```yaml
layers:                          # äußere layers-Liste des Templates
  - if: "startsWith(zug1.nr, 'ICN')"
    layers:                      # Sub-Layer dieses Blocks
      - type: image
        file: icn.png
      - type: text
        value: "ICN Express"
        x: 5
        y: 20

  - elif: "startsWith(zug1.nr, 'IC')"
    layers:
      - type: image
        file: ic.png

  - else:                        # auch: else: true — beides ist gleichwertig
    layers:
      - type: text
        value: "{{zug1.nr}}"
```

**Unterschied zum Layer-Level `if`:**

| Merkmal | Layer-Level | Block-Level |
|---|---|---|
| Hat `type:` | ja | nein |
| Steuert | einen einzelnen Layer | beliebig viele Layer |
| Verschachtelbar | nein | ja (beliebig tief) |

**Regeln:**
- `else:` (ohne Wert) und `else: true` sind gleichwertig.
- Block-Nodes können beliebig tief verschachtelt werden.
- `type: loop` darf nicht innerhalb eines anderen `loop` vorkommen, auch nicht via Block-Node.
- `elif`/`else` ohne vorangehendes `if` ist ein Fehler.

---

## Verfügbare Bedingungsfunktionen

Das **erste Argument** darf auch eine Filter-Pipeline sein: `greaterThan(zug1.zeit | minutes, 600)`. Das zweite Argument bleibt ein Literal.

| Funktion                        | Beschreibung                              |
|---------------------------------|-------------------------------------------|
| `startsWith(feld, 'zeichen')`   | Beginnt der Wert mit diesem Zeichen?      |
| `endsWith(feld, 'zeichen')`     | Endet der Wert mit diesem Zeichen?        |
| `contains(feld, 'zeichen')`     | Enthält der Wert dieses Zeichen?          |
| `isEmpty(feld)`                 | Ist der Wert leer?                        |
| `equals(feld, 'wert')`          | Ist der Wert gleich diesem String?        |
| `greaterThan(feld, zahl)`       | Ist der Zahlenwert größer als X?          |

---

## Vollständiges Beispiel

```yaml
meta:
  name: "SBB 0.96\""
  description: "SBB-Design für 0.96 Zoll Display"
  author: "christoph"
  version: "1.0"
  display: "0.96 Zoll OLED"
  instructions: |
    Erwartet zug1 und zug2 mit zeit, vonnach, nr und via.
    Optional: gleis für die Gleisanzeige.
  canvas:
    width: 160
    height: 80

fonts:
  - id: regular
    file: Roboto-Regular.ttf
  - id: bold
    file: Roboto-Bold.ttf

layers:
  # Hintergrund
  - type: image
    file: background.png
    x: 0
    y: 0

  # Gelber Hinweis-Hintergrund — nur wenn Hinweis mit * beginnt
  - type: rect
    if: "startsWith(zug1.hinweis, '*')"
    x: 0
    y: 60
    width: 160
    height: 20
    color: "#FFFF00"

  # Abfahrtszeit
  - type: text
    value: "{{zug1.zeit}}"
    x: 5
    y: 10
    font: bold
    size: 14
    color:
      if: "greaterThan(zug1.abw, 0)"
      then: "#FFFF00"
      else: "#FFFFFF"
    align: left

  # Ziel
  - type: text
    value: "{{zug1.vonnach}}"
    x: 45
    y: 10
    font: bold
    size: 12
    color: "#FFFFFF"
    align: left
    max_width: 110

  # Via-Stationen als Liste (SBB-Stil: Icon + Text pro Station)
  - type: loop
    value: "{{zug1.via}}"
    split_by: "|"
    var: "via_item"
    max_items: 4
    layers:
      - type: image
        file: via-dot.png
        x: 45
        y: "{{i * 12 + 30}}"
      - type: text
        value: "{{via_item}}"
        x: 55
        y: "{{i * 12 + 30}}"
        font: regular
        size: 9
        color: "#AAAAAA"
        align: left
        max_width: 100

  # Hinweistext — Farbe abhängig ob Sternchen
  - type: text
    if: "not(isEmpty(zug1.hinweis))"
    value: "{{zug1.hinweis | strip('*')}}"
    x: 5
    y: 62
    font: regular
    size: 9
    color:
      if: "startsWith(zug1.hinweis, '*')"
      then: "#000000"
      else: "#FFFFFF"
    align: left
    max_width: 150
```

---

## Entschiedene Punkte

| Thema | Entscheidung |
|---|---|
| `elif`-Syntax (Eigenschaften) | Wiederholtes Schlüsselwort (wie oben gezeigt) |
| `elif`/`else` auf Layer-Ebene | `elif: "bedingung"` / `else: true` als Layer-Felder; bilden exklusive Kette |
| Rotation: Koordinaten | `x`/`y` bei gesetztem `rotate` = Mittelpunkt des Bildes auf dem Canvas; kein `pivot_x`/`pivot_y` |
| Rotation: Richtung | Uhrzeigersinn (positiver Winkel = CW) |
| Rotation: Drehachse | Liegt für jeden Winkel exakt auf `x`/`y` |
| Frame-Animation | `meta.animation.frames`/`frame_delay`/`loop`; `anim.*`-Variablen; `now` pro Render eingefroren; nicht mit `splitflap` kombinierbar |
| Zeitzone | `meta.timezone`, Standard `Europe/Berlin` |
| Filter kombinierbar | Ja — `\|`-Verkettung, links nach rechts |
| `strip` auf Textbereiche | Ja — `stripBetween('a', 'b')` löscht alles inkl. Begrenzungszeichen |
| Repeat/Loop | `type: loop` mit `split_by` — kein `step_y`, kein relatives Y; Positionierung via `{{i * step + base}}`-Ausdrücke |
| Koordinaten-Ausdrücke | `x`, `y`, `width`, `height`, `max_width` unterstützen (`size` nicht) `{{...}}`-Arithmetik; `i` als Kurzform für `loop.index` |
| Leere Felder | Werden leer dargestellt — kein Fehler. Sonderbehandlung via `if isEmpty(...)` |
| `line_height` | Float-Multiplikator relativ zur natürlichen Schrifthöhe; `0` und `1` = unverändert |
