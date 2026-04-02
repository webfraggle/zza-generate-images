// web/static/node-editor.js
// Node editor canvas: pan, zoom, node rendering, drag, connections, context menu.
import { NODE_TYPES } from './node-types.js';
import { renderFilterRow } from './node-filters.js';

// ── Layout constants (mirror node-parser.js) ──────────────────────────────────
const NODE_WIDTH    = 220;
const NODE_GAP      = 32;
const CANVAS_ORIGIN_X = 80;
const CANVAS_ORIGIN_Y = 40;
const NODE_HEADER_H = 30;
const NODE_FIELD_H  = 28;
const NODE_BODY_PAD = 22;

function _nodeHeight(type) {
  if (type === 'block') return NODE_HEADER_H + NODE_BODY_PAD + NODE_FIELD_H;
  const typeDef = NODE_TYPES[type] || {};
  const fields  = typeDef.fields || [];
  // +1 for the layer-if row always rendered above the fields
  // +1 per filterPipeline field for the chip+preview row
  const extraRows = 1 + fields.filter(f => f.filterPipeline).length;
  return NODE_HEADER_H + NODE_BODY_PAD + (fields.length + extraRows) * NODE_FIELD_H;
}

// ── State ──────────────────────────────────────────────────────────────────────
let _canvas   = null;
let _viewport = null;
let _svg      = null;
let _graph    = null;   // { nodes, chain }
let _fileList = [];
let _fontIds  = [];
let _testJson = null;   // parsed JSON object for live filter preview

// Callbacks registered by field-chip UIs to refresh their preview on testJson change
const _previewRefreshers = new Set();

// Pan/zoom state
let _panX = 40, _panY = 40, _zoom = 1;
let _panning = false, _panStart = null;

// ── Public API ─────────────────────────────────────────────────────────────────

export function initCanvas(canvasEl, viewportEl, svgEl) {
  // Remove listeners from previous canvas if re-initializing
  if (_canvas) {
    _canvas.removeEventListener('mousedown', _onCanvasMouseDown);
    _canvas.removeEventListener('wheel', _onWheel);
    _canvas.removeEventListener('contextmenu', _onContextMenu);
  }
  _canvas   = canvasEl;
  _viewport = viewportEl;
  _svg      = svgEl;
  _panX = 40; _panY = 40; _zoom = 1;
  _applyTransform();
  _canvas.addEventListener('mousedown', _onCanvasMouseDown);
  _canvas.addEventListener('wheel',     _onWheel, { passive: false });
  _canvas.addEventListener('contextmenu', _onContextMenu);
}

export function loadGraph(graph) {
  _graph = graph;
  _panX = 40; _panY = 40; _zoom = 1;
  _applyTransform();
  _renderAll();
}

export function setFileList(files) {
  _fileList = files.filter(f => /\.(png|jpe?g)$/i.test(f));
}

export function setFontIds(ids) {
  _fontIds = [...ids];
}

export function getGraph() {
  return _graph;
}

export function setTestJson(jsonStr) {
  try {
    _testJson = jsonStr ? JSON.parse(jsonStr) : null;
  } catch {
    _testJson = null;
  }
  for (const refresh of _previewRefreshers) refresh();
}

export function getTestJson() {
  return _testJson;
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
  const rect = _canvas.getBoundingClientRect();
  const cx = e.clientX - rect.left;
  const cy = e.clientY - rect.top;
  _panX = cx - (cx - _panX) * (newZoom / _zoom);
  _panY = cy - (cy - _panY) * (newZoom / _zoom);
  _zoom = newZoom;
  _applyTransform();
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

  // BLOCK group
  const blockGroup = document.createElement('div');
  blockGroup.className = 'ne-context-menu-group';
  const blockTitle = document.createElement('div');
  blockTitle.className = 'ne-context-menu-title';
  blockTitle.textContent = 'BLOCK';
  blockGroup.appendChild(blockTitle);
  for (const blockType of ['if', 'elif', 'else']) {
    const btn = document.createElement('button');
    btn.className = 'ne-context-menu-item';
    btn.textContent = 'BLOCK-' + blockType.toUpperCase();
    btn.addEventListener('click', () => {
      _hideContextMenu();
      _addNode('block', canvasX, canvasY, { blockType, blockCond: '' });
    });
    blockGroup.appendChild(btn);
  }
  menu.appendChild(blockGroup);

  document.body.appendChild(menu);
  const hide = (ev) => {
    if (menu.contains(ev.target)) return;
    _hideContextMenu();
    document.removeEventListener('mousedown', hide);
  };
  setTimeout(() => document.addEventListener('mousedown', hide), 0);
}

