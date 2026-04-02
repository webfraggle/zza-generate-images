// web/static/node-parser.js
// Pure function: converts YAML layers array to graph data model.
import { YAML_TO_DATA_KEY, NODE_TYPES } from './node-types.js';
import { parseValueAndFilters } from './node-filters.js';

const NODE_WIDTH  = 220;
const NODE_GAP    = 32;
const CANVAS_START_X = 80;
const CANVAS_START_Y = 40;

// Header + body padding + port-dot overhead (constants match app.css)
const NODE_HEADER_H = 30;
const NODE_FIELD_H  = 28;
const NODE_BODY_PAD = 22;

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
 * Convert YAML layers array to graph.
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

  const fieldMap   = YAML_TO_DATA_KEY[layer.type] || {};
  const typeFields = NODE_TYPES[layer.type]?.fields || [];
  const data       = {};

  for (const [yamlKey, dataKey] of Object.entries(fieldMap)) {
    if (layer[yamlKey] === undefined) continue;
    const fieldDef = typeFields.find(f => f.name === dataKey);
    const rawVal   = layer[yamlKey];

    if (fieldDef?.fieldIf && rawVal !== null && typeof rawVal === 'object' && 'if' in rawVal) {
      // Field-level if/then/else (e.g. color: {if, then, else})
      data[dataKey + 'If']   = String(rawVal.if   ?? '');
      data[dataKey + 'Then'] = String(rawVal.then ?? '');
      data[dataKey + 'Else'] = String(rawVal.else ?? '');
    } else if (fieldDef?.filterPipeline) {
      // Filter pipeline: split base + filters
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

  const node  = { id: newId(), type: layer.type, canvasX: x, canvasY: y, data };
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
