package renderer

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/f64"
	"golang.org/x/image/math/fixed"
	"gopkg.in/yaml.v3"
)

// Renderer loads templates and renders them to images.
// Safe for concurrent use.
type Renderer struct {
	TemplatesDir string
	fontMu       sync.RWMutex
	fontCache    map[string]*opentype.Font // key: absolute file path; guarded by fontMu
	nowFunc      func() time.Time          // injectable clock for tests; nil = time.Now
}

func (r *Renderer) clock() time.Time {
	if r.nowFunc != nil {
		return r.nowFunc()
	}
	return time.Now()
}

// newEvaluator creates an Evaluator whose `now` is the renderer clock in the
// template's time zone.
func (r *Renderer) newEvaluator(tmpl *Template, data map[string]interface{}) *Evaluator {
	return newEvaluatorAt(data, r.clock().In(tmpl.location())).withAnim(0, tmpl.animFrames(), tmpl.animDelayCS())
}

// New creates a new Renderer with the given templates directory.
func New(templatesDir string) *Renderer {
	return &Renderer{
		TemplatesDir: templatesDir,
		fontCache:    make(map[string]*opentype.Font),
	}
}

// LoadTemplate loads template.yaml from the named template directory.
// It sets tmpl.Dir to the absolute path of the template directory.
func LoadTemplate(templatesDir, name string) (*Template, error) {
	tmplPath, err := SafeTemplatePath(templatesDir, name)
	if err != nil {
		return nil, fmt.Errorf("renderer: LoadTemplate: %w", err)
	}

	yamlPath := filepath.Join(tmplPath, "template.yaml")
	f, err := os.Open(yamlPath)
	if err != nil {
		return nil, fmt.Errorf("renderer: LoadTemplate: opening template.yaml: %w", err)
	}
	defer f.Close()

	var tmpl Template
	dec := yaml.NewDecoder(f)
	if err := dec.Decode(&tmpl); err != nil {
		return nil, fmt.Errorf("renderer: LoadTemplate: decoding YAML: %w", err)
	}

	tmpl.Dir = tmplPath

	if c := tmpl.Meta.Canvas.Colors; c != 0 && (c < 2 || c > 256) {
		return nil, fmt.Errorf("renderer: LoadTemplate: canvas.colors must be between 2 and 256 (got %d)", c)
	}
	if err := tmpl.validateAnimation(); err != nil {
		return nil, fmt.Errorf("renderer: LoadTemplate: %w", err)
	}

	loc, err := loadTimezone(tmpl.Meta.Timezone)
	if err != nil {
		return nil, fmt.Errorf("renderer: LoadTemplate: invalid meta.timezone %q: %w", tmpl.Meta.Timezone, err)
	}
	tmpl.loc = loc

	return &tmpl, nil
}

const (
	maxCanvasDimension = 16384            // pixels — prevents OOM via malicious templates
	maxLayers          = 256              // total rendered layer limit (across all block nesting) — prevents CPU exhaustion
	maxBlockDepth      = 16               // maximum block-node nesting depth — prevents stack overflow
	maxFontFileBytes   = 50 * 1024 * 1024 // 50 MB
)

// renderCtx carries the state of one Render call through the layer walk.
type renderCtx struct {
	tmpl         *Template
	eval         *Evaluator             // root evaluator (splitflap is not allowed inside loops, so leaves use it)
	layerCount   *int                   // total layers rendered so far, checked against maxLayers
	flapLayerSeq int                    // running number of splitflap layers in walk order
	flapRects    []image.Rectangle      // all splitflap modules placed so far (overlap/limit check)
	imgCache     map[string]image.Image // decoded image files by filename, per render
	anim         *animState             // nil = static image
}

func newRenderCtx(tmpl *Template, eval *Evaluator) *renderCtx {
	count := 0
	return &renderCtx{
		tmpl:       tmpl,
		eval:       eval,
		layerCount: &count,
		imgCache:   make(map[string]image.Image),
	}
}

