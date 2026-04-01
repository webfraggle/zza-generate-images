# Node-Editor Phase 1: Foundation — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a NODES tab to the template editor that lets users visually build YAML templates as a node graph (image / rect / text / copy / loop), with bidirectional conversion to/from the YAML editor.

**Architecture:** Custom lightweight canvas (positioned divs + SVG for connections) in Vanilla JS, no build step. A pure `graphToLayers()` function converts the graph to a plain JS object; js-yaml (esm.sh) handles YAML string serialization. A pure `layersToGraph()` converts parsed YAML layers to graph data; unsupported features (if/elif/else, conditional properties, nested loops) lock the NODES tab.

**Tech Stack:** Vanilla JS ES Modules (esm.sh), js-yaml@4 via esm.sh, `node:test` for unit tests of pure functions.

**Phase scope:** Basic layer nodes only (image / rect / text / copy / loop). No if-badges, no block nodes, no filter chips — those are Phase 2 and Phase 3. The parser locks the tab for any YAML with `if:` on a layer, conditional field values `{if/then/else}`, or nested loops.

---

## File Map

| File | Action | Responsibility |
|---|---|---|
| `web/templates/edit-editor.html` | Modify | Tab bar (YAML ↔ NODES), node canvas container, lock message |
| `web/static/app.css` | Modify | Tab bar, node canvas, node cards, lock message styles |
| `web/static/node-types.js` | Create | Node type config: fields, colors, labels |
| `web/static/node-serializer.js` | Create | `graphToLayers(graph)` → plain JS layers array |
| `web/static/node-parser.js` | Create | `layersToGraph(layers)` → `{ok, nodes, chain}` or `{ok:false, reason}` |
| `web/static/node-editor.js` | Create | Canvas init, pan/zoom, node rendering, drag, connections, context menu |
| `web/static/test/node-serializer.test.mjs` | Create | Unit tests for `graphToLayers` |
| `web/static/test/node-parser.test.mjs` | Create | Unit tests for `layersToGraph` |

### Graph data model (used across all modules)

```javascript
// graph object
{
  nodes: [
    {
      id: string,          // "n1", "n2", ...
      type: 'image' | 'rect' | 'text' | 'copy' | 'loop',
      canvasX: number,     // visual position on canvas (pixels)
      canvasY: number,
      data: {
        // image
        file?: string,
        // rect
        // text
        value?: string, font?: string, size?: string, align?: string,
        // shared
        x?: string, y?: string, width?: string, height?: string,
        color?: string, rotate?: string,
        // copy
        src_x?: string, src_y?: string, src_width?: string, src_height?: string,
        // loop  (field names match what the serializer outputs)
        loopValue?: string,   // the {{expr}} source
        splitBy?: string,
        varName?: string,
        maxItems?: string,
      },
      bodyChain?: string[],   // loop only: ordered child node IDs
    }
  ],
  chain: string[],            // top-level node IDs in render order
}
```

---

## Task 1: Tab Switcher HTML + CSS

**Files:**
- Modify: `web/templates/edit-editor.html`
- Modify: `web/static/app.css`

- [ ] **Step 1: Add tab bar to HTML**

In `web/templates/edit-editor.html`, replace the middle column:

```html
  <!-- ── Middle: code editor ──────────────────────────── -->
  <div class="editor-code-col">
    <div class="code-toolbar">
      <span id="active-file" class="pane-label">template.yaml</span>
      <div class="tab-switcher" id="tab-switcher">
        <button class="tab-btn tab-btn--active" id="tab-yaml" data-tab="yaml">YAML</button>
        <button class="tab-btn" id="tab-nodes" data-tab="nodes">NODES</button>
      </div>
      <button id="btn-save" class="btn btn-primary btn-sm">Speichern</button>
    </div>
    <div id="cm-host" class="cm-host"></div>
    <div id="node-canvas-wrap" class="node-canvas-wrap" hidden>
      <div id="node-canvas" class="node-canvas">
        <div id="nc-viewport" class="nc-viewport">
          <svg id="nc-svg" class="nc-svg"></svg>
        </div>
      </div>
      <div id="node-lock" class="node-lock" hidden>
        <p>Diese YAML enthält Features die im Node-Editor nicht darstellbar sind.</p>
        <p id="node-lock-reason" class="node-lock-reason"></p>
      </div>
    </div>
    <p id="save-status" class="save-status" hidden></p>
  </div>
```

Note: The tab switcher is only visible when `currentFile === 'template.yaml'`. The `#tab-switcher` element will be shown/hidden by JS (Task 9).

- [ ] **Step 2: Add CSS**

Append to `web/static/app.css`:

```css
/* ── NODE EDITOR ─────────────────────────────────────────────────────────────── */

/* Tab bar */
.tab-switcher {
  display: flex;
  gap: 0;
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
  overflow: hidden;
}
.tab-btn {
  padding: 3px 14px;
  font-family: var(--font-mono);
  font-size: 11px;
  background: var(--surface);
  color: var(--light-text);
  border: none;
  cursor: pointer;
  transition: background 0.1s, color 0.1s;
}
.tab-btn:hover { background: var(--surface-2); }
.tab-btn--active {
  background: var(--brand);
  color: #fff;
}

/* Node canvas wrapper */
.node-canvas-wrap {
  flex: 1;
  position: relative;
  overflow: hidden;
  display: flex;
  flex-direction: column;
}
.node-canvas {
  flex: 1;
  overflow: hidden;
  position: relative;
  background: #F8F6F3;
  cursor: default;
  user-select: none;
}
.nc-viewport {
  position: absolute;
  top: 0; left: 0;
  transform-origin: 0 0;
  width: 10000px;
  height: 10000px;
}
.nc-svg {
  position: absolute;
  top: 0; left: 0;
  width: 10000px;
  height: 10000px;
  pointer-events: none;
  overflow: visible;
}

/* Node cards */
.ne-node {
  position: absolute;
  width: 220px;
  background: var(--surface);
  border: 1.5px solid var(--border);
  border-radius: var(--radius-md);
  font-family: var(--font-mono);
  font-size: 11px;
  box-shadow: 0 1px 4px rgba(0,0,0,.07);
  cursor: default;
}
.ne-node-header {
  display: flex;
  align-items: center;
  padding: 5px 8px;
  border-bottom: 1px solid var(--border);
  cursor: grab;
  gap: 6px;
}
.ne-node-header:active { cursor: grabbing; }
.ne-node-type-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  flex-shrink: 0;
}
.ne-node-label {
  color: var(--light-text);
  font-size: 9px;
  letter-spacing: .05em;
  flex: 1;
}
.ne-node-delete {
  background: none;
  border: none;
  color: var(--light-text);
  cursor: pointer;
  font-size: 14px;
  line-height: 1;
  padding: 0 2px;
}
.ne-node-delete:hover { color: var(--red); }
.ne-node-body {
  padding: 8px;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

/* Node field row */
.ne-field {
  display: grid;
  grid-template-columns: 60px 1fr;
  align-items: center;
  gap: 4px;
}
.ne-field-label {
  color: var(--light-text);
  font-size: 9px;
  text-align: right;
  padding-right: 4px;
}
.ne-field input[type="text"],
.ne-field select {
  width: 100%;
  font-family: var(--font-mono);
  font-size: 10px;
  background: var(--surface-2);
  border: 1px solid var(--border);
  border-radius: 3px;
  padding: 2px 5px;
  color: var(--ink);
  box-sizing: border-box;
}
.ne-field input[type="color"] {
  width: 100%;
  height: 22px;
  padding: 1px 2px;
  border: 1px solid var(--border);
  border-radius: 3px;
  background: var(--surface-2);
  cursor: pointer;
}

/* Port circles */
.ne-port-out,
.ne-port-in {
  position: absolute;
  left: 50%;
  transform: translateX(-50%);
  width: 10px;
  height: 10px;
  border-radius: 50%;
  background: var(--border-strong);
  border: 2px solid var(--surface);
  cursor: crosshair;
  z-index: 10;
}
.ne-port-in  { top: -6px; }
.ne-port-out { bottom: -6px; }

/* Loop body container */
.ne-node--loop .ne-body-chain {
  margin: 6px 8px 8px 8px;
  border: 1.5px solid #C83232;
  border-radius: 4px;
  padding: 6px;
  background: #fff8f8;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

/* Lock overlay */
.node-lock {
  position: absolute;
  inset: 0;
  background: rgba(248,246,243,.92);
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 8px;
  font-family: var(--font-mono);
  font-size: 12px;
  color: var(--light-text);
  text-align: center;
  padding: 24px;
}
.node-lock-reason {
  font-size: 10px;
  color: var(--border-strong);
}

/* Context menu */
.ne-context-menu {
  position: fixed;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
  box-shadow: 0 4px 12px rgba(0,0,0,.12);
  font-family: var(--font-mono);
  font-size: 11px;
  z-index: 1000;
  min-width: 160px;
  overflow: hidden;
}
.ne-context-menu-group {
  padding: 4px 0;
  border-bottom: 1px solid var(--border);
}
.ne-context-menu-group:last-child { border-bottom: none; }
.ne-context-menu-title {
  padding: 2px 12px;
  font-size: 9px;
  color: var(--light-text);
  letter-spacing: .05em;
}
.ne-context-menu-item {
  display: block;
  width: 100%;
  background: none;
  border: none;
  text-align: left;
  padding: 5px 16px;
  cursor: pointer;
  color: var(--ink);
  font-family: var(--font-mono);
  font-size: 11px;
}
.ne-context-menu-item:hover { background: var(--surface-2); }

/* Hide tab switcher when non-yaml file is open */
.tab-switcher--hidden { display: none; }

/* cm-host: fill remaining space */
.cm-host {
  flex: 1;
  overflow: hidden;
  display: flex;
  flex-direction: column;
}
.cm-host .cm-editor { flex: 1; height: 100%; }
```

