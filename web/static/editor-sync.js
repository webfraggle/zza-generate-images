// web/static/editor-sync.js
// Pure decision logic for reacting to external file changes (no DOM access).

/** Strips a weak-validator prefix and quotes from an ETag header value. */
export function normalizeEtag(h) {
  if (h == null) return '';
  return String(h).trim().replace(/^W\//, '').replace(/^"|"$/g, '');
}

/**
 * Decides what the editor does with a batch of changed files.
 * It is version-blind on purpose: echoes of the editor's own saves are
 * filtered by the caller via ETag comparison before 'silent'/'ask' take effect.
 *
 * @returns {{reloadList: boolean, currentFile: 'none'|'silent'|'ask',
 *            refreshJson: boolean, refreshPreview: boolean}}
 */
export function decide({ files, reset, currentFile, dirty, jsonDirty }) {
  const all = !!reset || files.includes('*');
  const changed = all || files.length > 0;
  const touchesCurrent = all || files.includes(currentFile);
  return {
    reloadList: changed,
    currentFile: touchesCurrent ? (dirty ? 'ask' : 'silent') : 'none',
    refreshJson: (all || files.includes('default.json')) && !jsonDirty,
    refreshPreview: changed,
  };
}

/** Delay in ms before retrying the long-poll after `failures` consecutive errors. */
export function nextDelay(failures) {
  return failures <= 0 ? 0 : 3000;
}