// renderLayers renders a slice of layers in order, handling:
//   - Layer-level if/elif/else chains on typed layers
//   - Block-level if/elif/else nodes (Type == "", has Layers)
//
// rc.layerCount tracks the total number of layers rendered across all recursive calls.
// depth tracks the block-node nesting depth.
// Set inLoop=true when called from renderLoop to prevent nested loops.
func (r *Renderer) renderLayers(rc *renderCtx, dst *image.NRGBA, layers []Layer, eval *Evaluator, inLoop bool, depth int) error {
	if depth > maxBlockDepth {
		return fmt.Errorf("block nesting depth exceeds maximum %d", maxBlockDepth)
	}
	chainSatisfied := false
	inChain := false
	for i, layer := range layers {
		*rc.layerCount++
		if *rc.layerCount > maxLayers {
			return fmt.Errorf("total layer count exceeds maximum %d", maxLayers)
		}
		render := false
		switch {
		case layer.Elif != "" || bool(layer.Else):
			if !inChain {
				return fmt.Errorf("layer %d: elif/else without preceding if", i)
			}
			if !chainSatisfied {
				if bool(layer.Else) {
					chainSatisfied = true
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
		// Note: inChain/chainSatisfied are NOT reset here — a block node participates
		// in the same if/elif/else chain as its siblings in the parent layer list.
		if layer.Type == "" {
			if err := r.renderLayers(rc, dst, layer.Layers, eval, inLoop, depth+1); err != nil {
				return fmt.Errorf("block layer %d: %w", i, err)
			}
			continue
		}

		// Typed layer dispatch.
		var err error
		switch layer.Type {
		case "image":
			err = r.renderImage(rc, dst, layer, eval)
		case "rect":
			err = r.renderRect(dst, layer, eval)
		case "text":
			err = r.renderText(dst, rc.tmpl, layer, eval)
		case "copy":
			err = renderCopy(dst, layer, eval)
		case "loop":
			if inLoop {
				return fmt.Errorf("layer %d: nested loops are not supported", i)
			}
			err = r.renderLoop(rc, dst, layer, eval, depth)
		case "splitflap":
			if inLoop {
				return fmt.Errorf("layer %d: splitflap inside loop is not supported", i)
			}
			err = r.renderSplitflap(rc, dst, &layers[i])
		default:
			return fmt.Errorf("layer %d: unknown type %q", i, layer.Type)
		}
		if err != nil {
			return fmt.Errorf("layer %d (%s): %w", i, layer.Type, err)
		}
	}
	return nil
}

// Render creates an image from the template and input data.
func (r *Renderer) Render(tmpl *Template, data map[string]interface{}) (*image.NRGBA, error) {
	if err := checkCanvas(tmpl.Meta.Canvas); err != nil {
		return nil, fmt.Errorf("renderer: Render: %w", err)
	}
	img, err := r.renderFrame(newRenderCtx(tmpl, r.newEvaluator(tmpl, data)))
	if err != nil {
		return nil, fmt.Errorf("renderer: Render: %w", err)
	}
	return img, nil
}

// checkCanvas validates the output image dimensions.
func checkCanvas(c Canvas) error {
	if c.Width <= 0 || c.Height <= 0 {
		return fmt.Errorf("invalid canvas dimensions %dx%d", c.Width, c.Height)
	}
	if c.Width > maxCanvasDimension || c.Height > maxCanvasDimension {
		return fmt.Errorf("canvas dimensions %dx%d exceed maximum %d", c.Width, c.Height, maxCanvasDimension)
	}
	return nil
}

// renderFrame renders all template layers onto a fresh canvas.
// The canvas size must already be validated.
func (r *Renderer) renderFrame(rc *renderCtx) (*image.NRGBA, error) {
	dst := image.NewNRGBA(image.Rect(0, 0, rc.tmpl.Meta.Canvas.Width, rc.tmpl.Meta.Canvas.Height))
	if err := r.renderLayers(rc, dst, rc.tmpl.Layers, rc.eval, false, 0); err != nil {
		return nil, err
	}
	return dst, nil
}

// renderImage loads and draws an image layer.
func (r *Renderer) renderImage(rc *renderCtx, dst *image.NRGBA, layer Layer, eval *Evaluator) error {
	// Security: layer.File is intentionally NOT passed through eval.Interpolate().
	// Allowing JSON input values to control file paths would be a path traversal vector.
	// Asset filenames must be static values defined in the template YAML.
	filename := layer.File

	// Security: only allow plain filenames (no path components).
	// Both checks are needed: filepath.Base is OS-dependent for backslashes.
	if strings.ContainsAny(filename, "/\\") || filepath.Base(filename) != filename {
		return fmt.Errorf("renderImage: filename %q must not contain path separators", filename)
	}

	// Whitelist: only .png and .jpg files.
	lower := strings.ToLower(filename)
	if !strings.HasSuffix(lower, ".png") && !strings.HasSuffix(lower, ".jpg") && !strings.HasSuffix(lower, ".jpeg") {
		return fmt.Errorf("renderImage: file %q is not a permitted image type (only .png and .jpg)", filename)
	}

	src, err := rc.loadImage(filename)
	if err != nil {
		return fmt.Errorf("renderImage: %w", err)
	}

	// Scale if Width or Height is specified.
	w := layer.Width.Resolve(eval)
	h := layer.Height.Resolve(eval)
	if w > 0 || h > 0 {
		if w > maxCanvasDimension || h > maxCanvasDimension {
			return fmt.Errorf("renderImage: scaled dimensions %dx%d exceed maximum %d", w, h, maxCanvasDimension)
		}
		if w <= 0 {
			// Preserve aspect ratio — use float to avoid integer division truncation.
			w = int(math.Round(float64(src.Bounds().Dx()) * float64(h) / float64(src.Bounds().Dy())))
			if w < 1 {
				w = 1
			}
		}
		if h <= 0 {
			h = int(math.Round(float64(src.Bounds().Dy()) * float64(w) / float64(src.Bounds().Dx())))
			if h < 1 {
				h = 1
			}
		}
		scaled := image.NewNRGBA(image.Rect(0, 0, w, h))
		xdraw.CatmullRom.Scale(scaled, scaled.Bounds(), src, src.Bounds(), xdraw.Over, nil)
		src = scaled
	}

	x := layer.X.Resolve(eval)
	y := layer.Y.Resolve(eval)

	// When rotate is set, x and y are the CENTER of the image on the canvas.
	// rotateImageCW handles 0° correctly (identity), so we always apply center placement.
	rotStr := strings.TrimSpace(eval.Interpolate(layer.Rotate.Resolve(eval)))
	if rotStr != "" {
		deg, err := strconv.ParseFloat(rotStr, 64)
		if err != nil {
			return fmt.Errorf("renderImage: invalid rotate value %q: %w", rotStr, err)
		}
		if math.IsNaN(deg) || math.IsInf(deg, 0) {
			return fmt.Errorf("renderImage: invalid rotate value %q: rotate must be finite", rotStr)
		}
		deg = math.Mod(deg, 360)
		rotated := rotateImageCW(src, deg)
		cx := x - rotated.Bounds().Dx()/2
		cy := y - rotated.Bounds().Dy()/2
		pt := image.Pt(cx, cy)
		r2 := rotated.Bounds().Add(pt)
		draw.Draw(dst, r2, rotated, rotated.Bounds().Min, draw.Over)
		return nil
	}

	pt := image.Pt(x, y)
	r2 := src.Bounds().Add(pt)
	draw.Draw(dst, r2, src, src.Bounds().Min, draw.Over)
	return nil
}

// loadImage returns the decoded image file from the template directory,
// decoding it at most once per render. filename must already be validated.
// The cached image is shared and must not be modified.
func (rc *renderCtx) loadImage(filename string) (image.Image, error) {
	if img, ok := rc.imgCache[filename]; ok {
		return img, nil
	}
	f, err := os.Open(filepath.Join(rc.tmpl.Dir, filename))
	if err != nil {
		return nil, fmt.Errorf("opening %q: %w", filename, err)
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("decoding %q: %w", filename, err)
	}
	rc.imgCache[filename] = img
	return img, nil
}

// rotateImageCW rotates src clockwise by deg degrees around its own center
// and returns a new image sized to exactly contain the rotated result.
func rotateImageCW(src image.Image, deg float64) *image.NRGBA {
	bounds := src.Bounds()
	srcW := float64(bounds.Dx())
	srcH := float64(bounds.Dy())

	T := deg * math.Pi / 180.0
	cosT := math.Cos(T)
	sinT := math.Sin(T)

	// Bounding box of the rotated image.
	newW := int(math.Ceil(srcW*math.Abs(cosT) + srcH*math.Abs(sinT)))
	newH := int(math.Ceil(srcW*math.Abs(sinT) + srcH*math.Abs(cosT)))
	// Keep the size difference even so the rotation axis lands exactly on the
	// pixel grid after renderImage's integer centring (x - Dx/2).
	if (newW-bounds.Dx())%2 != 0 {
		newW++
	}
	if (newH-bounds.Dy())%2 != 0 {
		newH++
	}

	dst := image.NewNRGBA(image.Rect(0, 0, newW, newH))

	// Source and destination centers (in image coordinates, accounting for sub-image offset).
	srcCX := float64(bounds.Min.X) + srcW/2
	srcCY := float64(bounds.Min.Y) + srcH/2
	dstCX := float64(newW) / 2
	dstCY := float64(newH) / 2

	// xdraw.BiLinear.Transform treats s2d as a src→dst forward transform and
	// internally inverts it for backward (dst→src) pixel sampling.
	// CW rotation src→dst: x'= cosT*x - sinT*y, y'= sinT*x + cosT*y
	// Translated so that the source center maps to the destination center.
	s2d := f64.Aff3{
		cosT, -sinT, dstCX - srcCX*cosT + srcCY*sinT,
		sinT, cosT, dstCY - srcCX*sinT - srcCY*cosT,
	}
	xdraw.BiLinear.Transform(dst, s2d, src, src.Bounds(), xdraw.Over, nil)
	return dst
}

// renderCopy copies a rectangular region of the canvas to another position.
// Used for displays where the top half is mirrored to the bottom half.
// The copy is performed before any overlap — source and dest should not overlap.
func renderCopy(dst *image.NRGBA, layer Layer, eval *Evaluator) error {
	if layer.SrcWidth <= 0 || layer.SrcHeight <= 0 {
		return fmt.Errorf("renderCopy: src_width and src_height must be positive (got %dx%d)", layer.SrcWidth, layer.SrcHeight)
	}
	src := dst.SubImage(image.Rect(layer.SrcX, layer.SrcY, layer.SrcX+layer.SrcWidth, layer.SrcY+layer.SrcHeight))
	x := layer.X.Resolve(eval)
	y := layer.Y.Resolve(eval)
	dstRect := image.Rect(x, y, x+layer.SrcWidth, y+layer.SrcHeight)
	draw.Draw(dst, dstRect, src, image.Pt(layer.SrcX, layer.SrcY), draw.Src)
	return nil
}

// renderRect draws a filled rectangle layer.
func (r *Renderer) renderRect(dst *image.NRGBA, layer Layer, eval *Evaluator) error {
	colorStr := eval.Interpolate(layer.Color.Resolve(eval))
	c, err := parseColor(colorStr)
	if err != nil {
		return fmt.Errorf("renderRect: %w", err)
	}

	x := layer.X.Resolve(eval)
	y := layer.Y.Resolve(eval)
	w := layer.Width.Resolve(eval)
	h := layer.Height.Resolve(eval)
	rect := image.Rect(x, y, x+w, y+h)
	draw.Draw(dst, rect, &image.Uniform{C: c}, image.Point{}, draw.Over)
	return nil
}

// renderText draws a text layer.
func (r *Renderer) renderText(dst *image.NRGBA, tmpl *Template, layer Layer, eval *Evaluator) error {
	text := eval.Interpolate(layer.Value.Resolve(eval))
	colorStr := eval.Interpolate(layer.Color.Resolve(eval))

	c, err := parseColor(colorStr)
	if err != nil {
		return fmt.Errorf("renderText: %w", err)
	}

	face, err := r.getFace(tmpl, layer)
	if err != nil {
		return fmt.Errorf("renderText: %w", err)
	}
	defer face.Close()

	metrics := face.Metrics()
	ascent := metrics.Ascent.Round()
	lineHeight := metrics.Height.Round()
	if layer.LineHeight > 0 && layer.LineHeight != 1 {
		lineHeight = int(math.Round(float64(lineHeight) * layer.LineHeight))
	}

	layerX := layer.X.Resolve(eval)
	layerY := layer.Y.Resolve(eval)
	layerW := layer.Width.Resolve(eval)
	layerH := layer.Height.Resolve(eval)
	maxW := layer.MaxWidth.Resolve(eval)

	// Use width as wrap boundary if set, otherwise wrap at max_width.
	wrapWidth := maxW
	if layerW > 0 && wrapWidth == 0 {
		wrapWidth = layerW
	}

	scale := layer.ScaleX
	if scale <= 0 || scale == 1 {
		scale = 0 // inactive: take the unscaled drawing path
	}
	baseline := layer.Valign == "baseline"

	if layer.FitItems != nil {
		items, xs := layoutFitItems(face, text, layer.FitItems, scale, maxW)
		baseY := layerY + ascent
		if baseline {
			baseY = layerY
		}
		for i, item := range items {
			drawTextLine(dst, face, item, layerX+xs[i], baseY, c, scale)
		}
		return nil
	}

	var lines []string
	if wrapWidth > 0 {
		if scale > 0 {
			// Wrap in natural (unscaled) space so scaled lines fit the box.
			wrapWidth = int(float64(wrapWidth) / scale)
		}
		lines = wrapText(face, text, wrapWidth)
	} else {
		lines = []string{text}
	}
	if len(lines) == 0 {
		return nil
	}

	// Vertical alignment — requires height to be set.
	startY := layerY
	if baseline {
		startY = layerY - ascent // baseline of the first line = layerY
	} else if layerH > 0 {
		totalHeight := len(lines) * lineHeight
		switch layer.Valign {
		case "middle":
			startY = layerY + (layerH-totalHeight)/2
		case "bottom":
			startY = layerY + layerH - totalHeight
		}
		// Clamp: never render above the box top (can happen if more lines than height).
		if startY < layerY {
			startY = layerY
		}
	}

	for i, line := range lines {
		y := startY + ascent + i*lineHeight

		// Horizontal alignment.
		// With width: align within the box [X, X+Width].
		// Without width: X is the anchor point (left edge / center / right edge).
		var x int
		switch layer.Align {
		case "center":
			lineW := scaledWidth(measureText(face, line), scale)
			if layerW > 0 {
				x = layerX + (layerW-lineW)/2
			} else {
				x = layerX - lineW/2
			}
		case "right":
			lineW := scaledWidth(measureText(face, line), scale)
			if layerW > 0 {
				x = layerX + layerW - lineW
			} else {
				x = layerX - lineW
			}
		default: // left
			x = layerX
		}

		drawTextLine(dst, face, line, x, y, c, scale)
	}

	return nil
}

// scaledWidth returns the on-canvas width of a natural width w; scale 0 = inactive.
func scaledWidth(w int, scale float64) int {
	if scale <= 0 {
		return w
	}
	return int(math.Ceil(float64(w) * scale))
}

// drawTextLine draws one line with its origin at (x, baselineY). With scale > 0
// the line is rendered offscreen at natural size and resampled horizontally.
func drawTextLine(dst *image.NRGBA, face font.Face, line string, x, baselineY int, c color.Color, scale float64) {
	src := &image.Uniform{C: c}
	if scale <= 0 {
		d := &font.Drawer{
			Dst:  dst,
			Src:  src,
			Face: face,
			Dot:  fixed.Point26_6{X: fixed.I(x), Y: fixed.I(baselineY)},
		}
		d.DrawString(line)
		return
	}

	m := face.Metrics()
	ascent, descent := m.Ascent.Round(), m.Descent.Round()
	pad := ascent // generous margin for overhanging glyph parts
	w := measureText(face, line)
	// The offscreen line and the resampler's buffer grow with the text width, and
	// the text can come from the request. A line far wider than the canvas is
	// therefore cut down to the glyphs that can reach the canvas.
	if b := dst.Bounds(); float64(w+2*pad)*scale > float64(2*b.Dx()+4*pad) {
		line, x = visibleRun(face, line, x, scale, b.Min.X-2*pad, b.Max.X+2*pad)
		w = measureText(face, line)
	}
	if w <= 0 {
		return
	}
	off := image.NewNRGBA(image.Rect(0, 0, w+2*pad, ascent+descent+2*pad))
	d := &font.Drawer{
		Dst:  off,
		Src:  src,
		Face: face,
		Dot:  fixed.Point26_6{X: fixed.I(pad), Y: fixed.I(pad + ascent)},
	}
	d.DrawString(line)

	dw := int(math.Ceil(float64(off.Bounds().Dx()) * scale))
	dx := x - int(math.Round(float64(pad)*scale))
	dy := baselineY - (pad + ascent)
	rect := image.Rect(dx, dy, dx+dw, dy+off.Bounds().Dy())
	xdraw.CatmullRom.Scale(dst, rect, off, off.Bounds(), xdraw.Over, nil)
}

// visibleRun returns the part of line whose glyphs, drawn from x and scaled
// horizontally by scale, can overlap [minX, maxX), and the x at which that part
// starts. Advances and kerning are accumulated as font.MeasureString does.
func visibleRun(face font.Face, line string, x int, scale float64, minX, maxX int) (string, int) {
	pos := func(adv fixed.Int26_6) float64 { return float64(x) + float64(adv)/64*scale }
	var adv, startAdv fixed.Int26_6
	start, end := -1, 0
	prev := rune(-1)
	for i, r := range line {
		if prev >= 0 {
			adv += face.Kern(prev, r)
		}
		left := adv
		if a, ok := face.GlyphAdvance(r); ok {
			adv += a
		}
		prev = r
		if pos(adv) < float64(minX) {
			continue // entirely left of the visible range
		}
		if pos(left) >= float64(maxX) {
			break // this and all following glyphs are right of it
		}
		if start < 0 {
			start, startAdv = i, left
		}
		end = i + utf8.RuneLen(r)
	}
	if start < 0 {
		return "", x
	}
	return line[start:end], int(math.Round(pos(startAdv)))
}

// fitItems implements the legacy "via" rule. widths are the item widths; item i
// is drawn (true) when xpos+width < maxWidth, with xpos starting at 0. xpos
// advances by width+gap for every item, drawn or not. maxWidth <= 0 = no limit.
func fitItems(widths []int, gap, maxWidth int) []bool {
	draw := make([]bool, len(widths))
	xpos := 0
	for i, w := range widths {
		draw[i] = maxWidth <= 0 || xpos+w < maxWidth
		xpos += w + gap
	}
	return draw
}

// layoutFitItems splits text, trims items (dropping empty ones), measures them
// (scaled when scale > 0) and returns the items to draw with their x offsets
// relative to the layer origin.
func layoutFitItems(face font.Face, text string, fi *FitItems, scale float64, maxWidth int) (items []string, xs []int) {
	parts := []string{text}
	if fi.SplitBy != "" {
		parts = strings.Split(text, fi.SplitBy)
	}
	var all []string
	var widths []int
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		all = append(all, p)
		widths = append(widths, scaledWidth(measureText(face, p), scale))
	}
	draw := fitItems(widths, fi.Gap, maxWidth)
	xpos := 0
	for i, p := range all {
		if draw[i] {
			items = append(items, p)
			xs = append(xs, xpos)
		}
		xpos += widths[i] + fi.Gap
	}
	return items, xs
}

// getFace finds the font definition for the layer and returns an opentype.Face.
func (r *Renderer) getFace(tmpl *Template, layer Layer) (font.Face, error) {
	// Find the FontDef with the matching ID.
	var fontFile string
	for _, fd := range tmpl.Fonts {
		if fd.ID == layer.Font {
			fontFile = fd.File
			break
		}
	}
	if fontFile == "" {
		return nil, fmt.Errorf("getFace: font ID %q not found in template", layer.Font)
	}

	otf, err := r.getFont(tmpl.Dir, fontFile)
	if err != nil {
		return nil, fmt.Errorf("getFace: %w", err)
	}

	size := layer.Size
	if size <= 0 {
		size = 12
	}

	face, err := opentype.NewFace(otf, &opentype.FaceOptions{
		Size: size,
		DPI:  72,
	})
	if err != nil {
		return nil, fmt.Errorf("getFace: creating face: %w", err)
	}
	return face, nil
}

// getFont loads (or returns from cache) an opentype.Font.
// filename must be a plain filename with no path components.
func (r *Renderer) getFont(dir, filename string) (*opentype.Font, error) {
	// Security: only allow plain filenames.
	// Both checks are needed: filepath.Base is OS-dependent for backslashes.
	if strings.ContainsAny(filename, "/\\") || filepath.Base(filename) != filename {
		return nil, fmt.Errorf("getFont: filename %q must not contain path separators", filename)
	}

	absPath := filepath.Join(dir, filename)

	r.fontMu.RLock()
	cached, ok := r.fontCache[absPath]
	r.fontMu.RUnlock()
	if ok {
		return cached, nil
	}

	// Security: limit font file size before reading into memory.
	info, err := os.Stat(absPath)
	if err != nil {
		return nil, fmt.Errorf("getFont: stat %q: %w", filename, err)
	}
	if info.Size() > maxFontFileBytes {
		return nil, fmt.Errorf("getFont: font file %q exceeds maximum size of %d MB", filename, maxFontFileBytes/1024/1024)
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("getFont: reading %q: %w", filename, err)
	}

	otf, err := opentype.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("getFont: parsing %q: %w", filename, err)
	}

	r.fontMu.Lock()
	r.fontCache[absPath] = otf
	r.fontMu.Unlock()
	return otf, nil
}

const (
	defaultMaxLoopItems = 20
	maxLoopItemsHardCap = 200 // upper bound regardless of max_items in YAML — prevents CPU/memory exhaustion
)

// renderLoop iterates over a split string and renders sub-layers for each item.
// All sub-layer coordinates are absolute; use {{i * step + base}} expressions for positioning.
// Loop variables available in sub-layers: i, loop.index (int); layer.Var (string item).
func (r *Renderer) renderLoop(rc *renderCtx, dst *image.NRGBA, layer Layer, eval *Evaluator, depth int) error {
	if len(layer.Layers) == 0 {
		return nil
	}

	value := eval.Interpolate(layer.Value.Resolve(eval))
	if value == "" {
		return nil
	}

	sep := layer.SplitBy
	if sep == "" {
		sep = "|"
	}

	maxItems := layer.MaxItems
	if maxItems <= 0 {
		maxItems = defaultMaxLoopItems
	} else if maxItems > maxLoopItemsHardCap {
		maxItems = maxLoopItemsHardCap
	}

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

		if err := r.renderLayers(rc, dst, layer.Layers, childEval, true, depth); err != nil {
			return fmt.Errorf("renderLoop: item %d: %w", displayIdx, err)
		}
		displayIdx++
	}
	return nil
}

