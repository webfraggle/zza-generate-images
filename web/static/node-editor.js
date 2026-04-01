// web/static/node-editor.js
// Node editor canvas: pan, zoom, node rendering, drag, connections, context menu.
import { NODE_TYPES } from './node-types.js';

// ── State ──────────────────────────────────────────────────────────────────────
let _canvas   = null;
let _viewport = null;
let _svg      = null;
let _graph    = null;   // { nodes, chain }
let _fileList = [];
let _fontIds  = [];

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

  document.body.appendChild(menu);
  const hide = () => { _hideContextMenu(); document.removeEventListener('mousedown', hide); };
  setTimeout(() => document.addEventListener('mousedown', hide), 0);
}

function _hideContextMenu() {
  document.getElementById('ne-context-menu')?.remove();
}

// ── Node management ───────────────────────────────────────────────────────────

function _addNode(type, canvasX, canvasY) {
  if (!_graph) return;
  const id = `n${Date.now()}`;
  const node = {
    id, type, canvasX, canvasY, data: {},
    ...(type === 'loop' ? { bodyChain: [] } : {}),
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
  if (!_graph) return;

  const nodeById = Object.fromEntries(_graph.nodes.map(n => [n.id, n]));
  const bodyNodeIds = new Set(_graph.nodes.flatMap(n => n.bodyChain || []));

  for (const node of _graph.nodes) {
    if (!bodyNodeIds.has(node.id)) {
      _renderNode(node, nodeById, _viewport);
    }
  }
  // Defer until after browser layout so offsetWidth/offsetHeight are available.
  requestAnimationFrame(_renderConnections);
}

// ── Stubs (filled by Tasks 6, 7, 8) ──────────────────────────────────────────

function _renderNode(node, nodeById, parent) {
  const cfg = NODE_TYPES[node.type];
  if (!cfg) return;

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
      input.value = /^#[0-9a-fA-F]{6}$/.test(node.data[field.name] || '')
        ? node.data[field.name]
        : '#000000';
      input.addEventListener('input', () => { node.data[field.name] = input.value; });
    } else if (field.inputType === 'dropdown') {
      input = document.createElement('select');
      const options = [...(field.options
        || (field.source === 'imageFiles' ? ['', ..._fileList]
          : field.source === 'fontIds'   ? ['', ..._fontIds]
          : ['']))];
      // If stored value is not in options, add it so the display matches data
      const storedVal = node.data[field.name] || '';
      if (storedVal && !options.includes(storedVal)) {
        options.push(storedVal);
      }
      for (const opt of options) {
        const o = document.createElement('option');
        o.value = opt;
        o.textContent = opt || '—';
        if (opt === (node.data[field.name] || '')) o.selected = true;
        input.appendChild(o);
      }
      input.addEventListener('change', () => { node.data[field.name] = input.value; });
    }

    if (input) row.appendChild(input);
    body.appendChild(row);
  }

  // Loop body sub-chain (rendered inside loop node)
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

  // Wire drag + port-drag (stubs for now, wired in Tasks 7 & 8)
  _makeDraggable(el, node);
  // _initPortDrag is a stub (Task 8) — the call is pre-wired here so Task 8
  // only needs to implement the function body, not find the call site.
  _initPortDrag(portOut, el, node);

  parent.appendChild(el);
  return el;
}
function _renderConnections() {
  _svg.innerHTML = '';
  if (!_graph) return;
  const nodeById = Object.fromEntries(_graph.nodes.map(n => [n.id, n]));
  for (let i = 0; i < _graph.chain.length - 1; i++) {
    const fromNode = nodeById[_graph.chain[i]];
    const toNode   = nodeById[_graph.chain[i + 1]];
    if (!fromNode || !toNode) continue;
    _drawConnection(fromNode, toNode);
  }
}

function _getPortPos(node, port) {
  const el = _viewport.querySelector(`.ne-node[data-id="${node.id}"]`);
  if (!el) {
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

      // Guard: prevent dropping onto a body-chain node (would violate chain invariant)
      const isBodyNode = _graph.nodes.some(n => n.bodyChain?.includes(toId));
      if (isBodyNode) return;

      // Reorder: remove toId from chain, insert after fromNode
      const fromIdx = _graph.chain.indexOf(fromNode.id);
      if (fromIdx === -1) return;
      _graph.chain = _graph.chain.filter(id => id !== toId);
      const newFromIdx = _graph.chain.indexOf(fromNode.id);
      _graph.chain.splice(newFromIdx + 1, 0, toId);
      _renderConnections();
    };

    document.addEventListener('mousemove', onMove);
    document.addEventListener('mouseup', onUp);
  });
}
