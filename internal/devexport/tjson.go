package devexport

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"gopkg.in/yaml.v3"
)

// Keys whose scalar is always the raw text (the renderer reads them as strings,
// so "FF0000" or "007" must not turn into numbers).
var stringKeys = map[string]bool{
	"type": true, "if": true, "elif": true, "file": true, "color": true, "value": true,
	"font": true, "align": true, "valign": true, "split_by": true, "var": true,
	"split": true, "id": true, "rotate": true, "name": true, "description": true,
	"author": true, "version": true, "display": true, "instructions": true, "timezone": true,
	"reel": true,
}

// IntOrExpr keys: an integer if the text parses as one, else the expression text.
var intExprKeys = map[string]bool{"x": true, "y": true, "width": true, "height": true, "max_width": true}

// Keys that may hold a conditional map (if/then/elif/then/…/else).
var condKeys = map[string]bool{"value": true, "color": true, "rotate": true}

// templateJSON converts template.yaml into the generic tree of t.json (see doc.go).
func templateJSON(path string) (map[string]any, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var root yaml.Node
	if err := yaml.Unmarshal(raw, &root); err != nil {
		return nil, err
	}
	c := &converter{expanding: map[*yaml.Node]bool{}}
	v, err := c.node(&root, "", false)
	if err != nil {
		return nil, err
	}
	m, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("template.yaml ist keine Map")
	}
	return m, nil
}

// maxYAMLNodes caps the nodes of template.yaml after expanding aliases (and
// merge keys). The largest test template (worst-layers) has about 1 900; the cap only
// stops alias bombs (a few lines expanding to billions of nodes).
const maxYAMLNodes = 200_000

// converter turns the YAML tree into the t.json tree. Aliases are expanded
// (like yaml.v3 does when decoding), with cycle detection and a node cap.
type converter struct {
	expanding map[*yaml.Node]bool // alias targets currently being expanded
	nodes     int
}

func (c *converter) node(n *yaml.Node, key string, str bool) (any, error) {
	if c.nodes++; c.nodes > maxYAMLNodes {
		return nil, fmt.Errorf("YAML hat nach dem Auflösen der Aliase mehr als %d Knoten", maxYAMLNodes)
	}
	switch n.Kind {
	case yaml.DocumentNode:
		if len(n.Content) == 0 {
			return nil, fmt.Errorf("leeres Dokument")
		}
		return c.node(n.Content[0], key, str)
	case yaml.AliasNode:
		if n.Alias == nil || c.expanding[n.Alias] {
			return nil, fmt.Errorf("zyklischer YAML-Alias *%s", n.Value)
		}
		c.expanding[n.Alias] = true
		v, err := c.node(n.Alias, key, str)
		delete(c.expanding, n.Alias)
		return v, err
	case yaml.MappingNode:
		if condKeys[key] {
			pairs := make([]any, 0, len(n.Content)/2)
			for i := 0; i+1 < len(n.Content); i += 2 {
				pairs = append(pairs, []any{n.Content[i].Value, n.Content[i+1].Value})
			}
			return map[string]any{"_cond": pairs}, nil
		}
		out := map[string]any{}
		var merges []*yaml.Node
		for i := 0; i+1 < len(n.Content); i += 2 {
			k, v := n.Content[i], n.Content[i+1]
			if k.Tag == "!!merge" {
				merges = append(merges, v)
				continue
			}
			cv, err := c.node(v, k.Value, false)
			if err != nil {
				return nil, err
			}
			if k.Value == "else" && v.Tag == "!!null" { // bare `else:` means true
				cv = true
			}
			out[k.Value] = cv
		}
		for _, mv := range merges {
			srcs := []*yaml.Node{mv}
			if mv.Kind == yaml.SequenceNode {
				srcs = mv.Content
			}
			for _, s := range srcs {
				cv, err := c.node(s, key, false)
				if err != nil {
					return nil, err
				}
				if m, ok := cv.(map[string]any); ok {
					for k, v := range m {
						if _, set := out[k]; !set {
							out[k] = v
						}
					}
				}
			}
		}
		return out, nil
	case yaml.SequenceNode:
		out := make([]any, 0, len(n.Content))
		for _, e := range n.Content {
			cv, err := c.node(e, key, key == "reel")
			if err != nil {
				return nil, err
			}
			out = append(out, cv)
		}
		return out, nil
	case yaml.ScalarNode:
		if stringKeys[key] || str {
			if n.Tag == "!!null" {
				return "", nil
			}
			return n.Value, nil
		}
		if intExprKeys[key] {
			if i, err := strconv.Atoi(n.Value); err == nil {
				return i, nil
			}
			return n.Value, nil
		}
		var v any
		if err := n.Decode(&v); err != nil {
			return nil, err
		}
		return v, nil
	}
	return nil, fmt.Errorf("unerwarteter YAML-Knoten")
}

// marshalJSON encodes v compactly without HTML escaping (map keys are sorted).
func marshalJSON(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}
