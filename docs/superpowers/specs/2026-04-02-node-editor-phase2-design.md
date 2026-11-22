# Node-Editor Phase 2 — Design-Spec

Datum: 2026-04-02
Status: Implementiert — wartet auf manuellen Test + Security/Code-Review + Merge

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

## Was Phase 2 nicht abdeckt (bewusst)

- Autocomplete für Bedingungsfelder (plain text input)
- `format`-Filter in Live-Vorschau (no-op, zu aufwändig)
- Verschachtelte Loops (weiterhin gesperrt)
- Filter auf anderen Feldern als `value` und `rotate`

---

## Implementierungs-Status (2026-04-02)

**Alle 10 Tasks implementiert, 67 Tests grün (28 filters + 20 parser + 19 serializer).**

Branch `feature/node-editor-phase2` in Worktree `.worktrees/node-editor-phase2`.

Commits (neueste zuerst):
- Task 10: CSS (app.css) — BLOCK-Badge, layer-if row, fieldIf toggle, filter chips
- Task 9: edit-editor.html — setTestJson Import + Aufruf bei JSON-Input-Änderung
- Task 8: node-editor.js — Phase 2 Field-UIs (layer-if row, filter chips, fieldIf toggle)
- Task 7: node-editor.js — BLOCK-Node (Kontextmenü, _renderBlockNode, _autoLayout, orange Verbindungen)
- Task 6: node-serializer.js — layerIf, colorIf, filterPipeline, blockNodeToLayer
- Frühere Tasks: node-parser.js, node-filters.js + Tests, node-types.js

**Nächster Schritt:** Manueller Test nach Anleitung in `docs/superpowers/plans/2026-04-02-node-editor-phase2-manual-tests.md`, danach Security-Review → Code-Review → Merge in `develop`.
