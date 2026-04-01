// web/static/node-serializer.js
// Pure function: converts graph data model to YAML layers array.
// No external dependencies — call jsYaml.dump(graphToLayers(graph)) in the browser.
import { YAML_FIELD_MAP, NODE_TYPES } from './node-types.js';

// Set of data-key names that must be serialized as numbers (not strings).
// Built from NODE_TYPES field definitions to stay in sync with node-types.js.
const NUMERIC_DATA_KEYS = new Set(
  Object.values(NODE_TYPES).flatMap(t =>
    t.fields.filter(f => f.numeric).map(f => f.name)
  )
);

function toYamlValue(dataKey, val) {
  if (NUMERIC_DATA_KEYS.has(dataKey)) {
    const n = Number(val);
    // Keep as string if not a plain number (e.g. template expressions like {{x}})
    return Number.isFinite(n) ? n : val;
  }
  return val;
}

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
      layer[yamlKey] = toYamlValue(dataKey, val);
    }
  }
  return layer;
}

function loopNodeToLayer(node, nodeById) {
  const layer = { type: 'loop' };
  const fieldMap = YAML_FIELD_MAP['loop'];
  for (const [dataKey, yamlKey] of Object.entries(fieldMap)) {
    const val = node.data[dataKey];
    if (val !== undefined && val !== '') {
      layer[yamlKey] = toYamlValue(dataKey, val);
    }
  }
  if (node.bodyChain && node.bodyChain.length > 0) {
    layer.layers = node.bodyChain
      .map(id => nodeById[id])
      .filter(Boolean)
      .map(n => nodeToLayer(n, nodeById));
  }
  return layer;
}
