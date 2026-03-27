package renderer

import (
	"fmt"
	"strconv"
	"strings"

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
	Elif     string       `yaml:"elif"` // continues an if-chain at layer level
	Else     bool         `yaml:"else"` // else: true — renders when preceding if/elif chain was not satisfied
	X        IntOrExpr    `yaml:"x"`
	Y        IntOrExpr    `yaml:"y"`
	Width    IntOrExpr    `yaml:"width"`
	Height   IntOrExpr    `yaml:"height"`
	File     string       `yaml:"file"`
	Color    StringOrCond `yaml:"color"`
	Value    StringOrCond `yaml:"value"`
	Font     string       `yaml:"font"`
	Size     float64      `yaml:"size"`
	Align    string       `yaml:"align"`    // left (default) | center | right
	Valign   string       `yaml:"valign"`   // top (default) | middle | bottom — needs height
	MaxWidth IntOrExpr    `yaml:"max_width"`
	// type: image — optional rotation
	// When rotate is set, x and y are the CENTER coordinates of the image on the canvas.
	Rotate StringOrCond `yaml:"rotate"` // degrees clockwise; supports expressions: "{{now.minute | mul(6)}}"
	// type: copy — source region to copy from
	SrcX      int `yaml:"src_x"`
	SrcY      int `yaml:"src_y"`
	SrcWidth  int `yaml:"src_width"`
	SrcHeight int `yaml:"src_height"`
	// type: loop
	SplitBy  string  `yaml:"split_by"`  // delimiter for splitting Value
	Var      string  `yaml:"var"`       // name of the loop item variable
	MaxItems int     `yaml:"max_items"` // safety cap; 0 = default (20)
	Layers   []Layer `yaml:"layers"`    // sub-layers rendered per iteration
}

// IntOrExpr holds either a plain integer or an arithmetic expression in {{...}} syntax.
// Supported in x, y, width, height, max_width fields.
type IntOrExpr struct {
	val    int
	expr   string // content between {{ }} when isExpr is true
	isExpr bool
}

// Resolve returns the integer value, evaluating the expression if needed.
// Returns 0 when eval is nil and the field is an expression.
func (ie IntOrExpr) Resolve(eval *Evaluator) int {
	if !ie.isExpr {
		return ie.val
	}
	if eval == nil {
		return 0
	}
	v, err := evalIntExpr(ie.expr, eval.intVars)
	if err != nil {
		return 0 // expression errors are silent; template authors see wrong layout, not a crash
	}
	return v
}

// UnmarshalYAML implements yaml.Unmarshaler for IntOrExpr.
// Accepts either a plain integer (e.g. x: 45) or an expression string (e.g. x: "{{i * 20 + 10}}").
func (ie *IntOrExpr) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind != yaml.ScalarNode {
		return fmt.Errorf("renderer: IntOrExpr: expected scalar, got kind %v", value.Kind)
	}
	// Try plain integer first.
	if n, err := strconv.Atoi(value.Value); err == nil {
		ie.val = n
		ie.isExpr = false
		return nil
	}
	// Expression: must be "{{...}}" form.
	s := strings.TrimSpace(value.Value)
	if strings.HasPrefix(s, "{{") && strings.HasSuffix(s, "}}") {
		ie.expr = strings.TrimSpace(s[2 : len(s)-2])
		ie.isExpr = true
		return nil
	}
	return fmt.Errorf("renderer: IntOrExpr: expected integer or {{expr}}, got %q", value.Value)
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
