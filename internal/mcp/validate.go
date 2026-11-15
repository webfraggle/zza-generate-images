package mcp

import (
	"encoding/json"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/webfraggle/zza-generate-images/internal/renderer"
)

var clockRe = regexp.MustCompile(`^\d{1,2}:\d{2}$`)

// validateTemplateYAML checks template.yaml content for template `template`:
// YAML/semantic parse, canvas 240x240 and a probe render with default.json.
// It returns design warnings (non-blocking) or the reason for rejection.
func validateTemplateYAML(e *env, template string, content []byte) (warnings []string, err error) {
	unknown := unknownKeyWarnings(content)
	warnings, err = validateTemplateContent(e, template, content)
	if err != nil {
		if len(unknown) > 0 {
			// A typo is often the cause of the rejection (src_widht → src_width 0).
			err = fmt.Errorf("%w (unknown keys, maybe typos: %s)", err, strings.Join(unknown, "; "))
		}
		return nil, err
	}
	return append(unknown, warnings...), nil
}

func validateTemplateContent(e *env, template string, content []byte) (warnings []string, err error) {
	dir := filepath.Join(e.templatesDir, template)
	tmpl, err := renderer.ParseTemplate(content, dir)
	if err != nil {
		return nil, err
	}
	c := tmpl.Meta.Canvas
	if c.Width != canvasSize || c.Height != canvasSize {
		return nil, fmt.Errorf("canvas must be %dx%d (only 1.05\" displays are supported), got %dx%d", canvasSize, canvasSize, c.Width, c.Height)
	}
	img, err := renderer.New(e.templatesDir).Render(tmpl, loadTestData(dir))
	if err != nil {
		return nil, err
	}
	if bottomHalfEmpty(img) {
		warnings = append(warnings, "bottom half (y 120-239) is empty — the back side of the display shows nothing; rebuild it with copy layers (see get_design_guide)")
	}
	walkLayers(tmpl.Layers, "layers", func(path string, l renderer.Layer) {
		if l.Type == "text" && l.Size > 0 && l.Size < 10 {
			warnings = append(warnings, fmt.Sprintf("%s (text): size %g px — below 10 px is hard to read on the 1.05\" display", path, l.Size))
		}
	})
	walkLayers(tmpl.Layers, "layers", func(path string, l renderer.Layer) {
		if l.Type != "text" {
			return
		}
		v := l.Value.String()
		if !strings.Contains(v, "{{") && clockRe.MatchString(strings.TrimSpace(v)) {
			warnings = append(warnings, fmt.Sprintf("%s (text): static text %q looks like train data — use {{zug1.…}} placeholders", path, v))
		}
	})
	return warnings, nil
}

func walkLayers(layers []renderer.Layer, prefix string, fn func(path string, l renderer.Layer)) {
	for i, l := range layers {
		p := fmt.Sprintf("%s[%d]", prefix, i)
		fn(p, l)
		if len(l.Layers) > 0 {
			walkLayers(l.Layers, p+".layers", fn)
		}
	}
}

func bottomHalfEmpty(img *image.NRGBA) bool {
	b := img.Bounds()
	for y := b.Min.Y + canvasSize/2; y < b.Max.Y; y++ {
		row := img.Pix[(y-b.Min.Y)*img.Stride : (y-b.Min.Y)*img.Stride+b.Dx()*4]
		for x := 3; x < len(row); x += 4 {
			if row[x] != 0 {
				return false
			}
		}
	}
	return true
}

// validateJSONObject requires content to be a syntactically valid JSON object.
func validateJSONObject(content []byte) error {
	var v any
	if err := json.Unmarshal(content, &v); err != nil {
		return fmt.Errorf("invalid JSON: %v", err)
	}
	if _, ok := v.(map[string]any); !ok {
		return fmt.Errorf("invalid JSON: must be an object")
	}
	return nil
}

// loadTestData returns the contents of default.json of the template folder as
// an object, or an empty object if it is missing, too large or invalid.
func loadTestData(dir string) map[string]any {
	empty := map[string]any{}
	p := filepath.Join(dir, "default.json")
	fi, err := os.Lstat(p)
	if err != nil || !fi.Mode().IsRegular() || fi.Size() > maxTextFileBytes {
		return empty
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return empty
	}
	var m map[string]any
	if json.Unmarshal(b, &m) != nil || m == nil {
		return empty
	}
	return m
}
