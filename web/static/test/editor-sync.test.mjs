// web/static/test/editor-sync.test.mjs
// Run with: node --test web/static/test/editor-sync.test.mjs
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { decide, normalizeEtag, nextDelay } from '../editor-sync.js';

const base = { files: [], reset: false, currentFile: 'template.yaml', dirty: false, jsonDirty: false };

test('empty batch changes nothing', () => {
  const d = decide(base);
  assert.equal(d.reloadList, false);
  assert.equal(d.currentFile, 'none');
  assert.equal(d.refreshJson, false);
  assert.equal(d.refreshPreview, false);
});

test('other file changed reloads list and preview', () => {
  const d = decide({ ...base, files: ['logo.png'] });
  assert.equal(d.reloadList, true);
  assert.equal(d.currentFile, 'none');
  assert.equal(d.refreshPreview, true);
  assert.equal(d.refreshJson, false);
});

test('open clean file reloads silently', () => {
  const d = decide({ ...base, files: ['template.yaml'] });
  assert.equal(d.currentFile, 'silent');
  assert.equal(d.refreshPreview, true);
});

test('open dirty file asks', () => {
  const d = decide({ ...base, files: ['template.yaml'], dirty: true });
  assert.equal(d.currentFile, 'ask');
});

test('star and reset touch everything', () => {
  for (const input of [{ files: ['*'] }, { files: [], reset: true }]) {
    const d = decide({ ...base, ...input });
    assert.equal(d.reloadList, true);
    assert.equal(d.currentFile, 'silent');
    assert.equal(d.refreshJson, true);
    assert.equal(d.refreshPreview, true);
  }
  assert.equal(decide({ ...base, files: ['*'], dirty: true }).currentFile, 'ask');
  assert.equal(decide({ ...base, reset: true, jsonDirty: true }).refreshJson, false);
});

test('default.json refreshes test json only when untouched', () => {
  assert.equal(decide({ ...base, files: ['default.json'], jsonDirty: false }).refreshJson, true);
  assert.equal(decide({ ...base, files: ['default.json'], jsonDirty: true }).refreshJson, false);
});

test('own save echo is ignored', () => {
  // decide() is deliberately version-blind: the caller compares ETags before
  // acting on 'silent'/'ask'. A clean echo therefore yields only 'silent',
  // which the caller turns into a no-op because the version is unchanged.
  const d = decide({ ...base, files: ['template.yaml'], dirty: false });
  assert.equal(d.currentFile, 'silent');
  assert.equal(d.reloadList, true);
});

test('normalizeEtag', () => {
  assert.equal(normalizeEtag('"abc"'), 'abc');
  assert.equal(normalizeEtag('W/"abc"'), 'abc');
  assert.equal(normalizeEtag(null), '');
});

test('nextDelay', () => {
  assert.equal(nextDelay(0), 0);
  assert.equal(nextDelay(1), 3000);
  assert.equal(nextDelay(5), 3000);
});
