# Node-Editor Phase 2 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [x]`) syntax for tracking.

**Goal:** Erweitert den Node-Editor um Layer if-Badge, Feld-if-Chips, BLOCK-Container-Node und Filter-Pipeline-Chips mit Live-Vorschau.

**Architecture:** Neues Modul `node-filters.js` für JS-Filter-Evaluator und Chip-UI. Node-Parser, -Serializer und -Editor werden um alle Phase-2-Features erweitert. BLOCK-Nodes sind separate Knoten in der Hauptkette, analog zur bestehenden Loop-Implementierung.

**Tech Stack:** Vanilla JS ES-Module, kein Build-Step, `node --test` für Unit-Tests.

---

## Datei-Übersicht

| Datei | Aktion | Verantwortung |
|---|---|---|
| `web/static/node-types.js` | Modify | `filterPipeline`, `fieldIf` Flags, `block` Typ |
| `web/static/node-filters.js` | Create | Filter-Parsing, JS-Evaluator, Chip-UI |
| `web/static/test/node-filters.test.mjs` | Create | Unit-Tests für node-filters.js |
| `web/static/node-parser.js` | Modify | layerIf-Badges, colorIf, BLOCK-Nodes, Filter-Pipeline parsen |
| `web/static/test/node-parser.test.mjs` | Modify | Phase-2 Parser-Tests |
| `web/static/node-serializer.js` | Modify | layerIf, colorIf, BLOCK, Filter-Pipeline serialisieren |
| `web/static/test/node-serializer.test.mjs` | Modify | Phase-2 Serializer-Tests |
| `web/static/node-editor.js` | Modify | BLOCK-Node, if-Badge-UI, Feld-if-Chips, Filter-Chips, setTestJson |
| `web/templates/edit-editor.html` | Modify | setTestJson-Import und -Aufruf |
| `web/static/app.css` | Modify | Styles für Badges, Chips, Block-Container |

---

## Task 1: node-types.js — Flags und block-Typ

**Files:**
- Modify: `web/static/node-types.js`

Kein TDD erforderlich — reine Konfiguration.

- [x] **Schritt 1: node-types.js aktualisieren**

Vollständige neue Datei:

```js
// web/static/node-types.js
export const NODE_TYPES = {
  image: {
    label: 'IMAGE',
    color: '#037F8C',
    fields: [
      { name: 'file',   label: 'file',   inputType: 'dropdown', source: 'imageFiles' },
      { name: 'x',      label: 'x',      inputType: 'text', numeric: true },
      { name: 'y',      label: 'y',      inputType: 'text', numeric: true },
      { name: 'width',  label: 'width',  inputType: 'text', numeric: true },
      { name: 'height', label: 'height', inputType: 'text', numeric: true },
      { name: 'rotate', label: 'rotate', inputType: 'text', numeric: true, filterPipeline: true },
    ],
  },
  rect: {
    label: 'RECT',
    color: '#037F8C',
    fields: [
      { name: 'x',      label: 'x',      inputType: 'text', numeric: true },
      { name: 'y',      label: 'y',      inputType: 'text', numeric: true },
      { name: 'width',  label: 'width',  inputType: 'text', numeric: true },
      { name: 'height', label: 'height', inputType: 'text', numeric: true },
      { name: 'color',  label: 'color',  inputType: 'color', fieldIf: true },
    ],
  },
  text: {
    label: 'TEXT',
    color: '#037F8C',
    fields: [
      { name: 'value',  label: 'value',  inputType: 'text', filterPipeline: true },
      { name: 'x',      label: 'x',      inputType: 'text', numeric: true },
      { name: 'y',      label: 'y',      inputType: 'text', numeric: true },
      { name: 'font',   label: 'font',   inputType: 'dropdown', source: 'fontIds' },
      { name: 'size',   label: 'size',   inputType: 'text', numeric: true },
      { name: 'color',  label: 'color',  inputType: 'color', fieldIf: true },
      { name: 'align',  label: 'align',  inputType: 'dropdown', options: ['', 'left', 'center', 'right'] },
      { name: 'width',  label: 'width',  inputType: 'text', numeric: true },
      { name: 'height', label: 'height', inputType: 'text', numeric: true },
    ],
  },
  copy: {
    label: 'COPY',
    color: '#037F8C',
    fields: [
      { name: 'src_x',      label: 'src_x',   inputType: 'text', numeric: true },
      { name: 'src_y',      label: 'src_y',   inputType: 'text', numeric: true },
      { name: 'src_width',  label: 'src_w',   inputType: 'text', numeric: true },
      { name: 'src_height', label: 'src_h',   inputType: 'text', numeric: true },
      { name: 'x',          label: 'x',       inputType: 'text', numeric: true },
      { name: 'y',          label: 'y',       inputType: 'text', numeric: true },
    ],
  },
  loop: {
    label: 'LOOP',
    color: '#C83232',
    fields: [
      { name: 'loopValue', label: 'value',     inputType: 'text' },
      { name: 'splitBy',   label: 'split_by',  inputType: 'text' },
      { name: 'varName',   label: 'var',       inputType: 'text' },
      { name: 'maxItems',  label: 'max_items', inputType: 'text', numeric: true },
    ],
  },
  block: {
    label: 'BLOCK',
    color: '#FD7014',
    blockNode: true,
    fields: [],   // rendered specially in node-editor.js
  },
};

export const YAML_FIELD_MAP = {
  image:  { file: 'file', x: 'x', y: 'y', width: 'width', height: 'height', rotate: 'rotate' },
  rect:   { x: 'x', y: 'y', width: 'width', height: 'height', color: 'color' },
  text:   { value: 'value', x: 'x', y: 'y', font: 'font', size: 'size', color: 'color', align: 'align', width: 'width', height: 'height' },
  copy:   { src_x: 'src_x', src_y: 'src_y', src_width: 'src_width', src_height: 'src_height', x: 'x', y: 'y' },
  loop:   { loopValue: 'value', splitBy: 'split_by', varName: 'var', maxItems: 'max_items' },
};

export const YAML_TO_DATA_KEY = Object.freeze(
  Object.fromEntries(
    Object.entries(YAML_FIELD_MAP).map(([type, map]) => [
      type,
      Object.freeze(Object.fromEntries(Object.entries(map).map(([dk, yk]) => [yk, dk]))),
    ])
  )
);
```

- [x] **Schritt 2: Bestehende Tests prüfen**

```bash
node --test web/static/test/node-serializer.test.mjs
node --test web/static/test/node-parser.test.mjs
```

Expected: alle Tests grün (node-types.js-Änderungen sind rückwärtskompatibel).

- [x] **Schritt 3: Commit**

```bash
git add web/static/node-types.js
git commit -m "feat(node-editor): add block type, filterPipeline and fieldIf flags to node-types"
```

---

## Task 2: node-filters.js — parseValueAndFilters + formatValueWithFilters (TDD)

**Files:**
- Create: `web/static/node-filters.js`
- Create: `web/static/test/node-filters.test.mjs`

- [x] **Schritt 1: Test-Datei schreiben**

`web/static/test/node-filters.test.mjs`:

```js
import { parseValueAndFilters, formatValueWithFilters } from '../node-filters.js';
import { strict as assert } from 'node:assert';
import { describe, it } from 'node:test';

describe('parseValueAndFilters', () => {
  it('kein Template → base unverändert, keine Filter', () => {
    assert.deepStrictEqual(
      parseValueAndFilters('statischer Text'),
      { base: 'statischer Text', filters: [] }
    );
  });

  it('Template ohne Filter', () => {
    assert.deepStrictEqual(
      parseValueAndFilters('{{zug1.hinweis}}'),
      { base: '{{zug1.hinweis}}', filters: [] }
    );
  });

  it('Template, ein Filter ohne Arg', () => {
    assert.deepStrictEqual(
      parseValueAndFilters('{{zug1.nr | upper}}'),
      { base: '{{zug1.nr}}', filters: [{ fn: 'upper', arg: null }] }
    );
  });

  it("Template, ein Filter mit Arg", () => {
    assert.deepStrictEqual(
      parseValueAndFilters("{{zug1.hinweis | strip('*')}}"),
      { base: '{{zug1.hinweis}}', filters: [{ fn: 'strip', arg: "'*'" }] }
    );
  });

  it('Template, mehrere Filter', () => {
    assert.deepStrictEqual(
      parseValueAndFilters("{{zug1.hinweis | strip('*') | upper}}"),
      {
        base: '{{zug1.hinweis}}',
        filters: [{ fn: 'strip', arg: "'*'" }, { fn: 'upper', arg: null }],
      }
    );
  });

  it('Template, stripBetween mit zwei Args', () => {
    assert.deepStrictEqual(
      parseValueAndFilters("{{zug1.hinweis | stripBetween('{', '}')}}"),
      { base: '{{zug1.hinweis}}', filters: [{ fn: 'stripBetween', arg: "'{', '}'" }] }
    );
  });

  it('Math-Filter: mul', () => {
    assert.deepStrictEqual(
      parseValueAndFilters('{{now.minute | mul(6)}}'),
      { base: '{{now.minute}}', filters: [{ fn: 'mul', arg: '6' }] }
    );
  });
});

describe('formatValueWithFilters', () => {
  it('keine Filter → base unverändert', () => {
    assert.strictEqual(formatValueWithFilters('{{zug1.hinweis}}', []), '{{zug1.hinweis}}');
  });

  it('ein Filter ohne Arg', () => {
    assert.strictEqual(
      formatValueWithFilters('{{zug1.nr}}', [{ fn: 'upper', arg: null }]),
      '{{zug1.nr | upper}}'
    );
  });

  it('ein Filter mit Arg', () => {
    assert.strictEqual(
      formatValueWithFilters('{{zug1.hinweis}}', [{ fn: 'strip', arg: "'*'" }]),
      "{{zug1.hinweis | strip('*')}}"
    );
  });

  it('mehrere Filter', () => {
    assert.strictEqual(
      formatValueWithFilters('{{zug1.hinweis}}', [{ fn: 'strip', arg: "'*'" }, { fn: 'upper', arg: null }]),
      "{{zug1.hinweis | strip('*') | upper}}"
    );
  });

  it('Roundtrip: parse → format', () => {
    const orig = "{{zug1.hinweis | strip('*') | upper}}";
    const { base, filters } = parseValueAndFilters(orig);
    assert.strictEqual(formatValueWithFilters(base, filters), orig);
  });

  it('kein Template → base unverändert (Filter nicht anwendbar)', () => {
    assert.strictEqual(formatValueWithFilters('statisch', [{ fn: 'upper', arg: null }]), 'statisch');
  });
});
```

