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
  display: "0.96 Zoll OLED"
  instructions: |
    Dieses Template erwartet zug1 und zug2 mit zeit, vonnach, nr und via.
    Optional: gleis für die Gleisanzeige.
  canvas:
    width: 160    # Bildbreite in Pixeln
    height: 160   # Bildhöhe in Pixeln
    colors: 32    # optional — Farbpalette reduzieren (2–256)
  timezone: "Europe/Berlin"   # optional — Zeitzone für {{now.*}}
  animation:                  # optional — nur für GIF-Ausgabe
    frames: 60                # Frame-Animation: Anzahl Frames (2–125)
    frame_delay: 1            # Sekunden pro Frame (0.02–10)
    loop: true                # endlos wiederholen
```

| Feld | Beschreibung |
|---|---|
| `name` | Anzeigename in der Galerie und Vorschau-Seite |
| `description` | Kurze Beschreibung des Templates |
| `author` | Name des Erstellers |
| `version` | Versionsnummer des Templates |
| `display` | Bezeichnung des Ziel-Displays (z.B. `"0.96 Zoll OLED"`) — erscheint auf der Galerie-Karte und Vorschau-Seite |
| `instructions` | Freitext-Anleitung für Nutzer (mehrzeilig mit YAML `\|`) — erscheint auf der Vorschau-Seite |
| `canvas` | Bildabmessungen (`width` × `height` in Pixeln) — Pflichtfeld |
| `canvas.colors` | Farbpalette reduzieren (2–256) — kleinere PNG-Dateien für Microcontroller (siehe unten) |
| `timezone` | IANA-Zeitzone für `{{now.*}}`, Standard `Europe/Berlin` (siehe [Zeitzone](#zeitzone)) |
| `animation` | GIF-Einstellungen: `frames`, `frame_delay`, `loop` für Frame-Animationen (siehe [Frame-Animation](#frame-animation)), `fps`/`duration` für Faltblatt-Animationen |

Alle Felder außer `canvas` sind optional, aber empfohlen — sie erscheinen in der Galerie und auf der Vorschau-Seite.

### Farbpaletten-Reduktion (`canvas.colors`)

TFT-Displays an Microcontrollern profitieren von kleinen PNG-Dateien. Da generierte Zuganzeigebilder nur wenige Farben enthalten, lässt sich die Dateigröße mit `canvas.colors` deutlich reduzieren — ohne sichtbaren Qualitätsverlust.

```yaml
canvas:
  width: 160
  height: 80
  colors: 32    # Ausgabe auf 32 Farben reduzieren
