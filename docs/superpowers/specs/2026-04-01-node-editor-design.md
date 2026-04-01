# Node-Editor — Design-Spec

Datum: 2026-04-01
Status: Approved

---

## Ziel

Ein nodebasierter Editor für YAML-Templates, eingebettet in den bestehenden Editor als Tab-Umschalter. Zielgruppe: Power-User. Inspiration: n8n, Unreal Blueprints.

---

## Integration ins bestehende UI

Der Node-Editor ersetzt **nicht** den YAML-Editor — er ist ein zweiter Tab in der mittleren Spalte des bestehenden 3-Spalten-Layouts:

```
[Dateien] | [YAML / NODES ← Tab-Umschalter] | [Test-JSON + Vorschau]
```

- Die rechte Spalte (Test-JSON + Vorschau) bleibt unverändert.
- Die linke Spalte (Dateiliste + Upload) bleibt unverändert.
- Der Tab-Umschalter erscheint in der Toolbar der mittleren Spalte.

### YAML ↔ Nodes Sync

| Richtung | Verhalten |
|---|---|
| Nodes → YAML | Immer — beim Speichern wird YAML aus dem Graph generiert |
| YAML → Nodes | Best-effort — schlägt fehl wenn YAML Features enthält die der Node-Editor nicht abdeckt |

Wenn YAML → Nodes fehlschlägt, ist der Node-Tab **gesperrt** mit einem Hinweis:
`"Diese YAML enthält Features die im Node-Editor nicht darstellbar sind."`

---

## Library

**Rete.js v2** via `esm.sh` — kein Build-Step, passt zum bestehenden Vanilla-JS + CodeMirror Ansatz.

Begründung: Nodes werden als HTML/DOM gerendert → Inline-Inputs (Farbpicker, Dropdowns, Textfelder) direkt im Node ohne Custom-Canvas-Drawing.

---

## Graph-Topologie

### Grundprinzip

Die Layer-Reihenfolge der YAML wird als **lineare Kette** abgebildet:

```
[image: background] → [text: zeit] → [loop: via] → ...
```

Jeder Node hat einen Eingang (oben) und einen Ausgang (unten). Die Verbindungen bestimmen die Render-Reihenfolge (= YAML-Reihenfolge). Es gibt **keinen merge-Node** — die Struktur spiegelt die YAML direkt wider.

### if / elif / else (Layer-Ebene)

`if`/`elif`/`else` sind **Badges auf Layer-Nodes**, keine separaten Node-Typen. Die Kette bildet sich durch die Sequenz der Nodes mit Badges — genau wie in der YAML:

```
[image: background]
→ [image: icn.png  〔IF: startsWith(nr,'ICN')〕]
→ [image: ic.png   〔ELIF: startsWith(nr,'IC')〕]
→ [text: {{nr}}    〔ELSE〕]
→ [text: {{zeit}}]          ← kein Badge → Kette endet hier
```

- Der **IF-Badge** erscheint oben rechts am Node mit einem Textfeld für die Bedingung
- **ELIF/ELSE-Badges** folgen in der normalen Sequenz
- Die Kette endet beim ersten Node ohne Badge
- Kein merge-Node nötig

### Block-Bedingung (mehrere Layer pro Ast)

Wenn mehrere Layer unter einer Bedingung gruppiert werden sollen, wird ein **Block-Container-Node** verwendet — gestrichelter oranger Rahmen, mit eigener Sub-Kette:

```
[BLOCK: startsWith(nr,'ICN') ┄┄┄┄┄┄┄┄┄┄┄]
│  [image: icn.png] → [text: ICN Express] │
[┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄]
[BLOCK ELIF: startsWith(nr,'IC') ┄┄┄┄┄┄┄┄]
│  [image: ic.png]                        │
[┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄]
[BLOCK ELSE ┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄]
│  [text: {{nr}}]                         │
[┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄]
→ [text: {{zeit}}]
```

### loop

Ein `loop`-Node öffnet eine eingerückte Sub-Kette für die Loop-Body-Layer:

```
[loop: {{zug1.via}} split "|" var:item max:6]
  └─ [image: via-dot  y: i*12+30] → [text: {{item}}  y: i*12+30]
→ [text: {{hinweis}}]
```

- Loop-Body-Nodes sind visuell durch Einrückung und roten Rahmen erkennbar
- `i`, `loop.index`, `loop.y` stehen als Variablen in Body-Nodes zur Verfügung

---

## Drei Arten von if-Bedingungen

Die YAML kennt drei verschiedene if-Konstrukte, die im Node-Editor unterschiedlich dargestellt werden:

### ① Layer-Sichtbarkeit (`if:` auf Layer)

Steuert ob der **gesamte Node** gerendert wird. Darstellung: Badge oben rechts am Node.

```yaml
- type: rect
  if: "greaterThan(zug1.abw, 0)"
```

→ Node zeigt `〔WENN: greaterThan(zug1.abw, 0)〕` Badge + Textfeld für die Bedingung.

### ② Bedingte Eigenschaft (`if/then/else` auf Feld)

Steuert den **Wert eines einzelnen Feldes**. Darstellung: kompakte wenn/dann/sonst-Chips direkt neben dem Feld innerhalb des Nodes.

```yaml
color:
  if: "greaterThan(zug1.abw, 0)"
  then: "#FF4444"
  else: "#FFFFFF"
```

→ Neben dem `color`-Feld erscheinen drei Chips: `wenn` · `#FF4444` · `#FFF`. Klick auf das Feld öffnet die Bedingungseingabe.

### ③ Block-Bedingung (Block-Node ohne `type:`)

Gruppiert **mehrere Layer** unter einer Bedingung. Darstellung: gestrichelter Container-Node mit Sub-Kette (siehe oben).