function _hideContextMenu() {
  document.getElementById('ne-context-menu')?.remove();
}

// ── Node management ───────────────────────────────────────────────────────────

let _idCounter = 1;
function _addNode(type, canvasX, canvasY, extra = {}) {
  if (!_graph) return;
  const id = `n${Date.now()}_${_idCounter++}`;
  const node = {
    id, type, canvasX, canvasY, data: {},
    ...(type === 'loop'  ? { bodyChain: [] } : {}),
    ...(type === 'block' ? { bodyChain: [], blockType: extra.blockType || 'if', blockCond: extra.blockCond || '' } : {}),
  };
  _graph.nodes.push(node);
  _graph.chain.push(id);
  // Invariant: a node must be in exactly one of [chain] or a bodyChain.
  // Task 8 connection-drag must remove the node from chain when assigning it to a bodyChain.
  const nodeById = Object.fromEntries(_graph.nodes.map(n => [n.id, n]));
  _renderNode(node, nodeById, _viewport);
  _renderConnections();
}

function _deleteNode(id) {
  if (!_graph) return;
  _graph.nodes = _graph.nodes.filter(n => n.id !== id);
  _graph.chain = _graph.chain.filter(i => i !== id);
  for (const n of _graph.nodes) {
    if (n.bodyChain) n.bodyChain = n.bodyChain.filter(i => i !== id);
  }
  _renderAll();
}

// ── Render all ────────────────────────────────────────────────────────────────

function _renderAll() {
  Array.from(_viewport.querySelectorAll('.ne-node')).forEach(el => el.remove());
  _svg.innerHTML = '';
  _previewRefreshers.clear();
  if (!_graph) return;

  const nodeById    = Object.fromEntries(_graph.nodes.map(n => [n.id, n]));
  const bodyNodeIds = new Set(_graph.nodes.flatMap(n => n.bodyChain || []));

  for (const node of _graph.nodes) {
    const el = _renderNode(node, nodeById, _viewport);
    if (el && bodyNodeIds.has(node.id)) {
      el.classList.add('ne-node--body');
      _addEjectButton(el, node);
    }
  }
  // Defer until after browser layout so offsetHeight is available for auto-layout.
  requestAnimationFrame(() => { _autoLayout(false); });
}

// ── Auto-layout ───────────────────────────────────────────────────────────────

function _autoLayout(animate) {
  if (!_graph) return;
  const nodeById = Object.fromEntries(_graph.nodes.map(n => [n.id, n]));

  // Compute target positions — main chain horizontal, body chains vertical
  let x = CANVAS_ORIGIN_X;
  for (const id of _graph.chain) {
    const node = nodeById[id];
    if (!node) continue;
    node.canvasX = x;
    node.canvasY = CANVAS_ORIGIN_Y;

    if ((node.type === 'loop' || node.type === 'block') && node.bodyChain?.length) {
      const parentEl = _viewport.querySelector(`.ne-node[data-id="${node.id}"]`);
      const parentH  = parentEl ? parentEl.offsetHeight : _nodeHeight(node.type);
      let bodyY = CANVAS_ORIGIN_Y + parentH + NODE_GAP;
      for (const bodyId of node.bodyChain) {
        const bodyNode = nodeById[bodyId];
        if (!bodyNode) continue;
        bodyNode.canvasX = x;
        bodyNode.canvasY = bodyY;
        const bodyEl = _viewport.querySelector(`.ne-node[data-id="${bodyId}"]`);
        bodyY += (bodyEl ? bodyEl.offsetHeight : _nodeHeight(bodyNode.type)) + NODE_GAP;
      }
    }

    x += NODE_WIDTH + NODE_GAP;
  }

  // Apply positions to existing DOM elements
  for (const node of _graph.nodes) {
    const el = _viewport.querySelector(`.ne-node[data-id="${node.id}"]`);
    if (!el) continue;
    if (animate) el.style.transition = 'left 0.35s ease, top 0.35s ease';
    el.style.left = node.canvasX + 'px';
    el.style.top  = node.canvasY + 'px';
  }

  // Redraw connections — continuously during animation, then strip transition
  const duration = animate ? 380 : 0;
  const start = performance.now();
  const tick = () => {
    requestAnimationFrame(_renderConnections);
    if (performance.now() - start < duration) {
      requestAnimationFrame(tick);
    } else if (animate) {
      for (const node of _graph.nodes) {
        const el = _viewport.querySelector(`.ne-node[data-id="${node.id}"]`);
        if (el) el.style.transition = '';
      }
    }
  };
  requestAnimationFrame(tick);
}

