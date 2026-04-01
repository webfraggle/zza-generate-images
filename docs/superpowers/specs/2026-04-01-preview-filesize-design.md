# Preview-Dateigröße — Design-Spec

Datum: 2026-04-01
Status: Approved

---

## Ziel

Unter dem Vorschaubild in beiden Editoren (`/edit` und `/admin/…/edit`) die Größe des gerenderten PNG anzeigen — als direktes Feedback über den Effekt von `canvas.colors` und anderen Einstellungen.

---

## Verhalten

- Anzeige erscheint nur wenn ein Bild erfolgreich gerendert wurde
- Bei Fehler, leerem JSON oder während des Ladens: versteckt
- Format: `< 1024 Bytes` → `"823 Bytes"`, sonst `"5.234 KB"` (3 Dezimalstellen)
- Quelle: `blob.size` nach `fetch(…/render)` — entspricht der komprimierten PNG-Größe

---

## Datei-Änderungen

| Datei | Änderung |
|---|---|
| `web/templates/edit-editor.html` | `<span id="preview-size">` nach `<img id="preview-img">`; DOM-Ref; hide/show in `renderPreview()` |
| `web/templates/admin-editor.html` | Identisch |
| `web/static/app.css` | `.preview-size` Klasse (zentriert, klein, `--light-text`) |

---

## Hilfsfunktion

```js
function formatSize(bytes) {
  return bytes < 1024
    ? bytes + ' Bytes'
    : (bytes / 1024).toFixed(3) + ' KB';
}
```

---

## Kein Server-Änderungsbedarf

`blob.size` ist clientseitig aus dem Response-Body verfügbar — kein neuer Header, kein neuer Endpoint.
