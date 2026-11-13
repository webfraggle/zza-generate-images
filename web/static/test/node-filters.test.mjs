import { parseValueAndFilters, formatValueWithFilters, evaluatePreview, splitTopLevel } from '../node-filters.js';
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

describe('verschachtelte Filter-Argumente', () => {
  const minutesExpr = '{{zug2.zeit | minutes | sub(zug1.zeit | minutes) | mod(1440) | add(5)}}';

  it('parse: Pipeline in Argumenten', () => {
    assert.deepStrictEqual(parseValueAndFilters(minutesExpr), {
      base: '{{zug2.zeit}}',
      filters: [
        { fn: 'minutes', arg: null },
        { fn: 'sub', arg: 'zug1.zeit | minutes' },
        { fn: 'mod', arg: '1440' },
        { fn: 'add', arg: '5' },
      ],
    });
  });

  it('Roundtrip', () => {
    for (const s of [
      minutesExpr,
      '{{now.second | add(anim.frame) | mul(6)}}',
      '{{now.hour12 | mul(30) | add(now.minute | div(2))}}',
    ]) {
      const p = parseValueAndFilters(s);
      assert.strictEqual(formatValueWithFilters(p.base, p.filters), s);
    }
  });

  it('Pipe in Quotes trennt nicht', () => {
    const p = parseValueAndFilters("{{v | stripAll('|')}}");
    assert.deepStrictEqual(p.filters, [{ fn: 'stripAll', arg: "'|'" }]);
  });

  it('splitTopLevel', () => {
    assert.deepStrictEqual(splitTopLevel('a | b(c | d) | e', '|'), ['a ', ' b(c | d) ', ' e']);
  });

  const tj = { zug1: { zeit: '10:15' }, zug2: { zeit: '10:18' }, t1: '23:58', t2: '00:04', n: '-3' };
  const rest = (first) => [
    { fn: 'minutes', arg: null },
    { fn: 'sub', arg: `${first} | minutes` },
    { fn: 'mod', arg: '1440' },
    { fn: 'add', arg: '5' },
  ];

  it('evaluatePreview: Minutenspalte', () => {
    assert.strictEqual(evaluatePreview('{{zug2.zeit}}', rest('zug1.zeit'), tj), '8');
    assert.strictEqual(evaluatePreview('{{t2}}', rest('t1'), tj), '11');
  });

  it('mod floored', () => {
    assert.strictEqual(evaluatePreview('{{n}}', [{ fn: 'mod', arg: '1440' }], tj), '1437');
  });

  it('minutes', () => {
    assert.strictEqual(evaluatePreview('{{zug1.zeit}}', [{ fn: 'minutes', arg: null }], tj), '615');
  });

  it('unbekannte Variable als Argument → leer', () => {
    assert.strictEqual(evaluatePreview('{{zug1.zeit}}', [{ fn: 'minutes', arg: null }, { fn: 'sub', arg: 'missing' }], tj), '');
  });
});


describe('Go/JS-Parität Mathe-Filter', () => {
  const tj = { a: '5', b: '2.5', n: '-3', z1: '10:15', empty: '', txt: 'abc', x: '-1e-20' };
  const ev = (expr) => { const { base, filters } = parseValueAndFilters(expr); return evaluatePreview(base, filters, tj); };
  it("{{z1 | add(5)}} \u2192 10:15", () => assert.strictEqual(ev("{{z1 | add(5)}}"), "10:15"));
  it("{{empty | minutes | mod(1440)}} \u2192 ", () => assert.strictEqual(ev("{{empty | minutes | mod(1440)}}"), ""));
  it("{{a | div(0)}} \u2192 0", () => assert.strictEqual(ev("{{a | div(0)}}"), "0"));
  it("{{a | mod(0)}} \u2192 0", () => assert.strictEqual(ev("{{a | mod(0)}}"), "0"));
  it("{{a | add('x')}} \u2192 5", () => assert.strictEqual(ev("{{a | add('x')}}"), "5"));
  it("{{a | add(txt)}} \u2192 ", () => assert.strictEqual(ev("{{a | add(txt)}}"), ""));
  it("{{a | add}} \u2192 5", () => assert.strictEqual(ev("{{a | add}}"), "5"));
  it("{{a | add()}} \u2192 5", () => assert.strictEqual(ev("{{a | add()}}"), "5"));
  it("{{b | mul(3)}} \u2192 7.5", () => assert.strictEqual(ev("{{b | mul(3)}}"), "7.5"));
  it("{{a | div(3)}} \u2192 1.6666666666666667", () => assert.strictEqual(ev("{{a | div(3)}}"), "1.6666666666666667"));
  it("{{a | add(inf)}} \u2192 ", () => assert.strictEqual(ev("{{a | add(inf)}}"), ""));
  it("{{a | add(a | add(a | add(a | add(a))))}} \u2192 25", () => assert.strictEqual(ev("{{a | add(a | add(a | add(a | add(a))))}}"), "25"));
  it("{{a | add(a | add(a | add(a | add(a | add(a)))))}} \u2192 ", () => assert.strictEqual(ev("{{a | add(a | add(a | add(a | add(a | add(a)))))}}"), ""));
  it("{{x | mod(1440)}} \u2192 0", () => assert.strictEqual(ev("{{x | mod(1440)}}"), "0"));
});