function _renderNode(node, nodeById, parent) {
  const cfg = NODE_TYPES[node.type];
  if (!cfg) return;

  if (node.type === 'block') return _renderBlockNode(node, parent);

  const el = document.createElement('div');
  el.className = 'ne-node';
  if (node.type === 'loop') el.classList.add('ne-node--loop');
  el.dataset.id = node.id;
  el.style.left = node.canvasX + 'px';
  el.style.top  = node.canvasY + 'px';

  // Input port (top)
  const portIn = document.createElement('div');
  portIn.className = 'ne-port-in';
  el.appendChild(portIn);

  // Header (drag handle + type label + delete)
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

  // ── Layer-if badge row ───────────────────────────────────────────────────────
  const layerIfRow = document.createElement('div');
  layerIfRow.className = 'ne-field ne-field--layer-if';

  const layerIfSelect = document.createElement('select');
  layerIfSelect.className = 'ne-layer-if-select';
  for (const opt of ['—', 'if', 'elif', 'else']) {
    const o = document.createElement('option');
    o.value = opt === '—' ? '' : opt;
    o.textContent = opt;
    if ((node.layerIfType || '') === (opt === '—' ? '' : opt)) o.selected = true;
    layerIfSelect.appendChild(o);
  }

  const layerIfInput = document.createElement('input');
  layerIfInput.type = 'text';
  layerIfInput.className = 'ne-layer-if-cond';
  layerIfInput.placeholder = 'Bedingung…';
  layerIfInput.value = node.layerIfCond || '';
  layerIfInput.style.display = node.layerIfType && node.layerIfType !== 'else' ? '' : 'none';

  layerIfSelect.addEventListener('change', () => {
    const v = layerIfSelect.value;
    node.layerIfType = v || undefined;
    node.layerIfCond = v ? (node.layerIfCond || '') : undefined;
    layerIfInput.style.display = v && v !== 'else' ? '' : 'none';
  });
  layerIfInput.addEventListener('input', () => { node.layerIfCond = layerIfInput.value; });

  layerIfRow.appendChild(layerIfSelect);
  layerIfRow.appendChild(layerIfInput);
  body.appendChild(layerIfRow);

  // ── Fields ───────────────────────────────────────────────────────────────────
  for (const field of cfg.fields) {
    if (field.fieldIf) {
      _renderFieldIfRow(body, node, field);
      continue;
    }

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
      input.addEventListener('input', () => {
        node.data[field.name] = input.value;
        if (field.filterPipeline) refresher();
      });
    } else if (field.inputType === 'dropdown') {
      input = document.createElement('select');
      const options = [...(field.options
        || (field.source === 'imageFiles' ? ['', ..._fileList]
          : field.source === 'fontIds'   ? ['', ..._fontIds]
          : ['']))];
      const storedVal = node.data[field.name] || '';
      const storedOk = field.source === 'imageFiles'
        ? /\.(png|jpe?g)$/i.test(storedVal)
        : true;
      if (storedVal && storedOk && !options.includes(storedVal)) options.push(storedVal);
      for (const opt of options) {
        const o = document.createElement('option');
        o.value = opt;
        o.textContent = opt || '—';
        if (opt === storedVal) o.selected = true;
        input.appendChild(o);
      }
      input.addEventListener('change', () => { node.data[field.name] = input.value; });
    }

    if (input) row.appendChild(input);
    body.appendChild(row);

    // Filter pipeline chips + preview row
    if (field.filterPipeline) {
      const filterContainer = document.createElement('div');
      filterContainer.className = 'ne-field-filter-row';
      let _latestPreview = () => {};
      _previewRefreshers.add(() => _latestPreview());

      function refresher() { _latestPreview(); }

      function renderFilters() {
        const filters = node.data[field.name + '_filters'] || [];
        const result = renderFilterRow(
          filterContainer,
          () => node.data[field.name] || '',
          filters,
          (newFilters) => {
            node.data[field.name + '_filters'] = newFilters;
            renderFilters();
          },
          () => _testJson
        );
        _latestPreview = result.updatePreview;
      }
      renderFilters();
      body.appendChild(filterContainer);
    }
  }

  el.appendChild(body);

  // Output port (bottom)
  const portOut = document.createElement('div');
  portOut.className = 'ne-port-out';
  el.appendChild(portOut);

  _makeDraggable(el, node);
  _initPortDrag(portOut, el, node);

  parent.appendChild(el);
  return el;
}

