package mcp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// setMetaScalars sets (or adds) scalar values under the top-level `meta` key
// of a template YAML document. It edits the text: only the lines of the
// given keys change (missing keys are inserted right after "meta:"), so
// blank lines and comments stay byte-identical. Values are written as JSON
// strings, which are valid single-line YAML double-quoted scalars, so any
// text is safe. Layouts the text edit does not handle (flow-style meta,
// block or multi-line values, no meta) fall back to re-encoding.
func setMetaScalars(src []byte, kv map[string]string) ([]byte, error) {
	if out, ok := setMetaScalarsText(src, kv); ok {
		return out, nil
	}
	return setMetaScalarsReencode(src, kv)
}

// setMetaScalarsText is the textual edit of setMetaScalars; ok is false if
// the document layout is not supported.
func setMetaScalarsText(src []byte, kv map[string]string) ([]byte, bool) {
	var doc yaml.Node
	if err := yaml.Unmarshal(src, &doc); err != nil || doc.Kind != yaml.DocumentNode ||
		len(doc.Content) != 1 || doc.Content[0].Kind != yaml.MappingNode {
		return nil, false
	}
	root := doc.Content[0]
	var metaKey, meta *yaml.Node
	for i := 0; i+1 < len(root.Content); i += 2 {
		if root.Content[i].Value == "meta" {
			metaKey, meta = root.Content[i], root.Content[i+1]
		}
	}
	if meta == nil || meta.Kind != yaml.MappingNode || meta.Style&yaml.FlowStyle != 0 || len(meta.Content) == 0 {
		return nil, false
	}
	text := string(src)
	nl := "\n"
	if strings.Contains(text, "\r\n") {
		nl = "\r\n"
	}
	lines := strings.Split(text, "\n")
	metaLine := metaKey.Line - 1
	if metaLine < 0 || metaLine >= len(lines) || strings.TrimRight(lines[metaLine], "\r") != "meta:" {
		return nil, false
	}
	indent := strings.Repeat(" ", meta.Content[0].Column-1)
	cr := strings.TrimSuffix(nl, "\n")
	var insert []string
	insertAfter := metaLine // new keys go after "meta:" or after the name line
	for _, k := range sortedMetaKeys(kv) {
		enc, err := json.Marshal(kv[k])
		if err != nil {
			return nil, false
		}
		line := indent + k + ": " + string(enc) + cr
		found := false
		for i := 0; i+1 < len(meta.Content); i += 2 {
			kn, vn := meta.Content[i], meta.Content[i+1]
			if kn.Value != k {
				continue
			}
			// Only single-line scalars on the key's own line are replaced.
			if vn.Kind != yaml.ScalarNode || vn.Line != kn.Line || vn.Style&(yaml.LiteralStyle|yaml.FoldedStyle) != 0 {
				return nil, false
			}
			lines[kn.Line-1] = line
			if k == "name" {
				insertAfter = kn.Line - 1
			}
			found = true
			break
		}
		if !found {
			insert = append(insert, line)
		}
	}
	if len(insert) > 0 {
		rest := append(insert, lines[insertAfter+1:]...)
		lines = append(lines[:insertAfter+1:insertAfter+1], rest...)
	}
	out := []byte(strings.Join(lines, "\n"))
	if !sameExceptMeta(src, out, kv) {
		return nil, false // e.g. a value continued on the next line
	}
	return out, true
}

// sameExceptMeta reports whether out decodes to the same document as src
// except that meta[k] == kv[k] for the given keys.
func sameExceptMeta(src, out []byte, kv map[string]string) bool {
	var a, b map[string]any
	if yaml.Unmarshal(src, &a) != nil || yaml.Unmarshal(out, &b) != nil {
		return false
	}
	am, _ := a["meta"].(map[string]any)
	bm, _ := b["meta"].(map[string]any)
	if am == nil || bm == nil {
		return false
	}
	for k, v := range kv {
		if bv, ok := bm[k].(string); !ok || bv != v {
			return false
		}
		delete(am, k)
		delete(bm, k)
	}
	return reflect.DeepEqual(a, b)
}

func sortedMetaKeys(kv map[string]string) []string {
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
	return keys
}

// setMetaScalarsReencode is the fallback of setMetaScalars: it edits the
// node tree and re-encodes the document (comments are kept, blank lines and
// formatting are not).
func setMetaScalarsReencode(src []byte, kv map[string]string) ([]byte, error) {
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
	for _, k := range sortedMetaKeys(kv) {
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
