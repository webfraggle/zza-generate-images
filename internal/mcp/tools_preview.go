package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/webfraggle/zza-generate-images/internal/renderer"
)

func init() { registerToolGroup("preview", registerPreviewTools) }

type previewIn struct {
	Template string         `json:"template" jsonschema:"template name"`
	Data     map[string]any `json:"data,omitempty" jsonschema:"test data object used instead of default.json"`
	Format   string         `json:"format,omitempty" jsonschema:"png (default) or gif"`
	Scale    int            `json:"scale,omitempty" jsonschema:"1-4, nearest neighbour; default 2 for png, 1 for gif"`
}

type previewOut struct {
	Width    int    `json:"width"`
	Height   int    `json:"height"`
	Scale    int    `json:"scale"`
	Frames   int    `json:"frames"`
	RenderMs int64  `json:"render_ms"`
	Note     string `json:"note,omitempty"`
}

func registerPreviewTools(s *sdk.Server, e *env) {
	sdk.AddTool(s, &sdk.Tool{
		Name:        "render_preview",
		Description: "Renders the template and returns the image (default: default.json as data, PNG scaled x2). Call it after EVERY change and compare with the reference. Pass data to test other trains (long destination, delay, empty fields). format \"gif\" returns a contact sheet of animation frames.",
	}, func(ctx context.Context, _ *sdk.CallToolRequest, in previewIn) (*sdk.CallToolResult, previewOut, error) {
		return e.renderPreview(in)
	})
}

// renderStill renders one still image of the template with a fresh renderer
// (no stale font cache), quantized like the server does.
func renderStill(e *env, template string, data map[string]any) (*image.NRGBA, *renderer.Template, error) {
	tmpl, err := renderer.LoadTemplate(e.templatesDir, template)
	if err != nil {
		return nil, nil, fmt.Errorf("render failed: %s", e.sanitizeErr(err))
	}
	img, err := renderer.New(e.templatesDir).Render(tmpl, data)
	if err != nil {
		return nil, nil, fmt.Errorf("render failed: %s", e.sanitizeErr(err))
	}
	return quantizeFor(tmpl, img), tmpl, nil
}

// quantizeFor reduces the palette if the template asks for it.
func quantizeFor(tmpl *renderer.Template, img *image.NRGBA) *image.NRGBA {
	if n := tmpl.Meta.Canvas.Colors; n > 0 {
		return toNRGBA(renderer.Quantize(img, n))
	}
	return img
}

// jsonImageResult builds a tool result with the JSON text of out followed by
// the given images (and optional trailing texts).
func jsonImageResult(out any, imgs []image.Image, texts ...string) (*sdk.CallToolResult, error) {
	b, err := json.Marshal(out)
	if err != nil {
		return nil, fmt.Errorf("cannot encode result")
	}
	res := &sdk.CallToolResult{Content: []sdk.Content{&sdk.TextContent{Text: string(b)}}}
	for _, img := range imgs {
		ic, err := imageContent(img)
		if err != nil {
			return nil, err
		}
		res.Content = append(res.Content, ic)
	}
	for _, t := range texts {
		res.Content = append(res.Content, &sdk.TextContent{Text: t})
	}
	return res, nil
}

