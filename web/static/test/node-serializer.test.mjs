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
  assert.deepEqual(layers[0], { type: 'image', file: 'bg.png', x: 0, y: 0 });
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
  assert.deepEqual(layers[0], { type: 'rect', x: 10, y: 20, width: 100, height: 50, color: '#FF0000' });
});

test('text node', () => {
  const graph = makeGraph(
    [node({ id: 'n1', type: 'text', data: { value: '{{zug1.zeit}}', x: '5', y: '60', font: 'regular', size: '16', color: '#000000', align: 'left' } })],
    ['n1']
  );
  const layers = graphToLayers(graph);
  assert.deepEqual(layers[0], { type: 'text', value: '{{zug1.zeit}}', x: 5, y: 60, font: 'regular', size: 16, color: '#000000', align: 'left' });
});

test('copy node', () => {
  const graph = makeGraph(
    [node({ id: 'n1', type: 'copy', data: { src_x: '0', src_y: '0', src_width: '240', src_height: '120', x: '0', y: '120' } })],
    ['n1']
  );
  const layers = graphToLayers(graph);
  assert.deepEqual(layers[0], { type: 'copy', src_x: 0, src_y: 0, src_width: 240, src_height: 120, x: 0, y: 120 });
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
  assert.equal(layers[0].max_items, 5);
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

test('loop node preserves falsy-but-valid field value "0"', () => {
  const nodes = [
    { id: 'n1', type: 'loop', canvasX: 0, canvasY: 0,
      data: { loopValue: '{{items}}', splitBy: '0', varName: 'item', maxItems: '3' },
      bodyChain: [] }
  ];
  const layers = graphToLayers(makeGraph(nodes, ['n1']));
  assert.equal(layers[0].split_by, '0');
});
import { describe, it } from 'node:test';

describe('graphToLayers — Layer if-Badge', () => {
  it('layerIfType="if" → layer.if', () => {
    const graph = {
      nodes: [{ id: 'n1', type: 'text', canvasX: 0, canvasY: 0,
                layerIfType: 'if', layerIfCond: 'not(isEmpty(zug1.hinweis))',
                data: { value: '{{zug1.hinweis}}' } }],
      chain: ['n1'],
    };
    const layers = graphToLayers(graph);
    assert.equal(layers[0].if, 'not(isEmpty(zug1.hinweis))');
    assert.equal(layers[0].type, 'text');
  });

  it('layerIfType="elif" → layer.elif', () => {
    const graph = {
      nodes: [{ id: 'n1', type: 'image', canvasX: 0, canvasY: 0,
                layerIfType: 'elif', layerIfCond: "startsWith(nr,'IC')",
                data: { file: 'ic.png' } }],
      chain: ['n1'],
    };
    assert.equal(graphToLayers(graph)[0].elif, "startsWith(nr,'IC')");
  });

  it('layerIfType="else" → layer.else = true', () => {
    const graph = {
      nodes: [{ id: 'n1', type: 'text', canvasX: 0, canvasY: 0,
                layerIfType: 'else', layerIfCond: '',
                data: { value: '{{nr}}' } }],
      chain: ['n1'],
    };
    assert.equal(graphToLayers(graph)[0].else, true);
  });
});

describe('graphToLayers — Feld-if', () => {
  it('colorIf/Then/Else → color als if/then/else-Objekt', () => {
    const graph = {
      nodes: [{ id: 'n1', type: 'rect', canvasX: 0, canvasY: 0,
                data: { colorIf: 'greaterThan(zug1.abw,0)', colorThen: '#FF4444', colorElse: '#FFFFFF' } }],
      chain: ['n1'],
    };
    const layer = graphToLayers(graph)[0];
    assert.deepEqual(layer.color, { if: 'greaterThan(zug1.abw,0)', then: '#FF4444', else: '#FFFFFF' });
  });
});

describe('graphToLayers — Filter-Pipeline', () => {
  it('value + value_filters → zusammengesetzter YAML-String', () => {
    const graph = {
      nodes: [{ id: 'n1', type: 'text', canvasX: 0, canvasY: 0,
                data: { value: '{{zug1.hinweis}}', value_filters: [{ fn: 'strip', arg: "'*'" }, { fn: 'upper', arg: null }] } }],
      chain: ['n1'],
    };
    assert.equal(graphToLayers(graph)[0].value, "{{zug1.hinweis | strip('*') | upper}}");
  });

  it('keine Filter → value unverändert', () => {
    const graph = {
      nodes: [{ id: 'n1', type: 'text', canvasX: 0, canvasY: 0,
                data: { value: '{{zug1.nr}}', value_filters: [] } }],
      chain: ['n1'],
    };
    assert.equal(graphToLayers(graph)[0].value, '{{zug1.nr}}');
  });
});

describe('graphToLayers — BLOCK-Nodes', () => {
  it("BLOCK-IF → {block: cond, layers: [...]}", () => {
    const graph = {
      nodes: [
        { id: 'n1', type: 'block', blockType: 'if', blockCond: "startsWith(nr,'ICN')",
          bodyChain: ['n2'], canvasX: 0, canvasY: 0, data: {} },
        { id: 'n2', type: 'image', canvasX: 0, canvasY: 0, data: { file: 'icn.png' } },
      ],
      chain: ['n1'],
    };
    const layer = graphToLayers(graph)[0];
    assert.equal(layer.block, "startsWith(nr,'ICN')");
    assert.equal(layer.layers[0].type, 'image');
  });

  it("BLOCK-ELIF → {elif: cond, layers: [...]}", () => {
    const graph = {
      nodes: [
        { id: 'n1', type: 'block', blockType: 'elif', blockCond: "startsWith(nr,'IC')",
          bodyChain: ['n2'], canvasX: 0, canvasY: 0, data: {} },
        { id: 'n2', type: 'image', canvasX: 0, canvasY: 0, data: { file: 'ic.png' } },
      ],
      chain: ['n1'],
    };
    assert.equal(graphToLayers(graph)[0].elif, "startsWith(nr,'IC')");
  });

  it("BLOCK-ELSE → {else: true, layers: [...]}", () => {
    const graph = {
      nodes: [
        { id: 'n1', type: 'block', blockType: 'else', blockCond: '',
          bodyChain: ['n2'], canvasX: 0, canvasY: 0, data: {} },
        { id: 'n2', type: 'text', canvasX: 0, canvasY: 0, data: { value: '{{nr}}' } },
      ],
      chain: ['n1'],
    };
    const layer = graphToLayers(graph)[0];
    assert.equal(layer.else, true);
    assert.equal(layer.layers[0].type, 'text');
  });
});