- [ ] **Step 3: Verify the editor still renders**

Open the editor in the browser (`GET /<template>/edit`). You should see:
- The YAML/NODES tab bar between the filename label and Save button
- YAML tab is active (orange background), NODES tab is inactive
- The CodeMirror editor works as before

---

## Task 2: Node Type Definitions

**Files:**
- Create: `web/static/node-types.js`

- [ ] **Step 1: Write node-types.js**

```javascript
// web/static/node-types.js
// Node type config: used by node-editor.js for rendering and node-parser.js for validation.

export const NODE_TYPES = {
  image: {
    label: 'IMAGE',
    color: '#037F8C',
    fields: [
      { name: 'file',   label: 'file',   inputType: 'dropdown', source: 'imageFiles' },
      { name: 'x',      label: 'x',      inputType: 'text' },
      { name: 'y',      label: 'y',      inputType: 'text' },
      { name: 'width',  label: 'width',  inputType: 'text' },
      { name: 'height', label: 'height', inputType: 'text' },
      { name: 'rotate', label: 'rotate', inputType: 'text' },
    ],
  },
  rect: {
    label: 'RECT',
    color: '#037F8C',
    fields: [
      { name: 'x',      label: 'x',      inputType: 'text' },
      { name: 'y',      label: 'y',      inputType: 'text' },
      { name: 'width',  label: 'width',  inputType: 'text' },
      { name: 'height', label: 'height', inputType: 'text' },
      { name: 'color',  label: 'color',  inputType: 'color' },
    ],
  },
  text: {
    label: 'TEXT',
    color: '#037F8C',
    fields: [
      { name: 'value',  label: 'value',  inputType: 'text' },
      { name: 'x',      label: 'x',      inputType: 'text' },
      { name: 'y',      label: 'y',      inputType: 'text' },
      { name: 'font',   label: 'font',   inputType: 'dropdown', source: 'fontIds' },
      { name: 'size',   label: 'size',   inputType: 'text' },
      { name: 'color',  label: 'color',  inputType: 'color' },
      { name: 'align',  label: 'align',  inputType: 'dropdown', options: ['', 'left', 'center', 'right'] },
      { name: 'width',  label: 'width',  inputType: 'text' },
      { name: 'height', label: 'height', inputType: 'text' },
    ],
  },
  copy: {
    label: 'COPY',
    color: '#037F8C',
    fields: [
      { name: 'src_x',      label: 'src_x',   inputType: 'text' },
      { name: 'src_y',      label: 'src_y',   inputType: 'text' },
      { name: 'src_width',  label: 'src_w',   inputType: 'text' },
      { name: 'src_height', label: 'src_h',   inputType: 'text' },
      { name: 'x',          label: 'x',       inputType: 'text' },
      { name: 'y',          label: 'y',       inputType: 'text' },
    ],
  },
  loop: {
    label: 'LOOP',
    color: '#C83232',
    fields: [
      { name: 'loopValue', label: 'value',   inputType: 'text' },
      { name: 'splitBy',   label: 'split_by', inputType: 'text' },
      { name: 'varName',   label: 'var',      inputType: 'text' },
      { name: 'maxItems',  label: 'max_items', inputType: 'text' },
    ],
  },
};

// YAML field names for each node type (maps data key → YAML key).
// Used by node-serializer.js.
export const YAML_FIELD_MAP = {
  image:  { file: 'file', x: 'x', y: 'y', width: 'width', height: 'height', rotate: 'rotate' },
  rect:   { x: 'x', y: 'y', width: 'width', height: 'height', color: 'color' },
  text:   { value: 'value', x: 'x', y: 'y', font: 'font', size: 'size', color: 'color', align: 'align', width: 'width', height: 'height' },
  copy:   { src_x: 'src_x', src_y: 'src_y', src_width: 'src_width', src_height: 'src_height', x: 'x', y: 'y' },
  loop:   { loopValue: 'value', splitBy: 'split_by', varName: 'var', maxItems: 'max_items' },
};

// YAML field names → data key (inverse map, used by node-parser.js).
export const YAML_TO_DATA_KEY = {};
for (const [type, map] of Object.entries(YAML_FIELD_MAP)) {
  YAML_TO_DATA_KEY[type] = Object.fromEntries(Object.entries(map).map(([dk, yk]) => [yk, dk]));
}
```

---

## Task 3: Serializer (graph → YAML layers array)

**Files:**
- Create: `web/static/node-serializer.js`
- Create: `web/static/test/node-serializer.test.mjs`

- [ ] **Step 1: Write failing tests**

Create `web/static/test/node-serializer.test.mjs`:

```javascript
// web/static/test/node-serializer.test.mjs
// Run with: node --test web/static/test/node-serializer.test.mjs
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { graphToLayers } from '../node-serializer.js';

// Minimal graph builder helper
function makeGraph(nodes, chain) { return { nodes, chain }; }
function node(overrides) {
  return { id: 'n1', type: 'image', canvasX: 0, canvasY: 0, data: {}, ...overrides };
}

test('empty graph returns empty layers', () => {
  const result = graphToLayers(makeGraph([], []));
  assert.deepEqual(result, []);
});

test('single image node', () => {
  const graph = makeGraph(
    [node({ id: 'n1', type: 'image', data: { file: 'bg.png', x: '0', y: '0' } })],
    ['n1']
  );
  const layers = graphToLayers(graph);
  assert.equal(layers.length, 1);
  assert.deepEqual(layers[0], { type: 'image', file: 'bg.png', x: '0', y: '0' });
});

test('image node omits empty fields', () => {
  const graph = makeGraph(
    [node({ id: 'n1', type: 'image', data: { file: 'a.png' } })],
    ['n1']
  );
  const layers = graphToLayers(graph);
  assert.equal(layers[0].x, undefined);
  assert.equal(layers[0].y, undefined);
  assert.equal(layers[0].width, undefined);
});

test('rect node with color', () => {
  const graph = makeGraph(
    [node({ id: 'n1', type: 'rect', data: { x: '10', y: '20', width: '100', height: '50', color: '#FF0000' } })],
    ['n1']
  );
  const layers = graphToLayers(graph);
  assert.deepEqual(layers[0], { type: 'rect', x: '10', y: '20', width: '100', height: '50', color: '#FF0000' });
});

test('text node', () => {
  const graph = makeGraph(
    [node({ id: 'n1', type: 'text', data: { value: '{{zug1.zeit}}', x: '5', y: '60', font: 'regular', size: '16', color: '#000000', align: 'left' } })],
    ['n1']
  );
  const layers = graphToLayers(graph);
  assert.deepEqual(layers[0], { type: 'text', value: '{{zug1.zeit}}', x: '5', y: '60', font: 'regular', size: '16', color: '#000000', align: 'left' });
});

test('copy node', () => {
  const graph = makeGraph(
    [node({ id: 'n1', type: 'copy', data: { src_x: '0', src_y: '0', src_width: '240', src_height: '120', x: '0', y: '120' } })],
    ['n1']
  );
  const layers = graphToLayers(graph);
  assert.deepEqual(layers[0], { type: 'copy', src_x: '0', src_y: '0', src_width: '240', src_height: '120', x: '0', y: '120' });
});

test('chain of nodes preserves order', () => {
  const nodes = [
    node({ id: 'n1', type: 'image', data: { file: 'bg.png' } }),
    node({ id: 'n2', type: 'text',  data: { value: 'hi' } }),
  ];
  const graph = makeGraph(nodes, ['n1', 'n2']);
  const layers = graphToLayers(graph);
  assert.equal(layers[0].type, 'image');
  assert.equal(layers[1].type, 'text');
});

test('loop node with body chain', () => {
  const nodes = [
    { id: 'n1', type: 'loop', canvasX: 0, canvasY: 0,
      data: { loopValue: '{{zug1.via}}', splitBy: '-', varName: 'via_item', maxItems: '5' },
      bodyChain: ['n2']
    },
    { id: 'n2', type: 'text', canvasX: 0, canvasY: 0, data: { value: '{{via_item}}' } },
  ];
  const graph = makeGraph(nodes, ['n1']);
  const layers = graphToLayers(graph);
  assert.equal(layers.length, 1);
  assert.equal(layers[0].type, 'loop');
  assert.equal(layers[0].value, '{{zug1.via}}');
  assert.equal(layers[0].split_by, '-');
  assert.equal(layers[0].var, 'via_item');
  assert.equal(layers[0].max_items, '5');
  assert.equal(layers[0].layers.length, 1);
  assert.equal(layers[0].layers[0].type, 'text');
  assert.equal(layers[0].layers[0].value, '{{via_item}}');
});

test('loop node without body chain produces no layers key', () => {
  const nodes = [
    { id: 'n1', type: 'loop', canvasX: 0, canvasY: 0,
      data: { loopValue: '{{zug1.via}}' }, bodyChain: [] }
  ];
  const layers = graphToLayers(makeGraph(nodes, ['n1']));
  assert.equal(layers[0].layers, undefined);
});
```

- [ ] **Step 2: Run tests — expect FAIL (module not found)**

```bash
node --test web/static/test/node-serializer.test.mjs
```

Expected: `Error [ERR_MODULE_NOT_FOUND]: Cannot find module '../node-serializer.js'`

- [ ] **Step 3: Create node-serializer.js**

```javascript
// web/static/node-serializer.js
// Pure function: converts graph data model to YAML layers array.
// No external dependencies — call jsYaml.dump(graphToLayers(graph)) in the browser.
import { YAML_FIELD_MAP } from './node-types.js';

/**
 * Convert graph to YAML-ready layers array.
 * @param {{ nodes: object[], chain: string[] }} graph
 * @returns {object[]} layers array suitable for jsYaml.dump
 */
export function graphToLayers({ nodes, chain }) {
  const nodeById = Object.fromEntries(nodes.map(n => [n.id, n]));
  return chain.map(id => nodeById[id]).filter(Boolean).map(n => nodeToLayer(n, nodeById));
}

function nodeToLayer(node, nodeById) {
  if (node.type === 'loop') {
    return loopNodeToLayer(node, nodeById);
  }

  const layer = { type: node.type };
  const fieldMap = YAML_FIELD_MAP[node.type] || {};
  for (const [dataKey, yamlKey] of Object.entries(fieldMap)) {
    const val = node.data[dataKey];
    if (val !== undefined && val !== '') {
      layer[yamlKey] = val;
    }
  }
  return layer;
}

function loopNodeToLayer(node, nodeById) {
  const layer = { type: 'loop' };
  const d = node.data;
  if (d.loopValue)  layer.value     = d.loopValue;
  if (d.splitBy)    layer.split_by  = d.splitBy;
  if (d.varName)    layer.var       = d.varName;
  if (d.maxItems)   layer.max_items = d.maxItems;
  if (node.bodyChain && node.bodyChain.length > 0) {
    layer.layers = node.bodyChain
      .map(id => nodeById[id])
      .filter(Boolean)
      .map(n => nodeToLayer(n, nodeById));
  }
  return layer;
}
```

- [ ] **Step 4: Run tests — expect PASS**

```bash
node --test web/static/test/node-serializer.test.mjs
```

Expected: all 8 tests pass, `✓` markers.

- [ ] **Step 5: Commit**

```bash
git add web/static/node-serializer.js web/static/test/node-serializer.test.mjs web/static/node-types.js
git commit -m "feat: add node-types and graph serializer with tests"
```

---

## Task 4: Parser (YAML layers → graph)

**Files:**
- Create: `web/static/node-parser.js`
- Create: `web/static/test/node-parser.test.mjs`

- [ ] **Step 1: Write failing tests**

Create `web/static/test/node-parser.test.mjs`:

```javascript
// web/static/test/node-parser.test.mjs
// Run with: node --test web/static/test/node-parser.test.mjs
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { layersToGraph } from '../node-parser.js';

test('empty layers returns empty graph', () => {
  const result = layersToGraph([]);
  assert.equal(result.ok, true);
  assert.deepEqual(result.chain, []);
  assert.deepEqual(result.nodes, []);
});

test('single image layer', () => {
  const layers = [{ type: 'image', file: 'bg.png', x: '0', y: '0' }];
  const result = layersToGraph(layers);
  assert.equal(result.ok, true);
  assert.equal(result.nodes.length, 1);
  assert.equal(result.chain.length, 1);
  const node = result.nodes[0];
  assert.equal(node.type, 'image');
  assert.equal(node.data.file, 'bg.png');
  assert.equal(node.data.x, '0');
  assert.equal(node.data.y, '0');
  assert.equal(result.chain[0], node.id);
});

test('chain of layers becomes chain', () => {
  const layers = [
    { type: 'image', file: 'bg.png' },
    { type: 'text', value: '{{zug1.zeit}}' },
  ];
  const result = layersToGraph(layers);
  assert.equal(result.ok, true);
  assert.equal(result.chain.length, 2);
  assert.equal(result.nodes[0].type, 'image');
  assert.equal(result.nodes[1].type, 'text');
  assert.deepEqual(result.chain, [result.nodes[0].id, result.nodes[1].id]);
});

test('loop layer with body layers', () => {
  const layers = [{
    type: 'loop',
    value: '{{zug1.via}}',
    split_by: '-',
    var: 'via_item',
    max_items: '5',
    layers: [{ type: 'text', value: '{{via_item}}' }],
  }];
  const result = layersToGraph(layers);
  assert.equal(result.ok, true);
  assert.equal(result.nodes.length, 2);
  const loopNode = result.nodes.find(n => n.type === 'loop');
  const textNode = result.nodes.find(n => n.type === 'text');
  assert.ok(loopNode);
  assert.ok(textNode);
  assert.equal(loopNode.data.loopValue, '{{zug1.via}}');
  assert.equal(loopNode.data.splitBy, '-');
  assert.equal(loopNode.data.varName, 'via_item');
  assert.equal(loopNode.data.maxItems, '5');
  assert.deepEqual(loopNode.bodyChain, [textNode.id]);
});

test('layer with if: locks (unsupported)', () => {
  const layers = [{ type: 'rect', if: 'greaterThan(abw, 0)', x: '0', y: '0' }];
  const result = layersToGraph(layers);
  assert.equal(result.ok, false);
  assert.ok(result.reason.includes('if'));
});

test('layer with conditional field value locks (unsupported)', () => {
  const layers = [{ type: 'rect', color: { if: 'equals(x,1)', then: '#F00', else: '#0F0' } }];
  const result = layersToGraph(layers);
  assert.equal(result.ok, false);
  assert.ok(result.reason.includes('if'));
});

test('nested loop locks (unsupported)', () => {
  const layers = [{
    type: 'loop',
    value: '{{a}}',
    layers: [{ type: 'loop', value: '{{b}}', layers: [] }],
  }];
  const result = layersToGraph(layers);
  assert.equal(result.ok, false);
  assert.ok(result.reason.toLowerCase().includes('loop'));
});

test('nodes get auto-positioned in a vertical stack', () => {
  const layers = [
    { type: 'image', file: 'a.png' },
    { type: 'text', value: 'hi' },
    { type: 'rect', x: '0', y: '0', width: '10', height: '10' },
  ];
  const result = layersToGraph(layers);
  assert.equal(result.ok, true);
  // Each node should be below the previous one
  const ys = result.nodes.map(n => n.canvasY);
  assert.ok(ys[1] > ys[0]);
  assert.ok(ys[2] > ys[1]);
  // X should be consistent
  assert.equal(result.nodes[0].canvasX, result.nodes[1].canvasX);
});
```

- [ ] **Step 2: Run tests — expect FAIL**

```bash
node --test web/static/test/node-parser.test.mjs
```

Expected: `Error [ERR_MODULE_NOT_FOUND]: Cannot find module '../node-parser.js'`

- [ ] **Step 3: Create node-parser.js**

```javascript
// web/static/node-parser.js
// Pure function: converts YAML layers array to graph data model.
// No external dependencies. Call layersToGraph(jsYaml.load(yamlStr).layers) in browser.
import { YAML_TO_DATA_KEY } from './node-types.js';

const NODE_WIDTH  = 220;
const NODE_HEIGHT = 120;  // estimated height for auto-layout
const NODE_GAP    = 24;
const CANVAS_START_X = 80;
const CANVAS_START_Y = 40;

let _idCounter = 1;
function newId() { return `n${_idCounter++}`; }

/**
 * Convert YAML layers array to graph.
 * @param {object[]} layers
 * @returns {{ ok: true, nodes: object[], chain: string[] }
 *          |{ ok: false, reason: string }}
 */
export function layersToGraph(layers) {
  _idCounter = 1;
  if (!Array.isArray(layers) || layers.length === 0) {
    return { ok: true, nodes: [], chain: [] };
  }

  const nodes = [];
  const chain = [];
  let y = CANVAS_START_Y;

  for (const layer of layers) {
    const check = checkSupported(layer, false);
    if (check) return { ok: false, reason: check };

    const { node, bodyNodes } = layerToNode(layer, CANVAS_START_X, y);
    nodes.push(node);
    nodes.push(...bodyNodes);
    chain.push(node.id);
    y += NODE_HEIGHT + NODE_GAP;
  }

  return { ok: true, nodes, chain };
}

/**
 * Returns an error string if the layer uses unsupported features, null otherwise.
 * @param {object} layer
 * @param {boolean} insideLoop - true when checking body layers
 */
function checkSupported(layer, insideLoop) {
  if (layer.if !== undefined) {
    return `Layer uses "if:" — edit in YAML tab (Layer-if not supported in Phase 1)`;
  }
  if (layer.type === 'loop' && insideLoop) {
    return `Nested loops are not supported in the node editor`;
  }
  // Check for conditional field values: {if, then, else} objects
  for (const [key, val] of Object.entries(layer)) {
    if (key === 'layers' || key === 'type') continue;
    if (val !== null && typeof val === 'object' && ('if' in val || 'then' in val)) {
      return `Field "${key}" uses conditional if/then/else — edit in YAML tab`;
    }
  }
  // Recurse into body layers for nested loop check
  if (layer.type === 'loop' && Array.isArray(layer.layers)) {
    for (const bodyLayer of layer.layers) {
      const err = checkSupported(bodyLayer, true);
      if (err) return err;
    }
  }
  return null;
}

function layerToNode(layer, x, y) {
  const fieldMap = YAML_TO_DATA_KEY[layer.type] || {};
  const data = {};
  for (const [yamlKey, dataKey] of Object.entries(fieldMap)) {
    if (layer[yamlKey] !== undefined) {
      data[dataKey] = String(layer[yamlKey]);
    }
  }

  if (layer.type === 'loop') {
    const bodyNodes = [];
    const bodyChain = [];
    let bodyY = y + 40;
    for (const bodyLayer of (layer.layers || [])) {
      const { node: bodyNode, bodyNodes: nested } = layerToNode(bodyLayer, x + 20, bodyY);
      bodyNodes.push(bodyNode, ...nested);
      bodyChain.push(bodyNode.id);
      bodyY += NODE_HEIGHT + NODE_GAP;
    }
    return {
      node: { id: newId(), type: 'loop', canvasX: x, canvasY: y, data, bodyChain },
      bodyNodes,
    };
  }

  return {
    node: { id: newId(), type: layer.type, canvasX: x, canvasY: y, data },
    bodyNodes: [],
  };
}
```

