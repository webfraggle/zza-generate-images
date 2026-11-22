# Color Palette Reduction — Manueller Testplan

Vorbedingung: Server läuft lokal oder auf gen.yuv.de.

---

### 1. Kein `colors`-Feld — Verhalten unverändert

- Ein bestehendes Template öffnen (z.B. `default`)
- `POST /{template}/render` mit JSON aufrufen
- **Erwartung:** Antwort ist ein vollfarbiges PNG (`Content-Type: image/png`), Verhalten identisch zu vor dem Feature

---

### 2. `colors: 32` in `template.yaml` — kleinere Datei

- In einem Template `template.yaml` unter `meta.canvas` eintragen:
  ```yaml
  canvas:
    width: 160
    height: 80
    colors: 32
  ```
- `POST /{template}/render` aufrufen
- **Erwartung:**
  - Antwort ist ein gültiges PNG
  - Datei deutlich kleiner als ohne `colors` (z.B. im Browser-DevTools unter Network → Response-Size prüfen, oder `curl -o out.png` + `ls -lh`)
  - Bild sieht visuell akzeptabel aus (Texte lesbar, keine Artefakte bei einfachen Grafiken)

---

### 3. Wenige Farben — extremer Test mit `colors: 2`

- `colors: 2` setzen
- Render aufrufen
- **Erwartung:** PNG hat nur 2 Farben, sieht stark reduziert aus aber ist ein gültiges PNG

---

### 4. Validierung — ungültige Werte

- `colors: 1` in `template.yaml` eintragen, Server neu starten (oder Template neu laden)
- `POST /{template}/render` aufrufen
- **Erwartung:** HTTP 404 oder 500 mit Fehlermeldung (`canvas.colors must be between 2 and 256`)

- `colors: 257` → gleiche Erwartung

---

### 5. CLI — `zza render` respektiert `colors`

- Template mit `colors: 16` verwenden
- `zza render -t {template} -i input.json -o out.png` ausführen
- **Erwartung:** `out.png` ist ein indexed PNG mit max. 16 Farben (prüfbar z.B. mit `file out.png` — zeigt "8-bit colormap" statt "8-bit/color RGBA")

---

### 6. Cache-Invalidierung bei Änderung von `colors`

- Template mit `colors: 32` rendern → Dateigröße notieren
- `colors: 32` auf `colors: 8` ändern
- Nochmals rendern
- **Erwartung:** Neue (kleinere) Datei, kein Cache-Hit mit altem Ergebnis (`X-Cache: MISS` im Response-Header)

---

Alle 6 Checks bestanden → Branch bereit zum Merge in `develop`.