- [x] **Schritt 2: Test ausführen — soll fehlschlagen**

```bash
node --test web/static/test/node-filters.test.mjs
```

Expected: FAIL mit `Cannot find module '../node-filters.js'`

- [x] **Schritt 3: Minimale Implementierung in node-filters.js**

`web/static/node-filters.js` (nur parseValueAndFilters + formatValueWithFilters, Rest folgt in Task 3+4):

```js
// web/static/node-filters.js
// Filter-Pipeline: Parsing, Evaluierung (für Live-Vorschau), Chip-UI.

export const FILTER_DEFS = [
  { fn: 'strip',        label: 'strip',        hasArg: true,  category: 'text' },
  { fn: 'stripAll',     label: 'stripAll',      hasArg: true,  category: 'text' },
  { fn: 'stripBetween', label: 'stripBetween',  hasArg: true,  category: 'text' },
  { fn: 'upper',        label: 'upper',         hasArg: false, category: 'text' },
  { fn: 'lower',        label: 'lower',         hasArg: false, category: 'text' },
  { fn: 'trim',         label: 'trim',          hasArg: false, category: 'text' },
  { fn: 'prefix',       label: 'prefix',        hasArg: true,  category: 'text' },
  { fn: 'suffix',       label: 'suffix',        hasArg: true,  category: 'text' },
  { fn: 'mul',          label: 'mul',           hasArg: true,  category: 'math' },
  { fn: 'div',          label: 'div',           hasArg: true,  category: 'math' },
  { fn: 'add',          label: 'add',           hasArg: true,  category: 'math' },
  { fn: 'sub',          label: 'sub',           hasArg: true,  category: 'math' },
  { fn: 'round',        label: 'round',         hasArg: false, category: 'math' },
  { fn: 'format',       label: 'format',        hasArg: true,  category: 'time' },
];

/**
 * Parst "{{expr | f1 | f2(arg)}}" → { base: "{{expr}}", filters: [{fn, arg}] }
 * Gibt {base: str, filters: []} zurück wenn keine Filter vorhanden.
 * @param {string} str
 * @returns {{ base: string, filters: Array<{fn: string, arg: string|null}> }}
 */
export function parseValueAndFilters(str) {
  const match = str.match(/^\{\{(.+)\}\}$/s);
  if (!match) return { base: str, filters: [] };

  const inner = match[1];
  const parts = inner.split(' | ');
  if (parts.length === 1) return { base: str, filters: [] };

  const [expr, ...filterParts] = parts;
  const filters = filterParts.map(part => {
    const parenIdx = part.indexOf('(');
    if (parenIdx === -1) return { fn: part.trim(), arg: null };
    const fn  = part.slice(0, parenIdx).trim();
    const arg = part.slice(parenIdx + 1, part.lastIndexOf(')'));
    return { fn, arg };
  });

  return { base: `{{${expr}}}`, filters };
}

/**
 * Setzt base + filters zusammen → "{{expr | f1 | f2(arg)}}"
 * @param {string} base
 * @param {Array<{fn: string, arg: string|null}>} filters
 * @returns {string}
 */
export function formatValueWithFilters(base, filters) {
  if (!filters || filters.length === 0) return base;
  const match = base.match(/^\{\{(.+)\}\}$/s);
  if (!match) return base;
  const inner = match[1];
  const filterStr = filters
    .map(f => (f.arg != null ? `${f.fn}(${f.arg})` : f.fn))
    .join(' | ');
  return `{{${inner} | ${filterStr}}}`;
}

// evaluatePreview und renderFilterRow folgen in Task 3 + 4
export function evaluatePreview(_base, _filters, _testJson) { return ''; }
export function renderFilterRow(_container, _getBase, _filters, _onChange, _getTestJson) {
  return { updatePreview: () => {} };
}
```

- [x] **Schritt 4: Tests ausführen — sollen grün sein**

```bash
node --test web/static/test/node-filters.test.mjs
```

Expected: alle 12 Tests PASS.

- [x] **Schritt 5: Commit**

```bash
git add web/static/node-filters.js web/static/test/node-filters.test.mjs
git commit -m "feat(node-filters): add parseValueAndFilters + formatValueWithFilters with tests"
```

---

## Task 3: node-filters.js — evaluatePreview (TDD)

**Files:**
- Modify: `web/static/node-filters.js`
- Modify: `web/static/test/node-filters.test.mjs`

- [x] **Schritt 1: Tests für evaluatePreview hinzufügen**

Am Ende von `web/static/test/node-filters.test.mjs` anfügen:

```js
import { evaluatePreview } from '../node-filters.js';

describe('evaluatePreview', () => {
  const testJson = {
    zug1: { hinweis: '* Abweichung', nr: 'ICN', abw: '5' },
    now: { minute: 30 },
  };

  it('kein Filter, Pfad auflösen', () => {
    assert.strictEqual(evaluatePreview('{{zug1.nr}}', [], testJson), 'ICN');
  });

  it('upper-Filter', () => {
    assert.strictEqual(evaluatePreview('{{zug1.nr}}', [{ fn: 'upper', arg: null }], testJson), 'ICN');
  });

  it('lower-Filter', () => {
    assert.strictEqual(evaluatePreview('{{zug1.nr}}', [{ fn: 'lower', arg: null }], testJson), 'icn');
  });

  it("strip-Filter entfernt führendes Zeichen", () => {
    assert.strictEqual(
      evaluatePreview('{{zug1.hinweis}}', [{ fn: 'strip', arg: "'*'" }], testJson),
      ' Abweichung'
    );
  });

  it('strip + upper Verkettung', () => {
    assert.strictEqual(
      evaluatePreview('{{zug1.hinweis}}', [{ fn: 'strip', arg: "'*'" }, { fn: 'upper', arg: null }], testJson),
      ' ABWEICHUNG'
    );
  });

  it('stripAll entfernt alle Vorkommen', () => {
    assert.strictEqual(
      evaluatePreview('{{zug1.hinweis}}', [{ fn: 'stripAll', arg: "'*'" }], testJson),
      ' Abweichung'
    );
  });

  it('mul-Filter', () => {
    assert.strictEqual(
      evaluatePreview('{{now.minute}}', [{ fn: 'mul', arg: '6' }], testJson),
      '180'
    );
  });

  it('add-Filter', () => {
    assert.strictEqual(
      evaluatePreview('{{zug1.abw}}', [{ fn: 'add', arg: '3' }], testJson),
      '8'
    );
  });

  it('round-Filter', () => {
    assert.strictEqual(
      evaluatePreview('{{zug1.abw}}', [{ fn: 'round', arg: null }], testJson),
      '5'
    );
  });

  it('prefix-Filter', () => {
    assert.strictEqual(
      evaluatePreview('{{zug1.abw}}', [{ fn: 'prefix', arg: "'+'" }], testJson),
      '+5'
    );
  });

  it('trim-Filter', () => {
    assert.strictEqual(
      evaluatePreview('{{zug1.hinweis}}', [{ fn: 'trim', arg: null }], { zug1: { hinweis: '  text  ' } }),
      'text'
    );
  });

  it('format-Filter → [format] (no-op)', () => {
    assert.strictEqual(
      evaluatePreview('{{now.minute}}', [{ fn: 'format', arg: "'HH:mm'" }], testJson),
      '[format]'
    );
  });

  it('unbekannter Pfad → leer', () => {
    assert.strictEqual(evaluatePreview('{{zug1.unknown}}', [], testJson), '');
  });

  it('null testJson → leer', () => {
    assert.strictEqual(evaluatePreview('{{zug1.nr}}', [], null), '');
  });

  it('kein Template → base unverändert', () => {
    assert.strictEqual(evaluatePreview('statisch', [], testJson), 'statisch');
  });
});
```

- [x] **Schritt 2: Tests ausführen — evaluatePreview-Tests sollen fehlschlagen**

```bash
node --test web/static/test/node-filters.test.mjs
```

Expected: parse/format-Tests PASS, evaluatePreview-Tests FAIL (`''` statt erwartetem Wert).

- [x] **Schritt 3: evaluatePreview implementieren**

In `web/static/node-filters.js` den Stub `evaluatePreview` ersetzen:

```js
/**
 * Wertet base-Expression gegen testJson aus und wendet Filter an.
 * @param {string} base   z.B. "{{zug1.hinweis}}"
 * @param {Array<{fn: string, arg: string|null}>} filters
 * @param {object|null} testJson
 * @returns {string}
 */
export function evaluatePreview(base, filters, testJson) {
  if (!testJson) return '';
  const match = base.match(/^\{\{(.+)\}\}$/s);
  if (!match) return base;

  const path = match[1].trim();
  const raw = _resolvePath(path, testJson);
  if (raw === undefined || raw === null) return '';

  let value = String(raw);
  for (const { fn, arg } of filters) {
    value = _applyFilter(value, fn, arg);
  }
  return value;
}

function _resolvePath(path, obj) {
  return path.split('.').reduce((o, k) => (o != null ? o[k] : undefined), obj);
}

function _parseArg(arg) {
  if (arg == null) return null;
  const t = arg.trim();
  if ((t.startsWith("'") && t.endsWith("'")) || (t.startsWith('"') && t.endsWith('"'))) {
    return t.slice(1, -1);
  }
  return t;
}

function _parseTwoArgs(arg) {
  // "'a', 'b'" → ['a', 'b']
  const parts = arg.split(/,\s*/);
  return parts.map(_parseArg);
}

function _applyFilter(value, fn, arg) {
  switch (fn) {
    case 'strip': {
      const c = _parseArg(arg);
      return value.startsWith(c) ? value.slice(c.length) : value;
    }
    case 'stripAll': {
      const c = _parseArg(arg);
      return value.split(c).join('');
    }
    case 'stripBetween': {
      const [a, b] = _parseTwoArgs(arg);
      let result = value;
      let start = result.indexOf(a);
      while (start !== -1) {
        const end = result.indexOf(b, start);
        if (end === -1) break;
        result = result.slice(0, start) + result.slice(end + b.length);
        start = result.indexOf(a);
      }
      return result;
    }
    case 'upper':  return value.toUpperCase();
    case 'lower':  return value.toLowerCase();
    case 'trim':   return value.trim();
    case 'prefix': return _parseArg(arg) + value;
    case 'suffix': return value + _parseArg(arg);
    case 'mul':    return String(parseFloat(value) * parseFloat(_parseArg(arg)));
    case 'div':    return String(parseFloat(value) / parseFloat(_parseArg(arg)));
    case 'add':    return String(parseFloat(value) + parseFloat(_parseArg(arg)));
    case 'sub':    return String(parseFloat(value) - parseFloat(_parseArg(arg)));
    case 'round':  return String(Math.round(parseFloat(value)));
    case 'format': return '[format]';
    default:       return value;
  }
}
```