- [ ] **Step 4: Run tests — expect PASS**

```bash
node --test web/static/test/node-parser.test.mjs
```

Expected: all 8 tests pass.

- [ ] **Step 5: Commit**

```bash
git add web/static/node-parser.js web/static/test/node-parser.test.mjs
git commit -m "feat: add yaml-to-graph parser with tests"
```

---

## Task 5: Canvas — Init, Pan, Zoom

**Files:**
- Create: `web/static/node-editor.js`

- [ ] **Step 1: Create node-editor.js skeleton with pan/zoom**

```javascript
// web/static/node-editor.js
// Node editor canvas: pan, zoom, node rendering, drag, connections, context menu.
import { NODE_TYPES } from './node-types.js';
import { graphToLayers } from './node-serializer.js';

// ── State ──────────────────────────────────────────────────────────────────────
let _canvas = null;      // outer container div #node-canvas
let _viewport = null;    // #nc-viewport (transformed)
let _svg = null;         // #nc-svg (SVG overlay for connections)
let _graph = null;       // { nodes, chain }  — mutable during editing
let _fileList = [];      // image filenames (e.g. ['bg.png']) for dropdowns
let _fontIds  = [];      // font ids from template.yaml fonts section

// Pan/zoom state
let _panX = 40, _panY = 40, _zoom = 1;
let _panning = false, _panStart = null;

// ── Public API ─────────────────────────────────────────────────────────────────

/**
 * Initialize the node editor.
 * @param {HTMLElement} canvasEl  — #node-canvas
 * @param {HTMLElement} viewportEl — #nc-viewport
 * @param {SVGElement}  svgEl     — #nc-svg
 */
export function initCanvas(canvasEl, viewportEl, svgEl) {
  _canvas   = canvasEl;
  _viewport = viewportEl;
  _svg      = svgEl;
  _panX = 40; _panY = 40; _zoom = 1;
  _applyTransform();
  _canvas.addEventListener('mousedown', _onCanvasMouseDown);
  _canvas.addEventListener('wheel',     _onWheel, { passive: false });
  _canvas.addEventListener('contextmenu', _onContextMenu);
}

/** Load and render a graph. */
export function loadGraph(graph) {
  _graph = graph;
  _panX = 40; _panY = 40; _zoom = 1;
  _applyTransform();
  _renderAll();
}

/** Set available file names for file dropdowns. */
export function setFileList(files) {
  _fileList = files.filter(f => /\.(png|jpe?g)$/i.test(f));
}

/** Set font IDs for font dropdowns. */
export function setFontIds(ids) {
  _fontIds = ids;
}

/** Return current graph state (for serialization). */
export function getGraph() {
  return _graph;
}

// ── Transform ─────────────────────────────────────────────────────────────────

function _applyTransform() {
  _viewport.style.transform = `translate(${_panX}px,${_panY}px) scale(${_zoom})`;
}

// ── Pan ───────────────────────────────────────────────────────────────────────

function _onCanvasMouseDown(e) {
  if (e.target !== _canvas && e.target !== _viewport && e.target !== _svg) return;
  if (e.button !== 0) return;
  _panning = true;
  _panStart = { x: e.clientX - _panX, y: e.clientY - _panY };
  const onMove = ev => {
    if (!_panning) return;
    _panX = ev.clientX - _panStart.x;
    _panY = ev.clientY - _panStart.y;
    _applyTransform();
  };
  const onUp = () => {
    _panning = false;
    document.removeEventListener('mousemove', onMove);
    document.removeEventListener('mouseup', onUp);
  };
  document.addEventListener('mousemove', onMove);
  document.addEventListener('mouseup', onUp);
}

function _onWheel(e) {
  e.preventDefault();
  const delta = e.deltaY > 0 ? 0.9 : 1.1;
  const newZoom = Math.min(2, Math.max(0.3, _zoom * delta));
  // Zoom towards cursor
  const rect = _canvas.getBoundingClientRect();
  const cx = e.clientX - rect.left;
  const cy = e.clientY - rect.top;
  _panX = cx - (cx - _panX) * (newZoom / _zoom);
  _panY = cy - (cy - _panY) * (newZoom / _zoom);
  _zoom = newZoom;
  _applyTransform();
}

// ── Render all ────────────────────────────────────────────────────────────────

function _renderAll() {
  // Clear nodes (keep SVG)
  Array.from(_viewport.querySelectorAll('.ne-node')).forEach(el => el.remove());
  _svg.innerHTML = '';
  if (!_graph) return;

  // Render top-level nodes
  const nodeById = Object.fromEntries(_graph.nodes.map(n => [n.id, n]));
  for (const node of _graph.nodes) {
    // Body nodes are rendered inside their loop — skip them at top level
    const isBodyNode = _graph.nodes.some(n => n.bodyChain && n.bodyChain.includes(node.id));
    if (!isBodyNode) _renderNode(node, nodeById, _viewport);
  }
  _renderConnections();
}

// ── Context menu ──────────────────────────────────────────────────────────────

function _onContextMenu(e) {
  if (e.target !== _canvas && e.target !== _viewport && e.target !== _svg) return;
  e.preventDefault();
  const rect = _canvas.getBoundingClientRect();
  const cx = (e.clientX - rect.left - _panX) / _zoom;
  const cy = (e.clientY - rect.top  - _panY) / _zoom;
  _showContextMenu(e.clientX, e.clientY, cx, cy);
}

function _showContextMenu(screenX, screenY, canvasX, canvasY) {
  _hideContextMenu();
  const menu = document.createElement('div');
  menu.className = 'ne-context-menu';
  menu.id = 'ne-context-menu';
  menu.style.left = screenX + 'px';
  menu.style.top  = screenY + 'px';

  const groups = [
    { title: 'LAYER', types: ['image', 'rect', 'text', 'copy'] },
    { title: 'LOOP',  types: ['loop'] },
  ];

  for (const group of groups) {
    const groupEl = document.createElement('div');
    groupEl.className = 'ne-context-menu-group';
    const titleEl = document.createElement('div');
    titleEl.className = 'ne-context-menu-title';
    titleEl.textContent = group.title;
    groupEl.appendChild(titleEl);
    for (const type of group.types) {
      const btn = document.createElement('button');
      btn.className = 'ne-context-menu-item';
      btn.textContent = NODE_TYPES[type].label;
      btn.addEventListener('click', () => {
        _hideContextMenu();
        _addNode(type, canvasX, canvasY);
      });
      groupEl.appendChild(btn);
    }
    menu.appendChild(groupEl);
  }

  document.body.appendChild(menu);
  const hide = () => { _hideContextMenu(); document.removeEventListener('mousedown', hide); };
  setTimeout(() => document.addEventListener('mousedown', hide), 0);
}

function _hideContextMenu() {
  document.getElementById('ne-context-menu')?.remove();
}

function _addNode(type, canvasX, canvasY) {
  const id = `n${Date.now()}`;
  const node = {
    id, type, canvasX, canvasY, data: {},
    ...(type === 'loop' ? { bodyChain: [] } : {}),
  };
  _graph.nodes.push(node);
  _graph.chain.push(id);
  const nodeById = Object.fromEntries(_graph.nodes.map(n => [n.id, n]));
  _renderNode(node, nodeById, _viewport);
  _renderConnections();
}
```

