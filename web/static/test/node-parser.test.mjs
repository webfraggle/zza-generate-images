// web/static/test/node-parser.test.mjs
// Run with: node --test web/static/test/node-parser.test.mjs
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { layersToGraph } from '../node-parser.js';

test('empty layers returns empty graph', () => {
  const result = layersToGraph([]);
  assert.equal(result.ok, true);
  assert.deepEqual(result.chain, []);
  assert.deepEqual(result.nodes, []);
});

test('single image layer', () => {
  const layers = [{ type: 'image', file: 'bg.png', x: '0', y: '0' }];
  const result = layersToGraph(layers);
  assert.equal(result.ok, true);
  assert.equal(result.nodes.length, 1);
  assert.equal(result.chain.length, 1);
  const node = result.nodes[0];
  assert.equal(node.type, 'image');
  assert.equal(node.data.file, 'bg.png');
  assert.equal(node.data.x, '0');
  assert.equal(node.data.y, '0');
  assert.equal(result.chain[0], node.id);
});

test('chain of layers becomes chain', () => {
  const layers = [
    { type: 'image', file: 'bg.png' },
    { type: 'text', value: '{{zug1.zeit}}' },
  ];
  const result = layersToGraph(layers);
  assert.equal(result.ok, true);
  assert.equal(result.chain.length, 2);
  assert.equal(result.nodes[0].type, 'image');
  assert.equal(result.nodes[1].type, 'text');
  assert.deepEqual(result.chain, [result.nodes[0].id, result.nodes[1].id]);
});

test('loop layer with body layers', () => {
  const layers = [{
    type: 'loop',
    value: '{{zug1.via}}',
    split_by: '-',
    var: 'via_item',
    max_items: '5',
    layers: [{ type: 'text', value: '{{via_item}}' }],
  }];
  const result = layersToGraph(layers);
  assert.equal(result.ok, true);
  assert.equal(result.nodes.length, 2);
  const loopNode = result.nodes.find(n => n.type === 'loop');
  const textNode = result.nodes.find(n => n.type === 'text');
  assert.ok(loopNode);
  assert.ok(textNode);
  assert.equal(loopNode.data.loopValue, '{{zug1.via}}');
  assert.equal(loopNode.data.splitBy, '-');
  assert.equal(loopNode.data.varName, 'via_item');
  assert.equal(loopNode.data.maxItems, '5');
  assert.deepEqual(loopNode.bodyChain, [textNode.id]);
});

test('layer with if: → layerIfType="if" (Phase 2)', () => {
  const layers = [{ type: 'rect', if: 'greaterThan(abw, 0)', x: '0', y: '0' }];
  const result = layersToGraph(layers);
  assert.equal(result.ok, true);
  assert.equal(result.nodes[0].layerIfType, 'if');
  assert.equal(result.nodes[0].layerIfCond, 'greaterThan(abw, 0)');
});

test('layer with conditional field value → colorIf (Phase 2)', () => {
  const layers = [{ type: 'rect', color: { if: 'equals(x,1)', then: '#F00', else: '#0F0' } }];
  const result = layersToGraph(layers);
  assert.equal(result.ok, true);
  assert.equal(result.nodes[0].data.colorIf, 'equals(x,1)');
  assert.equal(result.nodes[0].data.colorThen, '#F00');
  assert.equal(result.nodes[0].data.colorElse, '#0F0');
});

test('nested loop locks (unsupported)', () => {
  const layers = [{
    type: 'loop',
    value: '{{a}}',
    layers: [{ type: 'loop', value: '{{b}}', layers: [] }],
  }];
  const result = layersToGraph(layers);
  assert.equal(result.ok, false);
  assert.ok(result.reason.toLowerCase().includes('loop'));
});

test('nodes get auto-positioned in a horizontal row', () => {
  const layers = [
    { type: 'image', file: 'a.png' },
    { type: 'text', value: 'hi' },
    { type: 'rect', x: '0', y: '0', width: '10', height: '10' },
  ];
  const result = layersToGraph(layers);
  assert.equal(result.ok, true);
  // Each node should be to the right of the previous one
  const xs = result.nodes.map(n => n.canvasX);
  assert.ok(xs[1] > xs[0]);
  assert.ok(xs[2] > xs[1]);
  // Y should be consistent
  assert.equal(result.nodes[0].canvasY, result.nodes[1].canvasY);
});

test('null layer element locks (invalid)', () => {
  const result = layersToGraph([null]);
  assert.equal(result.ok, false);
  assert.ok(result.reason.includes('null') || result.reason.includes('Invalid'));
});

test('unknown layer type locks', () => {
  const result = layersToGraph([{ type: 'gradient', x: '0' }]);
  assert.equal(result.ok, false);
  assert.ok(result.reason.includes('gradient') || result.reason.toLowerCase().includes('unknown'));
});
import { describe, it } from 'node:test';

