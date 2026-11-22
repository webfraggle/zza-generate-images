# Preview-Dateigröße Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Dateigröße des gerenderten PNG unterhalb der Vorschau in edit- und admin-editor anzeigen.

**Architecture:** Client-only — `blob.size` aus dem bestehenden fetch-Response lesen, formatieren, in neuem `<span>` anzeigen.

**Tech Stack:** HTML, vanilla JS, CSS

---

### Task 1: CSS-Klasse hinzufügen

**Files:**
- Modify: `web/static/app.css`

- [ ] Nach `.no-preview { … }` einfügen:

```css
.preview-size { display: block; text-align: center; font-size: 0.75rem; color: var(--light-text); margin-top: 4px; }
```

- [ ] Commit

```bash
git add web/static/app.css
git commit -m "feat: add preview-size CSS class"
```

---

### Task 2: edit-editor.html — HTML + JS

**Files:**
- Modify: `web/templates/edit-editor.html`

- [ ] `<span id="preview-size">` nach `<img id="preview-img" …>` einfügen:

```html
<img id="preview-img" alt="" hidden>
<span id="preview-size" class="preview-size" hidden></span>
```

- [ ] DOM-Ref in JS-Block ergänzen (nach `const renderErr`):

```js
const previewSize = document.getElementById('preview-size');
```

- [ ] `formatSize`-Hilfsfunktion nach den DOM-Refs einfügen:

```js
function formatSize(bytes) {
  return bytes < 1024
    ? bytes + ' Bytes'
    : (bytes / 1024).toFixed(3) + ' KB';
}
```

- [ ] In `renderPreview()` — `previewSize` am Anfang verstecken (zusammen mit den anderen Elementen):

```js
previewSize.hidden = true;
```

- [ ] Im Erfolgs-Zweig nach `previewImg.hidden = false`:

```js
previewSize.textContent = formatSize(blob.size);
previewSize.hidden = false;
```

  Achtung: `res.blob()` muss in eine Variable — der Blob wird zweimal gebraucht (URL + size):

```js
const blob = await res.blob();
if (prevBlobUrl) URL.revokeObjectURL(prevBlobUrl);
prevBlobUrl       = URL.createObjectURL(blob);
previewImg.src    = prevBlobUrl;
previewImg.hidden = false;
previewSize.textContent = formatSize(blob.size);
previewSize.hidden = false;
```

- [ ] Commit

```bash
git add web/templates/edit-editor.html
git commit -m "feat: show PNG file size in edit editor preview"
```

---

### Task 3: admin-editor.html — HTML + JS

**Files:**
- Modify: `web/templates/admin-editor.html`

- [ ] Identische Änderungen wie Task 2 (HTML-Element, DOM-Ref, formatSize, hide/show)

- [ ] Im admin-editor ist der Erfolgs-Zweig in `renderPreview()` ab Zeile 196:

```js
const blob = await res.blob();
if (prevBlobUrl) URL.revokeObjectURL(prevBlobUrl);
prevBlobUrl       = URL.createObjectURL(blob);
previewImg.src    = prevBlobUrl;
previewImg.hidden = false;
previewSize.textContent = formatSize(blob.size);
previewSize.hidden = false;
```

- [ ] Commit

```bash
git add web/templates/admin-editor.html
git commit -m "feat: show PNG file size in admin editor preview"
```
