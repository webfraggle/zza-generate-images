package renderer

import (
	"fmt"
	"regexp"

	"gopkg.in/yaml.v3"
)

// identPathRe matches dotted identifier paths such as "gleis" or "zug1.vonnach".
var identPathRe = regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_]*(\.[A-Za-z_][A-Za-z0-9_]*)*`)

// ReferencedPaths returns every identifier path that appears in a scalar value
// of the given template YAML. Mapping keys and comments are ignored. The result
// is a superset of the data paths the template reads (it also contains function
// names, file names etc.); callers check membership of known data paths.
func ReferencedPaths(yamlSrc []byte) (map[string]bool, error) {
	var root yaml.Node
	if err := yaml.Unmarshal(yamlSrc, &root); err != nil {
		return nil, fmt.Errorf("parse template yaml: %w", err)
	}
	paths := make(map[string]bool)
	collectScalarValues(&root, paths)
	return paths, nil
}

// collectScalarValues walks n and adds identifier tokens from all scalar
// values (mapping values and sequence items, never mapping keys) to paths.
func collectScalarValues(n *yaml.Node, paths map[string]bool) {
	switch n.Kind {
	case yaml.ScalarNode:
		for _, tok := range identPathRe.FindAllString(n.Value, -1) {
			paths[tok] = true
		}
	case yaml.MappingNode:
		// Content alternates key, value; skip the keys.
		for i := 1; i < len(n.Content); i += 2 {
			collectScalarValues(n.Content[i], paths)
		}
	case yaml.DocumentNode, yaml.SequenceNode:
		for _, c := range n.Content {
			collectScalarValues(c, paths)
		}
	}
}
