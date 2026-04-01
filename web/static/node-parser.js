// web/static/node-parser.js
// Pure function: converts YAML layers array to graph data model.
// No external dependencies. Call layersToGraph(jsYaml.load(yamlStr).layers) in browser.
import { YAML_TO_DATA_KEY, NODE_TYPES } from './node-types.js';

const NODE_WIDTH  = 220;  // reserved for future horizontal layout
const NODE_GAP    = 32;
const CANVAS_START_X = 80;
const CANVAS_START_Y = 40;

// Header + body padding + port-dot overhead (constants match app.css)
const NODE_HEADER_H = 30;
const NODE_FIELD_H  = 28;  // field row incl. gap
const NODE_BODY_PAD = 22;

function nodeHeight(type) {
  const fields = NODE_TYPES[type]?.fields?.length ?? 4;
  return NODE_HEADER_H + NODE_BODY_PAD + fields * NODE_FIELD_H;
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
    const check = checkSupported(layer, false);
    if (check) return { ok: false, reason: check };

    const { node, bodyNodes } = layerToNode(layer, CANVAS_START_X, y, newId);
    nodes.push(node);
    nodes.push(...bodyNodes);
    chain.push(node.id);
    y += nodeHeight(layer.type) + NODE_GAP;
  }

  return { ok: true, nodes, chain };
}

/**
 * Returns an error string if the layer uses unsupported features, null otherwise.
 * @param {object} layer
 * @param {boolean} insideLoop - true when checking body layers
 */
function checkSupported(layer, insideLoop) {
  if (layer === null || typeof layer !== 'object') {
    return `Invalid layer (expected object, got ${layer === null ? 'null' : typeof layer})`;
  }

  const KNOWN_TYPES = new Set(['image', 'rect', 'text', 'copy', 'loop']);
  if (layer.type === undefined || layer.type === null) {
    return `Layer is missing a type field — edit in YAML tab`;
  }
  if (!KNOWN_TYPES.has(layer.type)) {
    return `Unknown layer type "${layer.type}" — edit in YAML tab`;
  }

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

function layerToNode(layer, x, y, newId) {
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

  return {
    node: { id: newId(), type: layer.type, canvasX: x, canvasY: y, data },
    bodyNodes: [],
  };
}
