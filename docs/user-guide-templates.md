# Template-Anleitung

Mit Templates kannst du eigene Anzeigedesigns für deinen Zugzielanzeiger erstellen — ohne Programmierkenntnisse. Ein Template besteht aus einer YAML-Datei und den zugehörigen Bild- und Schriftartdateien.

---

## Grundprinzip

Ein Template beschreibt, **was** auf dem Bild gezeichnet wird und **wo**. Die Elemente (Layer) werden von oben nach unten in der Liste gezeichnet — spätere Layer liegen also über früheren (wie Ebenen in Photoshop).

Die Zugdaten (Abfahrtszeit, Ziel, Zugnummer usw.) kommen von deinem Zugzielanzeiger als JSON — im Template referenzierst du sie mit `{{zug1.zeit}}` o.ä.

---

## Dateistruktur

Alle Dateien eines Templates liegen **flach** in einem Verzeichnis — keine Unterordner:

```
templates/
└── mein-design/
    ├── template.yaml       ← Pflicht: die Template-Datei
    ├── hintergrund.png     ← optionales Hintergrundbild
    ├── logo.png            ← weitere Bilder
    └── MeineSchrift.ttf    ← Schriftarten (.ttf oder .otf)
```

Erlaubte Dateitypen:
- Bilder: `.png`, `.jpg`
- Schriftarten: `.ttf`, `.otf`

Dateinamen dürfen nur **Kleinbuchstaben, Zahlen und Bindestriche** enthalten, max. 64 Zeichen. Sonderzeichen werden automatisch bereinigt.

---

## Aufbau der template.yaml

```yaml
meta:        # Metadaten
fonts:       # Schriftarten-Definitionen
layers:      # Liste der Zeichenobjekte
```

---

## meta — Metadaten

```yaml
meta:
  name: "Mein Design"
  description: "Kurze Beschreibung für die Galerie"
  author: "dein-name"
  version: "1.0"
  canvas:
    width: 160    # Bildbreite in Pixeln
    height: 160   # Bildhöhe in Pixeln
```

Alle Felder außer `canvas` sind optional, aber empfohlen — sie erscheinen in der Template-Galerie.

---

## fonts — Schriftarten

Schriftarten werden einmal definiert und dann über ihre `id` in Text-Layern referenziert.

```yaml
fonts:
  - id: regular
    file: NimbusSanL-Reg.otf

  - id: bold
    file: NimbusSanL-Bol.otf

  - id: kursiv
    file: NimbusSanL-RegIta.otf
```

- `id` — frei wählbarer Name, wird in Text-Layern mit `font: regular` referenziert
- `file` — Dateiname der Schriftart (liegt im selben Verzeichnis wie template.yaml)

---

## layers — Zeichenobjekte

### Koordinaten

- `x` / `y` — Position in Pixeln, von der **oberen linken Ecke** des Bildes aus
- `x: 0, y: 0` ist die obere linke Ecke
- `x: 160, y: 0` wäre die obere rechte Ecke eines 160px breiten Bildes

---

### type: rect — Gefülltes Rechteck

Zeichnet ein einfarbiges Rechteck.

```yaml
- type: rect
  x: 0
  y: 60
  width: 160
  height: 20
  color: "#FF0000"
```

| Feld | Beschreibung |
|---|---|
| `x`, `y` | Position der oberen linken Ecke |
| `width` | Breite in Pixeln |
| `height` | Höhe in Pixeln |
| `color` | Farbe als Hex-Wert: `#RRGGBB` oder `#RRGGBBAA` (mit Transparenz) |

**Farb-Beispiele:**
```
"#FFFFFF"    Weiß
"#000000"    Schwarz
"#FF0000"    Rot
"#FFFF00"    Gelb
"#132a9b"    SBB-Blau
"#FF000088"  Rot mit 50% Transparenz (letzten zwei Stellen = Alpha)
```

---

### type: image — Bild einfügen

Fügt eine Bilddatei in das Template ein.

```yaml
- type: image
  file: hintergrund.png
  x: 0
  y: 0
```

```yaml
- type: image
  file: logo.png
  x: 120
  y: 5
  width: 35    # optional: auf diese Breite skalieren
  height: 15   # optional: auf diese Höhe skalieren
```