```

- Der Renderer wählt automatisch die **optimale Palette** für das jeweilige Bild (Median-Cut-Algorithmus)
- Kein Dithering — bei kleinen Texten auf TFT-Displays würde Dithering fransige Schriftkanten erzeugen
- **Richtwert:** 16–32 Farben liefern bei einfachen Grafiken mit Anti-Aliasing gute Ergebnisse
- Weglassen oder `colors: 0` = volle 32-Bit-Farbe (Standard, kein Unterschied zu bisherigem Verhalten)
- Gültige Werte: 2–256 (technisches Limit von Indexed PNG)

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
| `valign` | Vertikale Ausrichtung: `top` (Standard), `middle`, `bottom` — benötigt `height`; `baseline` — `y` ist die Grundlinie der Schrift |
| `scale_x` | optional — horizontale Skalierung, z.B. `0.5` = halbe Breite |
| `scale_y` | optional — vertikale Skalierung, z.B. `1.2`; verankert an `y` (Oberkante bzw. Grundlinie bei `valign: baseline`) |
| `id`, `advance` | optional — `id: zeit` stellt `{{zeit.width}}` und `{{zeit.end}}` (= `x + Breite + advance`) für `x`/`y`/… folgender Layer bereit, z.B. `x: "{{zeit.end}}"`; Details in `yaml-template-spec.md` |
| `fit_items` | optional — Wert an `split_by` teilen und die Elemente einzeilig nebeneinander setzen; was nicht mehr in `max_width` passt, entfällt (z.B. Via-Stationen) |
| `width` | optional — Boxbreite in Pixeln für Ausrichtung und Zeilenumbruch |
| `height` | optional — Boxhöhe in Pixeln für vertikale Ausrichtung |
| `max_width` | optional — maximale Breite für Zeilenumbruch (alternativ zu `width`) |

**Alte PHP-Themes portieren:** Schriftgröße = Legacy-Größe × 16/9, das Legacy-`y` eines Textes ist die Grundlinie (`valign: baseline`), Legacy-`xFactor 0.5` ist `scale_x: 0.5`, Legacy-`yFactor` ist `scale_y`.

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

### type: splitflap — Faltblatt-Klappen

Bildet die Klappen einer mechanischen Faltblattanzeige nach. Ohne Animation zeigt jede Klappe einfach ihren Wert; als **GIF** (`?format=gif`) klappen alle Klappen nacheinander auf den neuen Text. Alle Felder stehen in der [YAML-Spezifikation](yaml-template-spec.md#type-splitflap), das fertige Beispiel ist `templates/faltblatt-105-v1/template.yaml`.

#### Faltblatt-Animation bauen

1. **Hintergrund zuerst.** Zeichne das Hintergrundbild mit den leeren Klappen als ersten Layer. Jede Klappe nimmt davon einen Ausschnitt als Blatt-Hintergrund — alles, was *vor* dem `splitflap`-Layer gezeichnet wird, gehört zum Blatt.
2. **Modul = ganze Klappe.** `x`, `y`, `width`, `height` umfassen das komplette Blatt, wie es im Hintergrund gezeichnet ist, nicht nur die Fläche um die Schrift. Feste Beschriftung auf der Klappe (z.B. „Abfahrt" neben der Stunde) kommt als konstanter Kind-Layer hinein und klappt mit.
3. **`split_y` aufs Scharnier.** Liegt der Scharnierstrich nicht auf halber Höhe, gib die Linie mit `split_y` an (relativ zur Oberkante des Moduls).
4. **Kind-Layer relativ zum Modul.** Text und Bilder in `layers:` haben Koordinaten relativ zur linken oberen Ecke des Moduls. Der aktuelle Wert steht in der Variable aus `var` (`{{ch}}`, `{{hour}}`, …).
5. **Einzelzeichen-Zeile:** `cells: 13`, `pitch` = Abstand von Klappe zu Klappe, `split: chars` (Pflicht bei `cells > 1`) und ein `reel` mit den Zeichen, durch die die Klappen blättern.
6. **Ganze Tafel:** ohne `split` und `cells` blättert die Klappe als Ganzes durch die Einträge des `reel` (z.B. `["rb", "re", "ic", "ice", ""]` für die Zugart).
7. **Overlay nach den Klappen.** Rahmen, Scharniere und Schattierung (z.B. `fg.png`) kommen **nach** allen `splitflap`-Layern. `copy`-Layer danach (obere Hälfte auf die untere spiegeln) folgen der Animation automatisch.
8. **Nicht überlappen, nicht auf Bewegtes legen.** Module dürfen sich nicht überlappen, und eine Klappe darf nicht auf etwas liegen, das sich pro Frame selbst ändert.

```yaml
meta:
  animation:
    fps: 20          # 5–25, Standard 20
    duration: 2.5    # maximale Laufzeit in Sekunden, 0.5–5, Standard 2.5

layers:
  - type: image
    file: bg.png
  - type: splitflap
    x: 54
    y: 82
    width: 11
    height: 30
    split_y: 14
    cells: 13
    pitch: 14
    split: chars
    value: "{{zug1.vonnach | upper}}"
    reel: " ABCDEFGHIJKLMNOPQRSTUVWXYZÄÖÜ-."
    var: ch
    layers:
      - type: text
        value: "{{ch}}"
        x: 5
        y: 23
        valign: baseline
        align: center
        font: bold
        size: 21.33
        color: "#3a3c3b"
  - type: image          # Overlay NACH den Klappen
    file: fg.png
