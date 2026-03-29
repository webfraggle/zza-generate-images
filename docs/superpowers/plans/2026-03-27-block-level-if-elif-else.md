# Block-Level if/elif/else Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add block-level `if`/`elif`/`else` to the YAML layer list, grouping multiple layers under one condition without changing existing layer-level if/elif/else behavior.

**Architecture:** A layer entry without a `type:` field but with `if:`/`elif:`/`else:` and `layers:` is a block node. The renderer detects this (by `layer.Type == ""`) and renders sub-layers recursively. Common iteration logic is extracted into `renderLayers(dst, tmpl, layers, eval, inLoop bool)`, used by `Render`, `renderLoop`, and block nodes themselves. No breaking change — existing templates work unmodified.

**Tech Stack:** Go, gopkg.in/yaml.v3

---

## File Structure

| File | Change |
|------|--------|
| `internal/renderer/template.go` | Add `ElseMarker` type; change `Layer.Else` field type |
| `internal/renderer/renderer.go` | Extract `renderLayers`; add block node dispatch |
| `internal/renderer/template_test.go` | New: tests for `ElseMarker` YAML parsing |
| `internal/renderer/renderer_test.go` | New: tests for block-level rendering |
| `docs/yaml-template-spec.md` | Document block-level if/elif/else syntax |
| `docs/user-guide-templates.md` | Add usage examples |

---

### Task 1: Null-safe `else:` YAML syntax (`ElseMarker`)

Template authors write `- else:` without a value. In YAML this is a null node, which `bool` would parse as `false`. We need a custom type.

**Files:**
- Modify: `internal/renderer/template.go`
- Create: `internal/renderer/template_test.go`

- [ ] **Step 1: Write failing test**

Create `internal/renderer/template_test.go`:

```go
package renderer

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestElseMarker_UnmarshalNull(t *testing.T) {
	// `else:` with no value → YAML null → must parse as true.
	input := "else:\n"
	var m struct {
		Else ElseMarker `yaml:"else"`
	}
	if err := yaml.Unmarshal([]byte(input), &m); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if !bool(m.Else) {
		t.Errorf("expected Else=true for null value, got false")
	}
}

func TestElseMarker_UnmarshalTrue(t *testing.T) {
	input := "else: true\n"
	var m struct {
		Else ElseMarker `yaml:"else"`
	}
	if err := yaml.Unmarshal([]byte(input), &m); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if !bool(m.Else) {
		t.Errorf("expected Else=true, got false")
	}
}

func TestElseMarker_UnmarshalFalse(t *testing.T) {
	input := "else: false\n"
	var m struct {
		Else ElseMarker `yaml:"else"`
	}
	if err := yaml.Unmarshal([]byte(input), &m); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if bool(m.Else) {
		t.Errorf("expected Else=false, got true")
	}
}
```

- [ ] **Step 2: Run tests to confirm they fail**

```bash
go test ./internal/renderer/ -run TestElseMarker -v
```

Expected: FAIL with `undefined: ElseMarker`

- [ ] **Step 3: Add `ElseMarker` to `template.go`**

In `internal/renderer/template.go`, add after the `maxCondBranches` constant (around line 120):

```go
// ElseMarker is a bool field that also accepts YAML null as true.
// This allows template authors to write either `else:` or `else: true`.
type ElseMarker bool

// UnmarshalYAML implements yaml.Unmarshaler.
// Null (from bare `else:` with no value) is treated as true.
func (e *ElseMarker) UnmarshalYAML(value *yaml.Node) error {
	if value.Tag == "!!null" {
		*e = true
		return nil
	}
	var b bool
	if err := value.Decode(&b); err != nil {
		return fmt.Errorf("renderer: ElseMarker: expected bool or null, got %q", value.Value)
	}
	*e = ElseMarker(b)
	return nil
}
```

