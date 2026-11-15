package mcp

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"math"
	"path/filepath"
	"strings"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	xdraw "golang.org/x/image/draw"

	"github.com/webfraggle/zza-generate-images/internal/editor"
	"github.com/webfraggle/zza-generate-images/internal/renderer"
)

func init() { registerToolGroup("photo", registerPhotoTools) }

type photoOut struct {
	Name    string   `json:"name"`
	Width   int      `json:"width"`
	Height  int      `json:"height"`
	Palette []string `json:"palette,omitempty"`
}

type photoBase struct {
	Template string `json:"template" jsonschema:"template name"`
	Src      string `json:"src" jsonschema:"source image in the template (.png, .jpg or .jpeg)"`
	Dst      string `json:"dst" jsonschema:"new PNG file name in the template (must differ from src)"`
}

type perspectiveIn struct {
	photoBase
	Corners []point `json:"corners" jsonschema:"4 corners {x,y} in source pixels: top-left, top-right, bottom-right, bottom-left"`
	Width   int     `json:"width" jsonschema:"output width 1-2048"`
	Height  int     `json:"height" jsonschema:"output height 1-2048"`
}

type cropIn struct {
	photoBase
	X      int `json:"x" jsonschema:"left edge in source pixels"`
	Y      int `json:"y" jsonschema:"top edge in source pixels"`
	Width  int `json:"width" jsonschema:"rectangle width"`
	Height int `json:"height" jsonschema:"rectangle height"`
}

type resizeIn struct {
	photoBase
	Width  int    `json:"width" jsonschema:"output width 1-2048"`
	Height int    `json:"height" jsonschema:"output height 1-2048"`
	Mode   string `json:"mode,omitempty" jsonschema:"nearest or smooth (default)"`
}

type quantizeIn struct {
	photoBase
	Colors int `json:"colors" jsonschema:"number of colours 2-64"`
}

type colorToAlphaIn struct {
	photoBase
	Color     string `json:"color" jsonschema:"colour to make transparent, #rrggbb"`
	Tolerance *int   `json:"tolerance,omitempty" jsonschema:"0-255 per channel, default 24"`
}

type sampleIn struct {
	Template string  `json:"template" jsonschema:"template name"`
	Src      string  `json:"src" jsonschema:"source image in the template (.png, .jpg or .jpeg)"`
	Points   []point `json:"points" jsonschema:"points {x,y} in source pixels (max 64)"`
	Radius   int     `json:"radius,omitempty" jsonschema:"0-5 px, average over a (2r+1)x(2r+1) square; default 0"`
}

type sampledColor struct {
	X   int    `json:"x"`
	Y   int    `json:"y"`
	Hex string `json:"hex"`
}

type sampleOut struct {
	Colors []sampledColor `json:"colors"`
}