- [ ] **Step 2: Verify no syntax errors**

```bash
node --input-type=module < web/static/node-editor.js 2>&1 | head -20
```

Expected: No output (module is valid — it just doesn't run because the imports are esm.sh CDN URLs which aren't available in Node.js, but syntax errors would show here).

Actually, the imports from `./node-types.js` and `./node-serializer.js` are local and will resolve. The check is just for syntax validity. The command might fail due to DOM globals — that's fine, we just want no syntax errors in the JS itself. If it outputs something about `document` that's expected.

---

## Task 6: Node Rendering with Inline Inputs

**Files:**
- Modify: `web/static/node-editor.js`

Add the `_renderNode` function to `node-editor.js`. Add after the `_addNode` function:

- [ ] **Step 1: Add _renderNode function**

```javascript
// ── Node rendering ────────────────────────────────────────────────────────────

function _renderNode(node, nodeById, parent) {
  const cfg = NODE_TYPES[node.type];
  if (!cfg) return;

  const el = document.createElement('div');
  el.className = 'ne-node';
  el.dataset.id = node.id;
  el.style.left = node.canvasX + 'px';
  el.style.top  = node.canvasY + 'px';

  // Input port (top)
  const portIn = document.createElement('div');
  portIn.className = 'ne-port-in';
  el.appendChild(portIn);

  // Header (drag handle)
  const header = document.createElement('div');
  header.className = 'ne-node-header';
  const dot = document.createElement('div');
  dot.className = 'ne-node-type-dot';
  dot.style.background = cfg.color;
  const label = document.createElement('span');
  label.className = 'ne-node-label';
  label.textContent = cfg.label;
  const delBtn = document.createElement('button');
  delBtn.className = 'ne-node-delete';
  delBtn.title = 'Node löschen';
  delBtn.textContent = '×';
  delBtn.addEventListener('click', e => {
    e.stopPropagation();
    _deleteNode(node.id);
  });
  header.appendChild(dot);
  header.appendChild(label);
  header.appendChild(delBtn);
  el.appendChild(header);

  // Body (fields)
  const body = document.createElement('div');
  body.className = 'ne-node-body';

  for (const field of cfg.fields) {
    const row = document.createElement('div');
    row.className = 'ne-field';
    const lbl = document.createElement('span');
    lbl.className = 'ne-field-label';
    lbl.textContent = field.label;
    row.appendChild(lbl);

    let input;
    if (field.inputType === 'text') {
      input = document.createElement('input');
      input.type = 'text';
      input.value = node.data[field.name] || '';
      input.addEventListener('input', () => { node.data[field.name] = input.value; });
    } else if (field.inputType === 'color') {
      input = document.createElement('input');
      input.type = 'color';
      // Color pickers need a 6-digit hex; use #000000 as fallback
      input.value = /^#[0-9a-fA-F]{6}$/.test(node.data[field.name] || '')
        ? node.data[field.name]
        : '#000000';
      input.addEventListener('input', () => { node.data[field.name] = input.value; });
    } else if (field.inputType === 'dropdown') {
      input = document.createElement('select');
      const options = field.options
        || (field.source === 'imageFiles' ? ['', ..._fileList]
          : field.source === 'fontIds'   ? ['', ..._fontIds]
          : ['']);
      for (const opt of options) {
        const o = document.createElement('option');
        o.value = opt; o.textContent = opt || '—';
        if (opt === (node.data[field.name] || '')) o.selected = true;
        input.appendChild(o);
      }
      input.addEventListener('change', () => { node.data[field.name] = input.value; });
    }

    row.appendChild(input);
    body.appendChild(row);
  }

  // Loop body chain (rendered inside loop node)
  if (node.type === 'loop' && node.bodyChain) {
    const bodyContainer = document.createElement('div');
    bodyContainer.className = 'ne-body-chain';
    bodyContainer.dataset.loopId = node.id;
    for (const childId of node.bodyChain) {
      const childNode = nodeById[childId];
      if (childNode) _renderNode(childNode, nodeById, bodyContainer);
    }
    body.appendChild(bodyContainer);
  }

  el.appendChild(body);

  // Output port (bottom)
  const portOut = document.createElement('div');
  portOut.className = 'ne-port-out';
  el.appendChild(portOut);

  // Node drag
  _makeDraggable(el, node);

  parent.appendChild(el);
  return el;
}

function _deleteNode(id) {
  _graph.nodes = _graph.nodes.filter(n => n.id !== id);
  _graph.chain = _graph.chain.filter(i => i !== id);
  // Also remove from any loop bodyChain
  for (const n of _graph.nodes) {
    if (n.bodyChain) n.bodyChain = n.bodyChain.filter(i => i !== id);
  }
  _renderAll();
}
```

- [ ] **Step 2: Verify nodes appear on canvas**

Open the editor, switch to NODES tab (after Task 9 integration — skip this verification step for now, revisit after Task 9).

---

## Task 7: SVG Connection Lines

**Files:**
- Modify: `web/static/node-editor.js`

- [ ] **Step 1: Add _renderConnections and _makeDraggable functions**

Add after `_deleteNode`:

```javascript
// ── Connections ───────────────────────────────────────────────────────────────

function _renderConnections() {
  _svg.innerHTML = '';
  if (!_graph) return;

  // Draw lines between consecutive nodes in chain
  for (let i = 0; i < _graph.chain.length - 1; i++) {
    const fromNode = _graph.nodes.find(n => n.id === _graph.chain[i]);
    const toNode   = _graph.nodes.find(n => n.id === _graph.chain[i + 1]);
    if (!fromNode || !toNode) continue;
    _drawConnection(fromNode, toNode);
  }
}

function _getPortPos(node, port) {
  // port: 'out' (bottom center) or 'in' (top center)
  const el = _viewport.querySelector(`.ne-node[data-id="${node.id}"]`);
  if (!el) {
    // Fallback: estimate from position
    const x = node.canvasX + 110;
    const y = port === 'out' ? node.canvasY + 120 : node.canvasY;
    return { x, y };
  }
  const x = node.canvasX + el.offsetWidth / 2;
  const y = port === 'out'
    ? node.canvasY + el.offsetHeight
    : node.canvasY;
  return { x, y };
}

function _drawConnection(fromNode, toNode) {
  const from = _getPortPos(fromNode, 'out');
  const to   = _getPortPos(toNode, 'in');

  // Cubic bezier with vertical handles
  const dy = Math.abs(to.y - from.y) * 0.5;
  const d = `M ${from.x} ${from.y} C ${from.x} ${from.y + dy}, ${to.x} ${to.y - dy}, ${to.x} ${to.y}`;

  const path = document.createElementNS('http://www.w3.org/2000/svg', 'path');
  path.setAttribute('d', d);
  path.setAttribute('fill', 'none');
  path.setAttribute('stroke', '#B8B0A8');
  path.setAttribute('stroke-width', '1.5');
  path.setAttribute('stroke-dasharray', '4,2');
  _svg.appendChild(path);

  // Arrowhead
  const arrow = document.createElementNS('http://www.w3.org/2000/svg', 'polygon');
  arrow.setAttribute('points', `${to.x},${to.y} ${to.x-4},${to.y-6} ${to.x+4},${to.y-6}`);
  arrow.setAttribute('fill', '#B8B0A8');
  _svg.appendChild(arrow);
}

// ── Node drag ─────────────────────────────────────────────────────────────────

function _makeDraggable(el, node) {
  const header = el.querySelector('.ne-node-header');
  header.addEventListener('mousedown', e => {
    if (e.button !== 0) return;
    e.stopPropagation();
    const startX = e.clientX;
    const startY = e.clientY;
    const origX = node.canvasX;
    const origY = node.canvasY;

    const onMove = ev => {
      node.canvasX = origX + (ev.clientX - startX) / _zoom;
      node.canvasY = origY + (ev.clientY - startY) / _zoom;
      el.style.left = node.canvasX + 'px';
      el.style.top  = node.canvasY + 'px';
      _renderConnections();
    };
    const onUp = () => {
      document.removeEventListener('mousemove', onMove);
      document.removeEventListener('mouseup', onUp);
    };
    document.addEventListener('mousemove', onMove);
    document.addEventListener('mouseup', onUp);
  });
}
```

