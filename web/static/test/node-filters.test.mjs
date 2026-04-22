import { parseValueAndFilters, formatValueWithFilters, evaluatePreview } from '../node-filters.js';
import { strict as assert } from 'node:assert';
import { describe, it } from 'node:test';

describe('parseValueAndFilters', () => {
  it('kein Template → base unverändert, keine Filter', () => {
    assert.deepStrictEqual(
      parseValueAndFilters('statischer Text'),
      { base: 'statischer Text', filters: [] }
    );
  });

  it('Template ohne Filter', () => {
    assert.deepStrictEqual(
      parseValueAndFilters('{{zug1.hinweis}}'),
      { base: '{{zug1.hinweis}}', filters: [] }
    );
  });

  it('Template, ein Filter ohne Arg', () => {
    assert.deepStrictEqual(
      parseValueAndFilters('{{zug1.nr | upper}}'),
      { base: '{{zug1.nr}}', filters: [{ fn: 'upper', arg: null }] }
    );
  });

  it("Template, ein Filter mit Arg", () => {
    assert.deepStrictEqual(
      parseValueAndFilters("{{zug1.hinweis | strip('*')}}"),
      { base: '{{zug1.hinweis}}', filters: [{ fn: 'strip', arg: "'*'" }] }
    );
  });

  it('Template, mehrere Filter', () => {
    assert.deepStrictEqual(
      parseValueAndFilters("{{zug1.hinweis | strip('*') | upper}}"),
      {
        base: '{{zug1.hinweis}}',
        filters: [{ fn: 'strip', arg: "'*'" }, { fn: 'upper', arg: null }],
      }
    );
  });

  it('Template, stripBetween mit zwei Args', () => {
    assert.deepStrictEqual(
      parseValueAndFilters("{{zug1.hinweis | stripBetween('{', '}')}}"),
      { base: '{{zug1.hinweis}}', filters: [{ fn: 'stripBetween', arg: "'{', '}'" }] }
    );
  });

  it('Math-Filter: mul', () => {
    assert.deepStrictEqual(
      parseValueAndFilters('{{now.minute | mul(6)}}'),
      { base: '{{now.minute}}', filters: [{ fn: 'mul', arg: '6' }] }
    );
  });
});

describe('formatValueWithFilters', () => {
  it('keine Filter → base unverändert', () => {
    assert.strictEqual(formatValueWithFilters('{{zug1.hinweis}}', []), '{{zug1.hinweis}}');
  });

  it('ein Filter ohne Arg', () => {
    assert.strictEqual(
      formatValueWithFilters('{{zug1.nr}}', [{ fn: 'upper', arg: null }]),
      '{{zug1.nr | upper}}'
    );
  });

  it('ein Filter mit Arg', () => {
    assert.strictEqual(
      formatValueWithFilters('{{zug1.hinweis}}', [{ fn: 'strip', arg: "'*'" }]),
      "{{zug1.hinweis | strip('*')}}"
    );
  });

  it('mehrere Filter', () => {
    assert.strictEqual(
      formatValueWithFilters('{{zug1.hinweis}}', [{ fn: 'strip', arg: "'*'" }, { fn: 'upper', arg: null }]),
      "{{zug1.hinweis | strip('*') | upper}}"
    );
  });

  it('Roundtrip: parse → format', () => {
    const orig = "{{zug1.hinweis | strip('*') | upper}}";
    const { base, filters } = parseValueAndFilters(orig);
    assert.strictEqual(formatValueWithFilters(base, filters), orig);
  });

  it('kein Template → base unverändert (Filter nicht anwendbar)', () => {
    assert.strictEqual(formatValueWithFilters('statisch', [{ fn: 'upper', arg: null }]), 'statisch');
  });
});

describe('evaluatePreview', () => {
  const testJson = {
    zug1: { hinweis: '* Abweichung', nr: 'ICN', abw: '5' },
    now: { minute: 30 },
  };

  it('kein Filter, Pfad auflösen', () => {
    assert.strictEqual(evaluatePreview('{{zug1.nr}}', [], testJson), 'ICN');
  });

  it('upper-Filter', () => {
    assert.strictEqual(evaluatePreview('{{zug1.nr}}', [{ fn: 'upper', arg: null }], testJson), 'ICN');
  });

  it('lower-Filter', () => {
    assert.strictEqual(evaluatePreview('{{zug1.nr}}', [{ fn: 'lower', arg: null }], testJson), 'icn');
  });

  it("strip-Filter entfernt führendes Zeichen", () => {
    assert.strictEqual(
      evaluatePreview('{{zug1.hinweis}}', [{ fn: 'strip', arg: "'*'" }], testJson),
      ' Abweichung'
    );
  });

  it('strip + upper Verkettung', () => {
    assert.strictEqual(
      evaluatePreview('{{zug1.hinweis}}', [{ fn: 'strip', arg: "'*'" }, { fn: 'upper', arg: null }], testJson),
      ' ABWEICHUNG'
    );
  });

  it('stripAll entfernt alle Vorkommen', () => {
    assert.strictEqual(
      evaluatePreview('{{zug1.hinweis}}', [{ fn: 'stripAll', arg: "'*'" }], testJson),
      ' Abweichung'
    );
  });

  it('mul-Filter', () => {
    assert.strictEqual(
      evaluatePreview('{{now.minute}}', [{ fn: 'mul', arg: '6' }], testJson),
      '180'
    );
  });

  it('add-Filter', () => {
    assert.strictEqual(
      evaluatePreview('{{zug1.abw}}', [{ fn: 'add', arg: '3' }], testJson),
      '8'
    );
  });

  it('round-Filter', () => {
    assert.strictEqual(
      evaluatePreview('{{zug1.abw}}', [{ fn: 'round', arg: null }], testJson),
      '5'
    );
  });

  it('prefix-Filter', () => {
    assert.strictEqual(
      evaluatePreview('{{zug1.abw}}', [{ fn: 'prefix', arg: "'+'" }], testJson),
      '+5'
    );
  });

  it('trim-Filter', () => {
    assert.strictEqual(
      evaluatePreview('{{zug1.hinweis}}', [{ fn: 'trim', arg: null }], { zug1: { hinweis: '  text  ' } }),
      'text'
    );
  });

  it('format-Filter → [format] (no-op)', () => {
    assert.strictEqual(
      evaluatePreview('{{now.minute}}', [{ fn: 'format', arg: "'HH:mm'" }], testJson),
      '[format]'
    );
  });

  it('unbekannter Pfad → leer', () => {
    assert.strictEqual(evaluatePreview('{{zug1.unknown}}', [], testJson), '');
  });

  it('null testJson → leer', () => {
    assert.strictEqual(evaluatePreview('{{zug1.nr}}', [], null), '');
  });

  it('kein Template → base unverändert', () => {
    assert.strictEqual(evaluatePreview('statisch', [], testJson), 'statisch');
  });
});
