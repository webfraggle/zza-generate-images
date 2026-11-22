package renderer

import (
	"fmt"
	"image"
	"image/draw"
)

const (
	maxFlapModules = 64 // splitflap modules per template render — bounds per-module rendering cost
	maxReelEntries = 64 // reel entries per splitflap layer
)

// flapModule is one split-flap module of a splitflap layer, resolved for a render.
type flapModule struct {
	layerIdx int             // running number of the splitflap layer (renderCtx.flapLayerSeq)
	cellIdx  int             // index of the module within its layer
	rect     image.Rectangle // module rectangle on the canvas
	splitY   int             // hinge line relative to rect.Min.Y
	target   string          // value the module shows when at rest
	layer    *Layer          // the splitflap layer (children, reel, timing)
}

// renderSplitflap plans the modules of a splitflap layer and draws each one
// with its target value, or in an animation frame with its state in that frame.
func (r *Renderer) renderSplitflap(rc *renderCtx, dst *image.NRGBA, layer *Layer) error {
	seq := rc.flapLayerSeq
	rc.flapLayerSeq++

	mods, err := planModules(layer, seq, rc.eval, dst.Bounds())
	if err != nil {
		return fmt.Errorf("renderSplitflap: %w", err)
	}
	if err := rc.registerModules(mods); err != nil {
		return fmt.Errorf("renderSplitflap: %w", err)
	}

	if rc.anim != nil {
		for _, m := range mods {
			if err := r.drawAnimatedModule(rc, dst, m); err != nil {
				return fmt.Errorf("renderSplitflap: module %d: %w", m.cellIdx, err)
			}
		}
		return nil
	}
	for _, m := range mods {
		leaf, err := r.renderLeaf(rc, m, snapshot(dst, m.rect), m.target)
		if err != nil {
			return fmt.Errorf("renderSplitflap: module %d: %w", m.cellIdx, err)
		}
		draw.Draw(dst, m.rect, leaf, image.Point{}, draw.Src)
	}
	return nil
}

// planModules resolves geometry and target values of a splitflap layer and
// validates the layer. Checks across layers (overlap, total module count) are
// done by renderCtx.registerModules.
func planModules(layer *Layer, layerIdx int, eval *Evaluator, canvas image.Rectangle) ([]flapModule, error) {
	x, y := layer.X.Resolve(eval), layer.Y.Resolve(eval)
	w, h := layer.Width.Resolve(eval), layer.Height.Resolve(eval)
	if w <= 0 || h <= 0 {
		return nil, fmt.Errorf("width and height must be positive (got %dx%d)", w, h)
	}

	cells := layer.Cells
	switch {
	case cells == 0:
		cells = 1
	case cells < 0:
		return nil, fmt.Errorf("cells must not be negative (got %d)", cells)
	case cells > maxFlapModules:
		return nil, fmt.Errorf("cells %d exceeds maximum %d", cells, maxFlapModules)
	}

	if n := len(layer.Reel); n > maxReelEntries {
		return nil, fmt.Errorf("reel has %d entries, exceeds maximum %d", n, maxReelEntries)
	}

	if f := layer.FlipFrames; f != 0 && (f < 1 || f > maxFlipFrames) {
		return nil, fmt.Errorf("flip_frames must be between 1 and %d (got %d)", maxFlipFrames, f)
	}

	pitch := layer.Pitch
	if pitch == 0 {
		pitch = w
	}

	splitY := layer.SplitY
	if splitY == 0 {
		splitY = h / 2
	}
	if splitY < 1 || splitY >= h {
		return nil, fmt.Errorf("split_y %d must be between 1 and %d", splitY, h-1)
	}

	value := eval.Interpolate(layer.Value.Resolve(eval))
	targets := make([]string, cells)
	switch layer.Split {
	case "":
		if cells > 1 {
			return nil, fmt.Errorf("cells > 1 requires split: chars")
		}
		targets[0] = value
	case "chars":
		runes := []rune(value)
		for i := range targets {
			if i < len(runes) {
				targets[i] = string(runes[i])
			}
		}
	default:
		return nil, fmt.Errorf("unknown split %q (expected chars or none)", layer.Split)
	}

	if err := validateFlapChildren(layer.Layers, 0); err != nil {
		return nil, err
	}

	// Bound the inputs before computing rectangles so that huge values cannot
	// overflow into a seemingly valid position.
	if !withinCanvasRange(x) || !withinCanvasRange(y) || w > maxCanvasDimension || h > maxCanvasDimension || !withinCanvasRange(pitch) {
		return nil, fmt.Errorf("module 0 lies outside the canvas %v", canvas)
	}

	mods := make([]flapModule, cells)
	for i := range mods {
		mx := x + i*pitch
		rect := image.Rect(mx, y, mx+w, y+h)
		if !rect.In(canvas) {
			return nil, fmt.Errorf("module %d %v lies outside the canvas %v", i, rect, canvas)
		}
		mods[i] = flapModule{
			layerIdx: layerIdx,
			cellIdx:  i,
			rect:     rect,
			splitY:   splitY,
			target:   targets[i],
			layer:    layer,
		}
	}
	return mods, nil
}

