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
	errSVGUseCycle = errors.New("svg contains a recursive <use> reference")
	errSVGUseBomb  = fmt.Errorf("svg has too many shapes after <use> expansion (max %d)", maxSVGPaths)
	errSVGTooDeep  = fmt.Errorf("svg is nested too deeply (max %d levels)", maxSVGDepth)
)

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
// shape count with saturating arithmetic. It also limits nesting depth.
func checkSVGStructure(svg string) error {
	dec := xml.NewDecoder(strings.NewReader(svg))
	dec.CharsetReader = charset.NewReaderLabel // same decoding as oksvg
	defs := map[string][]*svgDefRun{}
	var (
		depth, defsDepth int // defsDepth: depth of the open <defs>, 0 if none
		inGrad           bool
		cur              *svgDefRun // current run inside <defs>
		topShapes        int
		topUses          []string
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
			if name == "defs" && defsDepth == 0 {
				defsDepth = depth
				continue
			}
			href := ""
			id := ""
			for _, a := range t.Attr {
				switch a.Name.Local {
				case "href":
					href = strings.TrimSpace(a.Value)
				case "id":
					id = a.Value
				}
			}
			if defsDepth > 0 && !inGrad {
				if id != "" || cur == nil {
					cur = &svgDefRun{}
					// An element without id before the first id still forms a
					// run in oksvg (keyed ""); it can never be referenced.
					defs[id] = append(defs[id], cur)
				}
				if name == "use" {
					cur.uses = append(cur.uses, strings.TrimPrefix(href, "#"))
				} else if svgShapeTags[name] {
					cur.shapes++
				}
				continue
			}
			if defsDepth > 0 {
				continue // gradient children
			}
			if name == "use" {
				topUses = append(topUses, strings.TrimPrefix(href, "#"))
			} else if svgShapeTags[name] {
				topShapes++
			}
		case xml.EndElement:
			if t.Name.Local == "radialGradient" || t.Name.Local == "linearGradient" {
				inGrad = false
			}
			if t.Name.Local == "defs" && depth == defsDepth {
				defsDepth = 0
				closeRun()
			}
			depth--
		}
	}
	if len(topUses) == 0 && !anyUse(defs) {
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
	for _, r := range defs[""] {
		for _, u := range r.uses {
			if _, err := cost(u); err != nil {
				return err
			}
		}
	}
	total := topShapes
	for _, u := range topUses {
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

func anyUse(defs map[string][]*svgDefRun) bool {
	for _, runs := range defs {
		for _, r := range runs {
			if len(r.uses) > 0 {
				return true
			}
		}
	}
	return false
}

func truncateID(id string) string {
	if r := []rune(id); len(r) > 40 {
		return string(r[:40]) + "…"
	}
	return id
}
