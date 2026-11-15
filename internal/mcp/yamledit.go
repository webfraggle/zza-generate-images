package mcp

import (
	"bytes"
	"fmt"
	"sort"

	"gopkg.in/yaml.v3"
)

// setMetaScalars sets (or adds) scalar values under the top-level `meta` key
// of a template YAML document, keeping the rest of the document and its
// comments. Values are encoded by the YAML encoder, so any text is safe.
func setMetaScalars(src []byte, kv map[string]string) ([]byte, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(src, &doc); err != nil {
		return nil, fmt.Errorf("decoding template YAML: %w", err)
	}
	if doc.Kind != yaml.DocumentNode || len(doc.Content) != 1 || doc.Content[0].Kind != yaml.MappingNode {
		return nil, fmt.Errorf("template YAML is not a mapping")
	}
	root := doc.Content[0]
	var meta *yaml.Node
	for i := 0; i+1 < len(root.Content); i += 2 {
		if root.Content[i].Value == "meta" {
			meta = root.Content[i+1]
		}
	}
	if meta == nil {
		meta = &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		root.Content = append([]*yaml.Node{{Kind: yaml.ScalarNode, Tag: "!!str", Value: "meta"}, meta}, root.Content...)
	}
	if meta.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("meta is not a mapping")
	}
	keys := make([]string, 0, len(kv))
	for k := range kv {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if (keys[i] == "name") != (keys[j] == "name") {
			return keys[i] == "name"
		}
		return keys[i] < keys[j]
	})
	for _, k := range keys {
		val := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: kv[k]}
		found := false
		for i := 0; i+1 < len(meta.Content); i += 2 {
			if meta.Content[i].Value == k {
				meta.Content[i+1] = val
				found = true
				break
			}
		}
		if !found {
			meta.Content = append(meta.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: k}, val)
		}
	}
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(&doc); err != nil {
		return nil, fmt.Errorf("encoding template YAML: %w", err)
	}
	if err := enc.Close(); err != nil {
		return nil, fmt.Errorf("encoding template YAML: %w", err)
	}
	return buf.Bytes(), nil
}