- [x] **Schritt 4: Alle Tests ausführen — grün**

```bash
node --test web/static/test/node-filters.test.mjs
```

Expected: alle Tests PASS.

- [x] **Schritt 5: Commit**

```bash
git add web/static/node-filters.js web/static/test/node-filters.test.mjs
git commit -m "feat(node-filters): add evaluatePreview with full filter implementations"
```

---

## Task 4: node-filters.js — renderFilterRow

**Files:**
- Modify: `web/static/node-filters.js`

Keine Unit-Tests (DOM). Visuell getestet in Task 8.

- [x] **Schritt 1: renderFilterRow implementieren**

Stub `renderFilterRow` in `web/static/node-filters.js` ersetzen:

```js
/**
 * Rendert Filter-Chips + [+]-Button + Vorschau-Zeile in container.
 * @param {HTMLElement} container
 * @param {() => string} getBase  — callback zum Abrufen der aktuellen Base-Expression
 * @param {Array<{fn, arg}>} filters
 * @param {(newFilters: Array) => void} onChange
 * @param {() => object|null} getTestJson
 * @returns {{ updatePreview: () => void }}
 */
export function renderFilterRow(container, getBase, filters, onChange, getTestJson) {
  container.innerHTML = '';

  const chipRow = document.createElement('div');
  chipRow.className = 'filter-chip-row';

  // ── Chips ──────────────────────────────────────────────────────────────────
  filters.forEach((filter, i) => {
    const chip = document.createElement('span');
    chip.className = 'filter-chip';
    chip.draggable = true;
    chip.dataset.index = String(i);

    const chipLabel = document.createElement('span');
    chipLabel.className = 'filter-chip-label';
    chipLabel.textContent = filter.arg != null ? `${filter.fn}(${filter.arg})` : filter.fn;
    chip.appendChild(chipLabel);

    const removeBtn = document.createElement('span');
    removeBtn.className = 'filter-chip-remove';
    removeBtn.textContent = '✕';
    removeBtn.addEventListener('click', e => {
      e.stopPropagation();
      onChange(filters.filter((_, j) => j !== i));
    });
    chip.appendChild(removeBtn);

    // Drag-to-reorder
    chip.addEventListener('dragstart', e => {
      e.dataTransfer.setData('text/plain', String(i));
      chip.classList.add('filter-chip--dragging');
    });
    chip.addEventListener('dragend', () => chip.classList.remove('filter-chip--dragging'));
    chip.addEventListener('dragover', e => { e.preventDefault(); chip.classList.add('filter-chip--drag-over'); });
    chip.addEventListener('dragleave', () => chip.classList.remove('filter-chip--drag-over'));
    chip.addEventListener('drop', e => {
      e.preventDefault();
      chip.classList.remove('filter-chip--drag-over');
      const fromIdx = parseInt(e.dataTransfer.getData('text/plain'), 10);
      if (fromIdx === i) return;
      const newFilters = [...filters];
      const [moved] = newFilters.splice(fromIdx, 1);
      newFilters.splice(i, 0, moved);
      onChange(newFilters);
    });

    chipRow.appendChild(chip);
  });

  // ── [+] Button + Dropdown ─────────────────────────────────────────────────
  const addWrapper = document.createElement('span');
  addWrapper.className = 'filter-add-wrapper';

  const addBtn = document.createElement('button');
  addBtn.type = 'button';
  addBtn.className = 'filter-add-btn';
  addBtn.textContent = '+';

  const dropdown = document.createElement('div');
  dropdown.className = 'filter-add-dropdown';
  dropdown.hidden = true;

  // Group by category
  const categories = [
    { key: 'text', label: 'Text' },
    { key: 'math', label: 'Mathe' },
    { key: 'time', label: 'Zeit' },
  ];
  for (const { key, label } of categories) {
    const defs = FILTER_DEFS.filter(f => f.category === key);
    if (!defs.length) continue;
    const group = document.createElement('div');
    group.className = 'filter-add-group';
    const groupTitle = document.createElement('div');
    groupTitle.className = 'filter-add-group-title';
    groupTitle.textContent = label;
    group.appendChild(groupTitle);
    for (const def of defs) {
      const item = document.createElement('button');
      item.type = 'button';
      item.className = 'filter-add-item';
      item.textContent = def.fn + (def.hasArg ? '(…)' : '');
      item.addEventListener('click', () => {
        dropdown.hidden = true;
        let arg = null;
        if (def.hasArg) {
          // Inline arg input
          const argInput = document.createElement('input');
          argInput.type = 'text';
          argInput.className = 'filter-arg-input';
          argInput.placeholder = `Argument für ${def.fn}`;
          addWrapper.appendChild(argInput);
          argInput.focus();
          const commit = () => {
            argInput.remove();
            const val = argInput.value.trim();
            if (!val) return;
            onChange([...filters, { fn: def.fn, arg: val }]);
          };
          argInput.addEventListener('keydown', e => {
            if (e.key === 'Enter') commit();
            if (e.key === 'Escape') argInput.remove();
          });
          argInput.addEventListener('blur', commit);
          return;
        }
        onChange([...filters, { fn: def.fn, arg }]);
      });
      group.appendChild(item);
    }
    dropdown.appendChild(group);
  }

  addBtn.addEventListener('click', e => {
    e.stopPropagation();
    dropdown.hidden = !dropdown.hidden;
  });
  document.addEventListener('mousedown', e => {
    if (!addWrapper.contains(e.target)) dropdown.hidden = true;
  }, { capture: true });

  addWrapper.appendChild(addBtn);
  addWrapper.appendChild(dropdown);
  chipRow.appendChild(addWrapper);
  container.appendChild(chipRow);

  // ── Vorschau-Zeile ────────────────────────────────────────────────────────
  const preview = document.createElement('div');
  preview.className = 'filter-preview';
  container.appendChild(preview);

  function updatePreview() {
    const base = getBase ? getBase() : '';
    const tj = getTestJson ? getTestJson() : null;
    if (!base || !tj) { preview.textContent = ''; return; }
    const result = evaluatePreview(base, filters, tj);
    preview.textContent = result !== '' ? `→ ${result}` : '';
  }
  updatePreview();

  return { updatePreview };
}
```

- [x] **Schritt 2: Bestehende Tests noch grün**

```bash
node --test web/static/test/node-filters.test.mjs
```

Expected: alle Tests PASS.

- [x] **Schritt 3: Commit**

```bash
git add web/static/node-filters.js
git commit -m "feat(node-filters): add renderFilterRow with chip UI, drag-to-reorder, live preview"
```

---

## Task 5: node-parser.js — Phase-2-Parsing (TDD)

**Files:**
- Modify: `web/static/node-parser.js`
- Modify: `web/static/test/node-parser.test.mjs`

- [x] **Schritt 1: Phase-2-Tests zu node-parser.test.mjs hinzufügen**

Am Ende von `web/static/test/node-parser.test.mjs` einfügen:

```js
// ── Phase-2-Tests ─────────────────────────────────────────────────────────────

describe('layerToGraph — Layer if-Badge', () => {
  it('layer mit if: → node.layerIfType="if", node.layerIfCond', () => {
    const r = layersToGraph([{ type: 'text', if: 'not(isEmpty(zug1.hinweis))', value: '{{zug1.hinweis}}' }]);
    assert.ok(r.ok);
    assert.strictEqual(r.nodes[0].layerIfType, 'if');
    assert.strictEqual(r.nodes[0].layerIfCond, 'not(isEmpty(zug1.hinweis))');
  });

  it('layer mit elif: → node.layerIfType="elif"', () => {
    const r = layersToGraph([{ type: 'image', elif: "startsWith(nr,'IC')", file: 'ic.png' }]);
    assert.ok(r.ok);
    assert.strictEqual(r.nodes[0].layerIfType, 'elif');
    assert.strictEqual(r.nodes[0].layerIfCond, "startsWith(nr,'IC')");
  });

  it('layer mit else: true → node.layerIfType="else", layerIfCond=""', () => {
    const r = layersToGraph([{ type: 'text', else: true, value: '{{nr}}' }]);
    assert.ok(r.ok);
    assert.strictEqual(r.nodes[0].layerIfType, 'else');
    assert.strictEqual(r.nodes[0].layerIfCond, '');
  });
});

describe('layersToGraph — Feld-if (colorIf)', () => {
  it('color als if/then/else-Objekt → data.colorIf/Then/Else', () => {
    const r = layersToGraph([{
      type: 'rect',
      color: { if: 'greaterThan(zug1.abw,0)', then: '#FF4444', else: '#FFFFFF' },
    }]);
    assert.ok(r.ok);
    assert.strictEqual(r.nodes[0].data.colorIf,   'greaterThan(zug1.abw,0)');
    assert.strictEqual(r.nodes[0].data.colorThen,  '#FF4444');
    assert.strictEqual(r.nodes[0].data.colorElse,  '#FFFFFF');
  });
});

describe('layersToGraph — Filter-Pipeline', () => {
  it("value mit Filtern → data.value = base, data.value_filters = [{fn,arg}]", () => {
    const r = layersToGraph([{ type: 'text', value: "{{zug1.hinweis | strip('*') | upper}}" }]);
    assert.ok(r.ok);
    assert.strictEqual(r.nodes[0].data.value, '{{zug1.hinweis}}');
    assert.deepStrictEqual(r.nodes[0].data.value_filters, [
      { fn: 'strip', arg: "'*'" },
      { fn: 'upper', arg: null },
    ]);
  });

  it('rotate mit mul-Filter', () => {
    const r = layersToGraph([{ type: 'image', file: 'bg.png', rotate: '{{now.minute | mul(6)}}' }]);
    assert.ok(r.ok);
    assert.strictEqual(r.nodes[0].data.rotate, '{{now.minute}}');
    assert.deepStrictEqual(r.nodes[0].data.rotate_filters, [{ fn: 'mul', arg: '6' }]);
  });
});

describe('layersToGraph — BLOCK-Nodes', () => {
  it('block-Layer → node.type="block", blockType="if"', () => {
    const r = layersToGraph([{
      block: "startsWith(nr,'ICN')",
      layers: [{ type: 'image', file: 'icn.png' }],
    }]);
    assert.ok(r.ok);
    assert.strictEqual(r.nodes[0].type, 'block');
    assert.strictEqual(r.nodes[0].blockType, 'if');
    assert.strictEqual(r.nodes[0].blockCond, "startsWith(nr,'ICN')");
    assert.strictEqual(r.nodes[0].bodyChain.length, 1);
  });

  it('elif-Block → node.type="block", blockType="elif"', () => {
    const r = layersToGraph([{
      elif: "startsWith(nr,'IC')",
      layers: [{ type: 'image', file: 'ic.png' }],
    }]);
    assert.ok(r.ok);
    assert.strictEqual(r.nodes[0].blockType, 'elif');
    assert.strictEqual(r.nodes[0].blockCond, "startsWith(nr,'IC')");
  });

  it('else-Block → node.type="block", blockType="else", blockCond=""', () => {
    const r = layersToGraph([{ else: true, layers: [{ type: 'text', value: '{{nr}}' }] }]);
    assert.ok(r.ok);
    assert.strictEqual(r.nodes[0].blockType, 'else');
    assert.strictEqual(r.nodes[0].blockCond, '');
  });

  it('vollständige if/elif/else-Kette', () => {
    const r = layersToGraph([
      { block: "startsWith(nr,'ICN')", layers: [{ type: 'image', file: 'icn.png' }] },
      { elif: "startsWith(nr,'IC')",  layers: [{ type: 'image', file: 'ic.png' }] },
      { else: true,                   layers: [{ type: 'text', value: '{{nr}}' }] },
      { type: 'text', value: '{{zeit}}' },
    ]);
    assert.ok(r.ok);
    assert.strictEqual(r.chain.length, 4);
    assert.strictEqual(r.nodes.find(n => n.blockType === 'if')?.type, 'block');
    assert.strictEqual(r.nodes.find(n => n.blockType === 'elif')?.type, 'block');
    assert.strictEqual(r.nodes.find(n => n.blockType === 'else')?.type, 'block');
  });
});
```

