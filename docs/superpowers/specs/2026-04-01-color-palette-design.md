# Reduzierte Farbpaletten — Design-Spec

Datum: 2026-04-01
Status: Approved

---

## Ziel

PNG-Ausgabe auf eine konfigurierbare Anzahl Farben reduzieren, um die Dateigröße für die Übertragung an TFT-Displays über Mikrocontroller zu minimieren. Da die generierten Bilder ohnehin nur wenige Farben enthalten (einfache Grafiken + kleine Anti-Aliasing-Übergänge an Schriftkanten), genügen 16–32 Farben für eine visuell akzeptable Qualität.

---

## YAML-Konfiguration

Neues optionales Feld `colors` unter `meta.canvas`:

```yaml
meta:
  canvas:
    width: 160
    height: 80
    colors: 32   # optional; 2–256; weglassen = volle Farbe
```

- `colors: 0` oder Feld weggelassen → keine Reduktion, Ausgabe wie bisher (volle Farbe, `*image.NRGBA`)
- Gültige Werte: 2–256 (technisches Limit von indexed PNG)
- Ungültige Werte → Fehler beim Template-Laden (`LoadTemplate`)
- Kein Breaking Change — bestehende Templates ohne `colors` funktionieren unverändert

---

## Algorithmus: Median-Cut

Keine externe Bibliothek. Eigene Implementierung in `internal/renderer/quantize.go`.

**Schritte:**

1. Alle Pixelfarben des Bildes als Slice sammeln (Alpha wird ignoriert — nicht vorgesehen)
2. Farbraum rekursiv halbieren:
   - Achse mit dem größten Wertebereich (R, G oder B) bestimmen
   - Slice nach dieser Achse sortieren
   - Am Median splitten → zwei Buckets
   - Wiederholen bis `n` Buckets erreicht
3. Pro Bucket: Durchschnittswert (R, G, B) als Palettenfarbe
4. Jeden Pixel auf die nächste Palettenfarbe mappen (euklidischer Abstand im RGB-Raum)
5. Ergebnis: `*image.Paletted`

Kein Dithering — bei kleinen Texten auf TFT-Displays erzeugt Dithering fransige Schriftkanten.

`n` wird intern auf [2, 256] geclampt, auch wenn der YAML-Wert bereits validiert ist.

---

## Datei-Änderungen

| Datei | Änderung |
|---|---|
| `internal/renderer/template.go` | `Canvas.Colors int \`yaml:"colors"\`` + Validierung in `LoadTemplate` |
| `internal/renderer/quantize.go` | Neu — exportierte Funktion `Quantize(src *image.NRGBA, n int) *image.Paletted` |
| `internal/renderer/quantize_test.go` | Neu — Unit-Tests für Quantize |
| `internal/server/server.go` | Nach `Render()`: wenn `tmpl.Meta.Canvas.Colors > 0`, `Quantize` aufrufen |
| `internal/cli/render.go` | Gleiche Änderung wie server.go für `zza render` CLI |
| `docs/yaml-template-spec.md` | `colors`-Feld unter `meta.canvas` dokumentieren |

---

## Pipeline

```
Renderer.Render() → *image.NRGBA
    ↓ (wenn canvas.colors > 0)
renderer.Quantize(img, n) → *image.Paletted
    ↓
png.Encode() → indexed PNG (deutlich kleiner)
```

`Render()` gibt weiterhin `*image.NRGBA` zurück — keine Signaturänderung. Die Quantisierung ist ein nachgelagerter Schritt im Handler/CLI.

---

## Cache

Kein Änderungsbedarf. Der Cache-Key basiert auf Template-Name + JSON-Body + Mod-Time der `template.yaml`. Da `colors` Teil der `template.yaml` ist, wird eine Änderung des Feldes automatisch zu einem Cache-Miss.

---

## Was nicht abgedeckt wird

- Dithering (bewusst ausgelassen — verschlechtert Lesbarkeit bei kleinen Texten)
- GIF-Output
- Laufzeit-Override von `colors` per Query-Parameter (nur YAML-Konfiguration)