func withinCanvasRange(v int) bool {
	return v >= -maxCanvasDimension && v <= maxCanvasDimension
}

// validateFlapChildren rejects child layer types a splitflap module cannot
// render, including those in branches that are not taken for this value.
func validateFlapChildren(layers []Layer, depth int) error {
	if depth > maxBlockDepth {
		return fmt.Errorf("block nesting depth exceeds maximum %d", maxBlockDepth)
	}
	for i := range layers {
		switch layers[i].Type {
		case "loop", "copy", "splitflap":
			return fmt.Errorf("layer type %q is not allowed inside splitflap", layers[i].Type)
		case "":
			if err := validateFlapChildren(layers[i].Layers, depth+1); err != nil {
				return err
			}
		}
	}
	return nil
}

// registerModules records the modules of one splitflap layer for this render
// and enforces the total module limit and that no two modules overlap.
func (rc *renderCtx) registerModules(mods []flapModule) error {
	if len(rc.flapRects)+len(mods) > maxFlapModules {
		return fmt.Errorf("total splitflap modules exceed maximum %d", maxFlapModules)
	}
	for _, m := range mods {
		for _, other := range rc.flapRects {
			if m.rect.Overlaps(other) {
				return fmt.Errorf("module %d %v overlaps another splitflap module %v", m.cellIdx, m.rect, other)
			}
		}
		rc.flapRects = append(rc.flapRects, m.rect)
	}
	return nil
}

// renderLeaf renders one opaque flap of module size: a copy of bg with the
// module's child layers drawn on it, the layer's var set to value. Child
// coordinates are relative to the module; drawing is clipped to it.
//
// In the static pass (rc.anim == nil) the children count against maxLayers;
// animation frames render leaves repeatedly and use a per-leaf counter instead.
func (r *Renderer) renderLeaf(rc *renderCtx, m flapModule, bg *image.NRGBA, value string) (*image.NRGBA, error) {
	size := m.rect.Size()
	if bg.Bounds().Size() != size {
		return nil, fmt.Errorf("renderLeaf: background %v does not match module size %v", bg.Bounds().Size(), size)
	}
	leaf := image.NewNRGBA(image.Rect(0, 0, size.X, size.Y))
	draw.Draw(leaf, leaf.Bounds(), bg, bg.Bounds().Min, draw.Src)

	eval := rc.eval.withOwnIntVars() // text ids set by the children stay private to the leaf
	if m.layer.Var != "" {
		eval = eval.withLoopVars(map[string]string{m.layer.Var: value}, nil)
	}

	lrc := rc
	if rc.anim != nil {
		n := 0
		cp := *rc
		cp.layerCount = &n
		lrc = &cp
	}
	if err := r.renderLayers(lrc, leaf, m.layer.Layers, eval, false, 0); err != nil {
		return nil, fmt.Errorf("renderLeaf: %w", err)
	}
	return leaf, nil
}

// snapshot returns a copy of src under rect, with its origin at (0,0).
func snapshot(src *image.NRGBA, rect image.Rectangle) *image.NRGBA {
	dst := image.NewNRGBA(image.Rect(0, 0, rect.Dx(), rect.Dy()))
	draw.Draw(dst, dst.Bounds(), src, rect.Min, draw.Src)
	return dst
}
