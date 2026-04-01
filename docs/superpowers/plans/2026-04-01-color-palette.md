# Color Palette Reduction Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Reduziere PNG-Ausgabe auf eine konfigurierbare Anzahl Farben (2–256) via Median-Cut-Quantisierung, um die Dateigröße für TFT-Displays an Mikrocontrollern zu verringern.

**Architecture:** `canvas.colors` in der YAML steuert die Quantisierung. Nach `Renderer.Render()` (bleibt `*image.NRGBA`) wird — falls `colors > 0` — `renderer.Quantize()` aufgerufen, das ein `*image.Paletted` zurückgibt. `png.Encode` akzeptiert beide Typen als `image.Image`.

**Tech Stack:** Go stdlib (`image`, `image/color`, `image/png`, `sort`), keine neuen Deps.

---

## Datei-Übersicht

| Datei | Änderung |
|---|---|
| `internal/renderer/template.go` | `Canvas.Colors int` hinzufügen |
| `internal/renderer/renderer.go` | Validierung von `canvas.colors` in `LoadTemplate` |
| `internal/renderer/quantize.go` | Neu — `Quantize()` + Hilfsfunktionen (Median-Cut) |
| `internal/renderer/quantize_test.go` | Neu — Unit-Tests für `Quantize` |
| `internal/server/server.go` | `Quantize`-Aufruf nach `Render` im Handler |
| `internal/cli/render.go` | `Quantize`-Aufruf nach `Render` im CLI |
| `docs/yaml-template-spec.md` | `colors`-Feld dokumentieren |

---

## Projekt-Kontext

- Modul: `github.com/webfraggle/zza-generate-images`
- Go: 1.26.1
- `Renderer.Render()` gibt `*image.NRGBA` zurück (`internal/renderer/renderer.go:159`)
- `LoadTemplate` dekodiert YAML und gibt `*Template` zurück (`internal/renderer/renderer.go:43`)
- Server-Handler kodiert PNG in `internal/server/server.go:300–318`
- CLI kodiert PNG in `internal/cli/render.go:66`
- Tests laufen mit: `go test ./...`

---

### Task 1: `Canvas.Colors` Feld + Validierung

**Files:**
- Modify: `internal/renderer/template.go:31-34`
- Modify: `internal/renderer/renderer.go:43-64`
- Test: `internal/renderer/template_test.go`

- [ ] **Step 1: Schreibe den failing Test**

Füge in `internal/renderer/template_test.go` ans Ende:

```go
func TestLoadTemplate_ColorsValidation(t *testing.T) {
	dir := t.TempDir()
	write := func(yaml string) error {
		return os.WriteFile(filepath.Join(dir, "template.yaml"), []byte(yaml), 0644)
	}

	// colors: 0 (default, kein Feld) → valid
	if err := write("meta:\n  canvas:\n    width: 10\n    height: 10\n"); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadTemplate(filepath.Dir(dir), filepath.Base(dir)); err != nil {
		t.Errorf("colors omitted: unexpected error: %v", err)
	}

	// colors: 32 → valid
	if err := write("meta:\n  canvas:\n    width: 10\n    height: 10\n    colors: 32\n"); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadTemplate(filepath.Dir(dir), filepath.Base(dir)); err != nil {
		t.Errorf("colors: 32: unexpected error: %v", err)
	}

	// colors: 1 → invalid
	if err := write("meta:\n  canvas:\n    width: 10\n    height: 10\n    colors: 1\n"); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadTemplate(filepath.Dir(dir), filepath.Base(dir)); err == nil {
		t.Error("colors: 1: expected error, got nil")
	}

	// colors: 257 → invalid
	if err := write("meta:\n  canvas:\n    width: 10\n    height: 10\n    colors: 257\n"); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadTemplate(filepath.Dir(dir), filepath.Base(dir)); err == nil {
		t.Error("colors: 257: expected error, got nil")
	}
}
```

Füge in die Imports von `template_test.go` hinzu (die Datei hat bereits `testing` und `gopkg.in/yaml.v3`):

