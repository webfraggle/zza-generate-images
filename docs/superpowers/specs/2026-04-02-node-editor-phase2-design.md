# Node-Editor Phase 2 — Design-Spec

Datum: 2026-04-02
Status: Approved

Basis: Phase 1 auf `develop` (gemergt aus `feature/node-editor`).
Branch: `feature/node-editor-phase2` (von `develop`)

---

## Ziel

Phase 2 erweitert den Node-Editor um volle YAML-Spec-Abdeckung:
1. **Layer if-Badge** — `if:` auf Layer als Badge oben rechts am Node
2. **Feld-if Chips** — `if/then/else` auf dem `color`-Feld als kompakte Chips
3. **BLOCK-Container-Node** — Block-Level if/elif/else mit Sub-Kette
4. **Filter-Pipeline-Chips** — klickbare Chips für Filter auf `text.value` und `image.rotate`, mit Live-Vorschau aus dem Test-JSON

---

## Architektur-Entscheidung

**Ansatz B:** Neues Modul `node-filters.js` für Filter-Evaluator + Chip-UI. Alle anderen Features in bestehenden Dateien.

---

## Datenmodell-Erweiterungen

Das bestehende Graph-Modell `{nodes, chain}` bleibt kompatibel. Neue optionale Felder:

### ① Layer if-Badge

Der YAML-Spec kennt drei Varianten auf Layer-Ebene: `if: "cond"`, `elif: "cond"`, `else: true`.

```js
{
  id: 'n1', type: 'text', canvasX: 80, canvasY: 40,
  data: { value: '{{zug1.hinweis}}', ... },
  layerIfType: 'if',                          // 'if' | 'elif' | 'else' | undefined
  layerIfCond: "not(isEmpty(zug1.hinweis))"   // leer bei 'else', undefined wenn kein Badge
}
```

- `layerIfType` fehlt → kein Badge
- `layerIfType = 'else'` → `layerIfCond` ist leer (YAML: `else: true`)
- Im Serializer: `layerIfType = 'if'` → `layer.if`, `'elif'` → `layer.elif`, `'else'` → `layer.else = true`

### ② Feld-if (color)

Nur für `color`-Felder in `rect` und `text`. In `node.data` flache Keys:

```js
data.colorIf    = "greaterThan(zug1.abw, 0)"
data.colorThen  = "#FF4444"
data.colorElse  = "#FFFFFF"
```

Wenn `colorIf` gesetzt ist, wird das `color`-Feld als Chips dargestellt statt als Farbpicker.

### ③ BLOCK-Node

Neuer Typ `block` in NODE_TYPES. BLOCK-IF, BLOCK-ELIF, BLOCK-ELSE sind **separate Nodes in der Hauptkette** — spiegelt YAML-Struktur 1:1 wider.

```js
{
  id: 'n5', type: 'block',
  blockType: 'if',         // 'if' | 'elif' | 'else'
  blockCond: "startsWith(nr,'ICN')",  // leer bei 'else'
  bodyChain: ['n6', 'n7'],
  canvasX: 80, canvasY: 200,
  data: {}
}
```

BLOCK-ELIF und BLOCK-ELSE folgen unmittelbar nach BLOCK-IF in der Hauptkette. Der Parser erkennt die Sequenz anhand von `blockType`.

### ④ Filter-Pipeline

Felder mit `filterPipeline: true` in NODE_TYPES (→ `text.value`, `image.rotate`) werden geteilt:

```js
data.value         = "{{zug1.hinweis}}"           // Base-Expression ohne Filter
data.value_filters = [
  { fn: 'strip', arg: "'*'" },
  { fn: 'upper', arg: null }
]
```

Konvention: `data[fieldName + '_filters']` — Array von `{fn: string, arg: string|null}`.

**YAML-Roundtrip:** Parser splittet `{{expr | f1 | f2(arg)}}` beim Laden; Serializer setzt es beim Speichern zusammen.

---

## Neue Datei: node-filters.js

`web/static/node-filters.js` — kein Build-Step, ES-Modul.

### Exports

