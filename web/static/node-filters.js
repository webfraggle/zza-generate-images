// web/static/node-filters.js
// Filter-Pipeline: Parsing, Evaluierung (für Live-Vorschau), Chip-UI.

export const FILTER_DEFS = [
  { fn: 'strip',        label: 'strip',        hasArg: true,  category: 'text' },
  { fn: 'stripAll',     label: 'stripAll',      hasArg: true,  category: 'text' },
  { fn: 'stripBetween', label: 'stripBetween',  hasArg: true,  category: 'text' },
  { fn: 'upper',        label: 'upper',         hasArg: false, category: 'text' },
  { fn: 'lower',        label: 'lower',         hasArg: false, category: 'text' },
  { fn: 'trim',         label: 'trim',          hasArg: false, category: 'text' },
  { fn: 'prefix',       label: 'prefix',        hasArg: true,  category: 'text' },
  { fn: 'suffix',       label: 'suffix',        hasArg: true,  category: 'text' },
  { fn: 'mul',          label: 'mul',           hasArg: true,  category: 'math' },
  { fn: 'div',          label: 'div',           hasArg: true,  category: 'math' },
  { fn: 'add',          label: 'add',           hasArg: true,  category: 'math' },
  { fn: 'sub',          label: 'sub',           hasArg: true,  category: 'math' },
  { fn: 'round',        label: 'round',         hasArg: false, category: 'math' },
  { fn: 'format',       label: 'format',        hasArg: true,  category: 'time' },
];

/**
 * Parst "{{expr | f1 | f2(arg)}}" → { base: "{{expr}}", filters: [{fn, arg}] }
 * Gibt {base: str, filters: []} zurück wenn keine Filter vorhanden.
 * @param {string} str
 * @returns {{ base: string, filters: Array<{fn: string, arg: string|null}> }}
 */
export function parseValueAndFilters(str) {
  const match = str.match(/^\{\{(.+)\}\}$/s);
  if (!match) return { base: str, filters: [] };

  const inner = match[1];
  const parts = inner.split(' | ');
  if (parts.length === 1) return { base: str, filters: [] };

  const [expr, ...filterParts] = parts;
  const filters = filterParts.map(part => {
    const parenIdx = part.indexOf('(');
    if (parenIdx === -1) return { fn: part.trim(), arg: null };
    const fn  = part.slice(0, parenIdx).trim();
    const arg = part.slice(parenIdx + 1, part.lastIndexOf(')'));
    return { fn, arg };
  });

  return { base: `{{${expr}}}`, filters };
}

/**
 * Setzt base + filters zusammen → "{{expr | f1 | f2(arg)}}"
 * @param {string} base
 * @param {Array<{fn: string, arg: string|null}>} filters
 * @returns {string}
 */
export function formatValueWithFilters(base, filters) {
  if (!filters || filters.length === 0) return base;
  const match = base.match(/^\{\{(.+)\}\}$/s);
  if (!match) return base;
  const inner = match[1];
  const filterStr = filters
    .map(f => (f.arg != null ? `${f.fn}(${f.arg})` : f.fn))
    .join(' | ');
  return `{{${inner} | ${filterStr}}}`;
}

/**
 * Wertet base-Expression gegen testJson aus und wendet Filter an.
 * @param {string} base   z.B. "{{zug1.hinweis}}"
 * @param {Array<{fn: string, arg: string|null}>} filters
 * @param {object|null} testJson
 * @returns {string}
 */
export function evaluatePreview(base, filters, testJson) {
  if (!testJson) return '';
  const match = base.match(/^\{\{(.+)\}\}$/s);
  if (!match) return base;

  const path = match[1].trim();
  const raw = _resolvePath(path, testJson);
  if (raw === undefined || raw === null) return '';

  let value = String(raw);
  for (const { fn, arg } of filters) {
    value = _applyFilter(value, fn, arg);
  }
  return value;
}

function _resolvePath(path, obj) {
  return path.split('.').reduce((o, k) => (o != null ? o[k] : undefined), obj);
}

function _parseArg(arg) {
  if (arg == null) return null;
  const t = arg.trim();
  if ((t.startsWith("'") && t.endsWith("'")) || (t.startsWith('"') && t.endsWith('"'))) {
    return t.slice(1, -1);
  }
  return t;
}

function _parseTwoArgs(arg) {
  const parts = arg.split(/,\s*/);
  return parts.map(_parseArg);
}

function _applyFilter(value, fn, arg) {
  switch (fn) {
    case 'strip': {
      const c = _parseArg(arg);
      return value.startsWith(c) ? value.slice(c.length) : value;
    }
    case 'stripAll': {
      const c = _parseArg(arg);
      return value.split(c).join('');
    }
    case 'stripBetween': {
      const [a, b] = _parseTwoArgs(arg);
      let result = value;
      let start = result.indexOf(a);
      while (start !== -1) {
        const end = result.indexOf(b, start);
        if (end === -1) break;
        result = result.slice(0, start) + result.slice(end + b.length);
        start = result.indexOf(a);
      }
      return result;
    }
    case 'upper':  return value.toUpperCase();
    case 'lower':  return value.toLowerCase();
    case 'trim':   return value.trim();
    case 'prefix': return _parseArg(arg) + value;
    case 'suffix': return value + _parseArg(arg);
    case 'mul':    return String(parseFloat(value) * parseFloat(_parseArg(arg)));
    case 'div':    return String(parseFloat(value) / parseFloat(_parseArg(arg)));
    case 'add':    return String(parseFloat(value) + parseFloat(_parseArg(arg)));
    case 'sub':    return String(parseFloat(value) - parseFloat(_parseArg(arg)));
    case 'round':  return String(Math.round(parseFloat(value)));
    case 'format': return '[format]';
    default:       return value;
  }
}