```go
import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)
```

- [ ] **Step 2: Test schlägt fehl**

```bash
go test ./internal/renderer/ -run TestLoadTemplate_ColorsValidation -v
```

Erwartete Ausgabe: `FAIL` — `LoadTemplate` kennt `colors` noch nicht.

- [ ] **Step 3: `Canvas.Colors` Feld hinzufügen**

In `internal/renderer/template.go`, ersetze:

```go
// Canvas defines the output image dimensions.
type Canvas struct {
	Width  int `yaml:"width"`
	Height int `yaml:"height"`
}
```

durch:

```go
// Canvas defines the output image dimensions.
type Canvas struct {
	Width  int `yaml:"width"`
	Height int `yaml:"height"`
	Colors int `yaml:"colors"` // 0 = no reduction (default); 2–256 = indexed PNG output
}
```

- [ ] **Step 4: Validierung in `LoadTemplate` hinzufügen**

In `internal/renderer/renderer.go`, ersetze:

```go
	tmpl.Dir = tmplPath
	return &tmpl, nil
}
```

durch:

```go
	tmpl.Dir = tmplPath

	if c := tmpl.Meta.Canvas.Colors; c != 0 && (c < 2 || c > 256) {
		return nil, fmt.Errorf("renderer: LoadTemplate: canvas.colors must be between 2 and 256 (got %d)", c)
	}

	return &tmpl, nil
}
```

- [ ] **Step 5: Test läuft durch**

```bash
go test ./internal/renderer/ -run TestLoadTemplate_ColorsValidation -v
```

Erwartete Ausgabe: `PASS`

- [ ] **Step 6: Gesamte Test-Suite grün**

```bash
go test ./...
```

Erwartete Ausgabe: alle Tests `PASS`, kein Fehler.

- [ ] **Step 7: Commit**

```bash
git add internal/renderer/template.go internal/renderer/renderer.go internal/renderer/template_test.go
git commit -m "feat: add canvas.colors field with validation to template config"
```

---

### Task 2: `Quantize()` Funktion

**Files:**
- Create: `internal/renderer/quantize.go`
- Create: `internal/renderer/quantize_test.go`

- [ ] **Step 1: Schreibe die failing Tests**

Erstelle `internal/renderer/quantize_test.go`:

```go
package renderer

import (
	"image"
	"image/color"
	"testing"
)

// makeNRGBA creates a w×h NRGBA image filled with the given color.
func makeNRGBA(w, h int, c color.NRGBA) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetNRGBA(x, y, c)
		}
	}
	return img
}

// paletteContains reports whether p contains c (by RGBA equality).
func paletteContains(p color.Palette, c color.Color) bool {
	r1, g1, b1, a1 := c.RGBA()
	for _, pc := range p {
		r2, g2, b2, a2 := pc.RGBA()
		if r1 == r2 && g1 == g2 && b1 == b2 && a1 == a2 {
			return true
		}
	}
	return false
}

func TestQuantize_OutputDimensions(t *testing.T) {
	src := makeNRGBA(10, 5, color.NRGBA{R: 100, G: 150, B: 200, A: 255})
	out := Quantize(src, 4)
	if out.Bounds() != src.Bounds() {
		t.Errorf("bounds: got %v, want %v", out.Bounds(), src.Bounds())
	}
}

func TestQuantize_PaletteSizeDoesNotExceedN(t *testing.T) {
	src := makeNRGBA(20, 20, color.NRGBA{R: 255, G: 0, B: 0, A: 255})
	// Two-color image: half red, half blue
	for y := 0; y < 20; y++ {
		for x := 10; x < 20; x++ {
			src.SetNRGBA(x, y, color.NRGBA{R: 0, G: 0, B: 255, A: 255})
		}
	}
	out := Quantize(src, 8)
	if len(out.Palette) > 8 {
		t.Errorf("palette size %d exceeds n=8", len(out.Palette))
	}
}

func TestQuantize_SingleColorImage(t *testing.T) {
	red := color.NRGBA{R: 200, G: 50, B: 50, A: 255}
	src := makeNRGBA(8, 8, red)
	out := Quantize(src, 4)
	// All output pixels must map to the same (or very close) color
	idx0 := out.ColorIndexAt(0, 0)
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			if out.ColorIndexAt(x, y) != idx0 {
				t.Errorf("pixel (%d,%d) has different palette index", x, y)
			}
		}
	}
}

func TestQuantize_ClampN_TooSmall(t *testing.T) {
	src := makeNRGBA(4, 4, color.NRGBA{R: 100, G: 100, B: 100, A: 255})
	out := Quantize(src, 0) // clamped to 2
	if len(out.Palette) < 1 {
		t.Error("palette must not be empty")
	}
}

func TestQuantize_ClampN_TooLarge(t *testing.T) {
	src := makeNRGBA(4, 4, color.NRGBA{R: 100, G: 100, B: 100, A: 255})
	out := Quantize(src, 300) // clamped to 256
	if len(out.Palette) > 256 {
		t.Errorf("palette size %d exceeds maximum 256", len(out.Palette))
	}
}

func TestQuantize_AllPixelsValidPaletteIndex(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 16, 16))
	// Fill with gradient
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			src.SetNRGBA(x, y, color.NRGBA{R: uint8(x * 16), G: uint8(y * 16), B: 128, A: 255})
		}
	}
	out := Quantize(src, 16)
	maxIdx := uint8(len(out.Palette) - 1)
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			idx := out.ColorIndexAt(x, y)
			if idx > maxIdx {
				t.Errorf("pixel (%d,%d) has index %d >= palette size %d", x, y, idx, len(out.Palette))
			}
		}
	}
}
```

- [ ] **Step 2: Tests schlagen fehl**

```bash
go test ./internal/renderer/ -run TestQuantize -v
```

Erwartete Ausgabe: `FAIL` — `Quantize` nicht definiert.

- [ ] **Step 3: Implementiere `quantize.go`**

Erstelle `internal/renderer/quantize.go`:

```go
package renderer

import (
	"image"
	"image/color"
	"sort"
)

// Quantize reduces src to a palette of at most n colors using the median-cut
// algorithm and returns an indexed *image.Paletted. n is clamped to [2, 256].
// No dithering is applied — each pixel is mapped to its nearest palette color.
func Quantize(src *image.NRGBA, n int) *image.Paletted {
	if n < 2 {
		n = 2
	}
	if n > 256 {
		n = 256
	}

	bounds := src.Bounds()
	pixels := make([]color.NRGBA, 0, bounds.Dx()*bounds.Dy())
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			pixels = append(pixels, src.NRGBAAt(x, y))
		}
	}

	palette := medianCut(pixels, n)

	p := make(color.Palette, len(palette))
	for i, c := range palette {
		p[i] = c
	}

	out := image.NewPaletted(bounds, p)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			out.SetColorIndex(x, y, uint8(p.Index(src.NRGBAAt(x, y))))
		}
	}
	return out
}

// medianCut partitions pixels into at most n buckets using the median-cut
// algorithm and returns the average color of each bucket as a palette.
func medianCut(pixels []color.NRGBA, n int) []color.NRGBA {
	if len(pixels) == 0 {
		return []color.NRGBA{{R: 0, G: 0, B: 0, A: 255}}
	}

	buckets := [][]color.NRGBA{append([]color.NRGBA(nil), pixels...)}

	for len(buckets) < n {
		idx := bucketWithLargestRange(buckets)
		b := buckets[idx]
		if len(b) <= 1 {
			break // cannot split further
		}
		axis := dominantAxis(b)
		sortByAxis(b, axis)
		mid := len(b) / 2
		buckets[idx] = b[:mid]
		buckets = append(buckets, b[mid:])
	}

	result := make([]color.NRGBA, len(buckets))
	for i, b := range buckets {
		result[i] = avgColor(b)
	}
	return result
}

// bucketWithLargestRange returns the index of the bucket with the greatest
// color range along any single axis.
func bucketWithLargestRange(buckets [][]color.NRGBA) int {
	best, bestRange := 0, -1
	for i, b := range buckets {
		_, r := dominantAxisAndRange(b)
		if r > bestRange {
			bestRange = r
			best = i
		}
	}
	return best
}

// dominantAxis returns the axis (0=R, 1=G, 2=B) with the largest value range in b.
func dominantAxis(b []color.NRGBA) int {
	ax, _ := dominantAxisAndRange(b)
	return ax
}

// dominantAxisAndRange returns the dominant axis and its value range for bucket b.
func dominantAxisAndRange(b []color.NRGBA) (axis int, rangeVal int) {
	if len(b) == 0 {
		return 0, 0
	}
	minR, maxR := int(b[0].R), int(b[0].R)
	minG, maxG := int(b[0].G), int(b[0].G)
	minB, maxB := int(b[0].B), int(b[0].B)
	for _, c := range b[1:] {
		if int(c.R) < minR {
			minR = int(c.R)
		}
		if int(c.R) > maxR {
			maxR = int(c.R)
		}
		if int(c.G) < minG {
			minG = int(c.G)
		}
		if int(c.G) > maxG {
			maxG = int(c.G)
		}
		if int(c.B) < minB {
			minB = int(c.B)
		}
		if int(c.B) > maxB {
			maxB = int(c.B)
		}
	}
	rR, rG, rB := maxR-minR, maxG-minG, maxB-minB
	if rR >= rG && rR >= rB {
		return 0, rR
	}
	if rG >= rB {
		return 1, rG
	}
	return 2, rB
}

// sortByAxis sorts b in-place by the given axis (0=R, 1=G, 2=B).
func sortByAxis(b []color.NRGBA, axis int) {
	sort.Slice(b, func(i, j int) bool {
		switch axis {
		case 0:
			return b[i].R < b[j].R
		case 1:
			return b[i].G < b[j].G
		default:
			return b[i].B < b[j].B
		}
	})
}

// avgColor returns the per-channel average of all pixels in b.
func avgColor(b []color.NRGBA) color.NRGBA {
	if len(b) == 0 {
		return color.NRGBA{A: 255}
	}
	var sumR, sumG, sumB int
	for _, c := range b {
		sumR += int(c.R)
		sumG += int(c.G)
		sumB += int(c.B)
	}
	n := len(b)
	return color.NRGBA{
		R: uint8(sumR / n),
		G: uint8(sumG / n),
		B: uint8(sumB / n),
		A: 255,
	}
}
```

- [ ] **Step 4: Tests laufen durch**

```bash
go test ./internal/renderer/ -run TestQuantize -v
```

Erwartete Ausgabe: alle 6 Tests `PASS`.

- [ ] **Step 5: Gesamte Test-Suite grün**

```bash
go test ./...
```

Erwartete Ausgabe: alle Tests `PASS`.

- [ ] **Step 6: Commit**

```bash
git add internal/renderer/quantize.go internal/renderer/quantize_test.go
git commit -m "feat: add median-cut color quantizer"
```

---

### Task 3: Quantize-Aufruf im Server-Handler

**Files:**
- Modify: `internal/server/server.go:1-24` (Imports) und `internal/server/server.go:292-318` (Handler)

- [ ] **Step 1: Schreibe den failing Test**

In `internal/server/server_test.go` gibt es bereits Tests für den Render-Handler. Füge einen neuen Test hinzu, der ein Template mit `canvas.colors: 2` rendert und prüft, dass die Antwort ein gültiges PNG ist.

Der Test erstellt seine eigene temporäre Templates-Verzeichnisstruktur und einen eigenen Server — so bleibt er unabhängig von den echten Templates im Repo.

Füge in `internal/server/server_test.go` ans Ende:

```go
func TestHandleRender_WithColorPalette(t *testing.T) {
	// Eigenes TemplatesDir mit einem Mini-Template das colors: 2 setzt.
	templatesDir := t.TempDir()
	tmplDir := filepath.Join(templatesDir, "palette-test")
	if err := os.MkdirAll(tmplDir, 0755); err != nil {
		t.Fatal(err)
	}
	tmplYAML := "meta:\n  canvas:\n    width: 4\n    height: 4\n    colors: 2\nlayers:\n  - type: rect\n    x: 0\n    y: 0\n    width: 4\n    height: 4\n    color: \"#FF0000\"\n"
	if err := os.WriteFile(filepath.Join(tmplDir, "template.yaml"), []byte(tmplYAML), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		Port:             "8080",
		TemplatesDir:     templatesDir,
		CacheDir:         t.TempDir(),
		CacheMaxAgeHours: 1,
		CacheMaxSizeMB:   10,
	}
	srv, err := New(cfg, web.FS)
	if err != nil {
		t.Fatal(err)
	}

	body := strings.NewReader("{}")
	req := httptest.NewRequest(http.MethodPost, "/palette-test/render", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("render with palette: got %d, body: %s", rr.Code, rr.Body.String())
	}
	if ct := rr.Header().Get("Content-Type"); ct != "image/png" {
		t.Errorf("Content-Type: got %q, want image/png", ct)
	}
	b := rr.Body.Bytes()
	if len(b) < 8 || b[0] != 0x89 || b[1] != 0x50 || b[2] != 0x4E || b[3] != 0x47 {
		t.Error("response body is not a valid PNG")
	}
}
```

`path/filepath` und `config` sind bereits importiert. `os` fehlt noch — ergänze es im Import-Block von `server_test.go`:

```go
import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/webfraggle/zza-generate-images/internal/config"
	"github.com/webfraggle/zza-generate-images/web"
)
```

- [ ] **Step 2: Test schlägt fehl**

```bash
go test ./internal/server/ -run TestHandleRender_WithColorPalette -v
```

Erwartete Ausgabe: `FAIL` — der Handler ignoriert `canvas.colors` noch.

- [ ] **Step 3: Import `"image"` in `server.go` ergänzen**

In `internal/server/server.go`, ersetze den Import-Block (Zeilen 3–24):

```go
import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"image"
	"image/png"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/webfraggle/zza-generate-images/internal/config"
	"github.com/webfraggle/zza-generate-images/internal/gallery"
	"github.com/webfraggle/zza-generate-images/internal/renderer"
	"github.com/webfraggle/zza-generate-images/internal/version"
)
```

- [ ] **Step 4: Quantize-Aufruf im Handler einbauen**

In `internal/server/server.go`, ersetze den Abschnitt nach `Render` (Zeilen ~292–306):

```go
	// Render.
	img, err := s.rend.Render(tmpl, data)
	if err != nil {
		log.Printf("render: %q: %v", templateName, err)
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}

	// Optionally reduce color palette.
	var encImg image.Image = img
	if tmpl.Meta.Canvas.Colors > 0 {
		encImg = renderer.Quantize(img, tmpl.Meta.Canvas.Colors)
	}

	// Encode PNG.
	var buf bytes.Buffer
	if err := png.Encode(&buf, encImg); err != nil {
		http.Error(w, "PNG encode error", http.StatusInternalServerError)
		log.Printf("render: png encode %q: %v", templateName, err)
		return
	}
```

- [ ] **Step 5: Test läuft durch**

```bash
go test ./internal/server/ -run TestHandleRender_WithColorPalette -v
```

Erwartete Ausgabe: `PASS`

- [ ] **Step 6: Gesamte Test-Suite grün**

```bash
go test ./...
```

Erwartete Ausgabe: alle Tests `PASS`.

- [ ] **Step 7: Commit**

```bash
git add internal/server/server.go internal/server/server_test.go
git commit -m "feat: apply color palette quantization in render handler"
```

---

### Task 4: Quantize-Aufruf im CLI

**Files:**
- Modify: `internal/cli/render.go`

- [ ] **Step 1: Schreibe den failing Test**