In the `Layer` struct, change the `Else` field from:
```go
Else     bool         `yaml:"else"` // else: true — renders when preceding if/elif chain was not satisfied
```
to:
```go
Else     ElseMarker   `yaml:"else"` // else: or else: true — renders when preceding if/elif chain was not satisfied
```

Note: `type ElseMarker bool` behaves as a boolean in all `if layer.Else` expressions — no other changes needed.

- [ ] **Step 4: Run tests to confirm they pass**

```bash
go test ./internal/renderer/ -run TestElseMarker -v
```

Expected: PASS (3 tests)

- [ ] **Step 5: Run full test suite — confirm no regression**

```bash
go test ./...
```

Expected: all existing tests pass

- [ ] **Step 6: Commit**

```bash
git add internal/renderer/template.go internal/renderer/template_test.go
git commit -m "feat(renderer): add ElseMarker — null-safe else: YAML syntax"
```

---

### Task 2: Failing tests for block-level rendering

Write all tests before touching the renderer. They must fail at this point.

**Files:**
- Create: `internal/renderer/renderer_test.go`

- [ ] **Step 1: Create renderer_test.go with helpers and tests**

Create `internal/renderer/renderer_test.go`:

```go
package renderer

import (
	"image"
	"image/color"
	"testing"
)

// --- test helpers ---

// rectLayer returns a rect Layer with the given geometry and hex color.
func rectLayer(x, y, w, h int, hexColor string) Layer {
	return Layer{
		Type:   "rect",
		X:      IntOrExpr{val: x},
		Y:      IntOrExpr{val: y},
		Width:  IntOrExpr{val: w},
		Height: IntOrExpr{val: h},
		Color:  StringOrCond{raw: hexColor},
	}
}

// blockIf returns a block-level if node (no type, has condition + sub-layers).
func blockIf(cond string, layers []Layer) Layer {
	return Layer{If: cond, Layers: layers}
}

// blockElif returns a block-level elif node.
func blockElif(cond string, layers []Layer) Layer {
	return Layer{Elif: cond, Layers: layers}
}

// blockElse returns a block-level else node.
func blockElse(layers []Layer) Layer {
	return Layer{Else: true, Layers: layers}
}

// makeTemplate creates a minimal in-memory Template (no fonts/files needed for rect layers).
func makeTemplate(w, h int, layers []Layer) *Template {
	return &Template{
		Meta:   Meta{Canvas: Canvas{Width: w, Height: h}},
		Layers: layers,
	}
}

// nrgbaAt returns the NRGBA color at pixel (x, y).
func nrgbaAt(img *image.NRGBA, x, y int) color.NRGBA {
	return img.NRGBAAt(x, y)
}

// mustRender renders tmpl and fails the test on error.
func mustRender(t *testing.T, r *Renderer, tmpl *Template, data map[string]interface{}) *image.NRGBA {
	t.Helper()
	img, err := r.Render(tmpl, data)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}
	return img
}

// --- block-level if tests ---

// TestRender_BlockIf_True: condition true → sub-layers render.
func TestRender_BlockIf_True(t *testing.T) {
	r := New("/tmp") // no file assets needed for rect-only templates
	tmpl := makeTemplate(10, 10, []Layer{
		blockIf("eq(flag, '1')", []Layer{
			rectLayer(0, 0, 4, 4, "#ff0000"),
		}),
	})

	img := mustRender(t, r, tmpl, map[string]interface{}{"flag": "1"})

	got := nrgbaAt(img, 0, 0)
	want := color.NRGBA{R: 0xff, A: 0xff}
	if got != want {
		t.Errorf("pixel (0,0) = %v, want %v", got, want)
	}
}

// TestRender_BlockIf_False: condition false → sub-layers do not render.
func TestRender_BlockIf_False(t *testing.T) {
	r := New("/tmp")
	tmpl := makeTemplate(10, 10, []Layer{
		blockIf("eq(flag, '1')", []Layer{
			rectLayer(0, 0, 4, 4, "#ff0000"),
		}),
	})

	img := mustRender(t, r, tmpl, map[string]interface{}{"flag": "0"})

	got := nrgbaAt(img, 0, 0)
	if got.A != 0 {
		t.Errorf("pixel should be transparent (not rendered), got %v", got)
	}
}

// TestRender_BlockIfElifElse: first matching branch renders, others don't.
func TestRender_BlockIfElifElse(t *testing.T) {
	r := New("/tmp")
	tmpl := makeTemplate(10, 10, []Layer{
		blockIf("eq(x, 'a')", []Layer{
			rectLayer(0, 0, 4, 4, "#ff0000"), // red
		}),
		blockElif("eq(x, 'b')", []Layer{
			rectLayer(0, 0, 4, 4, "#00ff00"), // green
		}),
		blockElse([]Layer{
			rectLayer(0, 0, 4, 4, "#0000ff"), // blue
		}),
	})

	cases := []struct {
		x    string
		want color.NRGBA
		name string
	}{
		{"a", color.NRGBA{R: 0xff, A: 0xff}, "x=a → red"},
		{"b", color.NRGBA{G: 0xff, A: 0xff}, "x=b → green"},
		{"c", color.NRGBA{B: 0xff, A: 0xff}, "x=c → blue (else)"},
	}

	for _, tc := range cases {
		img := mustRender(t, r, tmpl, map[string]interface{}{"x": tc.x})
		got := nrgbaAt(img, 0, 0)
		if got != tc.want {
			t.Errorf("%s: pixel (0,0) = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// TestRender_BlockIf_MultipleSublayers: all sub-layers in a matching block render.
func TestRender_BlockIf_MultipleSublayers(t *testing.T) {
	r := New("/tmp")
	tmpl := makeTemplate(10, 10, []Layer{
		blockIf("eq(flag, '1')", []Layer{
			rectLayer(0, 0, 2, 2, "#ff0000"), // red at (0,0)
			rectLayer(6, 6, 2, 2, "#00ff00"), // green at (6,6)
		}),
	})

	img := mustRender(t, r, tmpl, map[string]interface{}{"flag": "1"})

	if got := nrgbaAt(img, 0, 0); got != (color.NRGBA{R: 0xff, A: 0xff}) {
		t.Errorf("pixel (0,0) = %v, want red", got)
	}
	if got := nrgbaAt(img, 6, 6); got != (color.NRGBA{G: 0xff, A: 0xff}) {
		t.Errorf("pixel (6,6) = %v, want green", got)
	}
}

// TestRender_BlockIf_Nested: block if inside another block if.
func TestRender_BlockIf_Nested(t *testing.T) {
	r := New("/tmp")
	tmpl := makeTemplate(10, 10, []Layer{
		blockIf("eq(outer, '1')", []Layer{
			blockIf("eq(inner, '1')", []Layer{
				rectLayer(0, 0, 4, 4, "#ff0000"), // red
			}),
			blockElse([]Layer{
				rectLayer(0, 0, 4, 4, "#00ff00"), // green
			}),
		}),
	})

	cases := []struct {
		outer, inner string
		want         color.NRGBA
		name         string
	}{
		{"1", "1", color.NRGBA{R: 0xff, A: 0xff}, "outer=1,inner=1 → red"},
		{"1", "0", color.NRGBA{G: 0xff, A: 0xff}, "outer=1,inner=0 → green"},
		{"0", "1", color.NRGBA{}, "outer=0 → nothing"},
	}

	for _, tc := range cases {
		img := mustRender(t, r, tmpl, map[string]interface{}{"outer": tc.outer, "inner": tc.inner})
		got := nrgbaAt(img, 0, 0)
		if tc.want.A == 0 {
			if got.A != 0 {
				t.Errorf("%s: pixel should be transparent, got %v", tc.name, got)
			}
		} else if got != tc.want {
			t.Errorf("%s: pixel (0,0) = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// TestRender_BlockElif_NoChain: elif without preceding if must return an error.
func TestRender_BlockElif_NoChain(t *testing.T) {
	r := New("/tmp")
	tmpl := makeTemplate(10, 10, []Layer{
		blockElif("eq(x, 'a')", []Layer{
			rectLayer(0, 0, 4, 4, "#ff0000"),
		}),
	})

	_, err := r.Render(tmpl, map[string]interface{}{"x": "a"})
	if err == nil {
		t.Error("expected error for elif without preceding if, got nil")
	}
}

// TestRender_BlockIf_MixedWithRegularLayers: block nodes and regular layers coexist.
func TestRender_BlockIf_MixedWithRegular(t *testing.T) {
	r := New("/tmp")
	// Regular layer always renders (blue background).
	// Block if (true) renders a red rect on top.
	tmpl := makeTemplate(10, 10, []Layer{
		rectLayer(0, 0, 10, 10, "#0000ff"), // always: blue background
		blockIf("eq(flag, '1')", []Layer{
			rectLayer(2, 2, 4, 4, "#ff0000"), // conditional: red square
		}),
	})

	img := mustRender(t, r, tmpl, map[string]interface{}{"flag": "1"})

	// Corner (0,0) is blue (not covered by red square).
	if got := nrgbaAt(img, 0, 0); got != (color.NRGBA{B: 0xff, A: 0xff}) {
		t.Errorf("pixel (0,0) = %v, want blue", got)
	}
	// Center (2,2) is red (covered by red square).
	if got := nrgbaAt(img, 2, 2); got != (color.NRGBA{R: 0xff, A: 0xff}) {
		t.Errorf("pixel (2,2) = %v, want red", got)
	}
}
```

