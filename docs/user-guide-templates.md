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
| `align` | Textausrichtung: `left` (Standard), `center`, `right` |
| `max_width` | optional — maximale Breite in Pixeln, danach Zeilenumbruch |

**Ausrichtung:**
- `left` — `x` ist die **linke** Kante des Textes
- `center` — `x` ist der **Mittelpunkt** des Textes
- `right` — `x` ist die **rechte** Kante des Textes

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

## Noch nicht verfügbar (kommt bald)

Die folgenden Features sind geplant und werden in einer der nächsten Versionen verfügbar:

- **Bedingungen** — Layer nur anzeigen wenn eine Bedingung erfüllt ist (z.B. nur Hinweis anzeigen wenn nicht leer)
- **Filter** — Text transformieren, z.B. `| strip('*')` um ein führendes Sternchen zu entfernen
- **Zeit und Datum** — aktuelle Uhrzeit/Datum einbinden mit `{{now.hour}}`, `{{now | format('HH:mm')}}`
- **Mathe-Filter** — für analoge Uhren: `{{now.minute | mul(6)}}` berechnet den Winkel des Minutenzeigers
- **Bild-Rotation** — Zeigerbild um berechneten Winkel drehen für analoge Uhren
