package mcp

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	// layerPrefixRe matches the renderer's layer prefix, e.g.
	// "block layer 2: layer 0 (text): " or "layer 3: ".
	layerPrefixRe = regexp.MustCompile(`^((?:block layer \d+: )*)layer (\d+)((?: \([a-z]+\))?): `)
	blockNumRe    = regexp.MustCompile(`block layer (\d+): `)
	// funcPrefixRe matches Go function-name prefixes such as "renderImage: ".
	funcPrefixRe  = regexp.MustCompile(`^(?:renderer: |Render: |[a-z]+[A-Z][A-Za-z]*: )`)
	openingRe     = regexp.MustCompile(`^opening "([^"]+)": .*(?:no such file|cannot find|not exist)`)
	missingFontRe = regexp.MustCompile(`^font ID "[^"]*" not found in template$`)
)

// tidyRenderMsg turns renderer error texts into short, actionable messages
// for the model: Go function prefixes are dropped, the layer position is
// written like in warnings (layers[2].layers[0]), and missing image files and
// font ids get a hint which tool fixes them.
func tidyRenderMsg(s string) string {
	s = stripFuncPrefixes(s)
	path := ""
	if m := layerPrefixRe.FindStringSubmatch(s); m != nil {
		path = "layers"
		for _, b := range blockNumRe.FindAllStringSubmatch(m[1], -1) {
			path += "[" + b[1] + "].layers"
		}
		path += "[" + m[2] + "]" + m[3] + ": "
		s = stripFuncPrefixes(s[len(m[0]):])
	}
	switch {
	case openingRe.MatchString(s):
		f := openingRe.FindStringSubmatch(s)[1]
		s = fmt.Sprintf("file %q not found in template — create it first with svg_to_png, upload_image or import_image", f)
	case missingFontRe.MatchString(s):
		s += " — add it with add_font and reference its id"
	}
	return path + s
}

func stripFuncPrefixes(s string) string {
	for {
		loc := funcPrefixRe.FindStringIndex(s)
		if loc == nil {
			return strings.TrimSpace(s)
		}
		s = s[loc[1]:]
	}
}
