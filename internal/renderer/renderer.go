package renderer

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
	xdraw "golang.org/x/image/draw"
	"gopkg.in/yaml.v3"
)

// Renderer loads templates and renders them to images.
type Renderer struct {
	TemplatesDir string
	fontCache    map[string]*opentype.Font // key: absolute file path
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
	return &tmpl, nil
}

const (
	maxCanvasDimension = 16384 // pixels — prevents OOM via malicious templates
	maxLayers          = 256   // layer count limit — prevents CPU exhaustion
	maxFontFileBytes   = 50 * 1024 * 1024 // 50 MB
)

// Render creates an image from the template and input data.
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

	for i, layer := range tmpl.Layers {
		// Phase 1: if-conditions on layers are not evaluated — always render.
		var err error
		switch layer.Type {
		case "image":
			err = r.renderImage(dst, tmpl, layer, eval)
		case "rect":
			err = r.renderRect(dst, layer, eval)
		case "text":
			err = r.renderText(dst, tmpl, layer, eval)
		case "copy":
			err = renderCopy(dst, layer)
		default:
			return nil, fmt.Errorf("renderer: Render: layer %d: unknown type %q", i, layer.Type)
		}
		if err != nil {
			return nil, fmt.Errorf("renderer: Render: layer %d (%s): %w", i, layer.Type, err)
		}
	}

	return dst, nil
}

// renderImage loads and draws an image layer.
func (r *Renderer) renderImage(dst *image.NRGBA, tmpl *Template, layer Layer, eval *Evaluator) error {
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

	imgPath := filepath.Join(tmpl.Dir, filename)
	f, err := os.Open(imgPath)
	if err != nil {
		return fmt.Errorf("renderImage: opening %q: %w", filename, err)
	}
	defer f.Close()

	src, _, err := image.Decode(f)
	if err != nil {
		return fmt.Errorf("renderImage: decoding %q: %w", filename, err)
	}

	// Scale if Width or Height is specified.
	if layer.Width > 0 || layer.Height > 0 {
		w := layer.Width
		h := layer.Height
		if w > maxCanvasDimension || h > maxCanvasDimension {
			return fmt.Errorf("renderImage: scaled dimensions %dx%d exceed maximum %d", w, h, maxCanvasDimension)
		}
		if w <= 0 {
			// Preserve aspect ratio.
			w = src.Bounds().Dx() * h / src.Bounds().Dy()
		}
		if h <= 0 {
			h = src.Bounds().Dy() * w / src.Bounds().Dx()
		}
		scaled := image.NewNRGBA(image.Rect(0, 0, w, h))
		xdraw.CatmullRom.Scale(scaled, scaled.Bounds(), src, src.Bounds(), xdraw.Over, nil)
		src = scaled
	}

	pt := image.Pt(layer.X, layer.Y)
	r2 := src.Bounds().Add(pt)
	draw.Draw(dst, r2, src, src.Bounds().Min, draw.Over)
	return nil
}

// renderCopy copies a rectangular region of the canvas to another position.
// Used for displays where the top half is mirrored to the bottom half.
func renderCopy(dst *image.NRGBA, layer Layer) error {
	src := dst.SubImage(image.Rect(layer.SrcX, layer.SrcY, layer.SrcX+layer.SrcWidth, layer.SrcY+layer.SrcHeight))
	dstRect := image.Rect(layer.X, layer.Y, layer.X+layer.SrcWidth, layer.Y+layer.SrcHeight)
	draw.Draw(dst, dstRect, src, image.Pt(layer.SrcX, layer.SrcY), draw.Src)
	return nil
}

// renderRect draws a filled rectangle layer.
func (r *Renderer) renderRect(dst *image.NRGBA, layer Layer, eval *Evaluator) error {
	colorStr := eval.Interpolate(layer.Color.String())
	c, err := parseColor(colorStr)
	if err != nil {
		return fmt.Errorf("renderRect: %w", err)
	}

	rect := image.Rect(layer.X, layer.Y, layer.X+layer.Width, layer.Y+layer.Height)
	draw.Draw(dst, rect, &image.Uniform{C: c}, image.Point{}, draw.Over)
	return nil
}

// renderText draws a text layer.
func (r *Renderer) renderText(dst *image.NRGBA, tmpl *Template, layer Layer, eval *Evaluator) error {
	text := eval.Interpolate(layer.Value.String())
	colorStr := eval.Interpolate(layer.Color.String())

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

	// Use width as wrap boundary if set, otherwise wrap at max_width.
	wrapWidth := layer.MaxWidth
	if layer.Width > 0 && wrapWidth == 0 {
		wrapWidth = layer.Width
	}

	var lines []string
	if wrapWidth > 0 {
		lines = wrapText(face, text, wrapWidth)
	} else {
		lines = []string{text}
	}
	if len(lines) == 0 {
		return nil
	}

	// Vertical alignment — requires height to be set.
	startY := layer.Y
	if layer.Height > 0 {
		totalHeight := len(lines) * lineHeight
		switch layer.Valign {
		case "middle":
			startY = layer.Y + (layer.Height-totalHeight)/2
		case "bottom":
			startY = layer.Y + layer.Height - totalHeight
		}
	}

	img := &image.Uniform{C: c}

	for i, line := range lines {
		y := startY + ascent + i*lineHeight

		// Horizontal alignment.
		// With width: align within the box [X, X+Width].
		// Without width: X is the anchor point (left edge / center / right edge).
		var x int
		switch layer.Align {
		case "center":
			lineW := measureText(face, line)
			if layer.Width > 0 {
				x = layer.X + (layer.Width-lineW)/2
			} else {
				x = layer.X - lineW/2
			}
		case "right":
			lineW := measureText(face, line)
			if layer.Width > 0 {
				x = layer.X + layer.Width - lineW
			} else {
				x = layer.X - lineW
			}
		default: // left
			x = layer.X
		}

		d := &font.Drawer{
			Dst:  dst,
			Src:  img,
			Face: face,
			Dot: fixed.Point26_6{
				X: fixed.I(x),
				Y: fixed.I(y),
			},
		}
		d.DrawString(line)
	}

	return nil
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

	if cached, ok := r.fontCache[absPath]; ok {
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

	r.fontCache[absPath] = otf
	return otf, nil
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
		text     string
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
