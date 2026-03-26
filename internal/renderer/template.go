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
	// type: image — optional rotation
	Rotate StringOrCond `yaml:"rotate"`  // degrees; supports expressions: "{{now.minute | mul(6)}}"
	PivotX int          `yaml:"pivot_x"` // rotation pivot X relative to image; 0+0 defaults to center
	PivotY int          `yaml:"pivot_y"` // rotation pivot Y relative to image; 0+0 defaults to center
	// type: copy — source region to copy from
	SrcX      int `yaml:"src_x"`
	SrcY      int `yaml:"src_y"`
	SrcWidth  int `yaml:"src_width"`
	SrcHeight int `yaml:"src_height"`
}

// maxCondBranches limits the number of if/elif branches in a single StringOrCond to prevent DoS.
const maxCondBranches = 50

// StringOrCond can be either a plain string value or a conditional map (if/elif/then/else).
type StringOrCond struct {
	raw  string
	cond *condMap
}

// condBranch holds a single if/elif condition and its then-value.
type condBranch struct {
	ifExpr string
	then   string
}

// condMap holds an ordered list of if/elif branches and a final else fallback.
type condMap struct {
	branches []condBranch
	els      string
}

// String returns the plain value or the else-branch for conditionals.
// Use Resolve(eval) when an Evaluator is available to properly evaluate conditions.
func (s StringOrCond) String() string {
	return s.Resolve(nil)
}

// Resolve evaluates branches in order and returns the first matching then-value.
// Falls back to the else-branch when eval is nil or no branch matches.
func (s StringOrCond) Resolve(eval *Evaluator) string {
	if s.cond == nil {
		return s.raw
	}
	if eval != nil {
		for _, b := range s.cond.branches {
			if eval.EvalCondition(b.ifExpr) {
				return b.then
			}
		}
	}
	return s.cond.els
}

// UnmarshalYAML implements yaml.Unmarshaler for StringOrCond.
// Supports if/then/else (single branch) and if/then/elif/then/.../else (multi-branch).
// yaml.Node preserves duplicate keys (elif, then) in Content order.
func (s *StringOrCond) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.ScalarNode:
		s.raw = value.Value
		s.cond = nil
		return nil

	case yaml.MappingNode:
		cm := &condMap{}
		var pendingExpr string
		hasPending := false

		for i := 0; i+1 < len(value.Content); i += 2 {
			key := value.Content[i].Value
			val := value.Content[i+1].Value
			switch key {
			case "if", "elif":
				if hasPending {
					return fmt.Errorf("renderer: '%s' without preceding 'then'", key)
				}
				if len(cm.branches) >= maxCondBranches {
					return fmt.Errorf("renderer: too many if/elif branches (max %d)", maxCondBranches)
				}
				pendingExpr = val
				hasPending = true
			case "then":
				if !hasPending {
					return fmt.Errorf("renderer: 'then' without preceding 'if' or 'elif'")
				}
				cm.branches = append(cm.branches, condBranch{ifExpr: pendingExpr, then: val})
				hasPending = false
			case "else":
				cm.els = val
			}
		}
		if hasPending {
			return fmt.Errorf("renderer: 'if'/'elif' without following 'then'")
		}
		s.cond = cm
		s.raw = ""
		return nil

	default:
		return fmt.Errorf("renderer: StringOrCond: unexpected YAML node kind %v", value.Kind)
	}
}
