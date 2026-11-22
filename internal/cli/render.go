package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/webfraggle/zza-generate-images/internal/devexport"
	"github.com/webfraggle/zza-generate-images/internal/devicerender"
	"github.com/webfraggle/zza-generate-images/internal/renderer"
)

// RenderCmd returns the "render" cobra command of the desktop binary (cmd/zza). Desktop-only:
// it imports the cgo bridge internal/devicerender (--zza), which the CGO-free server build
// (cmd/zza-server) must not link.
func RenderCmd() *cobra.Command {
	var (
		templateName string
		inputFile    string
		outputFile   string
		templatesDir string
		format       string
		zza          bool
		timeFlag     string
	)

	cmd := &cobra.Command{
		Use:   "render",
		Short: "Render a template with JSON input data to a PNG image or animated GIF",
		Long: "Render a template with JSON input data to a PNG image or an animated GIF.\n" +
			"The format is taken from --format, else from the extension of -o (.gif or .png, case-insensitive), else PNG.\n" +
			"A GIF of a template without animation (splitflap or meta.animation.frames) has a single frame.\n" +
			"--zza renders with the on-device renderer core (as the ZZA controller would; PNG only).\n" +
			"--time fixes the clock, e.g. 2026-01-01T10:08:37 (without a zone: the template's time zone).",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Resolve output format: --format wins over the -o extension.
			if format == "" {
				format = "png"
				if strings.EqualFold(filepath.Ext(outputFile), ".gif") {
					format = "gif"
				}
			}
			if format != "png" && format != "gif" {
				return fmt.Errorf("render: unknown format %q (use png or gif)", format)
			}
			if zza && format != "png" {
				return fmt.Errorf("render: --zza kann nur PNG")
			}

			// Validate template name.
			if err := renderer.ValidateTemplateName(templateName); err != nil {
				return fmt.Errorf("render: %w", err)
			}

			// Read and parse JSON input.
			jsonBytes, err := os.ReadFile(inputFile)
			if err != nil {
				return fmt.Errorf("render: reading input file: %w", err)
			}
			var data map[string]interface{}
			if err := json.Unmarshal(jsonBytes, &data); err != nil {
				return fmt.Errorf("render: parsing JSON: %w", err)
			}

			// Load template.
			tmpl, err := renderer.LoadTemplate(templatesDir, templateName)
			if err != nil {
				return fmt.Errorf("render: loading template: %w", err)
			}

			// Clock: --time (in the template's time zone) or now.
			loc, err := time.LoadLocation(templateZone(tmpl.Meta.Timezone))
			if err != nil {
				return fmt.Errorf("render: time zone: %w", err)
			}
			now := time.Now().In(loc)
			var clock func() time.Time // nil = the renderer's own clock
			if timeFlag != "" {
				fixed, err := parseRenderTime(timeFlag, loc)
				if err != nil {
					return err
				}
				now = fixed
				clock = func() time.Time { return fixed }
			}

			// Render.
			r := renderer.New(templatesDir)
			if clock != nil {
				r = renderer.NewWithClock(templatesDir, clock)
			}
			var encode func(io.Writer) error
			if zza {
				img, err := renderZZA(templatesDir, templateName, jsonBytes, now)
				if err != nil {
					return err
				}
				encode = func(w io.Writer) error { return png.Encode(w, img) }
			} else if format == "gif" {
				seed := renderer.AnimationSeed(templateName, jsonBytes)
				anim, err := r.RenderAnimation(tmpl, data, seed)
				if err != nil {
					return fmt.Errorf("render: rendering: %w", err)
				}
				encode = func(w io.Writer) error { return renderer.EncodeGIF(w, anim, tmpl.Meta.Canvas.Colors) }
			} else {
				img, err := r.Render(tmpl, data)
				if err != nil {
					return fmt.Errorf("render: rendering: %w", err)
				}
				// Optionally reduce color palette.
				var encImg image.Image = img
				if tmpl.Meta.Canvas.Colors > 0 {
					encImg = renderer.Quantize(img, tmpl.Meta.Canvas.Colors)
				}
				encode = func(w io.Writer) error { return png.Encode(w, encImg) }
			}

			// Write output.
			outF, err := os.Create(outputFile)
			if err != nil {
				return fmt.Errorf("render: creating output file: %w", err)
			}
			defer func() {
				if outF != nil {
					outF.Close()
					os.Remove(outputFile) // clean up if not already closed successfully
				}
			}()

			if err := encode(outF); err != nil {
				return fmt.Errorf("render: encoding %s: %w", strings.ToUpper(format), err)
			}

			f := outF
			outF = nil // prevent defer from removing the file
			if err := f.Close(); err != nil {
				os.Remove(outputFile)
				return fmt.Errorf("render: closing output file: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Rendered %s to %s\n", templateName, outputFile)
			return nil
		},
	}

	cmd.Flags().StringVarP(&templateName, "template", "t", "", "Template name (required)")
	cmd.Flags().StringVarP(&inputFile, "input", "i", "", "Input JSON file (required)")
	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "Output file, .png or .gif (required)")
	cmd.Flags().StringVar(&format, "format", "", "Output format: png or gif (default: from -o extension, else png)")
	cmd.Flags().StringVar(&templatesDir, "templates-dir", "./templates", "Templates directory")
	cmd.Flags().BoolVar(&zza, "zza", false, "Render with the on-device renderer core (PNG only; fails for templates that are not ZZA compatible)")
	cmd.Flags().StringVar(&timeFlag, "time", "", "Fixed render time, RFC 3339; without a zone it is read in the template's time zone")

	_ = cmd.MarkFlagRequired("template")
	_ = cmd.MarkFlagRequired("input")
	_ = cmd.MarkFlagRequired("output")

	return cmd
}

// templateZone is the template's IANA zone, Europe/Berlin if unset.
func templateZone(name string) string {
	if name == "" {
		return "Europe/Berlin"
	}
	return name
}

// parseRenderTime reads an RFC 3339 time; without a zone offset it is taken
// in loc. The result is in loc.
func parseRenderTime(s string, loc *time.Location) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.In(loc), nil
	}
	if t, err := time.ParseInLocation("2006-01-02T15:04:05", s, loc); err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("render: invalid --time %q (want e.g. 2026-01-01T10:08:37)", s)
}

// zzaMemBudget is the memory budget of the preview core (same as internal/devpreview).
const zzaMemBudget = 16 << 20

// renderZZA exports the template and renders it with the device core.
func renderZZA(templatesDir, name string, data []byte, now time.Time) (*image.NRGBA, error) {
	pkg, err := devexport.Export(templatesDir, name)
	var inc *devexport.Incompatible
	if errors.As(err, &inc) {
		return nil, fmt.Errorf("render: %w", inc)
	}
	if err != nil {
		return nil, fmt.Errorf("render: exporting for ZZA: %w", err)
	}
	rd := devicerender.New(zzaMemBudget)
	defer rd.Close()
	if err := rd.Load(pkg); err != nil {
		return nil, fmt.Errorf("render: loading into the ZZA core: %w", err)
	}
	img, err := rd.Render(data, now)
	if err != nil {
		return nil, fmt.Errorf("render: ZZA rendering: %w", err)
	}
	return img, nil
}