- [ ] **Step 2: Run tests to confirm they fail**

```bash
go test ./internal/renderer/ -run TestRender_Block -v
```

Expected: FAIL — block nodes (`Type == ""`) currently hit the `default: return nil, fmt.Errorf("... unknown type %q", ...)` path.

- [ ] **Step 3: Commit test file**

```bash
git add internal/renderer/renderer_test.go
git commit -m "test(renderer): add failing tests for block-level if/elif/else"
```

---

### Task 3: Extract `renderLayers` and add block node dispatch

**Files:**
- Modify: `internal/renderer/renderer.go`

- [ ] **Step 1: Add `renderLayers` helper to `renderer.go`**

Insert this function **before** `Render` (before line 73):

```go
// renderLayers renders a slice of layers in order, handling:
//   - Layer-level if/elif/else chains on typed layers
//   - Block-level if/elif/else nodes (Type == "", has Layers)
//
// Set inLoop=true when called from renderLoop to prevent nested loops.
func (r *Renderer) renderLayers(dst *image.NRGBA, tmpl *Template, layers []Layer, eval *Evaluator, inLoop bool) error {
	chainSatisfied := false
	inChain := false
	for i, layer := range layers {
		render := false
		switch {
		case layer.Elif != "" || layer.Else:
			if !inChain {
				return fmt.Errorf("layer %d: elif/else without preceding if", i)
			}
			if !chainSatisfied {
				if layer.Else {
					render = true
				} else if eval.EvalCondition(layer.Elif) {
					chainSatisfied = true
					render = true
				}
			}
		case layer.If != "":
			inChain = true
			chainSatisfied = false
			if eval.EvalCondition(layer.If) {
				chainSatisfied = true
				render = true
			}
		default:
			inChain = false
			chainSatisfied = false
			render = true
		}
		if !render {
			continue
		}

		// Block-level node: no type, has sub-layers.
		if layer.Type == "" {
			if err := r.renderLayers(dst, tmpl, layer.Layers, eval, inLoop); err != nil {
				return fmt.Errorf("block layer %d: %w", i, err)
			}
			continue
		}

		// Typed layer dispatch.
		var err error
		switch layer.Type {
		case "image":
			err = r.renderImage(dst, tmpl, layer, eval)
		case "rect":
			err = r.renderRect(dst, layer, eval)
		case "text":
			err = r.renderText(dst, tmpl, layer, eval)
		case "copy":
			err = renderCopy(dst, layer, eval)
		case "loop":
			if inLoop {
				return fmt.Errorf("layer %d: nested loops are not supported", i)
			}
			err = r.renderLoop(dst, tmpl, layer, eval)
		default:
			return fmt.Errorf("layer %d: unknown type %q", i, layer.Type)
		}
		if err != nil {
			return fmt.Errorf("layer %d (%s): %w", i, layer.Type, err)
		}
	}
	return nil
}
```