```js
// Metadaten aller unterstützten Filter
export const FILTER_DEFS = [
  { fn: 'strip',        label: 'strip',        hasArg: true,  category: 'text' },
  { fn: 'stripAll',     label: 'stripAll',     hasArg: true,  category: 'text' },
  { fn: 'stripBetween', label: 'stripBetween', hasArg: true,  category: 'text' },
  { fn: 'upper',        label: 'upper',        hasArg: false, category: 'text' },
  { fn: 'lower',        label: 'lower',        hasArg: false, category: 'text' },
  { fn: 'trim',         label: 'trim',         hasArg: false, category: 'text' },
  { fn: 'prefix',       label: 'prefix',       hasArg: true,  category: 'text' },
  { fn: 'suffix',       label: 'suffix',       hasArg: true,  category: 'text' },
  { fn: 'mul',          label: 'mul',          hasArg: true,  category: 'math' },
  { fn: 'div',          label: 'div',          hasArg: true,  category: 'math' },
  { fn: 'add',          label: 'add',          hasArg: true,  category: 'math' },
  { fn: 'sub',          label: 'sub',          hasArg: true,  category: 'math' },
  { fn: 'round',        label: 'round',        hasArg: false, category: 'math' },
  { fn: 'format',       label: 'format',       hasArg: true,  category: 'time' },
];

// Parst "{{expr | f1 | f2(arg)}}" → { base: "{{expr}}", filters: [{fn, arg}] }
// Gibt {base: str, filters: []} zurück wenn keine Filter vorhanden.
export function parseValueAndFilters(str) { ... }

// Setzt base + filters zusammen → "{{expr | f1 | f2(arg)}}"
export function formatValueWithFilters(base, filters) { ... }

// Wertet base-Expression gegen testJson aus und wendet Filter an.
// base: "{{zug1.hinweis}}", testJson: {zug1: {hinweis: '* Abweichung'}}
// Gibt einen String zurück (Vorschau-Wert) oder '' bei Fehler.
export function evaluatePreview(base, filters, testJson) { ... }

// Rendert Filter-Chips + [+]-Button + Vorschau-Zeile in container.
// onChange(filters) wird bei jeder Änderung aufgerufen.
// getTestJson() → gibt aktuelles testJson-Objekt zurück (kann null sein).
export function renderFilterRow(container, filters, onChange, getTestJson) { ... }
```

### evaluatePreview — Implementierung

1. Extrahiert den Pfad aus `{{path}}` (einfache Punkt-Notation: `zug1.hinweis`)
2. Löst Pfad im testJson auf → Rohwert als String
3. Wendet Filter sequenziell an (JS-Reimplementierung der Go-Filter)
4. `format`-Filter wird als `"[format]"` angezeigt (no-op im Preview, zu aufwändig)

### renderFilterRow — UI

```
[strip('*')] [upper] [+]
→ ABWEICHENDE WAGENREIHUNG
```

- Jeder Chip: `<span class="filter-chip">strip('*') ✕</span>` — Klick auf ✕ entfernt
- Chips per `draggable=true` + dragstart/dragover/drop umsortierbar
- `[+]` öffnet ein `<select>`-Dropdown mit Filtern gruppiert nach Kategorie
- Vorschau-Zeile (`→ ...`) nur wenn testJson verfügbar und base-Expression auflösbar

---

## Geänderte Dateien

### node-types.js

- `text.value`: `filterPipeline: true` hinzufügen
- `image.rotate`: `filterPipeline: true` hinzufügen
- `text.color`, `rect.color`: `fieldIf: true` hinzufügen
- Neuer Eintrag `block`: Farbe `#FD7014`, `blockNode: true`, ein Feld `blockCond` (label 'condition', inputType 'text')

```js
block: {
  label: 'BLOCK',
  color: '#FD7014',
  blockNode: true,
  fields: [
    { name: 'blockCond', label: 'condition', inputType: 'text' },
  ],
},
```

### node-parser.js

Entfernt als Sperr-Grund, wird stattdessen geparst:
- `layer.if` → `node.layerIf`
- `color: {if, then, else}` → `data.colorIf/colorThen/colorElse`
- `layer.type === 'block'` (und `elif:`/`else:` auf Layer-Ebene) → BLOCK-Node

Neue Logik:
- Felder mit `filterPipeline: true`: `parseValueAndFilters(val)` aus node-filters.js → `data.fieldName` + `data.fieldName_filters`
- BLOCK-Parsing: Layer ohne `type:` aber mit `block:`/`elif:`/`else:` Schlüssel → BLOCK-Node

Gesperrt bleibt:
- Verschachtelte Loops
- Unbekannte Typen

### node-serializer.js

- `node.layerIf` → `layer.if = node.layerIf` (vor allen anderen Feldern)
- `data.colorIf` → `layer.color = {if: colorIf, then: colorThen, else: colorElse}`
- BLOCK-Node → `{block: blockCond, layers: [...]}` bzw. `{elif: ..., layers: [...]}` / `{else: {layers: [...]}}`
- Felder mit `filterPipeline: true`: `formatValueWithFilters(data.fieldName, data.fieldName_filters)` aus node-filters.js

### node-editor.js

