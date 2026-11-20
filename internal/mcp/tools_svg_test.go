package mcp

import (
	"context"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const redSquare = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10"><rect x="2" y="2" width="6" height="6" fill="#ff0000"/></svg>`

func TestCheckSVGSource(t *testing.T) {
	for _, ok := range []string{redSquare, `<use href="#a"/>`, `<rect fill="url(#g)"/>`} {
		if err := checkSVGSource(ok); err != nil {
			t.Errorf("%q: unexpected error %v", ok, err)
		}
	}
	const textErr = "svg must not contain text — put text in a text layer of template.yaml"
	const extErr = "svg must not reference images or external files"
	const docErr = "svg must not contain DOCTYPE or ENTITY declarations"
	const styleErr = "svg must not contain style sheets, scripts or foreignObject"
	bad := []struct{ in, want string }{
		{`<text>a</text>`, textErr},
		{`<tspan>a</tspan>`, textErr},
		{`<TEXT>a</TEXT>`, textErr},
		{`<image href="a.png"/>`, extErr},
		{`<!DOCTYPE svg>`, docErr},
		{`<!ENTITY x "y">`, docErr},
		{`<use xlink:href="http://x"/>`, extErr},
		{`<style>@import url(x.css)</style>`, styleErr},
		{`<rect fill="url(http://x)"/>`, extErr},
		{`<script>x</script>`, styleErr},
		{`<foreignObject/>`, styleErr},
	}
	for _, b := range bad {
		err := checkSVGSource(b.in)
		if err == nil || !strings.Contains(err.Error(), b.want) {
			t.Errorf("%q: got %v, want %q", b.in, err, b.want)
		}
	}
}

func TestRasterizeSVG_Antialias(t *testing.T) {
	svg := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20"><circle cx="10" cy="10" r="7" fill="#ff0000"/></svg>`
	img, err := rasterizeSVG(svg, 20, 20, true)
	if err != nil {
		t.Fatal(err)
	}
	if c := img.NRGBAAt(10, 10); c != (color.NRGBA{255, 0, 0, 255}) {
		t.Errorf("center = %v", c)
	}
	partial := false
	for y := 0; y < 20; y++ {
		for x := 0; x < 20; x++ {
			if a := img.NRGBAAt(x, y).A; a > 0 && a < 255 {
				partial = true
			}
		}
	}
	if !partial {
		t.Error("no antialiased edge pixel")
	}
}

func TestRasterizeSVG_NoAntialias(t *testing.T) {
	svg := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20"><circle cx="8" cy="10" r="7" fill="#ff0000"/><circle cx="13" cy="10" r="7" fill="#0000ff"/></svg>`
	img, err := rasterizeSVG(svg, 20, 20, false)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[color.NRGBA]bool{}
	for y := 0; y < 20; y++ {
		for x := 0; x < 20; x++ {
			c := img.NRGBAAt(x, y)
			if c.A != 0 && c.A != 255 {
				t.Fatalf("pixel %d,%d alpha %d", x, y, c.A)
			}
			if c.A == 0 {
				c = color.NRGBA{}
			}
			seen[c] = true
		}
	}
	for c := range seen {
		if c != (color.NRGBA{}) && c != (color.NRGBA{255, 0, 0, 255}) && c != (color.NRGBA{0, 0, 255, 255}) {
			t.Errorf("unexpected colour %v", c)
		}
	}
	if !seen[color.NRGBA{255, 0, 0, 255}] || !seen[color.NRGBA{0, 0, 255, 255}] {
		t.Error("missing red or blue")
	}
}

func TestRasterizeSVG_TooManyPaths(t *testing.T) {
	svg := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10">` + strings.Repeat(`<rect width="1" height="1"/>`, 2001) + `</svg>`
	_, err := rasterizeSVG(svg, 10, 10, true)
	if err == nil || !strings.Contains(err.Error(), "svg has too many shapes (max 2000)") {
		t.Errorf("got %v", err)
	}
}

func TestSVGToPNG_Tool(t *testing.T) {
	dir := copyTemplates(t, "sbb-105-v1")
	cs := newSession(t, dir)
	res := callTool(t, cs, "svg_to_png", map[string]any{"template": "sbb-105-v1", "name": "logo.png", "svg": redSquare, "width": 40, "height": 20})
	requireOK(t, res)
	f, err := os.Open(filepath.Join(dir, "sbb-105-v1", "logo.png"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	cfg, _, err := image.DecodeConfig(f)
	if err != nil || cfg.Width != 40 || cfg.Height != 20 {
		t.Fatalf("file: %v %+v", err, cfg)
	}
	if b := resultImage(t, res).Bounds(); b.Dx() != 80 || b.Dy() != 40 {
		t.Errorf("result image %v", b)
	}
	txt := resultText(res)
	if !strings.Contains(txt, `"width":40`) || !strings.Contains(txt, `"height":20`) {
		t.Errorf("text %s", txt)
	}

	args := func(m map[string]any) map[string]any {
		a := map[string]any{"template": "sbb-105-v1", "name": "x.png", "svg": redSquare, "width": 10, "height": 10}
		for k, v := range m {
			a[k] = v
		}
		return a
	}
	// failing calls must not write a file
	fail := func(m map[string]any, want string) {
		t.Helper()
		a := args(m)
		requireToolError(t, callTool(t, cs, "svg_to_png", a), want)
		if _, err := os.Stat(filepath.Join(dir, "sbb-105-v1", a["name"].(string))); err == nil {
			t.Errorf("file %v written despite error %q", a["name"], want)
		}
	}
	fail(map[string]any{"name": "logo.jpg"}, "name must end in .png")
	fail(map[string]any{"width": 0}, "width and height must be 1-1024")
	fail(map[string]any{"height": 1025}, "width and height must be 1-1024")
	big := redSquare + strings.Repeat(" ", maxSVGBytes+1-len(redSquare))
	fail(map[string]any{"svg": big}, "svg too large (max 256 KiB)")
	fail(map[string]any{"svg": `<svg xmlns="http://www.w3.org/2000/svg"><rect width="1" height="1"</svg>`}, "svg parse error: ")
	fail(map[string]any{"svg": `<svg xmlns="http://www.w3.org/2000/svg"><text>a</text></svg>`}, "must not contain text")
	fail(map[string]any{"svg": ""}, "svg contains no drawable shapes")
	fail(map[string]any{"svg": `<svg/>`}, "svg contains no drawable shapes")
	fail(map[string]any{"svg": `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10">` + strings.Repeat(`<rect width="1" height="1"/>`, 100) + `</svg>`, "width": 1024, "height": 1024},
		"svg too complex for this output size (reduce shapes or size)")
}

func TestSVGToPNG_NoAntialiasViaTool(t *testing.T) {
	dir := copyTemplates(t, "sbb-105-v1")
	cs := newSession(t, dir)
	circle := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20"><circle cx="10" cy="10" r="7" fill="#ff0000"/></svg>`
	res := callTool(t, cs, "svg_to_png", map[string]any{"template": "sbb-105-v1", "name": "dot.png", "svg": circle, "width": 20, "height": 20, "antialias": false})
	requireOK(t, res)
	f, err := os.Open(filepath.Join(dir, "sbb-105-v1", "dot.png"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	for y := 0; y < 20; y++ {
		for x := 0; x < 20; x++ {
			if _, _, _, a := img.At(x, y).RGBA(); a != 0 && a != 0xffff {
				t.Fatalf("pixel %d,%d has partial alpha", x, y)
			}
		}
	}
}

func TestSVGToPNG_Timeout(t *testing.T) {
	old := svgRenderTimeout
	svgRenderTimeout = time.Nanosecond
	defer func() { svgRenderTimeout = old }()
	dir := copyTemplates(t, "sbb-105-v1")
	cs := newSession(t, dir)
	res := callTool(t, cs, "svg_to_png", map[string]any{"template": "sbb-105-v1", "name": "t.png", "svg": redSquare, "width": 500, "height": 500})
	requireToolError(t, res, "svg rendering timed out")
	if _, err := os.Stat(filepath.Join(dir, "sbb-105-v1", "t.png")); err == nil {
		t.Error("file written after timeout")
	}
}

// A client cancel is not a timeout.
func TestSVGToPNG_CancelledNotTimeout(t *testing.T) {
	e, _ := sbbEnv(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, err := e.svgToPNG(ctx, svgToPNGIn{Template: "sbb-105-v1", Name: "c.png", SVG: redSquare, Width: 500, Height: 500})
	if err == nil || err.Error() != "svg rendering cancelled" {
		t.Fatalf("got %v", err)
	}
}
