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
  _renderConnections();
}

// ── Stubs (filled by Tasks 6, 7, 8) ──────────────────────────────────────────

// eslint-disable-next-line no-unused-vars
function _renderNode(node, nodeById, parent) { /* Task 6 */ }
// eslint-disable-next-line no-unused-vars
function _renderConnections() { /* Task 7 */ }
// eslint-disable-next-line no-unused-vars
function _makeDraggable(el, node) { /* Task 7 */ }
// eslint-disable-next-line no-unused-vars
function _initPortDrag(portOutEl, nodeEl, fromNode) { /* Task 8 */ }