```

**Gut zu wissen**

- Der Zielwert muss nicht im `reel` stehen; er wird am Ende trotzdem angezeigt (freier Via-Text, ungewöhnliche Zeichen). Zeichen außerhalb des `reel` werden einfach so gezeichnet.
- `min_flips`, `max_flips` und `flip_frames` steuern, wie lange und wie schnell eine Klappe rattert. Ohne Angabe: mindestens 4 Schritte, höchstens so viele, wie in `duration` passen.
- **Determinismus:** Gleiches Template + exakt gleicher Request-Body (Bytes, also auch gleiche Leerzeichen und Schlüsselreihenfolge) ergibt immer dasselbe GIF (bytegleich); ein anders formatierter Body kann ein anderes Ablaufmuster ergeben. Die Klappen stoppen aber zu unterschiedlichen Zeiten. Das gilt nicht für Templates, die `{{now.*}}` verwenden — die Uhrzeit wird einmal pro Animation eingefroren, ändert sich aber von Aufruf zu Aufruf.
- Das GIF spielt einmal ab und bleibt auf dem letzten Frame stehen. Dieses Bild ist identisch mit dem normalen PNG der gleichen Daten. Das erste Frame zeigt die Klappen bereits in Bewegung, weil das Display vorher das Standbild des alten Zugs zeigt.
- Ein Template ohne `splitflap`-Layer liefert bei `?format=gif` ein GIF mit nur einem Frame.
- Grenzen: max. 64 Klappen pro Template, max. 64 `reel`-Einträge pro Layer, max. 125 Frames, maximal mögliche Frames (`duration × fps`) × Breite × Höhe ≤ 32.000.000 Pixel (Obergrenze, vorab geprüft).

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
| `before(':')` | Teil vor dem ersten Trenner | `{{zug1.zeit \| before(':')}}` | `13:30` → `13` |
| `after(':')` | Teil nach dem ersten Trenner | `{{zug1.zeit \| after(':')}}` | `13:30` → `30` |

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
| `mod(x)` | Rest der Division (Vorzeichen des Teilers) | `{{x \| mod(1440)}}` | `-3` → `1437` |
| `round` | Runden (ganzzahlig) | `{{x \| round}}` | `3.7` → `4` |
| `minutes` | Uhrzeit `H:MM` in Minuten seit Mitternacht | `{{zug1.zeit \| minutes}}` | `10:15` → `615` |
| `clock` | Minuten als `HH:MM` (umgekehrt zu `minutes`, modulo 1440) | `{{m \| clock}}` | `963` → `16:03`, `1445` → `00:05` |
| `replace('a','b')` | Text ersetzen | `{{v \| replace(' - ', '~')}}` | `A - B` → `A~B` |
| `match('re', n)` | Gruppe `n` des ersten Treffers (RE2, 0 = ganzer Treffer), sonst leer | `{{nr \| match('([A-Z]+)([0-9]+)', 2)}}` | `S1` → `1` |

Division durch 0 ergibt `0`. Nicht-numerische Werte werden unverändert durchgereicht. `minutes` liefert bei einem Wert, der nicht wie `10:15` aussieht (leer, `ca. 10`), eine leere Ausgabe.

**Variablen als Argument:** Das Argument von `add`, `sub`, `mul`, `div` und `mod` darf auch eine Variable oder eine ganze Filterkette sein (ohne Anführungszeichen):

```yaml
rotate: "{{now.second | add(anim.frame) | mul(6)}}"
value:  "{{zug2.zeit | minutes | sub(zug1.zeit | minutes)}}"
```

Zahlen (`6`, `-0.5`) und Texte in Anführungszeichen (`'6'`) bleiben Literale. Ist das Argument eine Variable ohne Zahlenwert, ist die Ausgabe leer.

**Beispiel: Minuten bis zur Abfahrt.** Zug 1 steht bei 10:15, Zug 2 bei 10:18, angezeigt werden soll „Minuten nach Zug 1 plus 5" (wie auf den Münchner Anzeigen: 5 / 8 / 16):

```yaml
- type: text
  value: "{{zug2.zeit | minutes | sub(zug1.zeit | minutes) | mod(1440) | add(5)}}"   # → 8
  x: 238
  y: 62
  font: regular
  size: 12
  align: right