- [x] **Schritt 2: Tests ausführen — neue Tests sollen fehlschlagen**

```bash
node --test web/static/test/node-parser.test.mjs
```

Expected: bestehende Tests PASS, neue Phase-2-Tests FAIL.

- [x] **Schritt 3: node-parser.js für Phase 2 aktualisieren**

Vollständige neue `web/static/node-parser.js`:

```js
// web/static/node-parser.js
import { YAML_TO_DATA_KEY, NODE_TYPES } from './node-types.js';
import { parseValueAndFilters } from './node-filters.js';

const NODE_WIDTH    = 220;
const NODE_GAP      = 32;
const CANVAS_START_X = 80;
const CANVAS_START_Y = 40;
const NODE_HEADER_H  = 30;
const NODE_FIELD_H   = 28;
const NODE_BODY_PAD  = 22;

function nodeHeight(type) {
  // block type has no fields → use a fixed small height for layout
  if (type === 'block') return NODE_HEADER_H + NODE_BODY_PAD + NODE_FIELD_H;
  const fields = NODE_TYPES[type]?.fields?.length ?? 4;
  return NODE_HEADER_H + NODE_BODY_PAD + fields * NODE_FIELD_H;
}

/** Erkennt ob ein Layer ein Block/Elif/Else-Block oder ein regulärer Layer ist. */
function detectLayerKind(layer) {
  if (layer.block !== undefined) return 'block-if';
  if (layer.layers !== undefined && layer.elif !== undefined) return 'block-elif';
  if (layer.layers !== undefined && layer.else !== undefined) return 'block-else';
  if (layer.type !== undefined) return 'regular';
  return 'unknown';
}

function getLayerBadge(layer) {
  if (layer.if   !== undefined) return { type: 'if',   cond: String(layer.if) };
  if (layer.elif !== undefined) return { type: 'elif', cond: String(layer.elif) };
  if (layer.else !== undefined) return { type: 'else', cond: '' };
  return null;
}

/**
 * @param {object[]} layers
 * @returns {{ ok: true, nodes: object[], chain: string[] }
 *          |{ ok: false, reason: string }}
 */
export function layersToGraph(layers) {
  let idCounter = 1;
  const newId = () => `n${idCounter++}`;

  if (!Array.isArray(layers) || layers.length === 0) {
    return { ok: true, nodes: [], chain: [] };
  }

  const nodes = [];
  const chain = [];
  let y = CANVAS_START_Y;

  for (const layer of layers) {
    const err = checkSupported(layer, false);
    if (err) return { ok: false, reason: err };

    const { node, bodyNodes } = layerToNode(layer, CANVAS_START_X, y, newId);
    nodes.push(node, ...bodyNodes);
    chain.push(node.id);
    y += nodeHeight(node.type) + NODE_GAP;
  }

  return { ok: true, nodes, chain };
}

function checkSupported(layer, insideLoop) {
  if (layer === null || typeof layer !== 'object') {
    return `Invalid layer (expected object, got ${layer === null ? 'null' : typeof layer})`;
  }

  const kind = detectLayerKind(layer);

  if (kind === 'unknown') {
    return `Layer hat keinen erkennbaren Typ — im YAML-Tab bearbeiten`;
  }

  if (kind === 'block-if' || kind === 'block-elif' || kind === 'block-else') {
    if (insideLoop) return `Block-Nodes innerhalb von Loops werden nicht unterstützt`;
    for (const bodyLayer of (layer.layers || [])) {
      const err = checkSupported(bodyLayer, insideLoop);
      if (err) return err;
    }
    return null;
  }

  // Regulärer Layer
  const KNOWN_TYPES = new Set(['image', 'rect', 'text', 'copy', 'loop']);
  if (!KNOWN_TYPES.has(layer.type)) {
    return `Unbekannter Layer-Typ "${layer.type}" — im YAML-Tab bearbeiten`;
  }
  if (layer.type === 'loop' && insideLoop) {
    return `Verschachtelte Loops werden im Node-Editor nicht unterstützt`;
  }
  if (layer.type === 'loop' && Array.isArray(layer.layers)) {
    for (const bodyLayer of layer.layers) {
      const err = checkSupported(bodyLayer, true);
      if (err) return err;
    }
  }
  return null;
}

function layerToNode(layer, x, y, newId) {
  const kind = detectLayerKind(layer);

  if (kind === 'block-if' || kind === 'block-elif' || kind === 'block-else') {
    return blockLayerToNode(layer, kind, x, y, newId);
  }

  const fieldMap    = YAML_TO_DATA_KEY[layer.type] || {};
  const typeFields  = NODE_TYPES[layer.type]?.fields || [];
  const data        = {};

  for (const [yamlKey, dataKey] of Object.entries(fieldMap)) {
    if (layer[yamlKey] === undefined) continue;
    const fieldDef = typeFields.find(f => f.name === dataKey);
    const rawVal   = layer[yamlKey];

    if (fieldDef?.fieldIf && rawVal !== null && typeof rawVal === 'object' && 'if' in rawVal) {
      data[dataKey + 'If']   = String(rawVal.if   ?? '');
      data[dataKey + 'Then'] = String(rawVal.then ?? '');
      data[dataKey + 'Else'] = String(rawVal.else ?? '');
    } else if (fieldDef?.filterPipeline) {
      const { base, filters } = parseValueAndFilters(String(rawVal));
      data[dataKey]              = base;
      data[dataKey + '_filters'] = filters;
    } else {
      data[dataKey] = String(rawVal);
    }
  }

  if (layer.type === 'loop') {
    const bodyNodes = [];
    const bodyChain = [];
    let bodyX = x + NODE_WIDTH + 30;
    for (const bodyLayer of (layer.layers || [])) {
      const { node: bodyNode, bodyNodes: nested } = layerToNode(bodyLayer, bodyX, y, newId);
      bodyNodes.push(bodyNode, ...nested);
      bodyChain.push(bodyNode.id);
      bodyX += NODE_WIDTH + 30;
    }
    return {
      node: { id: newId(), type: 'loop', canvasX: x, canvasY: y, data, bodyChain },
      bodyNodes,
    };
  }

  const node = { id: newId(), type: layer.type, canvasX: x, canvasY: y, data };
  const badge = getLayerBadge(layer);
  if (badge) { node.layerIfType = badge.type; node.layerIfCond = badge.cond; }
  return { node, bodyNodes: [] };
}

function blockLayerToNode(layer, kind, x, y, newId) {
  const blockType = kind === 'block-if' ? 'if' : kind === 'block-elif' ? 'elif' : 'else';
  const blockCond = kind === 'block-if'   ? String(layer.block)
                  : kind === 'block-elif' ? String(layer.elif)
                  : '';

  const bodyNodes = [];
  const bodyChain = [];
  let bodyX = x + NODE_WIDTH + 30;
  for (const bodyLayer of (layer.layers || [])) {
    const { node: bodyNode, bodyNodes: nested } = layerToNode(bodyLayer, bodyX, y, newId);
    bodyNodes.push(bodyNode, ...nested);
    bodyChain.push(bodyNode.id);
    bodyX += NODE_WIDTH + 30;
  }

  return {
    node: { id: newId(), type: 'block', blockType, blockCond, bodyChain, canvasX: x, canvasY: y, data: {} },
    bodyNodes,
  };
}
```

- [x] **Schritt 4: Alle Parser-Tests ausführen — grün**

```bash
node --test web/static/test/node-parser.test.mjs
```

Expected: alle Tests PASS.

- [x] **Schritt 5: Commit**

```bash
git add web/static/node-parser.js web/static/test/node-parser.test.mjs
git commit -m "feat(node-parser): parse layerIf badges, colorIf, BLOCK nodes and filter pipelines"
```

---

## Task 6: node-serializer.js — Phase-2-Serialisierung (TDD)

**Files:**
- Modify: `web/static/node-serializer.js`
- Modify: `web/static/test/node-serializer.test.mjs`

- [x] **Schritt 1: Phase-2-Tests zu node-serializer.test.mjs hinzufügen**

Am Ende von `web/static/test/node-serializer.test.mjs` einfügen:

```js
// ── Phase-2-Tests ─────────────────────────────────────────────────────────────

describe('graphToLayers — Layer if-Badge', () => {
  it('layerIfType="if" → layer.if', () => {
    const graph = {
      nodes: [{ id: 'n1', type: 'text', canvasX: 0, canvasY: 0,
                layerIfType: 'if', layerIfCond: 'not(isEmpty(zug1.hinweis))',
                data: { value: '{{zug1.hinweis}}' } }],
      chain: ['n1'],
    };
    const layers = graphToLayers(graph);
    assert.strictEqual(layers[0].if, 'not(isEmpty(zug1.hinweis))');
    assert.strictEqual(layers[0].type, 'text');
  });

  it('layerIfType="elif" → layer.elif', () => {
    const graph = {
      nodes: [{ id: 'n1', type: 'image', canvasX: 0, canvasY: 0,
                layerIfType: 'elif', layerIfCond: "startsWith(nr,'IC')",
                data: { file: 'ic.png' } }],
      chain: ['n1'],
    };
    assert.strictEqual(graphToLayers(graph)[0].elif, "startsWith(nr,'IC')");
  });

  it('layerIfType="else" → layer.else = true', () => {
    const graph = {
      nodes: [{ id: 'n1', type: 'text', canvasX: 0, canvasY: 0,
                layerIfType: 'else', layerIfCond: '',
                data: { value: '{{nr}}' } }],
      chain: ['n1'],
    };
    assert.strictEqual(graphToLayers(graph)[0].else, true);
  });
});

describe('graphToLayers — Feld-if', () => {
  it('colorIf/Then/Else → color als if/then/else-Objekt', () => {
    const graph = {
      nodes: [{ id: 'n1', type: 'rect', canvasX: 0, canvasY: 0,
                data: { colorIf: 'greaterThan(zug1.abw,0)', colorThen: '#FF4444', colorElse: '#FFFFFF' } }],
      chain: ['n1'],
    };
    const layer = graphToLayers(graph)[0];
    assert.deepStrictEqual(layer.color, { if: 'greaterThan(zug1.abw,0)', then: '#FF4444', else: '#FFFFFF' });
  });
});

describe('graphToLayers — Filter-Pipeline', () => {
  it('value + value_filters → zusammengesetzter YAML-String', () => {
    const graph = {
      nodes: [{ id: 'n1', type: 'text', canvasX: 0, canvasY: 0,
                data: { value: '{{zug1.hinweis}}', value_filters: [{ fn: 'strip', arg: "'*'" }, { fn: 'upper', arg: null }] } }],
      chain: ['n1'],
    };
    assert.strictEqual(graphToLayers(graph)[0].value, "{{zug1.hinweis | strip('*') | upper}}");
  });

  it('keine Filter → value unverändert', () => {
    const graph = {
      nodes: [{ id: 'n1', type: 'text', canvasX: 0, canvasY: 0,
                data: { value: '{{zug1.nr}}', value_filters: [] } }],
      chain: ['n1'],
    };
    assert.strictEqual(graphToLayers(graph)[0].value, '{{zug1.nr}}');
  });
});

describe('graphToLayers — BLOCK-Nodes', () => {
  it('BLOCK-IF → {block: cond, layers: [...]}', () => {
    const graph = {
      nodes: [
        { id: 'n1', type: 'block', blockType: 'if', blockCond: "startsWith(nr,'ICN')",
          bodyChain: ['n2'], canvasX: 0, canvasY: 0, data: {} },
        { id: 'n2', type: 'image', canvasX: 0, canvasY: 0, data: { file: 'icn.png' } },
      ],
      chain: ['n1'],
    };
    const layer = graphToLayers(graph)[0];
    assert.strictEqual(layer.block, "startsWith(nr,'ICN')");
    assert.strictEqual(layer.layers[0].type, 'image');
  });

  it('BLOCK-ELIF → {elif: cond, layers: [...]}', () => {
    const graph = {
      nodes: [
        { id: 'n1', type: 'block', blockType: 'elif', blockCond: "startsWith(nr,'IC')",
          bodyChain: ['n2'], canvasX: 0, canvasY: 0, data: {} },
        { id: 'n2', type: 'image', canvasX: 0, canvasY: 0, data: { file: 'ic.png' } },
      ],
      chain: ['n1'],
    };
    const layer = graphToLayers(graph)[0];
    assert.strictEqual(layer.elif, "startsWith(nr,'IC')");
  });

  it('BLOCK-ELSE → {else: true, layers: [...]}', () => {
    const graph = {
      nodes: [
        { id: 'n1', type: 'block', blockType: 'else', blockCond: '',
          bodyChain: ['n2'], canvasX: 0, canvasY: 0, data: {} },
        { id: 'n2', type: 'text', canvasX: 0, canvasY: 0, data: { value: '{{nr}}' } },
      ],
      chain: ['n1'],
    };
    const layer = graphToLayers(graph)[0];
    assert.strictEqual(layer.else, true);
    assert.strictEqual(layer.layers[0].type, 'text');
  });

  it('Roundtrip: BLOCK-Kette parse → serialize', () => {
    const { layersToGraph } = await import('../node-parser.js');
    const inputLayers = [
      { block: "startsWith(nr,'ICN')", layers: [{ type: 'image', file: 'icn.png' }] },
      { elif: "startsWith(nr,'IC')",  layers: [{ type: 'image', file: 'ic.png'  }] },
      { else: true,                   layers: [{ type: 'text',  value: '{{nr}}'  }] },
    ];
    const graph = layersToGraph(inputLayers);
    assert.ok(graph.ok);
    const output = graphToLayers(graph);
    assert.strictEqual(output[0].block,  "startsWith(nr,'ICN')");
    assert.strictEqual(output[1].elif,   "startsWith(nr,'IC')");
    assert.strictEqual(output[2].else,   true);
  });
});
```

- [x] **Schritt 2: Tests ausführen — neue Tests sollen fehlschlagen**

```bash
node --test web/static/test/node-serializer.test.mjs
```

Expected: bestehende Tests PASS, neue Tests FAIL.

- [x] **Schritt 3: node-serializer.js für Phase 2 aktualisieren**

Vollständige neue `web/static/node-serializer.js`:

```js
// web/static/node-serializer.js
import { YAML_FIELD_MAP, NODE_TYPES } from './node-types.js';
import { formatValueWithFilters } from './node-filters.js';

const NUMERIC_DATA_KEYS = new Set(
  Object.values(NODE_TYPES).flatMap(t =>
    t.fields.filter(f => f.numeric).map(f => f.name)
  )
);

function toYamlValue(dataKey, val) {
  if (NUMERIC_DATA_KEYS.has(dataKey)) {
    const n = Number(val);
    return Number.isFinite(n) ? n : val;
  }
  return val;
}

/**
 * @param {{ nodes: object[], chain: string[] }} graph
 * @returns {object[]}
 */
export function graphToLayers({ nodes, chain }) {
  const nodeById = Object.fromEntries(nodes.map(n => [n.id, n]));
  return chain.map(id => nodeById[id]).filter(Boolean).map(n => nodeToLayer(n, nodeById));
}

function nodeToLayer(node, nodeById) {
  if (node.type === 'loop')  return loopNodeToLayer(node, nodeById);
  if (node.type === 'block') return blockNodeToLayer(node, nodeById);

  const layer     = {};
  const typeFields = NODE_TYPES[node.type]?.fields || [];

  // Badge (if/elif/else) — kommt vor type:
  if (node.layerIfType === 'if')   layer.if   = node.layerIfCond;
  if (node.layerIfType === 'elif') layer.elif = node.layerIfCond;
  if (node.layerIfType === 'else') layer.else = true;

  layer.type = node.type;

  const fieldMap = YAML_FIELD_MAP[node.type] || {};
  for (const [dataKey, yamlKey] of Object.entries(fieldMap)) {
    const fieldDef = typeFields.find(f => f.name === dataKey);

    if (fieldDef?.fieldIf) {
      const ifCond = node.data[dataKey + 'If'];
      if (ifCond) {
        layer[yamlKey] = {
          if:   node.data[dataKey + 'If']   || '',
          then: node.data[dataKey + 'Then'] || '',
          else: node.data[dataKey + 'Else'] || '',
        };
      } else {
        const val = node.data[dataKey];
        if (val !== undefined && val !== '') layer[yamlKey] = toYamlValue(dataKey, val);
      }
    } else if (fieldDef?.filterPipeline) {
      const base    = node.data[dataKey] || '';
      const filters = node.data[dataKey + '_filters'] || [];
      const composed = formatValueWithFilters(base, filters);
      if (composed !== undefined && composed !== '') layer[yamlKey] = toYamlValue(dataKey, composed);
    } else {
      const val = node.data[dataKey];
      if (val !== undefined && val !== '') layer[yamlKey] = toYamlValue(dataKey, val);
    }
  }
  return layer;
}

function loopNodeToLayer(node, nodeById) {
  const layer = { type: 'loop' };
  const fieldMap = YAML_FIELD_MAP['loop'];
  for (const [dataKey, yamlKey] of Object.entries(fieldMap)) {
    const val = node.data[dataKey];
    if (val !== undefined && val !== '') layer[yamlKey] = toYamlValue(dataKey, val);
  }
  if (node.bodyChain?.length) {
    layer.layers = node.bodyChain
      .map(id => nodeById[id])
      .filter(Boolean)
      .map(n => nodeToLayer(n, nodeById));
  }
  return layer;
}

function blockNodeToLayer(node, nodeById) {
  const bodyLayers = (node.bodyChain || [])
    .map(id => nodeById[id])
    .filter(Boolean)
    .map(n => nodeToLayer(n, nodeById));

  if (node.blockType === 'if')   return { block: node.blockCond || '', layers: bodyLayers };
  if (node.blockType === 'elif') return { elif:  node.blockCond || '', layers: bodyLayers };
  return { else: true, layers: bodyLayers };
}
```

- [x] **Schritt 4: Alle Serializer-Tests ausführen — grün**

```bash
node --test web/static/test/node-serializer.test.mjs
```

Expected: alle Tests PASS.

- [x] **Schritt 5: Auch Parser-Tests noch grün**

```bash
node --test web/static/test/node-parser.test.mjs
```

Expected: alle Tests PASS.

- [x] **Schritt 6: Commit**

```bash
git add web/static/node-serializer.js web/static/test/node-serializer.test.mjs
git commit -m "feat(node-serializer): serialize layerIf badges, colorIf, BLOCK nodes and filter pipelines"
```

---

## Task 7: node-editor.js — BLOCK-Node und Kern-Erweiterungen

**Files:**
- Modify: `web/static/node-editor.js`