- [ ] **Step 2: Replace the loop in `Render` with a `renderLayers` call**

In `Render`, replace everything from `chainSatisfied := false` through `return dst, nil` (the entire layer loop) with:

```go
	if err := r.renderLayers(dst, tmpl, tmpl.Layers, eval, false); err != nil {
		return nil, fmt.Errorf("renderer: Render: %w", err)
	}
	return dst, nil
```

The complete `Render` function body after the canvas/eval setup becomes:

```go
func (r *Renderer) Render(tmpl *Template, data map[string]interface{}) (*image.NRGBA, error) {
	w, h := tmpl.Meta.Canvas.Width, tmpl.Meta.Canvas.Height
	if w <= 0 || h <= 0 {
		return nil, fmt.Errorf("renderer: Render: invalid canvas dimensions %dx%d", w, h)
	}
	if w > maxCanvasDimension || h > maxCanvasDimension {
		return nil, fmt.Errorf("renderer: Render: canvas dimensions %dx%d exceed maximum %d", w, h, maxCanvasDimension)
	}
	if len(tmpl.Layers) > maxLayers {
		return nil, fmt.Errorf("renderer: Render: template has %d layers, maximum is %d", len(tmpl.Layers), maxLayers)
	}

	dst := image.NewNRGBA(image.Rect(0, 0, w, h))
	eval := NewEvaluator(data)

	if err := r.renderLayers(dst, tmpl, tmpl.Layers, eval, false); err != nil {
		return nil, fmt.Errorf("renderer: Render: %w", err)
	}
	return dst, nil
}
```

