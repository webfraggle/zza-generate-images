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
  const fieldMap = YAML_FIELD_MAP['loop'];
  for (const [dataKey, yamlKey] of Object.entries(fieldMap)) {
    const val = node.data[dataKey];
    if (val !== undefined && val !== '') {
      layer[yamlKey] = val;
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
