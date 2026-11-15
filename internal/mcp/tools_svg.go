package mcp

import (
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"path/filepath"
	"regexp"
	"strings"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/srwiley/oksvg"
	"github.com/srwiley/rasterx"

	"github.com/webfraggle/zza-generate-images/internal/editor"
)

func init() { registerToolGroup("svg", registerSVGTools) }

type svgToPNGIn struct {
	Template  string `json:"template" jsonschema:"template name"`
	Name      string `json:"name" jsonschema:"target file name, must end in .png"`
	SVG       string `json:"svg" jsonschema:"SVG source, no text, no external references (max 256 KiB)"`
	Width     int    `json:"width" jsonschema:"PNG width in pixels, 1-1024"`
	Height    int    `json:"height" jsonschema:"PNG height in pixels, 1-1024"`
	Antialias *bool  `json:"antialias,omitempty" jsonschema:"default true; false gives hard pixel edges"`
}

type svgToPNGOut struct {
	Name   string `json:"name"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

func registerSVGTools(s *sdk.Server, e *env) {
	sdk.AddTool(s, &sdk.Tool{
		Name:        "svg_to_png",
		Description: "Rasterises SVG you write into a PNG in the template — for logos, pictograms, arrows, frames, dot grids. NO text in the SVG (use text layers), no external references. width/height = pixel size of the PNG (max 1024). antialias=false gives hard pixel edges (good for LED/dot-matrix looks; semi-transparency is dropped). Returns the image.",
	}, func(ctx context.Context, _ *sdk.CallToolRequest, in svgToPNGIn) (*sdk.CallToolResult, svgToPNGOut, error) {
		return e.svgToPNG(in)
	})
}

var (
	hrefRe = regexp.MustCompile(`(?:xlink:)?href\s*=\s*["']([^"']*)["']`)
	urlRe  = regexp.MustCompile(`url\(\s*['"]?([^)'"]*)`)

	errSVGExternal = errors.New("svg must not reference images or external files")
)

// checkSVGSource rejects SVG features that must not reach the rasteriser.
func checkSVGSource(svg string) error {
	l := strings.ToLower(svg)
	if strings.Contains(l, "<!doctype") || strings.Contains(l, "<!entity") {
		return errors.New("svg must not contain DOCTYPE or ENTITY declarations")
	}
	if strings.Contains(l, "<text") || strings.Contains(l, "<tspan") {
		return errors.New("svg must not contain text — put text in a text layer of template.yaml")
	}
	for _, bad := range []string{"<style", "<script", "<foreignobject", "@import"} {
		if strings.Contains(l, bad) {
			return errors.New("svg must not contain style sheets, scripts or foreignObject")
		}
	}
	if strings.Contains(l, "<image") {
		return errSVGExternal
	}
	for _, m := range hrefRe.FindAllStringSubmatch(l, -1) {
		if !strings.HasPrefix(strings.TrimSpace(m[1]), "#") {
			return errSVGExternal
		}
	}
	for _, m := range urlRe.FindAllStringSubmatch(l, -1) {
		if !strings.HasPrefix(strings.TrimSpace(m[1]), "#") {
			return errSVGExternal
		}
	}
	return nil
}

// rasterizeSVG draws svg stretched onto a w x h transparent canvas. Without
// antialiasing every path is drawn on its own layer and pixels with alpha
// >= 128 become fully opaque.
func rasterizeSVG(svg string, w, h int, antialias bool) (*image.NRGBA, error) {
	icon, err := oksvg.ReadIconStream(strings.NewReader(svg), oksvg.StrictErrorMode)
	if err != nil {
		return nil, fmt.Errorf("svg parse error: %v", err)
	}
	if len(icon.SVGPaths) > maxSVGPaths {
		return nil, fmt.Errorf("svg has too many shapes (max %d)", maxSVGPaths)
	}
	icon.SetTarget(0, 0, float64(w), float64(h))
	dst := image.NewNRGBA(image.Rect(0, 0, w, h))
	if antialias {
		icon.Draw(rasterx.NewDasher(w, h, rasterx.NewScannerGV(w, h, dst, dst.Bounds())), 1)
		return dst, nil
	}
	for i := range icon.SVGPaths {
		layer := image.NewNRGBA(image.Rect(0, 0, w, h))
		d := rasterx.NewDasher(w, h, rasterx.NewScannerGV(w, h, layer, layer.Bounds()))
		icon.SVGPaths[i].DrawTransformed(d, 1, icon.Transform)
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				if c := layer.NRGBAAt(x, y); c.A >= 128 {
					dst.SetNRGBA(x, y, color.NRGBA{R: c.R, G: c.G, B: c.B, A: 255})
				}
			}
		}
	}
	return dst, nil
}

func (e *env) svgToPNG(in svgToPNGIn) (*sdk.CallToolResult, svgToPNGOut, error) {
	var zero svgToPNGOut
	e.writeMu.Lock()
	defer e.writeMu.Unlock()

	if _, err := e.templateDir(in.Template); err != nil {
		return nil, zero, err
	}
	name := editor.SanitizeFilename(in.Name)
	if strings.ToLower(filepath.Ext(name)) != ".png" {
		return nil, zero, errors.New("name must end in .png")
	}
	if in.Width < 1 || in.Width > maxSVGSide || in.Height < 1 || in.Height > maxSVGSide {
		return nil, zero, fmt.Errorf("width and height must be 1-%d", maxSVGSide)
	}
	if len(in.SVG) > maxSVGBytes {
		return nil, zero, fmt.Errorf("svg too large (max %d KiB)", maxSVGBytes>>10)
	}
	if err := checkSVGSource(in.SVG); err != nil {
		return nil, zero, err
	}
	aa := in.Antialias == nil || *in.Antialias
	img, err := rasterizeSVG(in.SVG, in.Width, in.Height, aa)
	if err != nil {
		return nil, zero, err
	}
	if err := writePNGFile(e, in.Template, name, img); err != nil {
		return nil, zero, err
	}
	out := svgToPNGOut{Name: name, Width: in.Width, Height: in.Height}
	shown := image.Image(img)
	if in.Width <= canvasSize && in.Height <= canvasSize {
		shown = scaleNearest(img, 2)
	}
	res, err := jsonImageResult(out, []image.Image{shown})
	if err != nil {
		return nil, zero, err
	}
	return res, out, nil
}