// measureText returns the advance width of a string in pixels.
func measureText(face font.Face, text string) int {
	advance := font.MeasureString(face, text)
	return advance.Round()
}

// wrapText breaks text into lines so that no line exceeds maxWidth pixels.
// It splits at spaces first, then at hyphens, then forces a break mid-word
// if a single token is still too wide.
func wrapText(face font.Face, text string, maxWidth int) []string {
	// Split into space-separated words, then further split each word at hyphens
	// so that e.g. "Sennhof-Kyburg" can break after the hyphen.
	type token struct {
		text        string
		spaceBefore bool
	}
	var tokens []token
	for i, word := range strings.Fields(text) {
		parts := strings.SplitAfter(word, "-") // keeps the hyphen on the left part
		for j, part := range parts {
			tokens = append(tokens, token{
				text:        part,
				spaceBefore: i > 0 && j == 0, // space only before first part of each word
			})
		}
	}
	if len(tokens) == 0 {
		return nil
	}

	var lines []string
	current := ""

	for _, tok := range tokens {
		sep := ""
		if current != "" && tok.spaceBefore {
			sep = " "
		}
		candidate := current + sep + tok.text

		if measureText(face, candidate) <= maxWidth {
			current = candidate
		} else {
			// Current token doesn't fit — flush current line first.
			if current != "" {
				lines = append(lines, current)
				current = tok.text
			} else {
				// Single token wider than maxWidth — force character-level break.
				for _, ch := range tok.text {
					if measureText(face, current+string(ch)) <= maxWidth {
						current += string(ch)
					} else {
						lines = append(lines, current)
						current = string(ch)
					}
				}
			}
		}
	}
	if current != "" {
		lines = append(lines, current)
	}
	return lines
}