| Feld | Beschreibung |
|---|---|
| `file` | Dateiname des Bildes (`.png` oder `.jpg`) |
| `x`, `y` | Position der oberen linken Ecke |
| `width` | optional — Zielbreite in Pixeln (Seitenverhältnis bleibt erhalten wenn nur eines angegeben) |
| `height` | optional — Zielhöhe in Pixeln |

Wenn weder `width` noch `height` angegeben sind, wird das Bild in Originalgröße eingefügt.

---

### type: text — Text ausgeben

Gibt Text auf dem Bild aus. Der Text kann feste Werte oder Zugdaten enthalten (siehe [Variablen](#variablen)).

```yaml
- type: text
  value: "Abfahrt"
  x: 5
  y: 10
  font: bold
  size: 14
  color: "#FFFFFF"
```

```yaml
- type: text
  value: "{{zug1.zeit}}"
  x: 5
  y: 10
  font: bold
  size: 16
  color: "#FFFFFF"
  align: left
  max_width: 80
```

| Feld | Beschreibung |
|---|---|
| `value` | Der anzuzeigende Text (kann Variablen enthalten) |
| `x`, `y` | Position der oberen linken Textkante |
| `font` | Schriftart-ID aus der `fonts`-Liste |
| `size` | Schriftgröße in Punkten — Kommazahlen erlaubt, z.B. `8.2` |
| `color` | Textfarbe als Hex-Wert |
| `align` | Horizontale Ausrichtung: `left` (Standard), `center`, `right` |
| `valign` | Vertikale Ausrichtung: `top` (Standard), `middle`, `bottom` — benötigt `height` |
| `width` | optional — Boxbreite in Pixeln für Ausrichtung und Zeilenumbruch |
| `height` | optional — Boxhöhe in Pixeln für vertikale Ausrichtung |
| `max_width` | optional — maximale Breite für Zeilenumbruch (alternativ zu `width`) |

**Horizontale Ausrichtung:**

Ohne `width` ist `x` der Ankerpunkt:
- `left` — `x` ist die linke Kante des Textes
- `center` — `x` ist der Mittelpunkt des Textes
- `right` — `x` ist die rechte Kante des Textes

Mit `width` wird eine Box aufgespannt und der Text darin ausgerichtet:
```yaml
- type: text
  value: "{{zug1.vonnach}}"
  x: 80        # Box beginnt bei x=80
  y: 10
  width: 75    # Box ist 75px breit
  align: right # Text rechtsbündig innerhalb der Box
  font: bold
  size: 14
  color: "#FFFFFF"
```

**Vertikale Ausrichtung** (benötigt `height`):
```yaml
- type: text
  value: "Gleis"
  x: 130
  y: 0
  width: 30
  height: 80   # Box ist 80px hoch
  align: center
  valign: middle  # Text vertikal zentriert in der Box
  font: bold
  size: 12
  color: "#FFFFFF"
```

---

### type: copy — Bereich kopieren

Kopiert einen Bereich des bereits gezeichneten Bildes an eine andere Stelle. Typisch für Zugzielanzeiger, bei denen die obere Hälfte auf die untere gespiegelt wird.

```yaml
- type: copy
  src_x: 0          # Quellbereich: linke Kante
  src_y: 0          # Quellbereich: obere Kante
  src_width: 160     # Quellbereich: Breite
  src_height: 80     # Quellbereich: Höhe
  x: 0              # Zielposition: links
  y: 81             # Zielposition: oben
```

**Wichtig:** Der `copy`-Layer muss **nach** allen Layern stehen, die er kopieren soll — er kopiert den Stand des Bildes zum Zeitpunkt seiner Ausführung.

---

## Variablen

Zugdaten werden mit doppelten geschweiften Klammern eingebettet: `{{feldname}}`

### Zugdaten (vom Zugzielanzeiger geliefert)

```
{{zug1.zeit}}        Abfahrtszeit          z.B. "15:53"
{{zug1.vonnach}}     Ziel / Herkunft       z.B. "Bern"
{{zug1.nr}}          Zugnummer/-typ        z.B. "S1", "IC 123"
{{zug1.via}}         Via-Stationen         z.B. "Zürich HB - Winterthur"
{{zug1.abw}}         Verspätung in Minuten z.B. 5
{{zug1.hinweis}}     Hinweistext           z.B. "Abweichende Wagenreihung"
{{gleis}}            Gleisnummer           z.B. "3"
```

Für den zweiten und dritten Zug: `{{zug2.zeit}}`, `{{zug3.vonnach}}` usw.

### Leere Felder

Wenn ein Feld leer ist (z.B. kein Hinweis vorhanden), wird es einfach als leerer Text dargestellt — kein Fehler, keine Fehlermeldung.

---

## Vollständiges Beispiel

Ein einfaches zweizeiliges Display (oben Zug 1, unten gespiegelt):

```yaml
meta:
  name: "Mein Abfahrtsplan"
  description: "Einfaches zweizeiliges Display"
  author: "max"
  version: "1.0"
  canvas:
    width: 160
    height: 160

fonts:
  - id: fett
    file: MeineSchrift-Bold.ttf
  - id: normal
    file: MeineSchrift-Regular.ttf

layers:
  # Hintergrund dunkelblau
  - type: rect
    x: 0
    y: 0
    width: 160
    height: 80
    color: "#132a9b"

  # Abfahrtszeit
  - type: text
    value: "{{zug1.zeit}}"
    x: 3
    y: 5
    font: fett
    size: 16
    color: "#FFFFFF"

  # Ziel
  - type: text
    value: "{{zug1.vonnach}}"
    x: 3
    y: 50
    font: normal
    size: 14
    color: "#FFFFFF"
    max_width: 130

  # Gleis
  - type: text
    value: "Gl. {{gleis}}"
    x: 140
    y: 50
    font: normal
    size: 10
    color: "#FFFFFF"
    align: right

  # Obere Hälfte auf untere spiegeln
  - type: copy
    src_x: 0
    src_y: 0
    src_width: 160
    src_height: 80
    x: 0
    y: 81
```

---

## Variablen und Ausdrücke

Template-Werte wie `value`, `color`, `rotate` können Platzhalter enthalten: `{{ausdruck}}`.

### Zugdaten

Zugdaten kommen als JSON vom Zugzielanzeiger. Verschachtelte Felder werden mit Punkt adressiert:

```yaml
value: "{{zug1.zeit}}"       # z.B. "15:54"
value: "{{zug1.vonnach}}"    # z.B. "Zürich HB"
value: "Gl. {{gleis}}"       # Textmix mit statischem Anteil
```

Fehlende Felder ergeben einen leeren String (kein Fehler).

---

## Filter-Pipeline

Variablen können durch Filter verarbeitet werden: `{{variable | filter1 | filter2(arg)}}`.
Mehrere Filter können hintereinandergeschaltet werden.

### Text-Filter

| Filter | Beschreibung | Beispiel | Ergebnis |
|---|---|---|---|
| `upper` | Grossbuchstaben | `{{zug.nr \| upper}}` | `IC23` → `IC23` |
| `lower` | Kleinbuchstaben | `{{zug.nr \| lower}}` | `IC23` → `ic23` |
| `trim` | Leerzeichen entfernen | `{{x \| trim}}` | `" hallo "` → `"hallo"` |
| `strip('x')` | Präfix entfernen | `{{hinweis \| strip('*')}}` | `*Abw.` → `Abw.` |
| `stripAll('x')` | Alle Vorkommen entfernen | `{{x \| stripAll('-')}}` | `a-b-c` → `abc` |
| `stripBetween('{','}')` | Bereich entfernen | `{{x \| stripBetween('{','}')}}` | `Halt {x} ok` → `Halt  ok` |
| `prefix('text')` | Präfix hinzufügen | `{{v \| prefix('+')}}` | `5` → `+5` |
| `suffix(' min')` | Suffix hinzufügen | `{{v \| suffix(' min')}}` | `5` → `5 min` |

**Beispiel mit Verkettung:**
```yaml
value: "{{zug1.hinweis | strip('*') | upper}}"
# "*Abweichende Wagenreihung" → "ABWEICHENDE WAGENREIHUNG"
```

### Mathe-Filter

| Filter | Beschreibung | Beispiel | Ergebnis |
|---|---|---|---|
| `mul(x)` | Multiplizieren | `{{now.minute \| mul(6)}}` | `30` → `180` |
| `div(x)` | Dividieren | `{{x \| div(4)}}` | `10` → `2.5` |
| `add(x)` | Addieren | `{{x \| add(5)}}` | `10` → `15` |
| `sub(x)` | Subtrahieren | `{{x \| sub(3)}}` | `10` → `7` |
| `round` | Runden (ganzzahlig) | `{{x \| round}}` | `3.7` → `4` |

Division durch 0 ergibt `0`. Nicht-numerische Werte werden unverändert durchgereicht.

---

## Zeit-Variablen

Die aktuelle Zeit steht über `{{now.*}}` zur Verfügung (wird einmal pro Render-Aufruf erfasst):

| Variable | Beschreibung | Beispiel |
|---|---|---|
| `{{now}}` | Aktuelle Zeit HH:MM | `15:54` |
| `{{now.hour}}` | Stunde (0–23) | `15` |
| `{{now.hour12}}` | Stunde (1–12) | `3` |
| `{{now.minute}}` | Minute (0–59) | `54` |
| `{{now.second}}` | Sekunde (0–59) | `7` |
| `{{now.day}}` | Tag (1–31) | `24` |
| `{{now.month}}` | Monat (1–12) | `3` |
| `{{now.year}}` | Jahr | `2026` |
| `{{now.weekday}}` | Wochentag (Deutsch) | `Dienstag` |

### Format-Filter

Mit `format(muster)` kann die Zeit frei formatiert werden:

| Token | Bedeutung | Beispiel |
|---|---|---|
| `HH` | Stunde 00–23 (2-stellig) | `15` |
| `hh` | Stunde 01–12 (2-stellig) | `03` |
| `mm` | Minute (2-stellig) | `54` |
| `ss` | Sekunde (2-stellig) | `07` |
| `dd` | Tag (2-stellig) | `24` |
| `MM` | Monat (2-stellig) | `03` |
| `yyyy` | Jahr (4-stellig) | `2026` |
| `EE` | Wochentag kurz | `Di` |
| `EEEE` | Wochentag lang | `Dienstag` |

```yaml
value: "{{now | format('HH:mm:ss')}}"    # → "15:54:07"
value: "{{now | format('dd.MM.yyyy')}}"  # → "24.03.2026"
value: "{{now | format('EEEE')}}"        # → "Dienstag"
```

---

## Bedingungen

### Layer-Bedingung (`if`)

Ein Layer wird nur gezeichnet wenn die Bedingung erfüllt ist. Mit `elif` und `else: true` können mehrere Layer zu einer exklusiven Kette verbunden werden (siehe [Layer-Ketten](#layer-ketten-elif--else)).

```yaml
layers:
  # Hinweis-Layer nur anzeigen wenn nicht leer
  - type: text
    if: "not(isEmpty(zug1.hinweis))"
    value: "{{zug1.hinweis | strip('*')}}"
    x: 5
    y: 60
    font: normal
    size: 10
    color: "#FFCC00"

  # Verspätungs-Rechteck nur anzeigen wenn Verspätung > 0
  - type: rect
    if: "greaterThan(zug1.abw, 0)"
    x: 120
    y: 5
    width: 40
    height: 16
    color: "#FF0000"
```

### Bedingte Eigenschaftswerte (`if/then/else`)

Eigenschaften wie `color` und `value` können bedingte Werte haben:

```yaml
- type: text
  value: "{{zug1.nr}}"
  color:
    if: "greaterThan(zug1.abw, 0)"
    then: "#FF4444"   # rot bei Verspätung
    else: "#FFFFFF"   # weiss sonst
```

Fehlt `else`, ist der Fallback ein leerer String.

### Verfügbare Bedingungsfunktionen

| Funktion | Beschreibung |
|---|---|
| `isEmpty(feld)` | Wahr wenn das Feld fehlt oder leer ist |
| `not(bedingung)` | Negation; verschachtelbar: `not(not(...))` |
| `startsWith(feld, 'text')` | Wahr wenn Feldwert mit `text` beginnt |
| `endsWith(feld, 'text')` | Wahr wenn Feldwert mit `text` endet |
| `contains(feld, 'text')` | Wahr wenn Feldwert `text` enthält |
| `equals(feld, 'wert')` | Wahr wenn Feldwert gleich `wert` ist |
| `greaterThan(feld, zahl)` | Wahr wenn Feldwert (numerisch) grösser als `zahl` |

---

## Bild-Rotation

Bilder (Typ `image`) können gedreht werden — nützlich für analoge Uhren oder drehbare Zeiger.

```yaml
- type: image
  file: minutenzeiger.png
  x: 120   # Mittelpunkt X auf dem Canvas
  y: 120   # Mittelpunkt Y auf dem Canvas
  rotate: "{{now.minute | mul(6)}}"   # 0–59 min × 6° = 0–354°
```

**`rotate`** — Drehwinkel in Grad im **Uhrzeigersinn**. Unterstützt Ausdrücke und Filter.

> **Wichtig:** Wenn `rotate` gesetzt ist, sind `x` und `y` der **Mittelpunkt** des Bildes auf dem Canvas (nicht die obere linke Ecke). Das Bild wird um seine eigene Mitte gedreht und so platziert, dass sein Mittelpunkt auf `(x, y)` liegt.

### Beispiel: Analoge Uhr

Das Zifferblatt ist 200×200px groß, Mittelpunkt bei (100, 100):

```yaml
layers:
  - type: image
    file: zifferblatt.png
    x: 0
    y: 0

  - type: image
    file: stundenzeiger.png
    x: 100   # Mittelpunkt des Zifferblatts
    y: 100
    rotate: "{{now.hour12 | mul(30)}}"  # 12 Stunden × 30° = 360°

  - type: image
    file: minutenzeiger.png
    x: 100
    y: 100
    rotate: "{{now.minute | mul(6)}}"   # 60 Minuten × 6° = 360°

  - type: image
    file: sekundenzeiger.png
    x: 100
    y: 100
    rotate: "{{now.second | mul(6)}}"
```

---

## Layer-Ketten: elif / else

Mehrere Layer können zu einer exklusiven Kette verbunden werden — sobald eine Bedingung zutrifft, werden alle nachfolgenden übersprungen. Das ist nützlich wenn mehrere Icons für denselben Platz in Frage kommen:

```yaml
# Genau eines dieser Icons wird gerendert (spezifischstes zuerst):
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
  size: 9
  color: "#ffffff"
  align: left
```

- **`elif: "bedingung"`** — wird nur geprüft wenn kein vorheriges `if`/`elif` in der Kette wahr war
- **`else: true`** — Fallback; wird gerendert wenn kein `if`/`elif` zutraf
- Die Kette endet automatisch beim nächsten Layer ohne `if`/`elif`/`else`

### Block-Bedingungen

Wenn mehrere Layer nur bei einer bestimmten Bedingung gerendert werden sollen,
können sie in einem Block-Node gruppiert werden:

```yaml
# Zeige unterschiedliche Bilder und Texte je nach Zugnummer
- if: "startsWith(zug1.nr, 'ICN')"
  layers:
    - type: image
      file: icn-logo.png
      x: 5
      y: 5
    - type: text
      value: "Neigezug"
      x: 5
      y: 30
      font: regular
      size: 10
      color: "#ffffff"

- elif: "startsWith(zug1.nr, 'IC')"
  layers:
    - type: image
      file: ic-logo.png
      x: 5
      y: 5

- else:
  layers:
    - type: text
      value: "{{zug1.nr}}"
      x: 5
      y: 5
      font: regular
      size: 12
      color: "#ffffff"
```

Block-Nodes können beliebig tief verschachtelt werden:

```yaml
- if: "not(isEmpty(zug1.hinweis))"
  layers:
    - type: rect
      x: 0
      y: 50
      width: 160
      height: 20
      color:
        if: "startsWith(zug1.hinweis, '*')"
        then: "#ff0000"
        else: "#ffcc00"
    - type: text
      value: "{{zug1.hinweis | strip('*')}}"
      x: 3
      y: 52
      font: regular
      size: 9
      color: "#000000"
```