describe('layersToGraph — Layer if-Badge', () => {
  it('layer mit if: → node.layerIfType="if", node.layerIfCond', () => {
    const r = layersToGraph([{ type: 'text', if: 'not(isEmpty(zug1.hinweis))', value: '{{zug1.hinweis}}' }]);
    assert.ok(r.ok);
    assert.equal(r.nodes[0].layerIfType, 'if');
    assert.equal(r.nodes[0].layerIfCond, 'not(isEmpty(zug1.hinweis))');
  });

  it('layer mit elif: → node.layerIfType="elif"', () => {
    const r = layersToGraph([{ type: 'image', elif: "startsWith(nr,'IC')", file: 'ic.png' }]);
    assert.ok(r.ok);
    assert.equal(r.nodes[0].layerIfType, 'elif');
    assert.equal(r.nodes[0].layerIfCond, "startsWith(nr,'IC')");
  });

  it('layer mit else: true → layerIfType="else", layerIfCond=""', () => {
    const r = layersToGraph([{ type: 'text', else: true, value: '{{nr}}' }]);
    assert.ok(r.ok);
    assert.equal(r.nodes[0].layerIfType, 'else');
    assert.equal(r.nodes[0].layerIfCond, '');
  });
});

describe('layersToGraph — Feld-if (colorIf)', () => {
  it('color als if/then/else-Objekt → data.colorIf/Then/Else', () => {
    const r = layersToGraph([{
      type: 'rect',
      color: { if: 'greaterThan(zug1.abw,0)', then: '#FF4444', else: '#FFFFFF' },
    }]);
    assert.ok(r.ok);
    assert.equal(r.nodes[0].data.colorIf,   'greaterThan(zug1.abw,0)');
    assert.equal(r.nodes[0].data.colorThen,  '#FF4444');
    assert.equal(r.nodes[0].data.colorElse,  '#FFFFFF');
  });
});

describe('layersToGraph — Filter-Pipeline', () => {
  it("value mit Filtern → data.value = base, data.value_filters", () => {
    const r = layersToGraph([{ type: 'text', value: "{{zug1.hinweis | strip('*') | upper}}" }]);
    assert.ok(r.ok);
    assert.equal(r.nodes[0].data.value, '{{zug1.hinweis}}');
    assert.deepEqual(r.nodes[0].data.value_filters, [
      { fn: 'strip', arg: "'*'" },
      { fn: 'upper', arg: null },
    ]);
  });

  it('rotate mit mul-Filter', () => {
    const r = layersToGraph([{ type: 'image', file: 'bg.png', rotate: '{{now.minute | mul(6)}}' }]);
    assert.ok(r.ok);
    assert.equal(r.nodes[0].data.rotate, '{{now.minute}}');
    assert.deepEqual(r.nodes[0].data.rotate_filters, [{ fn: 'mul', arg: '6' }]);
  });
});

describe('layersToGraph — BLOCK-Nodes', () => {
  it("if+layers → node.type='block', blockType='if'", () => {
    const r = layersToGraph([{
      if: "startsWith(nr,'ICN')",
      layers: [{ type: 'image', file: 'icn.png' }],
    }]);
    assert.ok(r.ok);
    assert.equal(r.nodes[0].type, 'block');
    assert.equal(r.nodes[0].blockType, 'if');
    assert.equal(r.nodes[0].blockCond, "startsWith(nr,'ICN')");
    assert.equal(r.nodes[0].bodyChain.length, 1);
  });

  it("elif-Block → blockType='elif'", () => {
    const r = layersToGraph([{
      elif: "startsWith(nr,'IC')",
      layers: [{ type: 'image', file: 'ic.png' }],
    }]);
    assert.ok(r.ok);
    assert.equal(r.nodes[0].blockType, 'elif');
    assert.equal(r.nodes[0].blockCond, "startsWith(nr,'IC')");
  });

  it("else-Block → blockType='else', blockCond=''", () => {
    const r = layersToGraph([{ else: true, layers: [{ type: 'text', value: '{{nr}}' }] }]);
    assert.ok(r.ok);
    assert.equal(r.nodes[0].blockType, 'else');
    assert.equal(r.nodes[0].blockCond, '');
  });

  it('vollständige if/elif/else-Kette mit regulärem Layer danach', () => {
    const r = layersToGraph([
      { if: "startsWith(nr,'ICN')", layers: [{ type: 'image', file: 'icn.png' }] },
      { elif: "startsWith(nr,'IC')",  layers: [{ type: 'image', file: 'ic.png' }] },
      { else: true,                   layers: [{ type: 'text', value: '{{nr}}' }] },
      { type: 'text', value: '{{zeit}}' },
    ]);
    assert.ok(r.ok);
    assert.equal(r.chain.length, 4);
    assert.ok(r.nodes.some(n => n.blockType === 'if'));
    assert.ok(r.nodes.some(n => n.blockType === 'elif'));
    assert.ok(r.nodes.some(n => n.blockType === 'else'));
    assert.ok(r.nodes.some(n => n.type === 'text' && !n.blockType));
  });
});
