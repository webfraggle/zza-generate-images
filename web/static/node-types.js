// web/static/node-types.js
// Node type config: used by node-editor.js for rendering and node-parser.js for validation.

export const NODE_TYPES = {
  image: {
    label: 'IMAGE',
    color: '#037F8C',
    fields: [
      { name: 'file',   label: 'file',   inputType: 'dropdown', source: 'imageFiles' },
      { name: 'x',      label: 'x',      inputType: 'text', numeric: true },
      { name: 'y',      label: 'y',      inputType: 'text', numeric: true },
      { name: 'width',  label: 'width',  inputType: 'text', numeric: true },
      { name: 'height', label: 'height', inputType: 'text', numeric: true },
      { name: 'rotate', label: 'rotate', inputType: 'text', numeric: true, filterPipeline: true },
    ],
  },
  rect: {
    label: 'RECT',
    color: '#037F8C',
    fields: [
      { name: 'x',      label: 'x',      inputType: 'text', numeric: true },
      { name: 'y',      label: 'y',      inputType: 'text', numeric: true },
      { name: 'width',  label: 'width',  inputType: 'text', numeric: true },
      { name: 'height', label: 'height', inputType: 'text', numeric: true },
      { name: 'color',  label: 'color',  inputType: 'color', fieldIf: true },
    ],
  },
  text: {
    label: 'TEXT',
    color: '#037F8C',
    fields: [
      { name: 'value',  label: 'value',  inputType: 'text', filterPipeline: true },
      { name: 'x',      label: 'x',      inputType: 'text', numeric: true },
      { name: 'y',      label: 'y',      inputType: 'text', numeric: true },
      { name: 'font',   label: 'font',   inputType: 'dropdown', source: 'fontIds' },
      { name: 'size',   label: 'size',   inputType: 'text', numeric: true },
      { name: 'color',  label: 'color',  inputType: 'color', fieldIf: true },
      { name: 'align',  label: 'align',  inputType: 'dropdown', options: ['', 'left', 'center', 'right'] },
      { name: 'width',  label: 'width',  inputType: 'text', numeric: true },
      { name: 'height', label: 'height', inputType: 'text', numeric: true },
    ],
  },
  copy: {
    label: 'COPY',
    color: '#037F8C',
    fields: [
      { name: 'src_x',      label: 'src_x',   inputType: 'text', numeric: true },
      { name: 'src_y',      label: 'src_y',   inputType: 'text', numeric: true },
      { name: 'src_width',  label: 'src_w',   inputType: 'text', numeric: true },
      { name: 'src_height', label: 'src_h',   inputType: 'text', numeric: true },
      { name: 'x',          label: 'x',       inputType: 'text', numeric: true },
      { name: 'y',          label: 'y',       inputType: 'text', numeric: true },
      // Note: copy has no destination width/height — the src region is pasted 1:1 at (x,y)
    ],
  },
  loop: {
    label: 'LOOP',
    color: '#C83232',
    fields: [
      { name: 'loopValue', label: 'value',     inputType: 'text' },
      { name: 'splitBy',   label: 'split_by',  inputType: 'text' },
      { name: 'varName',   label: 'var',       inputType: 'text' },
      { name: 'maxItems',  label: 'max_items', inputType: 'text', numeric: true },
    ],
  },
  block: {
    label: 'BLOCK',
    color: '#FD7014',
    blockNode: true,
    fields: [],   // rendered specially in node-editor.js
  },
};

// YAML field names for each node type (maps data key → YAML key).
// Used by node-serializer.js.
export const YAML_FIELD_MAP = {
  image:  { file: 'file', x: 'x', y: 'y', width: 'width', height: 'height', rotate: 'rotate' },
  rect:   { x: 'x', y: 'y', width: 'width', height: 'height', color: 'color' },
  text:   { value: 'value', x: 'x', y: 'y', font: 'font', size: 'size', color: 'color', align: 'align', width: 'width', height: 'height' },
  copy:   { src_x: 'src_x', src_y: 'src_y', src_width: 'src_width', src_height: 'src_height', x: 'x', y: 'y' },
  loop:   { loopValue: 'value', splitBy: 'split_by', varName: 'var', maxItems: 'max_items' },
};

// YAML field names → data key (inverse map, used by node-parser.js).
export const YAML_TO_DATA_KEY = Object.freeze(
  Object.fromEntries(
    Object.entries(YAML_FIELD_MAP).map(([type, map]) => [
      type,
      Object.freeze(Object.fromEntries(Object.entries(map).map(([dk, yk]) => [yk, dk]))),
    ])
  )
);
