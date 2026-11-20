package mcp

import (
	"strings"
	"testing"
)

func TestTidyRenderMsg(t *testing.T) {
	cases := map[string]string{
		`renderer: Render: layer 5 (image): renderImage: opening "pfeil.png": open pfeil.png: no such file or directory`: `layers[5] (image): file "pfeil.png" not found in template — create it first with svg_to_png, upload_image or import_image`,
		`renderer: Render: block layer 2: layer 0 (text): getFace: font ID "bold" not found in template`:                    `layers[2].layers[0] (text): font ID "bold" not found in template — add it with add_font and reference its id`,
		`renderer: Render: layer 3: unknown type "rct"`:                                                                    `layers[3]: unknown type "rct"`,
		`renderer: Render: layer 1 (image): renderImage: file "a.gif" is not a permitted image type (only .png and .jpg)`:    `layers[1] (image): file "a.gif" is not a permitted image type (only .png and .jpg)`,
		`canvas must be 240x240`: `canvas must be 240x240`,
	}
	for in, want := range cases {
		if got := tidyRenderMsg(in); got != want {
			t.Errorf("tidyRenderMsg(%q)\n got %q\nwant %q", in, got, want)
		}
	}
}

// write_file rejections name the layer in the same form as the warnings
// (layers[i]) and say how to fix missing images and fonts.
func TestWriteFile_RejectionTexts(t *testing.T) {
	dir := copyTemplates(t, "sbb-105-v1")
	cs := newSession(t, dir)
	img := "meta:\n  canvas: {width: 240, height: 240}\nlayers:\n  - {type: rect, x: 0, y: 0, width: 240, height: 240, color: \"#000000\"}\n  - {type: image, file: pfeil.png, x: 0, y: 0}\n"
	res := callTool(t, cs, "write_file", map[string]any{"template": "sbb-105-v1", "name": "template.yaml", "content": img})
	requireToolError(t, res, `layers[1] (image): file "pfeil.png" not found in template — create it first with svg_to_png, upload_image or import_image`)
	for _, noise := range []string{"renderer:", "renderImage", "opening", dir} {
		if strings.Contains(resultText(res), noise) {
			t.Errorf("error contains %q: %s", noise, resultText(res))
		}
	}
	txt := "meta:\n  canvas: {width: 240, height: 240}\nlayers:\n  - {type: text, value: x, font: nope, x: 0, y: 0, size: 12, color: \"#ffffff\"}\n"
	res = callTool(t, cs, "write_file", map[string]any{"template": "sbb-105-v1", "name": "template.yaml", "content": txt})
	requireToolError(t, res, `layers[0] (text): font ID "nope" not found in template — add it with add_font and reference its id`)
}