- [ ] **Step 3: Replace the inner loop in `renderLoop` with a `renderLayers` call**

In `renderLoop`, replace the inner for-loop over `layer.Layers` (the `subChainSatisfied`/`subInChain` block) with:

```go
		if err := r.renderLayers(dst, tmpl, layer.Layers, childEval, true); err != nil {
			return fmt.Errorf("renderLoop: item %d: %w", displayIdx, err)
		}
```

The complete inner-loop section of `renderLoop` becomes:

```go
	displayIdx := 0
	for _, item := range strings.Split(value, sep) {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if displayIdx >= maxItems {
			break
		}

		strVars := map[string]string{}
		if layer.Var != "" {
			strVars[layer.Var] = item
		}
		childEval := eval.withLoopVars(strVars, map[string]int{
			"i":          displayIdx,
			"loop.index": displayIdx,
		})

		if err := r.renderLayers(dst, tmpl, layer.Layers, childEval, true); err != nil {
			return fmt.Errorf("renderLoop: item %d: %w", displayIdx, err)
		}
		displayIdx++
	}
	return nil
```

- [ ] **Step 4: Run all tests**

```bash
go test ./... -v 2>&1 | tail -40
```

Expected: all tests pass, including the new `TestRender_Block*` tests.

- [ ] **Step 5: Commit**

```bash
git add internal/renderer/renderer.go
git commit -m "feat(renderer): add block-level if/elif/else; extract renderLayers helper"
```

---

### Task 4: Update docs

**Files:**
- Modify: `docs/yaml-template-spec.md`
- Modify: `docs/user-guide-templates.md`

- [ ] **Step 1: Add block-level section to `docs/yaml-template-spec.md`**

Find the existing `if`/`elif`/`else` section in the spec and add a new subsection **Block-Level if/elif/else** directly after it:

````markdown
### Block-Level if/elif/else