#### if-Badge (① Layer)
- In `_renderNode()`: wenn `node.layerIfType` gesetzt, zeige Badge oben rechts
- Badge-UI: Typ-Toggle (IF / ELIF / ELSE) + Bedingungsfeld (ausgegraut bei ELSE)
- Toggle "Badge hinzufügen": setzt `layerIfType = 'if'`; Toggle "entfernen": löscht `layerIfType` + `layerIfCond`
- ELIF/ELSE-Badge sind nur an Nodes sinnvoll die einem IF/ELIF in der Kette folgen — wird im UI nicht erzwungen, Parser/Serializer vertrauen der Reihenfolge

#### Feld-if Chips (②)
- `color`-Felder mit `fieldIf: true`: wenn `data.colorIf` gesetzt, zeige statt Farbpicker drei Chips: `wenn [input] dann [color] sonst [color]`
- Toggle-Button neben dem `color`-Label schaltet zwischen normalem Farbpicker und if/then/else-Modus um

#### BLOCK-Node (③)
- `_renderNode()`: BLOCK-Node rendert wie Loop-Node, aber:
  - Gestrichelte orange Border (`block-node` CSS-Klasse)
  - Nur ein Textfeld: `blockCond` (bei `blockType='else'` ausgegraut/leer)
  - Header zeigt "BLOCK IF" / "BLOCK ELIF" / "BLOCK ELSE" je nach `blockType`
- `_autoLayout()`: BLOCK-Body-Nodes layouten wie Loop-Body-Nodes (horizontal rechts)
- Dashed Container: absolut positioniertes `<div class="block-container">` im Canvas-Viewport, Größe und Position werden in `_autoLayout()` berechnet und per `style` gesetzt
- "ELIF hinzufügen" / "ELSE hinzufügen" Buttons am unteren Rand des BLOCK-Nodes → fügt neuen BLOCK-ELIF/ELSE-Node direkt nach aktuellem Node in der Kette ein

#### Filter-Pipeline (④)
- In `_renderNode()`: für Felder mit `filterPipeline: true`, rufe `renderFilterRow(container, filters, onChange, getTestJson)` aus node-filters.js auf
- `setTestJson(str)` Export: parst JSON-String, speichert als Modul-Variable, löst Live-Vorschau-Update aus
- In `edit-editor.html`: `jsonInput.addEventListener('input', () => setTestJson(jsonInput.value))` + initialer Aufruf beim Laden

---

## Canvas-Layout: BLOCK-Node

```
[BLOCK IF: startsWith(nr,'ICN')]    [image: icn.png] → [text: ICN Express]
         ↓
[BLOCK ELIF: startsWith(nr,'IC')]   [image: ic.png]
         ↓
[BLOCK ELSE]                        [text: {{nr}}]
         ↓
[text: {{zeit}}]
```

BLOCK-Body-Nodes layouten horizontal rechts vom BLOCK-Node — identisch zu Loop.

Der `block-container`-Div wird für jeden BLOCK-Node separat gerendert: er umrahmt die Body-Nodes mit gestrichelter oranger Border und leicht transparentem Hintergrund. Größe = `{von body[0].links bis body[last].rechts + Padding}`.

---

## Neue Test-Datei

`web/static/test/node-filters.test.mjs` — Unit-Tests für:
- `parseValueAndFilters` — verschiedene Formate (kein Filter, ein Filter, mehrere, Filter mit Arg)
- `formatValueWithFilters` — Roundtrip
- `evaluatePreview` — alle Filter-Funktionen gegen bekannte Eingaben

Außerdem: Erweiterte Tests in `node-parser.test.mjs` und `node-serializer.test.mjs` für Phase-2-Features.

---

## YAML-Roundtrip Beispiele

### Layer if-Badge
```yaml
# YAML → Parser → Serializer → YAML
- type: text
  if: "not(isEmpty(zug1.hinweis))"
  value: "{{zug1.hinweis}}"
```

### Feld-if
```yaml
- type: rect
  color:
    if: "greaterThan(zug1.abw, 0)"
    then: "#FF4444"
    else: "#FFFFFF"
```

### BLOCK
```yaml
- block: "startsWith(nr,'ICN')"
  layers:
    - type: image
      file: icn.png
- elif: "startsWith(nr,'IC')"
  layers:
    - type: image
      file: ic.png
- else:
  layers:
    - type: text
      value: "{{nr}}"
- type: text
  value: "{{zeit}}"
```

### Filter-Pipeline
```yaml
- type: text
  value: "{{zug1.hinweis | strip('*') | upper}}"
```

---

## Was Phase 2 nicht abdeckt (bewusst)

- Autocomplete für Bedingungsfelder (plain text input)
- `format`-Filter in Live-Vorschau (no-op, zu aufwändig)
- Verschachtelte Loops (weiterhin gesperrt)
- Filter auf anderen Feldern als `value` und `rotate`