---

## Task 8: Chain Reordering via Connection Drag

**Files:**
- Modify: `web/static/node-editor.js`

The user needs to be able to connect nodes in a different order by dragging from the output port of one node to the input port of another.

- [ ] **Step 1: Add port drag-to-connect**

Add `_initPortDrag` and call it from `_renderNode` after creating ports:

```javascript
// After the output port is created in _renderNode, add:
_initPortDrag(portOut, el, node);
```

Add the function:

```javascript
// ── Port drag-to-connect ──────────────────────────────────────────────────────

function _initPortDrag(portOutEl, nodeEl, fromNode) {
  portOutEl.addEventListener('mousedown', e => {
    e.stopPropagation();
    e.preventDefault();

    // Temporary SVG line
    const tmpLine = document.createElementNS('http://www.w3.org/2000/svg', 'line');
    tmpLine.setAttribute('stroke', '#FD7014');
    tmpLine.setAttribute('stroke-width', '1.5');
    tmpLine.setAttribute('stroke-dasharray', '4,2');
    _svg.appendChild(tmpLine);

    const canvasRect = _canvas.getBoundingClientRect();

    const onMove = ev => {
      const from = _getPortPos(fromNode, 'out');
      const tx = (ev.clientX - canvasRect.left - _panX) / _zoom;
      const ty = (ev.clientY - canvasRect.top  - _panY) / _zoom;
      tmpLine.setAttribute('x1', from.x); tmpLine.setAttribute('y1', from.y);
      tmpLine.setAttribute('x2', tx);     tmpLine.setAttribute('y2', ty);
    };

    const onUp = ev => {
      tmpLine.remove();
      document.removeEventListener('mousemove', onMove);
      document.removeEventListener('mouseup', onUp);

      // Find which node's input port was released over
      const target = document.elementFromPoint(ev.clientX, ev.clientY);
      const targetNodeEl = target?.closest('.ne-node');
      if (!targetNodeEl || targetNodeEl === nodeEl) return;
      const toId = targetNodeEl.dataset.id;
      const toNode = _graph.nodes.find(n => n.id === toId);
      if (!toNode) return;

      // Reorder chain: remove toId from current position, insert after fromNode
      const fromIdx = _graph.chain.indexOf(fromNode.id);
      _graph.chain = _graph.chain.filter(id => id !== toId);
      const newFromIdx = _graph.chain.indexOf(fromNode.id);
      _graph.chain.splice(newFromIdx + 1, 0, toId);
      _renderConnections();
    };

    document.addEventListener('mousemove', onMove);
    document.addEventListener('mouseup', onUp);
  });
}
```

---

## Task 9: Full Integration into edit-editor.html

**Files:**
- Modify: `web/templates/edit-editor.html`

This is the final integration step. The JS in `edit-editor.html` gets:
1. Tab switch logic (YAML ↔ NODES)
2. On switch to NODES: parse YAML → graph, load into canvas, show lock if unsupported
3. On switch to YAML: serialize graph → update CodeMirror doc
4. Save in NODES tab: serialize graph → save via `/edit/${TOKEN}/save`

- [ ] **Step 1: Add module imports and state to the existing script**

After the existing CDN imports in the `<script type="module">` block, add:

```javascript
import { initCanvas, loadGraph, getGraph, setFileList, setFontIds } from '/static/node-editor.js';
import { graphToLayers } from '/static/node-serializer.js';
import { layersToGraph } from '/static/node-parser.js';
import jsYaml from 'https://esm.sh/js-yaml@4.1.0';
```

- [ ] **Step 2: Add tab state variables after the existing state block**

```javascript
let activeTab    = 'yaml';   // 'yaml' | 'nodes'
let nodeCanvasInitialized = false;
```

- [ ] **Step 3: Add DOM refs for new elements**

```javascript
const tabSwitcher  = document.getElementById('tab-switcher');
const tabYaml      = document.getElementById('tab-yaml');
const tabNodes     = document.getElementById('tab-nodes');
const nodeCanvasWrap = document.getElementById('node-canvas-wrap');
const nodeCanvas   = document.getElementById('node-canvas');
const ncViewport   = document.getElementById('nc-viewport');
const ncSvg        = document.getElementById('nc-svg');
const nodeLock     = document.getElementById('node-lock');
const nodeLockReason = document.getElementById('node-lock-reason');
```

- [ ] **Step 4: Add tab switch handler**

```javascript
// ── Tab switching ─────────────────────────────────────────────────────────────
tabYaml.addEventListener('click',  () => switchTab('yaml'));
tabNodes.addEventListener('click', () => switchTab('nodes'));

function switchTab(tab) {
  if (activeTab === tab) return;

  if (tab === 'nodes') {
    // Serialize YAML editor → graph
    if (!switchToNodes()) return;  // stays on yaml if unsupported
  } else {
    // Serialize graph → YAML editor
    switchToYaml();
  }

  activeTab = tab;
  tabYaml.classList.toggle('tab-btn--active',  tab === 'yaml');
  tabNodes.classList.toggle('tab-btn--active', tab === 'nodes');
  cmHost.hidden          = tab !== 'yaml';
  nodeCanvasWrap.hidden  = tab !== 'nodes';
}

function switchToNodes() {
  const yamlStr = cmView ? cmView.state.doc.toString() : '';

  let parsed;
  try {
    parsed = jsYaml.load(yamlStr) || {};
  } catch (e) {
    nodeLock.hidden = false;
    nodeLockReason.textContent = 'YAML-Fehler: ' + e.message;
    nodeCanvasWrap.hidden = false;
    cmHost.hidden = true;
    return true;  // show the lock, still switch tab
  }

  const layers = parsed.layers || [];
  const result = layersToGraph(layers);

  // Initialize canvas on first use
  if (!nodeCanvasInitialized) {
    initCanvas(nodeCanvas, ncViewport, ncSvg);
    nodeCanvasInitialized = true;
    // Populate file + font lists
    _updateNodeEditorAssets(parsed);
  }

  if (!result.ok) {
    nodeLock.hidden = false;
    nodeLockReason.textContent = result.reason;
    nodeCanvasWrap.hidden = false;
    cmHost.hidden = true;
    return true;
  }

  nodeLock.hidden = true;
  loadGraph({ nodes: result.nodes, chain: result.chain });
  _updateNodeEditorAssets(parsed);
  return true;
}

function switchToYaml() {
  const graph = getGraph();
  if (!graph) return;

  const yamlStr = cmView ? cmView.state.doc.toString() : '';
  let parsed = {};
  try { parsed = jsYaml.load(yamlStr) || {}; } catch { /* ignore */ }

  const newLayers = graphToLayers(graph);
  const newDoc = jsYaml.dump({ ...parsed, layers: newLayers }, {
    indent: 2,
    lineWidth: -1,
    quotingType: '"',
    forceQuotes: false,
  });

  if (cmView) {
    cmView.dispatch({
      changes: { from: 0, to: cmView.state.doc.length, insert: newDoc }
    });
  }
}

function _updateNodeEditorAssets(parsed) {
  // Extract image filenames from current file list
  const imgFiles = Array.from(document.querySelectorAll('.file-item'))
    .map(li => li.dataset.name)
    .filter(n => n && /\.(png|jpe?g)$/i.test(n));
  setFileList(imgFiles);

  // Extract font IDs from parsed YAML
  const fontIds = (parsed.fonts || []).map(f => f.id).filter(Boolean);
  setFontIds(fontIds);
}
```