// parseColor parses a hex color string of the form #RRGGBB or #RRGGBBAA.
func parseColor(s string) (color.NRGBA, error) {
	s = strings.TrimSpace(s)
	if len(s) == 0 {
		return color.NRGBA{}, fmt.Errorf("parseColor: empty color string")
	}
	if s[0] != '#' {
		return color.NRGBA{}, fmt.Errorf("parseColor: color %q must start with #", s)
	}
	hex := s[1:]
	switch len(hex) {
	case 6:
		r, err1 := strconv.ParseUint(hex[0:2], 16, 8)
		g, err2 := strconv.ParseUint(hex[2:4], 16, 8)
		b, err3 := strconv.ParseUint(hex[4:6], 16, 8)
		if err1 != nil || err2 != nil || err3 != nil {
			return color.NRGBA{}, fmt.Errorf("parseColor: invalid hex color %q", s)
		}
		return color.NRGBA{R: uint8(r), G: uint8(g), B: uint8(b), A: 0xff}, nil
	case 8:
		r, err1 := strconv.ParseUint(hex[0:2], 16, 8)
		g, err2 := strconv.ParseUint(hex[2:4], 16, 8)
		b, err3 := strconv.ParseUint(hex[4:6], 16, 8)
		a, err4 := strconv.ParseUint(hex[6:8], 16, 8)
		if err1 != nil || err2 != nil || err3 != nil || err4 != nil {
			return color.NRGBA{}, fmt.Errorf("parseColor: invalid hex color %q", s)
		}
		return color.NRGBA{R: uint8(r), G: uint8(g), B: uint8(b), A: uint8(a)}, nil
	default:
		return color.NRGBA{}, fmt.Errorf("parseColor: color %q has invalid length (expected #RRGGBB or #RRGGBBAA)", s)
	}
}
