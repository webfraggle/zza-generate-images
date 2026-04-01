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

test('layer with if: locks (unsupported)', () => {
  const layers = [{ type: 'rect', if: 'greaterThan(abw, 0)', x: '0', y: '0' }];
  const result = layersToGraph(layers);
  assert.equal(result.ok, false);
  assert.ok(result.reason.includes('if'));
});

test('layer with conditional field value locks (unsupported)', () => {
  const layers = [{ type: 'rect', color: { if: 'equals(x,1)', then: '#F00', else: '#0F0' } }];
  const result = layersToGraph(layers);
  assert.equal(result.ok, false);
  assert.ok(result.reason.includes('if'));
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

test('nodes get auto-positioned in a vertical stack', () => {
  const layers = [
    { type: 'image', file: 'a.png' },
    { type: 'text', value: 'hi' },
    { type: 'rect', x: '0', y: '0', width: '10', height: '10' },
  ];
  const result = layersToGraph(layers);
  assert.equal(result.ok, true);
  // Each node should be below the previous one
  const ys = result.nodes.map(n => n.canvasY);
  assert.ok(ys[1] > ys[0]);
  assert.ok(ys[2] > ys[1]);
  // X should be consistent
  assert.equal(result.nodes[0].canvasX, result.nodes[1].canvasX);
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