---

## Node-Typen

| Node | Farbe (linker Rand) | Felder |
|---|---|---|
| `image` | Teal `#037F8C` | file (Dropdown), x, y, width, height, rotate, ① if-Badge |
| `rect` | Teal `#037F8C` | x, y, width, height, color (Farbpicker + ② Feld-if), ① if-Badge |
| `text` | Teal `#037F8C` | value + Filter-Pipeline, x, y, font (Dropdown), size, color (Farbpicker + ② Feld-if), align, width, height, ① if-Badge |
| `copy` | Teal `#037F8C` | src_x, src_y, src_width, src_height, x, y, ① if-Badge |
| `loop` | Rot `#C83232` | value, split_by, var, max_items |
| `block` | Orange `#FD7014` gestrichelt | Bedingungsfeld, Sub-Kette, + ELIF/ELSE anhängbar |

---

## Filter-Pipeline

Filter (`strip`, `upper`, `mul`, `add`, …) werden als **klickbare Chips** in einer Zeile innerhalb des Nodes dargestellt:

```
value:  {{zug1.hinweis}}  [strip('*')] [upper] [+]
        → Vorschau: ABWEICHENDE WAGENREIHUNG
```

- Klick auf `[+]` öffnet ein Dropdown mit allen verfügbaren Filtern gruppiert nach Text / Mathe / Zeit
- Klick auf einen Chip entfernt ihn
- Chips können per Drag umsortiert werden
- Die Vorschau-Zeile zeigt das Ergebnis mit den aktuellen Test-JSON-Daten (live)
- Gilt auch für Mathe-Filter auf `rotate`: `{{now.minute}}` → `[mul(6)]` → `180`

---

## Inline-Inputs

Alle Felder sind direkt im Node editierbar:

- **Textfelder** für Koordinaten und Expressions (`{{zug1.zeit}}`)
- **Dropdown** für `file` (befüllt aus der Dateiliste), `font`, `align`
- **Farbpicker** für `color`
- **Bedingungsfeld** für if-Badge / Feld-if mit Autovervollständigung der Bedingungsfunktionen

---

## Nodes hinzufügen

Neue Nodes werden per **Rechtsklick auf den Canvas** eingefügt — ein Kontextmenü zeigt alle verfügbaren Node-Typen gruppiert nach Kategorie:

- **Layer:** image, rect, text, copy
- **Logik:** block (if/elif/else Container)
- **Loop:** loop

Alternativ: Klick auf den „+" Ausgang eines bestehenden Nodes öffnet dasselbe Menü und verbindet den neuen Node direkt.

---

## Design / Styling

Passend zu den bestehenden Design-Tokens:

- **Canvas-Hintergrund:** `#F8F6F3`
- **Node-Hintergrund:** `#FFFFFF` (`--surface`)
- **Node-Border:** `#DDD8D2` (`--border`)
- **Node-Linker-Rand:** typenabhängig (siehe Node-Typen)
- **Verbindungslinien:** `#B8B0A8` (`--border-strong`), gestrichelt
- **if-Badge:** `#FD7014` (`--brand`)
- **elif-Badge:** `#FD7014` mit reduzierter Opacity
- **else-Badge:** `#9A938C`
- **Block-Container-Border:** `#FD7014` gestrichelt
- **Tab aktiv:** `#FD7014` mit Border-Bottom
- **Schrift:** `IBM Plex Mono` (`--font-mono`)

---

## YAML-Generierung (Nodes → YAML)

Der Graph wird beim Speichern traversiert und in eine YAML-Struktur serialisiert:

1. Jeder Layer-Node → ein YAML-Layer-Eintrag
2. Layer-Node mit IF-Badge → `if:`-Feld auf dem Layer-Eintrag; ELIF/ELSE-Badges folgen in Sequenz
3. Block-Node mit einem Layer im Ast → Layer-Level if/elif/else
4. Block-Node mit mehreren Layern im Ast → Block-Level if/elif/else mit `layers:`-Sub-Liste
5. `loop`-Node → `type: loop` mit `layers:` Sub-Liste aus dem Body-Chain
6. Filter-Chips → werden in die `{{expression | filter1 | filter2}}`-Syntax serialisiert

---

## YAML-Parsing (YAML → Nodes)

Beim Wechsel auf den Node-Tab wird `template.yaml` client-seitig geparst (YAML ist bereits im CodeMirror):

- **Unterstützt:** alle Layer-Typen, if/elif/else auf Layer-Ebene, Block-Level if/elif/else, loop, property-level if/then/else, Filter-Pipeline
- **Nicht unterstützt → Node-Tab gesperrt:** verschachtelte Loops, unbekannte Felder, syntaktisch invalides YAML

---

## Datei-Änderungen

| Datei | Änderung |
|---|---|
| `web/templates/edit-editor.html` | Tab-Umschalter + Node-Canvas Container |
| `web/static/app.css` | Styles für Node-Canvas, Nodes, Badges, Filter-Chips, Block-Container |
| `web/static/node-editor.js` | Rete.js Init, Node-Definitionen, Graph ↔ YAML Konvertierung, Filter-Pipeline |

---

## Was der Node-Editor nicht abdeckt (bewusst)

- Verschachtelte Loops (`type: loop` in `type: loop`)

Bei diesem Feature bleibt der YAML-Editor die einzige Option — der Node-Tab wird gesperrt.

**Hinweis zu Koordinaten und Ausdrücken:** Alle Koordinatenfelder (`x`, `y`, `width`, `height`, `rotate`, `src_x` etc.) sind freie Textfelder — `{{i * 12 + 30}}` und andere Arithmetik-Ausdrücke können direkt eingetippt werden. Das gilt auch für `type: copy`.