```

`mod(1440)` hält den Wert über Mitternacht positiv (23:58 → 00:04 ergibt 6, plus 5 = 11). Eine leere oder ungültige Zeit ergibt eine leere Zelle.

---

## Zeit-Variablen

Die aktuelle Zeit steht über `{{now.*}}` zur Verfügung (wird einmal pro Render-Aufruf erfasst, in der Zeitzone des Templates, siehe [Zeitzone](#zeitzone)):

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
value: "{{now | format('HH:mm:ss', anim.time)}}"  # Uhr läuft in der Animation mit (zweites Argument: Sekunden-Versatz)
```

### Zeitzone

`{{now.*}}` rechnet standardmäßig in `Europe/Berlin`, unabhängig von der Zeitzone des Servers. Mit `meta.timezone` lässt sich eine andere IANA-Zeitzone wählen:

```yaml
meta:
  timezone: "UTC"        # oder "America/New_York", "Asia/Tokyo", …
```

Erlaubt sind IANA-Namen (höchstens 64 Zeichen, `Local` nicht); ein ungültiger Name ist ein Template-Fehler.

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

Das Zifferblatt ist 200×200px groß, Mittelpunkt bei (100, 100). Die Drehachse liegt exakt auf `x`/`y`, auch bei dünnen Zeigern:

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
    rotate: "{{now.hour12 | mul(30) | add(now.minute | div(2))}}"  # 30° pro Stunde + 0,5° pro Minute

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

Der Stundenzeiger rückt mit den Minuten weiter (10:15 → 307,5°). Der Sekundenzeiger steht im PNG auf der aktuellen Sekunde. Wie er im GIF läuft, zeigt der nächste Abschnitt.

### Frame-Animation

Mit `meta.animation.frames` wird das Template bei `?format=gif` mehrfach gerendert, in jedem Frame mit anderem `anim.frame`. So läuft der Sekundenzeiger:

```yaml
meta:
  name: "Uhr mit Sekundenzeiger"
  canvas:
    width: 200
    height: 200
  timezone: "Europe/Berlin"
  animation:
    frames: 60        # 60 Frames …
    frame_delay: 1    # … je 1 Sekunde …
    loop: true        # … und dann von vorn

layers:
  - type: image
    file: zifferblatt.png
    x: 0
    y: 0
  - type: image
    file: stundenzeiger.png
    x: 100
    y: 100
    rotate: "{{now.hour12 | mul(30) | add(now.minute | div(2))}}"
  - type: image
    file: minutenzeiger.png
    x: 100
    y: 100
    rotate: "{{now.minute | mul(6)}}"
  - type: image
    file: sekundenzeiger.png
    x: 100
    y: 100
    rotate: "{{now.second | add(anim.frame) | mul(6)}}"
```

Regeln:

- `frames`: 2–125, `frame_delay`: 0.02–10 Sekunden (in 0.01-Schritten), `frames × frame_delay` höchstens 120 s. Nicht kombinierbar mit `fps`/`duration` oder einem `splitflap`-Layer.
- `loop: true` wiederholt das GIF endlos, sonst spielt es einmal.
- `now` wird einmal pro Render gelesen: Stunden- und Minutenzeiger stehen im GIF still, nur der Sekundenzeiger läuft. Er beginnt bei der aktuellen Sekunde und steht nach 60 Frames wieder am Anfang.
- `anim.frame` (0 … frames−1) und `anim.frames` sind in Text-/Wert-Ausdrücken und in Koordinaten-Ausdrücken (x/y/width/height) verfügbar; `anim.time` (Sekunden seit Start) nur in Text-/Wert-Ausdrücken, nicht in Koordinaten. Im PNG ist `anim.frame` = 0, das PNG gleicht also Frame 0 des GIFs.
- Templates mit `now` werden vom Server nicht gecacht (`X-Cache: BYPASS`), die Uhr bleibt also nie stehen.

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
können sie in einem **Block-Node** gruppiert werden. Ein Block-Node hat **kein `type:`**,
dafür eine eigene `layers:`-Liste mit den Sub-Layern.

> **Hinweis:** Die `- if:` / `- elif:` / `- else:` Einträge stehen — genau wie normale Layer —
> innerhalb der `layers:`-Liste des Templates.

```yaml
layers:
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

  - else:               # auch: else: true — beides ist gleichwertig
    layers:
      - type: text
        value: "{{zug1.nr}}"
        x: 5
        y: 5
        font: regular
        size: 12
        color: "#ffffff"
```

Block-Nodes können mit normalen Layern gemischt und beliebig tief verschachtelt werden:

```yaml
layers:
  - type: image              # normaler Layer, immer gerendert
    file: background.png
    x: 0
    y: 0

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

---

## KI-Assistent (MCP)

Mit `zza mcp` kann eine KI (Claude Desktop oder Claude Code) Templates für den 1,05"-Zugzielanzeiger direkt in deinem Templates-Ordner bauen und ändern: Sie liest die Spezifikation und den Design-Leitfaden, legt Templates an, schreibt `template.yaml`, importiert Bilder und prüft jedes Ergebnis an einer Vorschau. Der Server läuft lokal als Unterprozess des KI-Clients (stdio), ohne Netzzugriff und ohne Token.

Die KI hat 19 Werkzeuge:

| Gruppe | Werkzeuge |
|---|---|
| Lesen | `get_template_spec`, `get_design_guide`, `list_templates`, `read_template`, `read_file` |
| Schreiben | `create_template`, `write_file`, `upload_image`, `import_image` |
| Vorschau | `render_preview` (PNG oder GIF-Kontaktbogen, mit optionalen Testdaten) |
| Vektorgrafik | `svg_to_png` |
| Foto-Werkzeuge | `perspective_correct`, `crop_image`, `resize_image`, `quantize_colors`, `color_to_alpha`, `sample_colors` |
| Schriften | `list_fonts`, `add_font` |

### Einrichtung in Claude Desktop

1. Konfigurationsdatei öffnen (Claude Desktop → Einstellungen → Entwickler → „Konfiguration bearbeiten“, oder direkt):
   - macOS: `~/Library/Application Support/Claude/claude_desktop_config.json`
   - Windows: `%APPDATA%\Claude\claude_desktop_config.json`
2. Server eintragen. macOS (Pfad zur Binary **in** der `.app`):

   ```json
   {
     "mcpServers": {
       "zza": {
         "command": "/Pfad/zu/zza.app/Contents/MacOS/zza",
         "args": ["mcp"]
       }
     }
   }
   ```

   Windows (Backslashes im JSON doppelt):

   ```json
   {
     "mcpServers": {
       "zza": {
         "command": "C:\\Pfad\\zu\\zza.exe",
         "args": ["mcp"]
       }
     }
   }
   ```

   Ohne weitere Angabe nutzt `zza mcp` wie die GUI den Ordner `templates/` neben der App. Einen anderen Ordner gibst du mit `"args": ["mcp", "--templates-dir", "/Pfad/zu/templates"]` an. Sollen GUI und KI dieselben Templates bearbeiten, müssen beide denselben Ordner verwenden.
3. Claude Desktop vollständig beenden und neu starten. Im Eingabefeld erscheint das Werkzeug-Symbol; dort muss der Server „zza“ mit seinen Werkzeugen stehen. Bei Problemen hilft das Log (macOS `~/Library/Logs/Claude/mcp-server-zza.log`, Windows `%APPDATA%\Claude\logs\mcp-server-zza.log`).
4. macOS: Eine frisch heruntergeladene App steht unter Quarantäne und startet dann nicht als Hintergrundprozess. Einmalig im Terminal: `xattr -dr com.apple.quarantine /Pfad/zu/zza.app`.

**Hinweis Windows:** Der Betrieb des Windows-GUI-Programms `zza.exe` als stdio-Server ist noch nicht praktisch geprüft (Prüfschritt 1.2 im manuellen Testplan). Falls Claude Desktop den Server dort nicht verbinden kann, bitte das Log sichern und melden.

### Einrichtung in Claude Code

- Mit gebauter Binary, von beliebigem Ort:

  ```bash
  claude mcp add zza -- /Pfad/zu/zza mcp --templates-dir /Pfad/zu/templates
  claude mcp list     # zza ... Connected
  ```
- Im Repo gibt es die Projektdatei `.mcp.json`, die den Server per `go run ./cmd/zza mcp --templates-dir ./templates` startet. Beim ersten Start fragt Claude Code, ob du dem Projekt-Server vertraust; mit `/mcp` siehst du den Status. Das erste Kompilieren kann länger als das Standard-Zeitlimit dauern: vorher einmal `go build ./cmd/zza` ausführen oder `MCP_TIMEOUT=60000 claude` starten.
- Änderungen der KI landen im Ordner `./templates` des Repos, also in der Git-Arbeitskopie, und sind dort mit `git diff` sichtbar.

### So arbeitest du mit der KI

Beschreibe in normalen Worten, was du willst. Die KI liest zuerst Spezifikation und Leitfaden, schaut sich ähnliche Templates an und zeigt nach jeder Änderung die Vorschau. Beispiel-Prompts:

- „Bau mir ein Template im Stil der Wiener Linien: rotes Band oben, Gleis außen.“
- „Hier ist ein Foto `/Users/ich/Fotos/anzeiger.jpg` — bau daraus ein Template.“
- „Mach die Zielzeile größer und teste mit einem langen Zielnamen.“
- „Lege die Schrift Roboto Bold in `test-105-v1` an und schreib ‚Gleis 6‘ damit ins weiße Feld.“

Template-Namen müssen auf `-105-v1` enden; fehlt das, ergänzt oder erfragt die KI es.

Wichtig: Fotos gibst du **als lokalen Dateipfad** an, nicht als Chat-Anhang. Die KI kann die Bytes eines Anhangs nicht zuverlässig weitergeben; sie importiert die Datei über `import_image` direkt von der Platte. Den Pfad kopierst du im Finder mit Alt + Rechtsklick „… als Pfadname kopieren“, in Windows mit Umschalt + Rechtsklick „Als Pfad kopieren“.

### Schriften

Die KI kann Schriften aus zwei Quellen verwenden (`list_fonts`, `add_font`):

- aus bereits vorhandenen Templates (die Schriftdatei wird in das Ziel-Template kopiert und in `fonts:` eingetragen),
- aus dem Ordner `fonts/` **neben** dem Templates-Ordner (also `templates/` und `fonts/` im selben Verzeichnis). Dort legst du eigene `.ttf`- und `.otf`-Dateien ab; sie erscheinen als `@fonts/Name.ttf`.

Die KI lädt keine Schriften aus dem Netz. Achte selbst auf die Lizenz der Schriften, die du in `fonts/` ablegst oder weitergibst.

### Foto als Vorlage

1. Die KI importiert das Foto mit `import_image` (Pfad statt Anhang). Große Bilder werden auf höchstens 2048 Pixel Kantenlänge verkleinert; JPEGs werden dabei immer neu kodiert, wodurch EXIF-Daten (z. B. der Aufnahmeort) entfernt werden. Die EXIF-Drehung wird nicht ausgewertet; ein gedrehtes Foto richtet die KI mit `perspective_correct` gerade.
2. Mit `perspective_correct` entzerrt sie ein schräg aufgenommenes Foto, mit `crop_image` und `resize_image` schneidet sie Details aus, mit `quantize_colors`, `sample_colors` und `color_to_alpha` ermittelt und bereinigt sie Farben.
3. Danach baut sie das Template aus Rechtecken, Text-Layern und Bildern nach und gleicht es in mehreren Runden mit der Vorschau ab.
4. Zwischendateien (importiertes Foto, entzerrte Fassungen) bleiben im Template-Ordner liegen. Die KI nennt sie am Ende; **lösche sie danach im Editor** (die KI löscht nichts), damit das Template klein bleibt.

### Gleichzeitig mit dem Editor arbeiten

GUI und KI können denselben Ordner gleichzeitig nutzen:

- Ist ein Template im Editor geöffnet, aktualisiert sich der Editor, wenn die KI Dateien ändert (Live-Aktualisierung per Long-Poll, nicht per SSE). Ungeänderte Dateien werden still neu geladen, neue Dateien erscheinen in der Dateiliste.
- Hast du die Datei im Editor bereits geändert und die KI ändert sie extern, erscheint eine Hinweisleiste „Datei wurde extern geändert“ mit Neu laden / Ignorieren. Deine ungespeicherte Änderung bleibt erhalten.
- Beim Speichern einer extern geänderten Datei fragt ein Dialog: **Neu laden** (Stand der KI übernehmen, eigene Änderung verwerfen), **Überschreiben** (eigenen Stand speichern) oder **Abbrechen**.
- Auch die KI bemerkt Konflikte: Wurde eine Datei seit dem Lesen geändert, lehnt `write_file` das Schreiben ab (`conflict: …`); die KI liest dann neu und wendet ihre Änderung auf den aktuellen Stand an.

### Grenzen und Sicherheit

- **Nur 1,05"-Anzeiger:** Canvas 240×240 (zwei Hälften zu 240×120). `write_file` lehnt eine `template.yaml` mit anderem Canvas ab. Von der KI angelegte Templates heißen `…-105-v1`. Die GUI erzwingt das nicht.
- **Nichts wird gelöscht:** Kein Werkzeug löscht Dateien oder Templates. Überschreiben ist möglich (bei `template.yaml` und `default.json` mit Versionsprüfung).
- **Prüfungen beim Schreiben:** `template.yaml` muss sich laden und probeweise rendern lassen, sonst wird nichts geschrieben. Zusätzlich liefert `write_file` Warnungen, die nicht blockieren: leere untere Hälfte, Text kleiner als 10 px, fest eingetragene Uhrzeiten und unbekannte YAML-Schlüssel (meist Tippfehler).
- **`svg_to_png`:** SVG ohne Text (Schrift nur über Text-Layer), mit Rechenbudget (Größe, Pfadzahl) und einem Zeitlimit von 10 Sekunden je Aufruf.
- **Größenlimits:** u. a. 256 KiB je Textdatei, 64 Dateien und 30 MiB je Template, Importbilder bis 20 MiB bzw. 8192 Pixel je Seite.
- **Erlaubnisdialoge:** Claude Desktop und Claude Code fragen vor jedem Werkzeugaufruf um Erlaubnis; Pfade und Parameter stehen im Aufruf. Prüfe bei `import_image`, dass der Pfad zu dem Foto passt, das du genannt hast.
- **`import_image` liest nur Bilder:** nur reguläre PNG/JPEG-Dateien (keine Symlinks), die sich als Bild dekodieren lassen. Andere Dateien scheitern, ohne dass Inhalt in Fehlertext oder Chat erscheint.
- Die Verbindung ist rein lokal; `zza mcp` öffnet keinen Netzwerkport.