/**
 * Rendert Filter-Chips + [+]-Button + Vorschau-Zeile in container.
 * @param {HTMLElement} container
 * @param {() => string} getBase
 * @param {Array<{fn, arg}>} filters
 * @param {(newFilters: Array) => void} onChange
 * @param {() => object|null} getTestJson
 * @returns {{ updatePreview: () => void }}
 */
export function renderFilterRow(container, getBase, filters, onChange, getTestJson) {
  container.innerHTML = '';

  const chipRow = document.createElement('div');
  chipRow.className = 'filter-chip-row';

  // ── Chips ─────────────────────────────────────────────────────────────────
  filters.forEach((filter, i) => {
    const chip = document.createElement('span');
    chip.className = 'filter-chip';
    chip.draggable = true;
    chip.dataset.index = String(i);

    const chipLabel = document.createElement('span');
    chipLabel.className = 'filter-chip-label';
    chipLabel.textContent = filter.arg != null ? `${filter.fn}(${filter.arg})` : filter.fn;
    chip.appendChild(chipLabel);

    const removeBtn = document.createElement('span');
    removeBtn.className = 'filter-chip-remove';
    removeBtn.textContent = '✕';
    removeBtn.addEventListener('click', e => {
      e.stopPropagation();
      onChange(filters.filter((_, j) => j !== i));
    });
    chip.appendChild(removeBtn);

    chip.addEventListener('dragstart', e => {
      e.dataTransfer.setData('text/plain', String(i));
      chip.classList.add('filter-chip--dragging');
    });
    chip.addEventListener('dragend', () => chip.classList.remove('filter-chip--dragging'));
    chip.addEventListener('dragover', e => { e.preventDefault(); chip.classList.add('filter-chip--drag-over'); });
    chip.addEventListener('dragleave', () => chip.classList.remove('filter-chip--drag-over'));
    chip.addEventListener('drop', e => {
      e.preventDefault();
      chip.classList.remove('filter-chip--drag-over');
      const fromIdx = parseInt(e.dataTransfer.getData('text/plain'), 10);
      if (fromIdx === i) return;
      const newFilters = [...filters];
      const [moved] = newFilters.splice(fromIdx, 1);
      newFilters.splice(i, 0, moved);
      onChange(newFilters);
    });

    chipRow.appendChild(chip);
  });

  // ── [+] Button + Dropdown ─────────────────────────────────────────────────
  const addWrapper = document.createElement('span');
  addWrapper.className = 'filter-add-wrapper';

  const addBtn = document.createElement('button');
  addBtn.type = 'button';
  addBtn.className = 'filter-add-btn';
  addBtn.textContent = '+';

  const dropdown = document.createElement('div');
  dropdown.className = 'filter-add-dropdown';
  dropdown.hidden = true;

  const categories = [
    { key: 'text', label: 'Text' },
    { key: 'math', label: 'Mathe' },
    { key: 'time', label: 'Zeit' },
  ];
  for (const { key, label } of categories) {
    const defs = FILTER_DEFS.filter(f => f.category === key);
    if (!defs.length) continue;
    const group = document.createElement('div');
    group.className = 'filter-add-group';
    const groupTitle = document.createElement('div');
    groupTitle.className = 'filter-add-group-title';
    groupTitle.textContent = label;
    group.appendChild(groupTitle);
    for (const def of defs) {
      const item = document.createElement('button');
      item.type = 'button';
      item.className = 'filter-add-item';
      item.textContent = def.fn + (def.hasArg ? '(…)' : '');
      item.addEventListener('click', () => {
        dropdown.hidden = true;
        if (def.hasArg) {
          const argInput = document.createElement('input');
          argInput.type = 'text';
          argInput.className = 'filter-arg-input';
          argInput.placeholder = `Argument für ${def.fn}`;
          addWrapper.appendChild(argInput);
          argInput.focus();
          const commit = () => {
            argInput.remove();
            const val = argInput.value.trim();
            if (!val) return;
            onChange([...filters, { fn: def.fn, arg: val }]);
          };
          argInput.addEventListener('keydown', e => {
            if (e.key === 'Enter') commit();
            if (e.key === 'Escape') argInput.remove();
          });
          argInput.addEventListener('blur', commit);
          return;
        }
        onChange([...filters, { fn: def.fn, arg: null }]);
      });
      group.appendChild(item);
    }
    dropdown.appendChild(group);
  }

  addBtn.addEventListener('click', e => {
    e.stopPropagation();
    dropdown.hidden = !dropdown.hidden;
  });
  document.addEventListener('mousedown', e => {
    if (!addWrapper.contains(e.target)) dropdown.hidden = true;
  }, { capture: true });

  addWrapper.appendChild(addBtn);
  addWrapper.appendChild(dropdown);
  chipRow.appendChild(addWrapper);
  container.appendChild(chipRow);

  // ── Vorschau-Zeile ────────────────────────────────────────────────────────
  const preview = document.createElement('div');
  preview.className = 'filter-preview';
  container.appendChild(preview);

  function updatePreview() {
    const base = getBase ? getBase() : '';
    const tj = getTestJson ? getTestJson() : null;
    if (!base || !tj) { preview.textContent = ''; return; }
    const result = evaluatePreview(base, filters, tj);
    preview.textContent = result !== '' ? `→ ${result}` : '';
  }
  updatePreview();

  return { updatePreview };
}