func (e *env) renderPreview(in previewIn) (*sdk.CallToolResult, previewOut, error) {
	var zero previewOut
	dir, err := e.templateDir(in.Template)
	if err != nil {
		return nil, zero, err
	}
	format := in.Format
	if format == "" {
		format = "png"
	}
	if format != "png" && format != "gif" {
		return nil, zero, fmt.Errorf(`format must be "png" or "gif"`)
	}
	scale := in.Scale // 0 = default, chosen below (gif: after the animation check)
	if scale != 0 && (scale < 1 || scale > 4) {
		return nil, zero, fmt.Errorf("scale must be 1-4")
	}
	data := in.Data
	if data != nil {
		b, err := json.Marshal(data)
		if err != nil {
			return nil, zero, fmt.Errorf("invalid data")
		}
		if len(b) > maxPreviewDataBytes {
			return nil, zero, fmt.Errorf("data too large (max %d KiB)", maxPreviewDataBytes>>10)
		}
	} else {
		data = loadTestData(dir)
	}

	start := time.Now()
	out := previewOut{Scale: scale, Frames: 1}
	var img image.Image
	if format == "png" && scale == 0 {
		scale = 2
	}
	if format == "gif" {
		img, err = e.renderSheet(in.Template, data, &scale, &out)
	} else {
		var still *image.NRGBA
		still, _, err = renderStill(e, in.Template, data)
		if err == nil {
			out.Width, out.Height = still.Bounds().Dx(), still.Bounds().Dy()
			img = scaleNearest(still, scale)
		}
	}
	if err != nil {
		return nil, zero, err
	}
	out.Scale = scale
	out.RenderMs = time.Since(start).Milliseconds()
	res, err := jsonImageResult(out, []image.Image{img})
	if err != nil {
		return nil, zero, err
	}
	return res, out, nil
}

// renderSheet renders the animation as a contact sheet; for templates that are
// not animated it returns the still image and sets a note.
func (e *env) renderSheet(template string, data map[string]any, scale *int, out *previewOut) (image.Image, error) {
	tmpl, err := renderer.LoadTemplate(e.templatesDir, template)
	if err != nil {
		return nil, fmt.Errorf("render failed: %s", e.sanitizeErr(err))
	}
	out.Width, out.Height = tmpl.Meta.Canvas.Width, tmpl.Meta.Canvas.Height
	if !tmpl.IsAnimated() {
		still, _, err := renderStill(e, template, data)
		if err != nil {
			return nil, err
		}
		out.Note = "template is not animated"
		if *scale == 0 {
			*scale = 2
		}
		return scaleNearest(still, *scale), nil
	}
	if *scale == 0 {
		*scale = 1
	}
	body, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("invalid data")
	}
	anim, err := renderer.New(e.templatesDir).RenderAnimation(tmpl, data, renderer.AnimationSeed(template, body))
	if err != nil {
		return nil, fmt.Errorf("render failed: %s", e.sanitizeErr(err))
	}
	out.Frames = len(anim.Frames)
	n := min(len(anim.Frames), maxSheetFrames)
	picked := make([]*image.NRGBA, 0, n)
	for i := 0; i < n; i++ {
		idx := 0
		if n > 1 {
			idx = i * (len(anim.Frames) - 1) / (n - 1)
		}
		picked = append(picked, quantizeFor(tmpl, anim.Frames[idx]))
	}
	sheet, used := contactSheet(picked, sheetColumns, *scale)
	*scale = used
	return sheet, nil
}

// contactSheet lays frames out left to right, top to bottom in cols columns,
// each enlarged by scale (nearest neighbour) and separated by a 2 px white
// gap. If the longest side would exceed maxSheetSide, scale is reduced; the
// scale actually used is returned.
func contactSheet(frames []*image.NRGBA, cols, scale int) (*image.NRGBA, int) {
	const gap = 2
	if len(frames) == 0 {
		return image.NewNRGBA(image.Rect(0, 0, 1, 1)), scale
	}
	fw, fh := frames[0].Bounds().Dx(), frames[0].Bounds().Dy()
	cols = min(cols, len(frames))
	rows := (len(frames) + cols - 1) / cols
	for scale > 1 && (cols*fw*scale+(cols-1)*gap > maxSheetSide || rows*fh*scale+(rows-1)*gap > maxSheetSide) {
		scale--
	}
	sheet := image.NewNRGBA(image.Rect(0, 0, cols*fw*scale+(cols-1)*gap, rows*fh*scale+(rows-1)*gap))
	draw.Draw(sheet, sheet.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	for i, f := range frames {
		x := (i % cols) * (fw*scale + gap)
		y := (i / cols) * (fh*scale + gap)
		big := scaleNearest(f, scale)
		draw.Draw(sheet, image.Rect(x, y, x+fw*scale, y+fh*scale), big, image.Point{}, draw.Over)
	}
	return sheet, scale
}