Dieser Task fügt BLOCK-Node-Support hinzu: Kontext-Menü, `_addNode`, `_renderNode` für BLOCK, `_autoLayout` und `_renderConnections`.

- [x] **Schritt 1: Import node-filters.js, setTestJson-State hinzufügen**

Ganz oben in `node-editor.js`, nach der bestehenden Import-Zeile:

```js
import { NODE_TYPES } from './node-types.js';
import { renderFilterRow } from './node-filters.js';   // ← NEU
```

Im State-Block (nach `let _fontIds = [];`) hinzufügen:

```js
let _testJson = null;
const _previewRefreshers = new Set();
```

Und als neue public API-Funktion nach `getGraph()`:

```js
export function setTestJson(str) {
  try { _testJson = str ? JSON.parse(str) : null; } catch { _testJson = null; }
  for (const fn of _previewRefreshers) fn();
}
```

- [x] **Schritt 2: Kontext-Menü um BLOCK erweitern**

In `_showContextMenu`, die `groups`-Variable ändern:

```js
const groups = [
  { title: 'LAYER',  types: ['image', 'rect', 'text', 'copy'] },
  { title: 'LOGIK',  types: ['block'] },
  { title: 'LOOP',   types: ['loop'] },
];
```

- [x] **Schritt 3: `_addNode` für BLOCK erweitern**

In `_addNode` den bestehenden Spread erweitern:

```js
const node = {
  id, type, canvasX, canvasY, data: {},
  ...(type === 'loop'  ? { bodyChain: [] } : {}),
  ...(type === 'block' ? { blockType: 'if', blockCond: '', bodyChain: [] } : {}),
};
```

- [x] **Schritt 4: `_renderAll` — Block-Container vor Nodes erstellen**

In `_renderAll` nach dem `if (!_graph) return;`-Block, vor dem Nodes-Loop:

```js
// Block-Container (hinter Nodes = zuerst erstellen)
Array.from(_viewport.querySelectorAll('.ne-block-container')).forEach(el => el.remove());
_previewRefreshers.clear();
for (const node of _graph.nodes) {
  if (node.type === 'block' && node.bodyChain?.length) {
    const bc = document.createElement('div');
    bc.className = 'ne-block-container';
    bc.dataset.blockId = node.id;
    _viewport.appendChild(bc);
  }
}
```

- [x] **Schritt 5: `_autoLayout` — BLOCK-Nodes layouten**

In `_autoLayout`, nach dem bestehenden `if (node.type === 'loop' ...)` Block einen analogen Block für BLOCK einfügen:

```js
if (node.type === 'block' && node.bodyChain?.length) {
  let bodyX = CANVAS_ORIGIN_X + NODE_WIDTH + 30;
  for (const bodyId of node.bodyChain) {
    const bodyNode = nodeById[bodyId];
    if (!bodyNode) continue;
    bodyNode.canvasX = bodyX;
    bodyNode.canvasY = y;
    bodyX += NODE_WIDTH + 30;
  }
  // Block-Container positionieren
  const lastBody = nodeById[node.bodyChain[node.bodyChain.length - 1]];
  if (lastBody) {
    const containerEl = _viewport.querySelector(`.ne-block-container[data-block-id="${node.id}"]`);
    if (containerEl) {
      const pad = 8;
      containerEl.style.left   = (node.canvasX - pad) + 'px';
      containerEl.style.top    = (node.canvasY - pad) + 'px';
      containerEl.style.width  = (lastBody.canvasX + NODE_WIDTH - node.canvasX + pad * 2) + 'px';
      containerEl.style.height = (_nodeHeight('block') + pad * 2) + 'px';
      if (animate) containerEl.style.transition = 'left 0.35s ease, top 0.35s ease, width 0.35s ease, height 0.35s ease';
      else containerEl.style.transition = '';
    }
  }
}
```

- [x] **Schritt 6: `_renderConnections` — BLOCK-Body-Verbindungen**

In `_renderConnections`, nach dem Loop-Circuit-Block:

```js
// BLOCK body connections
for (const node of _graph.nodes) {
  if (node.type !== 'block' || !node.bodyChain?.length) continue;
  const firstBody = nodeById[node.bodyChain[0]];
  if (firstBody) _drawBlockBodyEntry(node, firstBody);
  for (let i = 0; i < node.bodyChain.length - 1; i++) {
    const a = nodeById[node.bodyChain[i]];
    const b = nodeById[node.bodyChain[i + 1]];
    if (a && b) _drawBodyBodyConnection(a, b);  // reuse from loop
  }
}
```

Neue Funktion nach `_drawLoopBodyReturn`:

```js
function _drawBlockBodyEntry(blockNode, bodyNode) {
  const blockEl = _viewport.querySelector(`.ne-node[data-id="${blockNode.id}"]`);
  const blockW  = blockEl ? blockEl.offsetWidth  : NODE_WIDTH;
  const blockH  = blockEl ? blockEl.offsetHeight : 60;

  const fromX = blockNode.canvasX + blockW;
  const fromY = blockNode.canvasY + blockH / 2;
  const toX   = bodyNode.canvasX;
  const toY   = bodyNode.canvasY + (_nodeHeight(bodyNode.type) / 2);

  const dx = (toX - fromX) * 0.5;
  _svgPath(`M ${fromX} ${fromY} C ${fromX + dx} ${fromY}, ${toX - dx} ${toY}, ${toX} ${toY}`, '#FD7014');
  const arrow = document.createElementNS('http://www.w3.org/2000/svg', 'polygon');
  arrow.setAttribute('points', `${toX},${toY} ${toX-6},${toY-4} ${toX-6},${toY+4}`);
  arrow.setAttribute('fill', '#FD7014');
  _svg.appendChild(arrow);
}
```

- [x] **Schritt 7: `_renderNode` für BLOCK-Typ**

In `_renderNode`, nach `if (!cfg) return;`, vor dem `el`-Create:

```js
if (node.type === 'block') {
  return _renderBlockNode(node, parent);
}
```

Neue Funktion `_renderBlockNode` nach `_renderNode`:

```js
function _renderBlockNode(node, parent) {
  const el = document.createElement('div');
  el.className = 'ne-node ne-node--block';
  el.dataset.id = node.id;
  el.style.left = node.canvasX + 'px';
  el.style.top  = node.canvasY + 'px';

  // Input port
  const portIn = document.createElement('div');
  portIn.className = 'ne-port-in';
  el.appendChild(portIn);

  // Header
  const header = document.createElement('div');
  header.className = 'ne-node-header';

  const dot = document.createElement('div');
  dot.className = 'ne-node-type-dot';
  dot.style.background = '#FD7014';

  const headerLabel = {
    'if':   'BLOCK IF',
    'elif': 'BLOCK ELIF',
    'else': 'BLOCK ELSE',
  }[node.blockType] || 'BLOCK';

  const label = document.createElement('span');
  label.className = 'ne-node-label';
  label.textContent = headerLabel;

  const delBtn = document.createElement('button');
  delBtn.className = 'ne-node-delete';
  delBtn.title = 'Node löschen';
  delBtn.textContent = '×';
  delBtn.addEventListener('click', e => { e.stopPropagation(); _deleteNode(node.id); });

  header.appendChild(dot);
  header.appendChild(label);
  header.appendChild(delBtn);
  el.appendChild(header);

  // Body: Bedingungsfeld (außer bei else)
  const body = document.createElement('div');
  body.className = 'ne-node-body';

  if (node.blockType !== 'else') {
    const row = document.createElement('div');
    row.className = 'ne-field';
    const lbl = document.createElement('span');
    lbl.className = 'ne-field-label';
    lbl.textContent = 'condition';
    const condInput = document.createElement('input');
    condInput.type = 'text';
    condInput.value = node.blockCond || '';
    condInput.addEventListener('input', () => { node.blockCond = condInput.value; });
    row.appendChild(lbl);
    row.appendChild(condInput);
    body.appendChild(row);
  }

  // ELIF/ELSE-Buttons (nur auf BLOCK-IF und BLOCK-ELIF)
  if (node.blockType === 'if' || node.blockType === 'elif') {
    const btnRow = document.createElement('div');
    btnRow.className = 'ne-block-btn-row';

    if (node.blockType === 'if') {
      const addElif = document.createElement('button');
      addElif.type = 'button';
      addElif.className = 'ne-block-add-btn';
      addElif.textContent = '+ ELIF';
      addElif.addEventListener('click', () => _insertBlockNode(node.id, 'elif'));
      btnRow.appendChild(addElif);

      const addElse = document.createElement('button');
      addElse.type = 'button';
      addElse.className = 'ne-block-add-btn';
      addElse.textContent = '+ ELSE';
      addElse.addEventListener('click', () => _insertBlockNode(node.id, 'else'));
      btnRow.appendChild(addElse);
    } else {
      const addElse = document.createElement('button');
      addElse.type = 'button';
      addElse.className = 'ne-block-add-btn';
      addElse.textContent = '+ ELSE';
      addElse.addEventListener('click', () => _insertBlockNode(node.id, 'else'));
      btnRow.appendChild(addElse);
    }
    body.appendChild(btnRow);
  }

  el.appendChild(body);

  // Output port
  const portOut = document.createElement('div');
  portOut.className = 'ne-port-out';
  el.appendChild(portOut);

  _makeDraggable(el, node);
  _initPortDrag(portOut, el, node);

  parent.appendChild(el);
  return el;
}

function _insertBlockNode(afterId, blockType) {
  if (!_graph) return;
  const id = `n${Date.now()}_${_idCounter++}`;
  const node = { id, type: 'block', blockType, blockCond: '', bodyChain: [], canvasX: 0, canvasY: 0, data: {} };
  _graph.nodes.push(node);
  const idx = _graph.chain.indexOf(afterId);
  if (idx !== -1) _graph.chain.splice(idx + 1, 0, id);
  else _graph.chain.push(id);
  _renderAll();
  _autoLayout(false);
}
```

- [x] **Schritt 8: Commit**

```bash
git add web/static/node-editor.js
git commit -m "feat(node-editor): add BLOCK node rendering, layout, connections and setTestJson"
```

---

## Task 8: node-editor.js — Layer if-Badge, Feld-if Chips und Filter-Pipeline

**Files:**
- Modify: `web/static/node-editor.js`

- [x] **Schritt 1: `_renderNode` — Layer if-Badge**

In `_renderNode`, nach dem `header.appendChild(delBtn);` und vor `el.appendChild(header);`, einfügen:

```js
// Layer if-Badge (nicht für loop/block)
if (cfg && !cfg.blockNode && node.type !== 'loop') {
  _appendIfBadge(header, el, node);
}
```

Neue Funktion nach `_renderBlockNode`:

```js
function _appendIfBadge(header, nodeEl, node) {
  // Toggle-Button im Header
  const toggleBtn = document.createElement('button');
  toggleBtn.type = 'button';
  toggleBtn.className = 'ne-if-badge-toggle';
  toggleBtn.title = 'Bedingung (IF/ELIF/ELSE) hinzufügen';
  toggleBtn.textContent = 'IF';
  toggleBtn.classList.toggle('ne-if-badge-toggle--active', !!node.layerIfType);
  header.insertBefore(toggleBtn, header.querySelector('.ne-node-delete'));

  // Badge-Zeile
  const badgeRow = document.createElement('div');
  badgeRow.className = 'ne-if-badge-row';
  badgeRow.hidden = !node.layerIfType;

  const typeSelect = document.createElement('select');
  typeSelect.className = 'ne-if-badge-type';
  for (const t of ['if', 'elif', 'else']) {
    const opt = document.createElement('option');
    opt.value = t;
    opt.textContent = t.toUpperCase();
    if (t === (node.layerIfType || 'if')) opt.selected = true;
    typeSelect.appendChild(opt);
  }

  const condInput = document.createElement('input');
  condInput.type = 'text';
  condInput.className = 'ne-if-badge-cond';
  condInput.placeholder = 'Bedingung…';
  condInput.value = node.layerIfCond || '';
  condInput.hidden = node.layerIfType === 'else';

  typeSelect.addEventListener('change', () => {
    node.layerIfType = typeSelect.value;
    condInput.hidden = typeSelect.value === 'else';
    if (typeSelect.value === 'else') node.layerIfCond = '';
  });
  condInput.addEventListener('input', () => { node.layerIfCond = condInput.value; });

  badgeRow.appendChild(typeSelect);
  badgeRow.appendChild(condInput);
  nodeEl.appendChild(badgeRow);

  toggleBtn.addEventListener('click', () => {
    if (node.layerIfType) {
      node.layerIfType = undefined;
      node.layerIfCond = undefined;
      badgeRow.hidden = true;
      toggleBtn.classList.remove('ne-if-badge-toggle--active');
    } else {
      node.layerIfType = 'if';
      node.layerIfCond = '';
      typeSelect.value = 'if';
      condInput.hidden = false;
      condInput.value = '';
      badgeRow.hidden = false;
      toggleBtn.classList.add('ne-if-badge-toggle--active');
    }
  });
}
```

- [x] **Schritt 2: `_renderNode` — Feld-if Chips für `color`-Felder**

Im `_renderNode` Field-Loop (`for (const field of cfg.fields) {`), das `if (field.inputType === 'color')` ersetzen:

```js
} else if (field.inputType === 'color') {
  if (field.fieldIf) {
    input = _buildColorFieldIfWidget(field, node, row);
  } else {
    input = document.createElement('input');
    input.type = 'color';
    input.value = /^#[0-9a-fA-F]{6}$/.test(node.data[field.name] || '')
      ? node.data[field.name] : '#000000';
    input.addEventListener('input', () => { node.data[field.name] = input.value; });
  }
}
```

Neue Funktion:

```js
function _buildColorFieldIfWidget(field, node, row) {
  const ifKey   = field.name + 'If';
  const thenKey = field.name + 'Then';
  const elseKey = field.name + 'Else';
  const hasIf   = !!node.data[ifKey];

  const toggleBtn = document.createElement('button');
  toggleBtn.type = 'button';
  toggleBtn.className = 'ne-field-if-toggle';
  toggleBtn.title = 'Bedingten Wert';
  toggleBtn.textContent = '±';
  toggleBtn.classList.toggle('ne-field-if-toggle--active', hasIf);

  // Wrapper für normalen Picker ODER if/then/else-Chips
  const wrapper = document.createElement('div');
  wrapper.className = 'ne-field-if-wrapper';

  function buildColorPicker(dataKey, defaultVal) {
    const picker = document.createElement('input');
    picker.type = 'color';
    picker.value = /^#[0-9a-fA-F]{6}$/.test(node.data[dataKey] || defaultVal || '')
      ? (node.data[dataKey] || defaultVal) : '#000000';
    picker.addEventListener('input', () => { node.data[dataKey] = picker.value; });
    return picker;
  }

  function renderNormal() {
    wrapper.innerHTML = '';
    wrapper.appendChild(buildColorPicker(field.name, ''));
  }

  function renderConditional() {
    wrapper.innerHTML = '';
    const condInput = document.createElement('input');
    condInput.type = 'text';
    condInput.className = 'ne-field-if-cond';
    condInput.placeholder = 'Bedingung…';
    condInput.value = node.data[ifKey] || '';
    condInput.addEventListener('input', () => { node.data[ifKey] = condInput.value; });

    const thenLbl = document.createElement('span');
    thenLbl.className = 'ne-field-if-chip-label';
    thenLbl.textContent = 'dann';

    const elseLbl = document.createElement('span');
    elseLbl.className = 'ne-field-if-chip-label';
    elseLbl.textContent = 'sonst';

    wrapper.append(condInput, thenLbl, buildColorPicker(thenKey, '#000000'), elseLbl, buildColorPicker(elseKey, '#000000'));
  }

  if (hasIf) renderConditional(); else renderNormal();

  toggleBtn.addEventListener('click', () => {
    if (node.data[ifKey]) {
      delete node.data[ifKey];
      delete node.data[thenKey];
      delete node.data[elseKey];
      toggleBtn.classList.remove('ne-field-if-toggle--active');
      renderNormal();
    } else {
      node.data[ifKey]   = '';
      node.data[thenKey] = '#000000';
      node.data[elseKey] = '#000000';
      toggleBtn.classList.add('ne-field-if-toggle--active');
      renderConditional();
    }
  });

  // Append toggle to label cell, return wrapper as input
  const labelEl = row.querySelector('.ne-field-label');
  if (labelEl) labelEl.appendChild(toggleBtn);
  return wrapper;
}
```

- [x] **Schritt 3: `_renderNode` — Filter-Pipeline-Chips**

Im Field-Loop (`if (field.inputType === 'text') {`), NACH der input-Erstellung für `filterPipeline`-Felder eine Filter-Zeile hinzufügen. Den kompletten `if (field.inputType === 'text')` Block ersetzen:

```js
if (field.inputType === 'text') {
  input = document.createElement('input');
  input.type = 'text';
  input.value = node.data[field.name] || '';
  input.addEventListener('input', () => { node.data[field.name] = input.value; });

  if (field.filterPipeline) {
    row.appendChild(input);
    // Filter-Zeile direkt nach dem Input
    const filterContainer = document.createElement('div');
    filterContainer.className = 'ne-filter-container';
    const filters = node.data[field.name + '_filters'] || [];
    const { updatePreview } = renderFilterRow(
      filterContainer,
      () => node.data[field.name] || '',
      filters,
      newFilters => {
        node.data[field.name + '_filters'] = newFilters;
        filterContainer.innerHTML = '';
        const { updatePreview: up2 } = renderFilterRow(
          filterContainer,
          () => node.data[field.name] || '',
          newFilters,
          onChange => { /* rekursiv — onChange wird im nächsten Re-Render gesetzt */ },
          () => _testJson
        );
        _previewRefreshers.delete(updatePreview);
        _previewRefreshers.add(up2);
      },
      () => _testJson
    );
    _previewRefreshers.add(updatePreview);
    row.appendChild(filterContainer);
    body.appendChild(row);
    continue;   // ← row bereits appended, nicht nochmal unten
  }
}
```

**Hinweis:** Das `continue` setzt voraus, dass der Field-Loop ein `for...of` ist. Das ist der Fall. Den `if (input) row.appendChild(input); body.appendChild(row);` am Ende des Loops muss weiterhin für alle anderen Felder funktionieren.

Tatsächlich ist die sauberere Implementierung: statt `continue` den filterPipeline-Branch vollständig in einen separaten Block auslagern, der `row` selbst appended. Alternativ: eine Flag `rowHandled` setzen:

```js
let rowHandled = false;
if (field.inputType === 'text') {
  input = document.createElement('input');
  input.type = 'text';
  input.value = node.data[field.name] || '';
  input.addEventListener('input', () => { node.data[field.name] = input.value; });

  if (field.filterPipeline) {
    const filterContainer = document.createElement('div');
    filterContainer.className = 'ne-filter-container';
    const initialFilters = node.data[field.name + '_filters'] || [];

    function mountFilterRow(filters) {
      filterContainer.innerHTML = '';
      const { updatePreview } = renderFilterRow(
        filterContainer,
        () => node.data[field.name] || '',
        filters,
        newFilters => {
          node.data[field.name + '_filters'] = newFilters;
          _previewRefreshers.delete(updatePreview);
          mountFilterRow(newFilters);
        },
        () => _testJson
      );
      _previewRefreshers.add(updatePreview);
    }

    mountFilterRow(initialFilters);
    row.appendChild(input);
    row.appendChild(filterContainer);
    body.appendChild(row);
    rowHandled = true;
  }
}
// ... restliche inputType-Branches ...
if (!rowHandled) {
  if (input) row.appendChild(input);
  body.appendChild(row);
}
```

- [x] **Schritt 4: Commit**

```bash
git add web/static/node-editor.js
git commit -m "feat(node-editor): add layer if-badge, field-if chips and filter pipeline UI"
```

---

## Task 9: edit-editor.html — setTestJson verdrahten

**Files:**
- Modify: `web/templates/edit-editor.html`

- [x] **Schritt 1: setTestJson importieren**

In `edit-editor.html`, die bestehende Import-Zeile:

```js
import { initCanvas, loadGraph, getGraph, setFileList, setFontIds } from '/static/node-editor.js';
```

ändern zu:

```js
import { initCanvas, loadGraph, getGraph, setFileList, setFontIds, setTestJson } from '/static/node-editor.js';
```

- [x] **Schritt 2: setTestJson bei JSON-Änderung aufrufen**

Direkt nach der bestehenden Zeile `jsonInput.addEventListener('input', schedulePreview);` einfügen:

```js
jsonInput.addEventListener('input', () => setTestJson(jsonInput.value));
```

Und beim initialen Laden (innerhalb der `loadDefaultJson`-Funktion, nach dem `jsonInput.value = await res.text();`-Block):

