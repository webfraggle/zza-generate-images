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

Jeder Node hat einen Eingang (oben) und einen Ausgang (unten). Die Verbindungen bestimmen die Render-Reihenfolge (= YAML-Reihenfolge).

### if / elif / else

Ein `if`-Node öffnet **parallele Äste**. Jeder Ast endet in einem gemeinsamen `merge`-Node der die Hauptkette fortsetzt:

```
[if: startsWith(nr,'ICN')]
  ├─ true  → [image: icn.png]  ─┐
  ├─ elif  → [image: ic.png]   ─┼→ [merge] → [text: zeit] → ...
  └─ else  → [text: {{nr}}]   ─┘
```

- `if`-Node hat Outputs: `true`, `elif` (beliebig viele), `else`
- `elif`-Nodes haben eine eigene Bedingung als Inline-Input
- Alle Äste laufen in einem `merge`-Node zusammen
- `merge` hat keine Konfiguration — ist nur struktureller Knotenpunkt

### loop

Ein `loop`-Node öffnet einen **eingerückten Sub-Chain** für die Loop-Body-Layer. Nach dem letzten Body-Node verbindet eine „end loop"-Kante zurück zur Hauptkette:

```
[loop: {{via}} split "|" var:item max:6]
  └─ body → [image: via-dot] → [text: {{item}}] → [end loop]
↓
[text: {{hinweis}}] → ...
```

- Loop-Body-Nodes sind visuell durch Einrückung und farbige Umrandung erkennbar
- `i`, `loop.index`, `loop.y` stehen als Variablen in Body-Nodes zur Verfügung

---

## Node-Typen

| Node | Farbe (linker Rand) | Felder |
|---|---|---|
| `image` | Teal `#037F8C` | file (Dropdown), x, y, width, height, rotate, if |
| `rect` | Teal `#037F8C` | x, y, width, height, color (Farbpicker), if |
| `text` | Teal `#037F8C` | value, x, y, font (Dropdown), size, color (Farbpicker), align, width, height, if |
| `copy` | Teal `#037F8C` | src_x, src_y, src_width, src_height, x, y, if |
| `if` | Orange `#FD7014` | Bedingung (Textfeld mit Autovervollständigung) |
| `elif` | Orange `#FD7014` | Bedingung (Textfeld) |
| `else` | Grau `#9A938C` | keine |
| `merge` | Grau `#B8B0A8` | keine |
| `loop` | Rot `#C83232` | value, split_by, var, max_items |

### Inline-Inputs

Alle Felder sind direkt im Node editierbar (kein separates Panel):

- **Textfelder** für Koordinaten, Expressions (`{{zug1.zeit}}`)
- **Dropdown** für `file` (befüllt aus der Dateiliste), `font`, `align`
- **Farbpicker** für `color`
- **Bedingungsfeld** für `if`/`elif` mit Vorschlägen für Bedingungsfunktionen (`startsWith`, `isEmpty`, etc.)

### Bedingte Eigenschaften (property-level if/then/else)

Felder wie `color` können eine Bedingung haben. Im Node wird das als kleines Toggle-Icon neben dem Feld dargestellt. Bei Aktivierung erscheinen `if`/`then`/`else`-Subfelder direkt darunter.

---

## Design / Styling

Passend zu den bestehenden Design-Tokens:

- **Canvas-Hintergrund:** `#F8F6F3` (etwas dunkler als `--surface-2`)
- **Node-Hintergrund:** `#FFFFFF` (`--surface`)
- **Node-Border:** `#DDD8D2` (`--border`)
- **Node-Linker-Rand:** typenabhängig (siehe Node-Typen)
- **Verbindungslinien:** `#B8B0A8` (`--border-strong`), gestrichelt
- **Tab aktiv:** `#FD7014` (`--brand`) mit Border-Bottom
- **Schrift:** `IBM Plex Mono` (`--font-mono`)

---

## YAML-Generierung (Nodes → YAML)

Der Graph wird beim Speichern traversiert (von der Root-Node die Hauptkette entlang) und in eine YAML-Struktur serialisiert:

1. Jeder Layer-Node → ein YAML-Layer-Eintrag
2. `if`-Node mit **einem Layer pro Ast** → `if:`/`elif:`/`else:`-Felder direkt am Layer-Node (Layer-Level)
3. `if`-Node mit **mehreren Layern pro Ast** → Block-Level if/elif/else mit `layers:`-Sub-Liste
4. `loop`-Node → `type: loop` mit `layers:` Sub-Liste aus dem Body-Chain
5. `merge`-Node → wird nicht serialisiert (nur strukturell)

---

## YAML-Parsing (YAML → Nodes)

Beim Wechsel vom YAML-Tab zum Node-Tab wird die aktuelle `template.yaml` geparst:

- **Unterstützt:** alle Layer-Typen, if/elif/else auf Layer-Ebene, Block-Level if/elif/else, loop, property-level if/then/else
- **Nicht unterstützt (→ Node-Tab gesperrt):** verschachtelte Loops, unbekannte Felder, syntaktisch invalides YAML

Der Parser läuft client-seitig in JS (kein Server-Roundtrip nötig — die YAML ist bereits im CodeMirror geladen).

---

## Datei-Änderungen (Übersicht)

| Datei | Änderung |
|---|---|
| `web/templates/edit-editor.html` | Tab-Umschalter + Node-Canvas Container |
| `web/static/app.css` | Styles für Node-Canvas, Nodes, Tabs, Inline-Inputs |
| `web/static/node-editor.js` | Rete.js Initialisierung, Node-Definitionen, Graph ↔ YAML Konvertierung |

Rete.js wird via `esm.sh` geladen — kein neues Build-System nötig.

---

## Nodes hinzufügen

Neue Nodes werden per **Rechtsklick auf den Canvas** eingefügt — ein Kontextmenü zeigt alle verfügbaren Node-Typen gruppiert nach Kategorie:

- **Layer:** image, rect, text, copy
- **Logik:** if/elif/else, merge
- **Loop:** loop

Alternativ: Klick auf den „+" Ausgang eines bestehenden Nodes öffnet dasselbe Menü und verbindet den neuen Node direkt.

---

## Was der Node-Editor nicht abdeckt (bewusst)

- Verschachtelte Loops (`type: loop` in `type: loop`)
- Koordinaten-Ausdrücke mit komplexer Arithmetik (werden als Textfeld dargestellt, aber nicht visuell modelliert)
- `type: copy` mit komplexen Ausdrücken

Bei diesen Features bleibt der YAML-Editor die einzige Option — der Node-Tab wird gesperrt.