function _renderBlockNode(node, parent) {
  const cfg = NODE_TYPES['block'];

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
  dot.style.background = cfg.color;

  const label = document.createElement('span');
  label.className = 'ne-node-label';
  label.textContent = cfg.label;

  const badge = document.createElement('span');
  badge.className = 'ne-block-badge ne-block-badge--' + node.blockType;
  badge.textContent = node.blockType.toUpperCase();

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
  header.appendChild(badge);
  header.appendChild(delBtn);
  el.appendChild(header);

  // Body: condition input (not for 'else')
  const body = document.createElement('div');
  body.className = 'ne-node-body';

  const row = document.createElement('div');
  row.className = 'ne-field';
  const lbl = document.createElement('span');
  lbl.className = 'ne-field-label';
  lbl.textContent = 'cond';
  row.appendChild(lbl);

  if (node.blockType !== 'else') {
    const input = document.createElement('input');
    input.type = 'text';
    input.value = node.blockCond || '';
    input.addEventListener('input', () => { node.blockCond = input.value; });
    row.appendChild(input);
  }
  body.appendChild(row);
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

/**
 * Renders a fieldIf row (toggle: plain color vs if/then/else mode).
 * Used for fields with `fieldIf: true` (e.g. color).
 */
function _renderFieldIfRow(body, node, field) {
  const dk = field.name; // data key e.g. 'color'

  // Outer wrapper that we re-render in place
  const wrapper = document.createElement('div');
  wrapper.className = 'ne-field-if-wrapper';

  function render() {
    wrapper.innerHTML = '';
    const isIfMode = (dk + 'If') in node.data;

    if (!isIfMode) {
      // Simple mode: label + color input + toggle button
      const row = document.createElement('div');
      row.className = 'ne-field';
      const lbl = document.createElement('span');
      lbl.className = 'ne-field-label';
      lbl.textContent = field.label;
      row.appendChild(lbl);

      const colorInput = document.createElement('input');
      colorInput.type = 'color';
      colorInput.value = /^#[0-9a-fA-F]{6}$/.test(node.data[dk] || '')
        ? node.data[dk]
        : '#000000';
      colorInput.addEventListener('input', () => { node.data[dk] = colorInput.value; });
      row.appendChild(colorInput);

      const toggleBtn = document.createElement('button');
      toggleBtn.type = 'button';
      toggleBtn.className = 'ne-field-if-toggle';
      toggleBtn.title = 'Bedingung hinzufügen';
      toggleBtn.textContent = 'if';
      toggleBtn.addEventListener('click', () => {
        node.data[dk + 'If']   = '';
        node.data[dk + 'Then'] = node.data[dk] || '#000000';
        node.data[dk + 'Else'] = '#000000';
        delete node.data[dk];
        render();
        _autoLayout(true);
      });
      row.appendChild(toggleBtn);
      wrapper.appendChild(row);
    } else {
      // If/then/else mode: three rows
      const ifRow = document.createElement('div');
      ifRow.className = 'ne-field';
      const ifLbl = document.createElement('span');
      ifLbl.className = 'ne-field-label';
      ifLbl.textContent = field.label + ' if';
      ifRow.appendChild(ifLbl);
      const ifInput = document.createElement('input');
      ifInput.type = 'text';
      ifInput.value = node.data[dk + 'If'] || '';
      ifInput.placeholder = 'Bedingung…';
      ifInput.addEventListener('input', () => { node.data[dk + 'If'] = ifInput.value; });
      ifRow.appendChild(ifInput);

      const clearBtn = document.createElement('button');
      clearBtn.type = 'button';
      clearBtn.className = 'ne-field-if-toggle ne-field-if-toggle--clear';
      clearBtn.title = 'Bedingung entfernen';
      clearBtn.textContent = '×';
      clearBtn.addEventListener('click', () => {
        node.data[dk] = node.data[dk + 'Then'] || '#000000';
        delete node.data[dk + 'If'];
        delete node.data[dk + 'Then'];
        delete node.data[dk + 'Else'];
        render();
        _autoLayout(true);
      });
      ifRow.appendChild(clearBtn);
      wrapper.appendChild(ifRow);

      for (const sub of ['Then', 'Else']) {
        const subRow = document.createElement('div');
        subRow.className = 'ne-field ne-field--sub';
        const subLbl = document.createElement('span');
        subLbl.className = 'ne-field-label';
        subLbl.textContent = sub.toLowerCase();
        subRow.appendChild(subLbl);
        const subInput = document.createElement('input');
        subInput.type = 'color';
        subInput.value = /^#[0-9a-fA-F]{6}$/.test(node.data[dk + sub] || '')
          ? node.data[dk + sub]
          : '#000000';
        subInput.addEventListener('input', () => { node.data[dk + sub] = subInput.value; });
        subRow.appendChild(subInput);
        wrapper.appendChild(subRow);
      }
    }
  }

  render();
  body.appendChild(wrapper);
}

function _renderConnections() {
  _svg.innerHTML = '';
  if (!_graph) return;
  const nodeById = Object.fromEntries(_graph.nodes.map(n => [n.id, n]));

  // Main chain: bottom-out → top-in, grey
  for (let i = 0; i < _graph.chain.length - 1; i++) {
    const fromNode = nodeById[_graph.chain[i]];
    const toNode   = nodeById[_graph.chain[i + 1]];
    if (!fromNode || !toNode) continue;
    _drawConnection(fromNode, toNode, '#B8B0A8');
  }

  // Loop/Block body circuit: parent right-out → body[0] → … → body[n] → parent right-in
  for (const node of _graph.nodes) {
    if ((node.type !== 'loop' && node.type !== 'block') || !node.bodyChain?.length) continue;
    const color = node.type === 'block' ? '#FD7014' : '#C83232';
    const firstBody = nodeById[node.bodyChain[0]];
    const lastBody  = nodeById[node.bodyChain[node.bodyChain.length - 1]];
    if (firstBody) _drawLoopBodyEntry(node, firstBody, color);
    for (let i = 0; i < node.bodyChain.length - 1; i++) {
      const a = nodeById[node.bodyChain[i]];
      const b = nodeById[node.bodyChain[i + 1]];
      if (a && b) _drawBodyBodyConnection(a, b, color);
    }
    if (lastBody) _drawLoopBodyReturn(node, lastBody, color);
  }
}

function _getPortPos(node, port) {
  const el = _viewport.querySelector(`.ne-node[data-id="${node.id}"]`);
  const w  = el ? el.offsetWidth  : NODE_WIDTH;
  const h  = el ? el.offsetHeight : 120;
  // Body nodes: top (in) / bottom (out)
  if (_graph?.nodes.some(n => n.bodyChain?.includes(node.id))) {
    return {
      x: node.canvasX + w / 2,
      y: port === 'out' ? node.canvasY + h : node.canvasY,
    };
  }
  // Main chain nodes: left (in) / right (out)
  return {
    x: port === 'out' ? node.canvasX + w : node.canvasX,
    y: node.canvasY + h / 2,
  };
}

function _drawConnection(fromNode, toNode, color) {
  const from = _getPortPos(fromNode, 'out');
  const to   = _getPortPos(toNode, 'in');
  const dx   = Math.abs(to.x - from.x) * 0.5;
  _svgPath(`M ${from.x} ${from.y} C ${from.x + dx} ${from.y}, ${to.x - dx} ${to.y}, ${to.x} ${to.y}`, color);
  _svgArrowRight(to.x, to.y, color);
}

function _svgPath(d, color) {
  const path = document.createElementNS('http://www.w3.org/2000/svg', 'path');
  path.setAttribute('d', d);
  path.setAttribute('fill', 'none');
  path.setAttribute('stroke', color);
  path.setAttribute('stroke-width', '1.5');
  path.setAttribute('stroke-dasharray', '4,2');
  _svg.appendChild(path);
}

function _svgArrowDown(x, y, color) {
  const arrow = document.createElementNS('http://www.w3.org/2000/svg', 'polygon');
  arrow.setAttribute('points', `${x},${y} ${x-4},${y-6} ${x+4},${y-6}`);
  arrow.setAttribute('fill', color);
  _svg.appendChild(arrow);
}

function _svgArrowRight(x, y, color) {
  const arrow = document.createElementNS('http://www.w3.org/2000/svg', 'polygon');
  arrow.setAttribute('points', `${x},${y} ${x-6},${y-4} ${x-6},${y+4}`);
  arrow.setAttribute('fill', color);
  _svg.appendChild(arrow);
}

// Parent bottom-center → first body node top-center
function _drawLoopBodyEntry(loopNode, bodyNode, color = '#C83232') {
  const loopEl = _viewport.querySelector(`.ne-node[data-id="${loopNode.id}"]`);
  const loopW  = loopEl ? loopEl.offsetWidth  : 220;
  const loopH  = loopEl ? loopEl.offsetHeight : 120;
  const bodyEl = _viewport.querySelector(`.ne-node[data-id="${bodyNode.id}"]`);
  const bodyW  = bodyEl ? bodyEl.offsetWidth  : 220;

  const fromX = loopNode.canvasX + loopW / 2;
  const fromY = loopNode.canvasY + loopH;
  const toX   = bodyNode.canvasX + bodyW / 2;
  const toY   = bodyNode.canvasY;

  const dy = Math.abs(toY - fromY) * 0.5;
  const d  = `M ${fromX} ${fromY} C ${fromX} ${fromY + dy}, ${toX} ${toY - dy}, ${toX} ${toY}`;
  _svgPath(d, color);
  _svgArrowDown(toX, toY, color);
}

// Last body node right-center → parent right-center, arcing right
function _drawLoopBodyReturn(loopNode, lastBodyNode, color = '#C83232') {
  const loopEl  = _viewport.querySelector(`.ne-node[data-id="${loopNode.id}"]`);
  const bodyEl  = _viewport.querySelector(`.ne-node[data-id="${lastBodyNode.id}"]`);
  const loopW   = loopEl ? loopEl.offsetWidth  : 220;
  const loopH   = loopEl ? loopEl.offsetHeight : 120;
  const bodyW   = bodyEl ? bodyEl.offsetWidth  : 220;
  const bodyH   = bodyEl ? bodyEl.offsetHeight : 120;

  const fromX = lastBodyNode.canvasX + bodyW;
  const fromY = lastBodyNode.canvasY + bodyH / 2;
  const toX   = loopNode.canvasX + loopW;
  const toY   = loopNode.canvasY + loopH * 0.65;

  // Arc to the right: both control points extend rightward
  const arc = Math.max(50, bodyW * 0.4);
  const d = `M ${fromX} ${fromY} C ${fromX + arc} ${fromY}, ${toX + arc} ${toY}, ${toX} ${toY}`;
  _svgPath(d, color);
  // Arrowhead pointing left (←)
  const arrow = document.createElementNS('http://www.w3.org/2000/svg', 'polygon');
  arrow.setAttribute('points', `${toX},${toY} ${toX+6},${toY-4} ${toX+6},${toY+4}`);
  arrow.setAttribute('fill', color);
  _svg.appendChild(arrow);
}

// Vertical bottom-center → top-center connector between adjacent body nodes
function _drawBodyBodyConnection(fromNode, toNode, color = '#C83232') {
  const fromEl = _viewport.querySelector(`.ne-node[data-id="${fromNode.id}"]`);
  const fromW  = fromEl ? fromEl.offsetWidth  : 220;
  const fromH  = fromEl ? fromEl.offsetHeight : 120;
  const toEl   = _viewport.querySelector(`.ne-node[data-id="${toNode.id}"]`);
  const toW    = toEl   ? toEl.offsetWidth    : 220;

  const fromX = fromNode.canvasX + fromW / 2;
  const fromY = fromNode.canvasY + fromH;
  const toX   = toNode.canvasX + toW / 2;
  const toY   = toNode.canvasY;

  const dy = Math.abs(toY - fromY) * 0.4;
  _svgPath(`M ${fromX} ${fromY} C ${fromX} ${fromY + dy}, ${toX} ${toY - dy}, ${toX} ${toY}`, color);
  _svgArrowDown(toX, toY, color);
}

function _addEjectButton(el, node) {
  const header = el.querySelector('.ne-node-header');
  if (!header) return;
  const btn = document.createElement('button');
  btn.type = 'button';
  btn.className = 'ne-node-eject';
  btn.title = 'Aus Block/Loop entfernen';
  btn.textContent = '↑';
  btn.addEventListener('click', e => {
    e.stopPropagation();
    const parent = _graph.nodes.find(n => n.bodyChain?.includes(node.id));
    if (!parent) return;
    parent.bodyChain = parent.bodyChain.filter(id => id !== node.id);
    const parentIdx = _graph.chain.indexOf(parent.id);
    _graph.chain.splice(parentIdx >= 0 ? parentIdx + 1 : _graph.chain.length, 0, node.id);
    _renderAll();
  });
  const delBtn = header.querySelector('.ne-node-delete');
  if (delBtn) header.insertBefore(btn, delBtn);
  else header.appendChild(btn);
}

function _makeDraggable(el, node) {
  const header = el.querySelector('.ne-node-header');
  if (!header) return;
  header.addEventListener('mousedown', e => {
    if (e.button !== 0) return;
    e.stopPropagation();
    e.preventDefault();
    document.body.style.cursor = 'grabbing';
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
      document.body.style.cursor = '';
      document.removeEventListener('mousemove', onMove);
      document.removeEventListener('mouseup', onUp);
    };
    document.addEventListener('mousemove', onMove);
    document.addEventListener('mouseup', onUp);
  });
}
function _initPortDrag(portOutEl, nodeEl, fromNode) {
  portOutEl.addEventListener('mousedown', e => {
    if (e.button !== 0) return;
    e.stopPropagation();
    e.preventDefault();

    // Temporary dashed line during drag
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

      // Find which node's element is under the cursor
      const target = document.elementFromPoint(ev.clientX, ev.clientY);
      const targetNodeEl = target?.closest('.ne-node');
      if (!targetNodeEl || targetNodeEl === nodeEl) return;
      const toId = targetNodeEl.dataset.id;
      if (!toId || !_graph) return;

      // Is fromNode a body node? → reorder within its parent's bodyChain
      const fromParent = _graph.nodes.find(n => n.bodyChain?.includes(fromNode.id));
      if (fromParent) {
        if (!fromParent.bodyChain.includes(toId)) return; // only reorder within same parent
        fromParent.bodyChain = fromParent.bodyChain.filter(id => id !== toId);
        const newFromIdx = fromParent.bodyChain.indexOf(fromNode.id);
        fromParent.bodyChain.splice(newFromIdx + 1, 0, toId);
        _autoLayout(true);
        return;
      }

      // Main chain drag: drop onto block/loop → add fromNode to its bodyChain
      const toNode = _graph.nodes.find(n => n.id === toId);
      if (toNode && (toNode.type === 'block' || toNode.type === 'loop')) {
        if (!toNode.bodyChain) toNode.bodyChain = [];
        if (!toNode.bodyChain.includes(fromNode.id)) {
          _graph.chain = _graph.chain.filter(id => id !== fromNode.id);
          toNode.bodyChain.push(fromNode.id);
          _renderAll();
        }
        return;
      }

      // Prevent dropping onto a body node
      const isBodyNode = _graph.nodes.some(n => n.bodyChain?.includes(toId));
      if (isBodyNode) return;

      // Reorder: remove toId from chain, insert after fromNode
      const fromIdx = _graph.chain.indexOf(fromNode.id);
      if (fromIdx === -1) return;
      _graph.chain = _graph.chain.filter(id => id !== toId);
      const newFromIdx = _graph.chain.indexOf(fromNode.id);
      _graph.chain.splice(newFromIdx + 1, 0, toId);
      _autoLayout(true);
    };

    document.addEventListener('mousemove', onMove);
    document.addEventListener('mouseup', onUp);
  });
}
