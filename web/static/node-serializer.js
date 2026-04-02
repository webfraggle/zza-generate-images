// web/static/node-serializer.js
// Pure function: converts graph data model to YAML layers array.
// No external dependencies — call jsYaml.dump(graphToLayers(graph)) in the browser.
import { YAML_FIELD_MAP, NODE_TYPES } from './node-types.js';
import { formatValueWithFilters } from './node-filters.js';

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
  if (node.type === 'loop')  return loopNodeToLayer(node, nodeById);
  if (node.type === 'block') return blockNodeToLayer(node, nodeById);

  const layer = { type: node.type };
  const fieldMap  = YAML_FIELD_MAP[node.type] || {};
  const typeDef   = NODE_TYPES[node.type] || {};
  const fieldDefs = typeDef.fields || [];

  for (const [dataKey, yamlKey] of Object.entries(fieldMap)) {
    const fieldDef = fieldDefs.find(f => f.name === dataKey);

    if (fieldDef?.fieldIf) {
      // Field-level if/then/else: colorIf/colorThen/colorElse → {if, then, else}
      const ifVal   = node.data[dataKey + 'If'];
      const thenVal = node.data[dataKey + 'Then'];
      const elseVal = node.data[dataKey + 'Else'];
      if (ifVal !== undefined && ifVal !== '') {
        layer[yamlKey] = { if: ifVal, then: thenVal ?? '', else: elseVal ?? '' };
      } else {
        const val = node.data[dataKey];
        if (val !== undefined && val !== '') layer[yamlKey] = val;
      }
    } else if (fieldDef?.filterPipeline) {
      // Filter pipeline: compose base + filters → "{{expr | f1 | f2(arg)}}"
      const base    = node.data[dataKey];
      const filters = node.data[dataKey + '_filters'] || [];
      if (base !== undefined && base !== '') {
        layer[yamlKey] = formatValueWithFilters(base, filters);
      }
    } else {
      const val = node.data[dataKey];
      if (val !== undefined && val !== '') {
        layer[yamlKey] = toYamlValue(dataKey, val);
      }
    }
  }

  // Layer-level if/elif/else badge
  if (node.layerIfType === 'if')   layer.if   = node.layerIfCond;
  if (node.layerIfType === 'elif') layer.elif  = node.layerIfCond;
  if (node.layerIfType === 'else') layer.else  = true;

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

function blockNodeToLayer(node, nodeById) {
  const bodyLayers = (node.bodyChain || [])
    .map(id => nodeById[id])
    .filter(Boolean)
    .map(n => nodeToLayer(n, nodeById));

  if (node.blockType === 'if') {
    return { block: node.blockCond, layers: bodyLayers };
  }
  if (node.blockType === 'elif') {
    return { elif: node.blockCond, layers: bodyLayers };
  }
  // else
  return { else: true, layers: bodyLayers };
}