Der CLI-Code hat keinen eigenen Test für PNG-Kodierung — das wird durch manuellen Test abgedeckt. Stattdessen: prüfe, dass der Code kompiliert. Füge in `internal/cli/render.go` keinen Test hinzu — stattdessen ist der Kompilierungsschritt der Test.

```bash
go build ./...
```

Erwartete Ausgabe: kein Fehler (Basis-Check vor der Änderung).

- [ ] **Step 2: Import `"image"` in `render.go` ergänzen**

In `internal/cli/render.go`, ersetze den Import-Block:

```go
import (
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"os"

	"github.com/spf13/cobra"
	"github.com/webfraggle/zza-generate-images/internal/renderer"
)
```

- [ ] **Step 3: Quantize-Aufruf nach `Render` einbauen**

In `internal/cli/render.go`, ersetze:

```go
			// Render.
			r := renderer.New(templatesDir)
			img, err := r.Render(tmpl, data)
			if err != nil {
				return fmt.Errorf("render: rendering: %w", err)
			}

			// Write PNG output.
			outF, err := os.Create(outputFile)
```

durch:

```go
			// Render.
			r := renderer.New(templatesDir)
			img, err := r.Render(tmpl, data)
			if err != nil {
				return fmt.Errorf("render: rendering: %w", err)
			}

			// Optionally reduce color palette.
			var encImg image.Image = img
			if tmpl.Meta.Canvas.Colors > 0 {
				encImg = renderer.Quantize(img, tmpl.Meta.Canvas.Colors)
			}

			// Write PNG output.
			outF, err := os.Create(outputFile)
```

- [ ] **Step 4: `png.Encode` auf `encImg` umstellen**

In `internal/cli/render.go`, ersetze:

```go
			if err := png.Encode(outF, img); err != nil {
```

durch:

```go
			if err := png.Encode(outF, encImg); err != nil {
```

- [ ] **Step 5: Kompiliert ohne Fehler**

```bash
go build ./...
```

Erwartete Ausgabe: kein Fehler, kein Output.

- [ ] **Step 6: Gesamte Test-Suite grün**

```bash
go test ./...
```

Erwartete Ausgabe: alle Tests `PASS`.

- [ ] **Step 7: Commit**

```bash
git add internal/cli/render.go
git commit -m "feat: apply color palette quantization in CLI render command"
```

---

### Task 5: Dokumentation

**Files:**
- Modify: `docs/yaml-template-spec.md`

- [ ] **Step 1: `colors`-Feld in den canvas-Block einfügen**

In `docs/yaml-template-spec.md`, ersetze den canvas-Abschnitt:

```yaml
  canvas:
    width: 160    # Breite in Pixeln
    height: 80    # Höhe in Pixeln
```

durch:

```yaml
  canvas:
    width: 160    # Breite in Pixeln
    height: 80    # Höhe in Pixeln
    colors: 32    # optional — Farbpalette reduzieren (2–256); weglassen = volle Farbe
```

- [ ] **Step 2: Erklärungstext nach dem canvas-Block ergänzen**

Nach der Zeile:
```
- `instructions` (optional) — Freitext-Anleitung für Nutzer des Templates. Wird auf der Vorschau-Seite angezeigt. Mehrzeilig möglich mit YAML Block-Scalar (`|`).
```

Füge hinzu:

```
- `canvas.colors` (optional) — Reduziert die Ausgabe auf eine Indexed-PNG mit maximal N Farben (2–256) via Median-Cut-Algorithmus. Verringert die Dateigröße bei einfachen Grafiken erheblich. Weglassen oder `0` = volle 32-Bit-Farbe.
```

- [ ] **Step 3: Commit**

```bash
git add docs/yaml-template-spec.md
git commit -m "docs: document canvas.colors palette reduction field"
```

---

## Abschluss

Nach Task 5:

```bash
go test ./...
```

Alle Tests grün → Branch `feature/color-palette` ist bereit zum Review und Merge in `develop`.
