package mcp

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strings"

	"golang.org/x/net/html/charset"
)

// maxSVGDepth limits the XML element nesting of svg_to_png input.
const maxSVGDepth = 64

var (
	errSVGUseCycle   = errors.New("svg contains a recursive <use> reference")
	errSVGUseBomb    = fmt.Errorf("svg has too many shapes after <use> expansion (max %d)", maxSVGPaths)
	errSVGTooDeep    = fmt.Errorf("svg is nested too deeply (max %d levels)", maxSVGDepth)
	errSVGNestedDefs = errors.New("svg contains nested <defs>")
	errSVGUseInPaint = errors.New("svg: <use> is not allowed inside gradients, patterns, clip paths or masks")
)

// svgPaintServers are elements whose children oksvg handles specially; a
// <use> inside them is expanded outside the definition model.
var svgPaintServers = map[string]bool{
	"linearGradient": true, "radialGradient": true, "pattern": true, "clipPath": true, "mask": true,
}

// svgShapeTags are the elements oksvg turns into a path.
var svgShapeTags = map[string]bool{
	"rect": true, "circle": true, "ellipse": true, "line": true,
	"polyline": true, "polygon": true, "path": true,
}

// svgDefRun mirrors an oksvg definition: inside <defs>, every element with
// an id starts a new run, and all following elements (until the next id,
// regardless of nesting) belong to it. A <use> expands the whole run.
type svgDefRun struct {
	shapes int
	uses   []string
}

// checkSVGStructure scans svg with encoding/xml before oksvg sees it. oksvg
// expands <use> recursively while parsing: a self-referencing <use> overflows
// the stack (fatal, not recoverable) and nested <use> fan-out grows
// exponentially before the path limit applies. This pre-scan models oksvg's
// definition runs, rejects unknown ids and cycles, and computes the expanded
// shape count with saturating arithmetic, counting every <use> of the
// document regardless of context. Nested <defs> and <use> inside paint
// servers (gradients, patterns, clip paths, masks) are rejected, because
// oksvg expands <use> there outside the modelled definition runs. It also
// limits nesting depth.
func checkSVGStructure(svg string) error {
	dec := xml.NewDecoder(strings.NewReader(svg))
	dec.CharsetReader = charset.NewReaderLabel // same decoding as oksvg
	defs := map[string][]*svgDefRun{}
	var (
		depth, defsDepth int // defsDepth: depth of the open <defs>, 0 if none
		inGrad           bool
		paintDepth       int        // number of open paint-server elements
		cur              *svgDefRun // current run inside <defs>
		topShapes        int
		allUses          []string // every <use> of the document, in any context
	)
	closeRun := func() { cur = nil }
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			// Reject instead of passing on: what this scan cannot read must
			// not reach oksvg unchecked.
			return fmt.Errorf("svg parse error: %v", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			depth++
			if depth > maxSVGDepth {
				return errSVGTooDeep
			}
			name := t.Name.Local
			if name == "radialGradient" || name == "linearGradient" {
				inGrad = true
			}
			if name == "defs" {
				if defsDepth != 0 {
					// oksvg leaves defs mode at the inner </defs>.
					return errSVGNestedDefs
				}
				defsDepth = depth
				continue
			}
			id := ""
			var hrefs []string // all href attributes (href, xlink:href, …)
			for _, a := range t.Attr {
				switch a.Name.Local {
				case "href":
					hrefs = append(hrefs, strings.TrimPrefix(strings.TrimSpace(a.Value), "#"))
				case "id":
					id = a.Value
				}
			}
			if name == "use" {
				if paintDepth > 0 {
					return errSVGUseInPaint
				}
				if len(hrefs) == 0 {
					hrefs = []string{""}
				}
				allUses = append(allUses, hrefs...)
			}
			if svgPaintServers[name] {
				paintDepth++
			}
			if defsDepth > 0 && !inGrad {
				if id != "" || cur == nil {
					cur = &svgDefRun{}
					// An element without id before the first id still forms a
					// run in oksvg (keyed ""); it can never be referenced.
					defs[id] = append(defs[id], cur)
				}
				if name == "use" {
					cur.uses = append(cur.uses, hrefs...)
				} else if svgShapeTags[name] {
					cur.shapes++
				}
				continue
			}
			if defsDepth > 0 {
				continue // gradient children
			}
			if svgShapeTags[name] {
				topShapes++
			}
		case xml.EndElement:
			if t.Name.Local == "radialGradient" || t.Name.Local == "linearGradient" {
				inGrad = false
			}
			if svgPaintServers[t.Name.Local] && paintDepth > 0 {
				paintDepth--
			}
			if t.Name.Local == "defs" && depth == defsDepth {
				defsDepth = 0
				closeRun()
			}
			depth--
		}
	}
	if len(allUses) == 0 {
		return nil
	}

	const limit = maxSVGPaths + 1 // saturation value
	sat := func(a, b int) int {
		if a+b > limit || a+b < 0 {
			return limit
		}
		return a + b
	}
	const (
		white = iota
		grey
		black
	)
	state := map[string]int{}
	memo := map[string]int{}
	var cost func(id string) (int, error)
	cost = func(id string) (int, error) {
		runs, ok := defs[id]
		if !ok || id == "" {
			return 0, fmt.Errorf("svg <use> references unknown id %q", truncateID(id))
		}
		switch state[id] {
		case grey:
			return 0, errSVGUseCycle
		case black:
			return memo[id], nil
		}
		state[id] = grey
		best := 0
		for _, r := range runs {
			n := r.shapes
			for _, u := range r.uses {
				c, err := cost(u)
				if err != nil {
					return 0, err
				}
				n = sat(n, c)
			}
			best = max(best, n)
		}
		state[id] = black
		memo[id] = best
		return best, nil
	}
	// Check every definition (cycles in unused defs are rejected too:
	// oksvg would only crash on use, but they are never legitimate).
	for id := range defs {
		if id == "" {
			continue
		}
		if _, err := cost(id); err != nil {
			return err
		}
	}
	// Every <use> counts, whatever its context (definition run, group,
	// marker, …): this over-estimates nested reuse but cannot be bypassed by
	// contexts in which oksvg expands a <use> differently than modelled above.
	total := topShapes
	for _, u := range allUses {
		c, err := cost(u)
		if err != nil {
			return err
		}
		total = sat(total, c)
	}
	if total > maxSVGPaths {
		return errSVGUseBomb
	}
	return nil
}

func truncateID(id string) string {
	if r := []rune(id); len(r) > 40 {
		return string(r[:40]) + "…"
	}
	return id
}