```js
setTestJson(jsonInput.value);
```

Außerdem: in `switchToNodes()` (in `edit-editor.html`) nach dem `loadGraph(result)`-Aufruf einfügen:

```js
setTestJson(jsonInput.value);
```

- [x] **Schritt 3: Commit**

```bash
git add web/templates/edit-editor.html
git commit -m "feat(editor): wire setTestJson for live filter preview"
```

---

## Task 10: app.css — Styles

**Files:**
- Modify: `web/static/app.css`

- [x] **Schritt 1: Neue CSS-Regeln ans Ende von app.css anfügen**

```css
/* ── Node-Editor Phase 2: Layer if-Badge ──────────────────────────────────── */

.ne-if-badge-toggle {
  margin-left: auto;
  padding: 0 5px;
  height: 18px;
  font-size: 10px;
  font-family: var(--font-mono);
  background: transparent;
  border: 1px solid var(--border);
  border-radius: 3px;
  color: var(--text-muted);
  cursor: pointer;
  flex-shrink: 0;
}
.ne-if-badge-toggle--active {
  background: var(--brand);
  border-color: var(--brand);
  color: #fff;
}
.ne-if-badge-row {
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 4px 10px 4px 10px;
  border-top: 1px solid var(--border);
  background: rgba(253,112,20,0.06);
}
.ne-if-badge-type {
  font-family: var(--font-mono);
  font-size: 10px;
  padding: 2px 4px;
  border: 1px solid var(--border);
  border-radius: 3px;
  background: var(--surface);
  color: var(--brand);
  font-weight: 600;
  cursor: pointer;
}
.ne-if-badge-cond {
  flex: 1;
  font-family: var(--font-mono);
  font-size: 11px;
  padding: 2px 5px;
  border: 1px solid var(--border);
  border-radius: 3px;
  background: var(--surface);
  min-width: 0;
}

/* ── Node-Editor Phase 2: Feld-if Chips ───────────────────────────────────── */

.ne-field-if-toggle {
  margin-left: 4px;
  padding: 0 3px;
  height: 14px;
  font-size: 10px;
  background: transparent;
  border: 1px solid var(--border);
  border-radius: 2px;
  color: var(--text-muted);
  cursor: pointer;
  vertical-align: middle;
}
.ne-field-if-toggle--active {
  background: var(--brand);
  border-color: var(--brand);
  color: #fff;
}
.ne-field-if-wrapper {
  display: flex;
  align-items: center;
  gap: 4px;
  flex: 1;
  flex-wrap: wrap;
  min-width: 0;
}
.ne-field-if-cond {
  flex: 1;
  font-family: var(--font-mono);
  font-size: 10px;
  padding: 1px 4px;
  border: 1px solid var(--border);
  border-radius: 3px;
  min-width: 60px;
}
.ne-field-if-chip-label {
  font-size: 10px;
  color: var(--text-muted);
  white-space: nowrap;
}

/* ── Node-Editor Phase 2: BLOCK-Node ─────────────────────────────────────── */

.ne-node--block {
  border: 2px dashed #FD7014;
  background: rgba(253,112,20,0.04);
}
.ne-block-container {
  position: absolute;
  border: 1.5px dashed #FD7014;
  border-radius: 6px;
  background: rgba(253,112,20,0.03);
  pointer-events: none;
  z-index: 0;
}
.ne-block-btn-row {
  display: flex;
  gap: 6px;
  padding: 4px 0 0 0;
}
.ne-block-add-btn {
  flex: 1;
  padding: 3px 6px;
  font-size: 10px;
  font-family: var(--font-mono);
  background: transparent;
  border: 1px solid #FD7014;
  border-radius: 3px;
  color: #FD7014;
  cursor: pointer;
}
.ne-block-add-btn:hover {
  background: rgba(253,112,20,0.1);
}

/* ── Node-Editor Phase 2: Filter-Pipeline-Chips ──────────────────────────── */

.ne-filter-container {
  width: 100%;
  margin-top: 3px;
}
.filter-chip-row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 3px;
  min-height: 22px;
}
.filter-chip {
  display: inline-flex;
  align-items: center;
  gap: 2px;
  padding: 1px 5px;
  background: var(--surface-raised, #F0EDE9);
  border: 1px solid var(--border);
  border-radius: 10px;
  font-size: 10px;
  font-family: var(--font-mono);
  cursor: grab;
  user-select: none;
}
.filter-chip--dragging {
  opacity: 0.4;
}
.filter-chip--drag-over {
  border-color: var(--brand);
  background: rgba(253,112,20,0.1);
}
.filter-chip-remove {
  font-size: 9px;
  color: var(--text-muted);
  cursor: pointer;
  padding: 0 1px;
}
.filter-chip-remove:hover { color: #c00; }

.filter-add-wrapper {
  position: relative;
  display: inline-flex;
  align-items: center;
}
.filter-add-btn {
  padding: 1px 6px;
  font-size: 11px;
  font-family: var(--font-mono);
  background: transparent;
  border: 1px dashed var(--border);
  border-radius: 10px;
  color: var(--text-muted);
  cursor: pointer;
  line-height: 1.4;
}
.filter-add-btn:hover { border-color: var(--brand); color: var(--brand); }

.filter-add-dropdown {
  position: absolute;
  top: calc(100% + 3px);
  left: 0;
  z-index: 200;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 6px;
  box-shadow: 0 4px 12px rgba(0,0,0,0.12);
  min-width: 140px;
  padding: 4px 0;
}
.filter-add-group-title {
  font-size: 9px;
  font-weight: 600;
  color: var(--text-muted);
  padding: 4px 10px 2px 10px;
  text-transform: uppercase;
  letter-spacing: 0.05em;
}
.filter-add-item {
  display: block;
  width: 100%;
  text-align: left;
  padding: 3px 10px;
  font-size: 11px;
  font-family: var(--font-mono);
  background: transparent;
  border: none;
  cursor: pointer;
  color: var(--text);
}
.filter-add-item:hover { background: rgba(0,0,0,0.05); }

.filter-arg-input {
  font-family: var(--font-mono);
  font-size: 11px;
  padding: 2px 5px;
  border: 1px solid var(--brand);
  border-radius: 3px;
  width: 100px;
  margin-left: 3px;
}

.filter-preview {
  font-size: 10px;
  font-family: var(--font-mono);
  color: var(--text-muted);
  padding: 1px 0;
  min-height: 14px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
```

- [x] **Schritt 2: Alle Tests noch grün**

```bash
node --test web/static/test/node-filters.test.mjs
node --test web/static/test/node-parser.test.mjs
node --test web/static/test/node-serializer.test.mjs
```

Expected: alle Tests PASS.

- [x] **Schritt 3: Commit**

```bash
git add web/static/app.css
git commit -m "feat(css): add styles for Phase 2 node editor features"
```

---

## Task 11: Branch erstellen und manuell testen

- [x] **Schritt 1: Branch erstellen (falls noch nicht geschehen)**

```bash
git checkout -b feature/node-editor-phase2
```

Alle vorigen Commits auf `develop` wurden dort gemacht — wenn der Branch erst jetzt erstellt wird, stattdessen:

```bash
git checkout -b feature/node-editor-phase2 HEAD
```

Hinweis: Idealerweise wird der Branch VOR Task 1 erstellt. Falls Tasks 1–10 auf `develop` gelaufen sind, können die Commits mit `git rebase --onto feature/node-editor-phase2` verschoben werden oder der Branch bleibt auf develop (wird später als Feature-Branch gehandhabt).

- [x] **Schritt 2: Manueller Test — Filter-Pipeline**

1. Server starten: `./zza --dev` oder `go run ./cmd/zza`
2. Template `default` im Editor öffnen
3. Auf NODES-Tab wechseln
4. TEXT-Node auswählen → `value`-Feld enthält `{{zug1.vonnach}}`
5. `[+]`-Button klicken → Dropdown erscheint mit Kategorien Text/Mathe
6. `upper` wählen → Chip `[upper ✕]` erscheint
7. Vorschau-Zeile zeigt groß geschriebenen Wert (wenn Test-JSON geladen ist)
8. Chip auf `[+]` ziehen → Reihenfolge ändert sich
9. Speichern → YAML hat `{{zug1.vonnach | upper}}`

- [x] **Schritt 3: Manueller Test — Layer if-Badge**

1. TEXT-Node → `IF`-Button klicken → Badge-Zeile erscheint
2. Bedingung eingeben: `not(isEmpty(zug1.hinweis))`
3. Typ auf ELIF wechseln → Bedingungsfeld bleibt sichtbar
4. Typ auf ELSE wechseln → Bedingungsfeld wird ausgeblendet
5. Speichern → YAML hat `else: true` auf dem Layer
6. Auf YAML-Tab zurückwechseln, wieder auf NODES → Badge ist noch vorhanden

- [x] **Schritt 4: Manueller Test — Feld-if (color)**

1. RECT-Node → `±`-Button neben `color`-Label klicken
2. Drei Elemente erscheinen: Bedingungsfeld + `dann`-Picker + `sonst`-Picker
3. Bedingung eingeben, Farben wählen
4. Speichern → YAML hat `color: {if: ..., then: '#...', else: '#...'}`

- [x] **Schritt 5: Manueller Test — BLOCK-Node**

1. Rechtsklick auf Canvas → Kategorie LOGIK → BLOCK
2. BLOCK-IF Node erscheint mit Bedingungsfeld
3. `+ ELIF` klicken → BLOCK-ELIF Node fügt sich in Kette ein
4. `+ ELSE` klicken → BLOCK-ELSE Node folgt
5. Body-Nodes per Port-Drag dem BLOCK-IF zuweisen
6. Gestrichelte orange Container sichtbar
7. Speichern → YAML hat korrekte `block:` / `elif:` / `else:`-Struktur

---

## Self-Review Checkliste (nach Implementierung)

- [x] Alle Unit-Tests grün: `node --test web/static/test/node-*.test.mjs`
- [x] Roundtrip parse→serialize für alle 4 Feature-Gruppen getestet
- [x] Bestehende Phase-1-Templates (ohne Phase-2-Features) laden noch fehlerfrei
- [x] `setTestJson` wird in `edit-editor.html` bei initialem Laden UND auf jsonInput-Input aufgerufen
- [x] Block-Container hat korrekte z-index-Sortierung (hinter Nodes)
- [x] Filter-Chip-Drag funktioniert ohne Events auf anderen Elementen zu stören