func registerPhotoTools(s *sdk.Server, e *env) {
	sdk.AddTool(s, &sdk.Tool{
		Name:        "perspective_correct",
		Description: "Straightens a photo of a display: corners = the 4 corners of the display area in source pixels, order top-left, top-right, bottom-right, bottom-left; width/height = output size (e.g. 240x120 for one side). Writes a new PNG; the original is kept.",
	}, func(ctx context.Context, _ *sdk.CallToolRequest, in perspectiveIn) (*sdk.CallToolResult, photoOut, error) {
		return e.photoTool(in.photoBase, func(img *image.NRGBA) (*image.NRGBA, []string, error) {
			if len(in.Corners) != 4 {
				return nil, nil, fmt.Errorf("corners must contain exactly 4 points")
			}
			if err := checkOutputSize(in.Width, in.Height); err != nil {
				return nil, nil, err
			}
			var c [4]point
			copy(c[:], in.Corners)
			out, err := warpPerspective(img, c, in.Width, in.Height)
			return out, nil, err
		})
	})

	sdk.AddTool(s, &sdk.Tool{
		Name:        "crop_image",
		Description: "Cuts a rectangle out of an image (e.g. an original logo) into a new PNG.",
	}, func(ctx context.Context, _ *sdk.CallToolRequest, in cropIn) (*sdk.CallToolResult, photoOut, error) {
		return e.photoTool(in.photoBase, func(img *image.NRGBA) (*image.NRGBA, []string, error) {
			b := img.Bounds()
			if in.Width < 1 || in.Height < 1 || in.X < 0 || in.Y < 0 ||
				in.X > b.Dx()-in.Width || in.Y > b.Dy()-in.Height {
				return nil, nil, fmt.Errorf("crop rectangle exceeds image bounds 0,0–%d,%d", b.Dx(), b.Dy())
			}
			if err := checkOutputSize(in.Width, in.Height); err != nil {
				return nil, nil, err
			}
			out := image.NewNRGBA(image.Rect(0, 0, in.Width, in.Height))
			xdraw.Copy(out, image.Point{}, img, image.Rect(in.X, in.Y, in.X+in.Width, in.Y+in.Height), xdraw.Src, nil)
			return out, nil, nil
		})
	})

	sdk.AddTool(s, &sdk.Tool{
		Name:        "resize_image",
		Description: `Scales an image to width x height. mode "nearest" keeps hard pixels (LED/pixel look), "smooth" (default) for photos and logos.`,
	}, func(ctx context.Context, _ *sdk.CallToolRequest, in resizeIn) (*sdk.CallToolResult, photoOut, error) {
		return e.photoTool(in.photoBase, func(img *image.NRGBA) (*image.NRGBA, []string, error) {
			var scaler xdraw.Interpolator
			switch in.Mode {
			case "nearest":
				scaler = xdraw.NearestNeighbor
			case "", "smooth":
				scaler = xdraw.CatmullRom
			default:
				return nil, nil, fmt.Errorf(`mode must be "nearest" or "smooth"`)
			}
			if err := checkOutputSize(in.Width, in.Height); err != nil {
				return nil, nil, err
			}
			out := image.NewNRGBA(image.Rect(0, 0, in.Width, in.Height))
			scaler.Scale(out, out.Bounds(), img, img.Bounds(), xdraw.Src, nil)
			return out, nil, nil
		})
	})

	sdk.AddTool(s, &sdk.Tool{
		Name:        "quantize_colors",
		Description: "Reduces an image to N colours (2-64), removing photo noise; returns the palette as hex values for use in template.yaml.",
	}, func(ctx context.Context, _ *sdk.CallToolRequest, in quantizeIn) (*sdk.CallToolResult, photoOut, error) {
		return e.photoTool(in.photoBase, func(img *image.NRGBA) (*image.NRGBA, []string, error) {
			if in.Colors < 2 || in.Colors > 64 {
				return nil, nil, fmt.Errorf("colors must be 2-64")
			}
			p := renderer.Quantize(img, in.Colors)
			used := map[uint8]bool{}
			var palette []string
			b := p.Bounds()
			for y := b.Min.Y; y < b.Max.Y; y++ {
				for x := b.Min.X; x < b.Max.X; x++ {
					idx := p.ColorIndexAt(x, y)
					if used[idx] {
						continue
					}
					used[idx] = true
					c := color.NRGBAModel.Convert(p.Palette[idx]).(color.NRGBA)
					palette = append(palette, fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B))
				}
			}
			return toNRGBA(p), palette, nil
		})
	})

	sdk.AddTool(s, &sdk.Tool{
		Name:        "color_to_alpha",
		Description: "Makes one colour (#rrggbb, ± tolerance 0-255 per channel, default 24) transparent, e.g. the background of an icon.",
	}, func(ctx context.Context, _ *sdk.CallToolRequest, in colorToAlphaIn) (*sdk.CallToolResult, photoOut, error) {
		return e.photoTool(in.photoBase, func(img *image.NRGBA) (*image.NRGBA, []string, error) {
			c, err := parseHexColor(in.Color)
			if err != nil {
				return nil, nil, err
			}
			tol := 24
			if in.Tolerance != nil {
				tol = *in.Tolerance
			}
			if tol < 0 || tol > 255 {
				return nil, nil, fmt.Errorf("tolerance must be 0-255")
			}
			return colorToAlpha(img, c, tol), nil, nil
		})
	})

	sdk.AddTool(s, &sdk.Tool{
		Name:        "sample_colors",
		Description: "Reads the average colour around points of an image (radius 0-5 px). Use it to take exact hex colours from a reference photo.",
	}, func(ctx context.Context, _ *sdk.CallToolRequest, in sampleIn) (*sdk.CallToolResult, sampleOut, error) {
		return e.sampleColors(in)
	})
}

func checkOutputSize(w, h int) error {
	if w < 1 || h < 1 || w > maxToolOutputSide || h > maxToolOutputSide {
		return fmt.Errorf("width and height must be 1-%d", maxToolOutputSide)
	}
	return nil
}