Mehrere Layer können unter einer gemeinsamen Bedingung gruppiert werden.
Ein Block-Eintrag hat **kein `type:`**, aber `if:`/`elif:`/`else:` und `layers:`.

```yaml
layers:
  - if: "startsWith(zug1.nr, 'ICN')"
    layers:
      - type: image
        file: icn.png
      - type: text
        value: "ICN Express"
        x: 5
        y: 20

  - elif: "startsWith(zug1.nr, 'IC')"
    layers:
      - type: image
        file: ic.png

  - else:
    layers:
      - type: text
        value: "{{zug1.nr}}"
```

**Unterschied zum Layer-Level `if`:**

| Merkmal | Layer-Level | Block-Level |
|---|---|---|
| Hat `type:` | ja | nein |
| Steuert | einen einzelnen Layer | beliebig viele Layer |
| Verschachtelbar | nein | ja (beliebig tief) |

**Regeln:**
- `else:` (ohne Wert) und `else: true` sind gleichwertig.
- Block-Nodes können beliebig tief verschachtelt werden.
- `type: loop` darf nicht innerhalb eines anderen `loop` vorkommen, auch nicht via Block-Node.
- `elif`/`else` ohne vorangehendes `if` ist ein Fehler.
````

- [ ] **Step 2: Add example to `docs/user-guide-templates.md`**

Add a section "Block-Bedingungen" with a practical example showing train-type branching:

````markdown
### Block-Bedingungen

Wenn mehrere Layer nur bei einer bestimmten Bedingung gerendert werden sollen,
können sie in einem Block-Node gruppiert werden:

```yaml
# Zeige unterschiedliche Bilder und Texte je nach Zugnummer
- if: "startsWith(zug1.nr, 'ICN')"
  layers:
    - type: image
      file: icn-logo.png
      x: 5
      y: 5
    - type: text
      value: "Neigezug"
      x: 5
      y: 30
      font: regular
      size: 10
      color: "#ffffff"

- elif: "startsWith(zug1.nr, 'IC')"
  layers:
    - type: image
      file: ic-logo.png
      x: 5
      y: 5

- else:
  layers:
    - type: text
      value: "{{zug1.nr}}"
      x: 5
      y: 5
      font: regular
      size: 12
      color: "#ffffff"
```

Block-Nodes können beliebig tief verschachtelt werden:

```yaml
- if: "not(isEmpty(zug1.hinweis))"
  layers:
    - type: rect
      x: 0
      y: 50
      width: 160
      height: 20
      color:
        if: "startsWith(zug1.hinweis, '*')"
        then: "#ff0000"
        else: "#ffcc00"
    - type: text
      value: "{{zug1.hinweis | strip('*')}}"
      x: 3
      y: 52
      font: regular
      size: 9
      color: "#000000"
```
````

- [ ] **Step 3: Commit**

```bash
git add docs/yaml-template-spec.md docs/user-guide-templates.md
git commit -m "docs: document block-level if/elif/else syntax"
```

---

## Self-Review

**Spec coverage:**
- ✅ `if`/`elif`/`else` block nodes with sub-`layers:`
- ✅ `else:` (null) and `else: true` both accepted via `ElseMarker`
- ✅ Nested block nodes (arbitrary depth)
- ✅ Block nodes inside `loop` sub-layers
- ✅ Layer-level if/elif/else on typed layers unchanged
- ✅ Error: elif/else without preceding if
- ✅ Error: nested loop inside loop (also via block node)
- ✅ Block nodes mixed with regular typed layers

**Placeholder scan:** None found.

**Type consistency:**
- `ElseMarker` defined once in `template.go`, used in `Layer.Else`; all existing `if layer.Else` expressions work unchanged (named bool type is boolean-compatible in Go)
- `renderLayers` signature `(dst, tmpl, layers, eval, inLoop bool)` consistent across all 3 call sites (`Render`, `renderLoop`, block node recursion)
- `blockElse` test helper uses `Else: true` (ElseMarker), matching the struct field type
