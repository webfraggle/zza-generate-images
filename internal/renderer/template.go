package renderer

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// Template holds the parsed YAML template data.
type Template struct {
	Meta   Meta      `yaml:"meta"`
	Fonts  []FontDef `yaml:"fonts"`
	Layers []Layer   `yaml:"layers"`
	Dir    string    // Absolute path to the template directory (not from YAML)
}

// Meta holds the template metadata.
type Meta struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Author      string `yaml:"author"`
	Version     string `yaml:"version"`
	Canvas      Canvas `yaml:"canvas"`
}

// Canvas defines the output image dimensions.
type Canvas struct {
	Width  int `yaml:"width"`
	Height int `yaml:"height"`
}

// FontDef maps a font ID to a font file.
type FontDef struct {
	ID   string `yaml:"id"`
	File string `yaml:"file"`
}

// Layer represents a single rendering layer.
type Layer struct {
	Type     string       `yaml:"type"`
	If       string       `yaml:"if"`
	X        int          `yaml:"x"`
	Y        int          `yaml:"y"`
	Width    int          `yaml:"width"`
	Height   int          `yaml:"height"`
	File     string       `yaml:"file"`
	Color    StringOrCond `yaml:"color"`
	Value    StringOrCond `yaml:"value"`
	Font     string       `yaml:"font"`
	Size     float64      `yaml:"size"`
	Align    string       `yaml:"align"`   // left (default) | center | right
	Valign   string       `yaml:"valign"`  // top (default) | middle | bottom — needs height
	MaxWidth int          `yaml:"max_width"`
	// type: copy — source region to copy from
	SrcX      int `yaml:"src_x"`
	SrcY      int `yaml:"src_y"`
	SrcWidth  int `yaml:"src_width"`
	SrcHeight int `yaml:"src_height"`
}

// StringOrCond can be either a plain string value or a conditional map (if/then/else).
// In Phase 1, the else value is used as the default when an if/then is present.
type StringOrCond struct {
	raw  string
	cond *condMap
}

type condMap struct {
	ifExpr string
	then   string
	els    string
}

// String returns the resolved value.
// Phase 1: returns plain value, or else-branch if conditional, or empty string.
func (s StringOrCond) String() string {
	if s.cond != nil {
		return s.cond.els
	}
	return s.raw
}

// UnmarshalYAML implements yaml.Unmarshaler for StringOrCond.
func (s *StringOrCond) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.ScalarNode:
		s.raw = value.Value
		s.cond = nil
		return nil

	case yaml.MappingNode:
		// Parse if/then/else map
		cm := &condMap{}
		for i := 0; i+1 < len(value.Content); i += 2 {
			key := value.Content[i].Value
			val := value.Content[i+1].Value
			switch key {
			case "if":
				cm.ifExpr = val
			case "then":
				cm.then = val
			case "else":
				cm.els = val
			}
		}
		s.cond = cm
		s.raw = ""
		return nil

	default:
		return fmt.Errorf("renderer: StringOrCond: unexpected YAML node kind %v", value.Kind)
	}
}
