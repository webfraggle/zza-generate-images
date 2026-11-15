package mcp

import (
	"fmt"
	"reflect"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/webfraggle/zza-generate-images/internal/renderer"
)

// maxUnknownKeyWarnings caps the unknown-key warnings of one write_file call.
const maxUnknownKeyWarnings = 10

var (
	unmarshalerType = reflect.TypeOf((*yaml.Unmarshaler)(nil)).Elem()
	layerType       = reflect.TypeOf(renderer.Layer{})
)

// unknownKeyWarnings reports mapping keys of template.yaml that the renderer
// types do not know (typos such as "src_widht" or "iff" are otherwise
// silently ignored). The renderer's Layer type has its own UnmarshalYAML, so
// yaml.v3's KnownFields would not reach it; instead the yaml.Node tree is
// walked and every mapping is checked against the yaml tags of the matching
// struct, found by reflection. Values of types with their own UnmarshalYAML
// (expressions, conditional strings, reels) are not descended into — except
// Layer, whose keys are plain struct fields. Unparsable YAML yields no
// warnings (the parse error is reported elsewhere).
func unknownKeyWarnings(content []byte) []string {
	var doc yaml.Node
	if err := yaml.Unmarshal(content, &doc); err != nil || len(doc.Content) == 0 {
		return nil
	}
	var out []string
	checkKeys(doc.Content[0], reflect.TypeOf(renderer.Template{}), &out)
	return out
}

func checkKeys(n *yaml.Node, t reflect.Type, out *[]string) {
	if len(*out) >= maxUnknownKeyWarnings {
		return
	}
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if n.Kind == yaml.AliasNode {
		return
	}
	switch t.Kind() {
	case reflect.Slice:
		if n.Kind == yaml.SequenceNode {
			for _, c := range n.Content {
				checkKeys(c, t.Elem(), out)
			}
		}
		return
	case reflect.Struct:
	default:
		return
	}
	if t != layerType && reflect.PointerTo(t).Implements(unmarshalerType) {
		return
	}
	if n.Kind != yaml.MappingNode {
		return
	}
	fields := yamlFields(t)
	for i := 0; i+1 < len(n.Content); i += 2 {
		k := n.Content[i]
		ft, ok := fields[k.Value]
		if !ok {
			if len(*out) >= maxUnknownKeyWarnings {
				return
			}
			*out = append(*out, fmt.Sprintf("line %d: unknown key %q", k.Line, k.Value))
			continue
		}
		checkKeys(n.Content[i+1], ft, out)
	}
}

// yamlFields maps the YAML key of each exported field of struct t to its
// type, following yaml.v3's naming: the tag name, else the lowercased field name.
func yamlFields(t reflect.Type) map[string]reflect.Type {
	m := make(map[string]reflect.Type, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}
		name := strings.Split(f.Tag.Get("yaml"), ",")[0]
		if name == "-" {
			continue
		}
		if name == "" {
			name = strings.ToLower(f.Name)
		}
		m[name] = f.Type
	}
	return m
}