// loadPhoto validates the source file name and loads it as NRGBA.
func (e *env) loadPhoto(template, src string) (*image.NRGBA, error) {
	if _, err := e.templateDir(template); err != nil {
		return nil, err
	}
	switch strings.ToLower(filepath.Ext(src)) {
	case ".png", ".jpg", ".jpeg":
	default:
		return nil, fmt.Errorf("src must be a .png, .jpg or .jpeg file")
	}
	p, err := editor.SafeFilePath(e.templatesDir, template, src)
	if err != nil {
		return nil, fmt.Errorf("invalid file name %q (allowed: letters, digits, . _ -)", src)
	}
	img, _, err := loadImageFile(p, maxImportBytes, maxToolInputSide, maxToolInputPixels)
	if err != nil {
		msg := err.Error()
		switch {
		case strings.HasPrefix(msg, "cannot read"):
			return nil, fmt.Errorf("cannot read %s", src)
		case strings.HasSuffix(msg, "pixels)"):
			msg = strings.Replace(msg, fmt.Sprintf("(max %d pixels)", maxToolInputPixels), fmt.Sprintf("(max %d megapixels)", maxToolInputPixels/1_000_000), 1)
			return nil, fmt.Errorf("%s", msg)
		}
		return nil, err
	}
	return toNRGBA(img), nil
}

// photoIO validates the common parameters, loads src and returns a write
// function that stores the result as dst and builds the tool result.
func photoIO(e *env, template, src, dst string) (img *image.NRGBA, write func(*image.NRGBA, ...string) (*sdk.CallToolResult, photoOut, error), err error) {
	if _, err := e.templateDir(template); err != nil {
		return nil, nil, err
	}
	if dst == src {
		return nil, nil, fmt.Errorf("dst must differ from src — originals are kept")
	}
	if strings.ToLower(filepath.Ext(dst)) != ".png" {
		return nil, nil, fmt.Errorf("dst must end in .png")
	}
	img, err = e.loadPhoto(template, src)
	if err != nil {
		return nil, nil, err
	}
	write = func(out *image.NRGBA, palette ...string) (*sdk.CallToolResult, photoOut, error) {
		var zero photoOut
		if err := writePNGFile(e, template, dst, out); err != nil {
			return nil, zero, err
		}
		po := photoOut{Name: dst, Width: out.Bounds().Dx(), Height: out.Bounds().Dy()}
		if len(palette) > 0 {
			po.Palette = palette
		}
		res, err := jsonImageResult(po, []image.Image{fitWithin(out, maxResultImageSide)})
		if err != nil {
			return nil, zero, err
		}
		return res, po, nil
	}
	return img, write, nil
}

// photoTool runs a transformation under the write lock and stores its result.
func (e *env) photoTool(b photoBase, transform func(*image.NRGBA) (*image.NRGBA, []string, error)) (*sdk.CallToolResult, photoOut, error) {
	var zero photoOut
	e.writeMu.Lock()
	defer e.writeMu.Unlock()
	img, write, err := photoIO(e, b.Template, b.Src, b.Dst)
	if err != nil {
		return nil, zero, err
	}
	out, palette, err := transform(img)
	if err != nil {
		return nil, zero, err
	}
	return write(out, palette...)
}

func (e *env) sampleColors(in sampleIn) (*sdk.CallToolResult, sampleOut, error) {
	var zero sampleOut
	if len(in.Points) == 0 {
		return nil, zero, fmt.Errorf("points must not be empty")
	}
	if len(in.Points) > maxSamplePoints {
		return nil, zero, fmt.Errorf("at most %d points", maxSamplePoints)
	}
	if in.Radius < 0 || in.Radius > maxSampleRadius {
		return nil, zero, fmt.Errorf("radius must be 0-%d", maxSampleRadius)
	}
	img, err := e.loadPhoto(in.Template, in.Src)
	if err != nil {
		return nil, zero, err
	}
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	out := sampleOut{Colors: make([]sampledColor, 0, len(in.Points))}
	for _, p := range in.Points {
		x, y := int(math.Floor(p.X)), int(math.Floor(p.Y))
		if x < 0 || y < 0 || x >= w || y >= h {
			return nil, zero, fmt.Errorf("point %d,%d is outside the image %dx%d", x, y, w, h)
		}
		c := sampleAt(img, x, y, in.Radius)
		out.Colors = append(out.Colors, sampledColor{X: x, Y: y, Hex: fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B)})
	}
	res, err := jsonImageResult(out, nil)
	if err != nil {
		return nil, zero, err
	}
	return res, out, nil
}
