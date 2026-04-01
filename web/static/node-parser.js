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
