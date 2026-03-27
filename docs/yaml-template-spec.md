# YAML Template Specification (Draft v0.1)

> **Status:** Zur Review — noch nicht implementiert

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
  canvas:
    width: 160    # Breite in Pixeln
    height: 80    # Höhe in Pixeln
```

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

---

### type: image

```yaml
- type: image
  file: background.png
  x: 0
  y: 0
  width: 160    # optional — Standard: Originalgröße
  height: 80    # optional — Standard: Originalgröße
  rotate: 0     # optional — Drehwinkel in Grad (kann Variable/Ausdruck sein)
  pivot_x: 80   # optional — Drehmittelpunkt X (Standard: Bildmitte)
  pivot_y: 40   # optional — Drehmittelpunkt Y (Standard: Bildmitte)
```

**Rotation** wird verwendet für analoge Uhren. Der Winkel kann über einen Ausdruck berechnet werden:

```yaml
# Minutenzeiger: 360° / 60 Minuten = 6° pro Minute
- type: image
  file: clock-minutes.png
  x: 0
  y: 0
  rotate: "{{now.minute | mul(6)}}"
  pivot_x: 80
  pivot_y: 80

# Stundenzeiger: 360° / 12 Stunden = 30° pro Stunde
- type: image
  file: clock-hour.png
  x: 0
  y: 0
  rotate: "{{now.hour12 | mul(30)}}"
  pivot_x: 80
  pivot_y: 80
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
  valign: top         # top | middle | bottom — benötigt height
  width: 80           # optional — Boxbreite für Ausrichtung + Zeilenumbruch
  height: 20          # optional — Boxhöhe für vertikale Ausrichtung
  max_width: 80       # optional — nur Zeilenumbruch (alternativ zu width)
```

---

### type: loop

Iteriert über einen gesplitteten String und rendert Sub-Layer für jedes Element. Typisch für Via-Stationen bei SBB (Icon + Text pro Station).

```yaml
- type: loop
  value: "{{zug1.via}}"   # String der gesplittet wird
  split_by: "|"            # Trennzeichen
  var: "item"              # Name der Loop-Variable (in Sub-Layern als {{item}} verfügbar)
  y: 30                    # Start-Y (absolut)
  step_y: 12               # Y-Versatz pro Iteration
  max_items: 6             # Sicherheits-Limit (DoS-Schutz)
  layers:                  # Sub-Layer — y-Positionen relativ zum aktuellen Loop-Y
    - type: image
      file: via-dot.png
      x: 45
      y: 0                 # relativ: tatsächliches Y = loop.y + 0
    - type: text
      value: "{{item}}"
      x: 55
      y: 0
      font: regular
      size: 9
      color: "#888888"
      max_width: 100
```

**Automatische Loop-Variablen** (in Sub-Layern verfügbar):

| Variable       | Beschreibung                              |
|----------------|-------------------------------------------|
| `{{item}}`     | Aktuelles Element (Name via `var`)        |
| `{{loop.index}}`| Aktueller Index, 0-basiert               |
| `{{loop.y}}`   | Absolutes Y des aktuellen Elements        |

**Leere Elemente** nach dem Split werden übersprungen.
**Wenn `value` leer ist**, wird der Loop nicht ausgeführt (kein Fehler).

---

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
| `round`                     | Rundet auf ganze Zahl                                     |

**Mathe-Filter Beispiele** (typisch für analoge Uhren):
```yaml
rotate: "{{now.minute | mul(6)}}"           # Minutenzeiger: 0–354°
rotate: "{{now.hour12 | mul(30)}}"          # Stundenzeiger grob: 0–330°
rotate: "{{now.second | mul(6)}}"           # Sekundenzeiger: 0–354°
value:  "{{now.hour | mul(30) | round}}"    # Winkel als Zahl ausgeben
```

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

### if / elif / else

```yaml
  color:
    if: "startsWith(zug1.hinweis, '*')"
    then: "#000000"
    elif: "isEmpty(zug1.hinweis)"
    then: "#888888"
    else: "#FFFFFF"
```

---

## Verfügbare Bedingungsfunktionen

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
    y: 30
    step_y: 12
    max_items: 4
    layers:
      - type: image
        file: via-dot.png
        x: 45
        y: 2
      - type: text
        value: "{{via_item}}"
        x: 55
        y: 0
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
| `elif`-Syntax | Wiederholtes Schlüsselwort (wie oben gezeigt) |
| Filter kombinierbar | Ja — `\|`-Verkettung, links nach rechts |
| `strip` auf Textbereiche | Ja — `stripBetween('a', 'b')` löscht alles inkl. Begrenzungszeichen |
| Repeat/Loop | `type: loop` mit `split_by` — kein eigenständiger split-Filter (würde Evaluator auf Listen-Rückgabe erweitern) |
| Leere Felder | Werden leer dargestellt — kein Fehler. Sonderbehandlung via `if isEmpty(...)` |