- [ ] **Step 5: Show/hide tab switcher based on open file**

In the existing `openFile` function, after setting `currentFile`, add:

```javascript
// Show tab switcher only for template.yaml
tabSwitcher.classList.toggle('tab-switcher--hidden', name !== 'template.yaml');
// If we switched away from template.yaml, go back to YAML tab
if (name !== 'template.yaml' && activeTab === 'nodes') {
  activeTab = 'yaml';
  tabYaml.classList.add('tab-btn--active');
  tabNodes.classList.remove('tab-btn--active');
  cmHost.hidden = false;
  nodeCanvasWrap.hidden = true;
}
```

- [ ] **Step 6: Adapt saveFile to handle NODES tab**

Modify the existing `saveFile` function: when in NODES tab, serialize graph to YAML first, then save:

```javascript
async function saveFile() {
  if (!cmView) return;

  // If NODES tab is active and graph is loaded, sync graph → YAML first
  if (activeTab === 'nodes' && getGraph()) {
    switchToYaml();
  }

  const content = cmView.state.doc.toString();
  const res = await fetch(`/edit/${TOKEN}/save`, {
    method:  'POST',
    headers: { 'Content-Type': 'application/json' },
    body:    JSON.stringify({ filename: currentFile, content }),
  });
  if (res.ok) {
    showSaveStatus('Gespeichert ✓', false);
    setTimeout(hideSaveStatus, 2500);
    schedulePreview();
  } else {
    showSaveStatus('Fehler beim Speichern.', true);
  }
}
```

- [ ] **Step 7: Manual integration test**

1. Open the editor for the default template (`GET /default/edit`).
2. Verify: YAML/NODES tab bar appears next to the file label.
3. Click NODES — the canvas should appear with nodes auto-laid out.
4. Drag a node to reposition it.
5. Right-click on canvas → context menu with IMAGE, RECT, TEXT, COPY, LOOP.
6. Click "IMAGE" → a new image node appears.
7. Click YAML tab → CodeMirror shows the updated YAML.
8. Open `default.json` (a non-YAML file) → tab switcher disappears.
9. Back to `template.yaml` → tab switcher reappears.
10. Click NODES → lock message should appear (the default template.yaml has `color: {if/then/else}`).

- [ ] **Step 8: Commit**

```bash
git add web/templates/edit-editor.html web/static/node-editor.js web/static/app.css
git commit -m "feat: integrate node editor canvas with tab switcher"
```

---

## Task 10: End-to-End Test with Simple Template

**Files:** None (verification only)

Create a simple template without conditional features to do a full round-trip test.

- [ ] **Step 1: Create a test template via the admin or file system**

Create `templates/test-node-editor/template.yaml`:

```yaml
meta:
  name: "Node Editor Test"
  description: "Einfaches Template zum Testen des Node-Editors"
  author: "test"
  version: "1.0"
  canvas:
    width: 160
    height: 80

fonts:
  - id: regular
    file: NimbusSanL-Reg.otf

layers:
  - type: image
    file: bg.png
    x: 0
    y: 0
  - type: text
    value: "{{zug1.zeit}}"
    x: 5
    y: 10
    font: regular
    size: 16
    color: "#000000"
    align: left
  - type: loop
    value: "{{zug1.via}}"
    split_by: "|"
    var: "item"
    max_items: 5
    layers:
      - type: text
        value: "{{item}}"
        x: 5
        y: "{{i * 12 + 30}}"
        font: regular
        size: 10
        color: "#888888"
```

- [ ] **Step 2: Open this template in the editor**

Navigate to `GET /test-node-editor/edit`. Click NODES.

Expected:
- 3 top-level nodes: IMAGE, TEXT, LOOP
- LOOP node shows its body (TEXT node inside it)
- Dashed connection lines between IMAGE → TEXT → LOOP

- [ ] **Step 3: Make a change in NODES tab**

Edit the `size` field of the TEXT node to `18`. Click YAML tab.

Expected: YAML shows `size: "18"` for the first text layer.

- [ ] **Step 4: Save**

Click Speichern. Expected: "Gespeichert ✓" status.

- [ ] **Step 5: Commit final state**

```bash
git add templates/test-node-editor/
git commit -m "feat: add test template for node editor round-trip verification"
```

---

## Self-Review

**Spec coverage check:**

| Spec requirement | Covered |
|---|---|
| Tab switcher YAML ↔ NODES | ✓ Task 1, 9 |
| Node-Canvas embedded in middle column | ✓ Task 1 |
| Right/left column unchanged | ✓ only middle column modified |
| YAML → Nodes best-effort + lock message | ✓ Task 4, 9 |
| Nodes → YAML always works | ✓ Task 3, 9 |
| image / rect / text / copy nodes | ✓ Task 2, 6 |
| loop node with body chain | ✓ Task 2, 3, 4, 6 |
| Inline inputs (text, dropdown, color) | ✓ Task 6 |
| Pan/zoom canvas | ✓ Task 5 |
| Node drag reposition | ✓ Task 7 |
| Connection lines | ✓ Task 7 |
| Context menu to add nodes | ✓ Task 5 |
| Connection drag to reorder chain | ✓ Task 8 |
| IBM Plex Mono font | ✓ via `--font-mono` in CSS |
| Color tokens match design | ✓ Task 1 CSS |
| No Rete.js / no build step | ✓ custom implementation |
| Save in NODES tab → YAML saved | ✓ Task 9 |
| Lock for nested loops | ✓ Task 4 |
| Lock for if/elif/else | ✓ Task 4 |
| Lock for conditional field values | ✓ Task 4 |

**Phases not covered (planned separately):**
- Phase 2: if-badge, elif-badge, else-badge on nodes; block container node
- Phase 3: filter pipeline chips; preview line with live test-JSON
