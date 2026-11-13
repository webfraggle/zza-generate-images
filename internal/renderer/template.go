package renderer

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Template holds the parsed YAML template data.
type Template struct {
	Meta   Meta           `yaml:"meta"`
	Fonts  []FontDef      `yaml:"fonts"`
	Layers []Layer        `yaml:"layers"`
	Dir    string         // Absolute path to the template directory (not from YAML)
	loc    *time.Location // resolved meta.timezone, set by LoadTemplate
}

// Meta holds the template metadata.
type Meta struct {
	Name         string        `yaml:"name"`
	Description  string        `yaml:"description"`
	Author       string        `yaml:"author"`
	Version      string        `yaml:"version"`
	Display      string        `yaml:"display"`
	Instructions string        `yaml:"instructions"`
	Canvas       Canvas        `yaml:"canvas"`
	Animation    AnimationMeta `yaml:"animation"`
	Timezone     string        `yaml:"timezone"` // IANA name; "" = Europe/Berlin
}

// AnimationMeta holds the animation settings of a template (GIF output).
// Zero values mean the defaults; see resolve and validateAnimation.
type AnimationMeta struct {
	FPS        int     `yaml:"fps"`         // splitflap: frames per second; 0 = 20, allowed 5–25
	Duration   float64 `yaml:"duration"`    // splitflap: max running time in s; 0 = 2.5, allowed 0.5–5
	Frames     int     `yaml:"frames"`      // frame mode: number of frames; 0 = off, allowed 2–125
	FrameDelay float64 `yaml:"frame_delay"` // frame mode: seconds per frame; 0 = 1, allowed 0.02–10 in 0.01 steps
	Loop       bool    `yaml:"loop"`        // GIF repeats forever (both modes)
}

// Canvas defines the output image dimensions.
type Canvas struct {
	Width  int `yaml:"width"`
	Height int `yaml:"height"`
	Colors int `yaml:"colors"` // 0 = no reduction (default); 2–256 = indexed PNG output
}

// FontDef maps a font ID to a font file.
type FontDef struct {
	ID   string `yaml:"id"`
	File string `yaml:"file"`
}

// Layer represents a single rendering layer.
type Layer struct {
	Type       string       `yaml:"type"`
	If         string       `yaml:"if"`
	Elif       string       `yaml:"elif"` // continues an if-chain at layer level
	Else       ElseMarker   `yaml:"else"` // else: true — renders when preceding if/elif chain was not satisfied
	X          IntOrExpr    `yaml:"x"`
	Y          IntOrExpr    `yaml:"y"`
	Width      IntOrExpr    `yaml:"width"`
	Height     IntOrExpr    `yaml:"height"`
	File       string       `yaml:"file"`
	Color      StringOrCond `yaml:"color"`
	Value      StringOrCond `yaml:"value"`
	Font       string       `yaml:"font"`
	Size       float64      `yaml:"size"`
	Align      string       `yaml:"align"`  // left (default) | center | right
	Valign     string       `yaml:"valign"` // top (default) | middle | bottom — needs height; baseline — first-line baseline = y
	MaxWidth   IntOrExpr    `yaml:"max_width"`
	LineHeight float64      `yaml:"line_height"` // multiplier relative to font height; 0/1 = default
	// type: text — optional horizontal scaling (applied only when > 0 and != 1)
	ScaleX float64 `yaml:"scale_x"`
	// type: text — optional vertical scaling, anchored at y (top edge, or baseline with valign: baseline)
	ScaleY float64 `yaml:"scale_y"`
	// type: text — id publishes <id>.width and <id>.end to following layers' coordinate
	// expressions; advance (px) is added to <id>.end when the text is not empty.
	ID      string `yaml:"id"`
	Advance int    `yaml:"advance"`
	// type: text — optional single-line item fitting (legacy "via" behaviour)
	FitItems *FitItems `yaml:"fit_items"`
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
	Var      string  `yaml:"var"`       // name of the loop item variable (also used by splitflap)
	MaxItems int     `yaml:"max_items"` // safety cap; 0 = default (20)
	Layers   []Layer `yaml:"layers"`    // sub-layers rendered per iteration (splitflap: per module)
	// type: splitflap — X/Y/Width/Height are the first module, Value the target, Var the child variable
	Cells      int    `yaml:"cells"`       // modules side by side; 0 = 1
	Pitch      int    `yaml:"pitch"`       // x distance module to module; 0 = width
	Split      string `yaml:"split"`       // "" = whole value in one module | "chars" = one rune per module
	Reel       Reel   `yaml:"reel"`        // values a module flips through (animation)
	SplitY     int    `yaml:"split_y"`     // hinge line relative to the module; 0 = height/2
	FlipFrames int    `yaml:"flip_frames"` // animation: frames per flip
	MinFlips   int    `yaml:"min_flips"`   // animation: minimum flips per module
	MaxFlips   int    `yaml:"max_flips"`   // animation: maximum flips per module
}

// Reel is the ordered supply of values of a splitflap module. In YAML it is
// either a string (one entry per rune) or a list of strings. Duplicates are kept.
type Reel []string

// UnmarshalYAML implements yaml.Unmarshaler for Reel.
func (r *Reel) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.ScalarNode:
		var s string
		if err := value.Decode(&s); err != nil {
			return fmt.Errorf("renderer: Reel: %w", err)
		}
		entries := make(Reel, 0, len(s))
		for _, ch := range s {
			entries = append(entries, string(ch))
		}
		*r = entries
		return nil
	case yaml.SequenceNode:
		var entries []string
		if err := value.Decode(&entries); err != nil {
			return fmt.Errorf("renderer: Reel: %w", err)
		}
		*r = entries
		return nil
	default:
		return fmt.Errorf("renderer: Reel: expected string or list, got kind %v", value.Kind)
	}
}

// HasSplitflap reports whether the template contains a splitflap layer,
// at top level or inside block nodes.
func (t *Template) HasSplitflap() bool {
	return hasSplitflap(t.Layers)
}

// IsAnimated reports whether the template is animated: frame mode or splitflap.
func (t *Template) IsAnimated() bool {
	return t.Meta.Animation.Frames > 0 || t.HasSplitflap()
}

// animFrames returns the number of frames of frame mode, else 1.
func (t *Template) animFrames() int {
	if t.Meta.Animation.Frames > 0 {
		return t.Meta.Animation.Frames
	}
	return 1
}

// animDelayCS returns the frame delay in centiseconds for frame-mode
// templates, 0 otherwise.
func (t *Template) animDelayCS() int {
	if t.Meta.Animation.Frames > 0 {
		return t.Meta.Animation.frameDelayCS()
	}
	return 0
}

func hasSplitflap(layers []Layer) bool {
	for i := range layers {
		switch {
		case layers[i].Type == "splitflap":
			return true
		case layers[i].Type == "" && hasSplitflap(layers[i].Layers):
			return true
		}
	}
	return false
}

// FitItems makes a text layer split its value at SplitBy and place the items
// left-aligned on one line, Gap pixels apart, omitting items that no longer
// fit into max_width.
type FitItems struct {
	SplitBy string `yaml:"split_by"`
	Gap     int    `yaml:"gap"`
}

// UnmarshalYAML implements yaml.Unmarshaler for Layer.
// It handles the `else:` key specially: yaml.v3 does not call custom unmarshalers
// for null nodes, so we scan the raw mapping node to detect a bare `else:` (null)
// and treat it as true.
// Allowed range of a text layer's scale_x (0 = unset).
const (
	minScaleX = 0.05
	maxScaleX = 8.0
)

func (l *Layer) UnmarshalYAML(value *yaml.Node) error {
	// layerAlias breaks the recursion that would occur if Layer implemented
	// UnmarshalYAML and we decoded into a Layer directly.
	type layerAlias Layer
	var alias layerAlias
	if err := value.Decode(&alias); err != nil {
		return err
	}
	*l = Layer(alias)

	// scale_x sizes an offscreen buffer; reject NaN, Inf and absurd factors.
	if s := l.ScaleX; s != 0 && !(s >= minScaleX && s <= maxScaleX) {
		return fmt.Errorf("line %d: scale_x must be between %g and %g (got %v)", value.Line, minScaleX, maxScaleX, s)
	}

	if s := l.ScaleY; s != 0 && !(s >= minScaleX && s <= maxScaleX) {
		return fmt.Errorf("line %d: scale_y must be between %g and %g (got %v)", value.Line, minScaleX, maxScaleX, s)
	}
	if err := l.validateID(value.Line); err != nil {
		return err
	}

	// Scan the raw mapping for the "else" key and set ElseMarker correctly
	// when the value is a null node (bare `else:` with no value).
	if value.Kind == yaml.MappingNode {
		for i := 0; i+1 < len(value.Content); i += 2 {
			if value.Content[i].Value == "else" {
				v := value.Content[i+1]
				if v.Tag == "!!null" {
					l.Else = true
				}
				break
			}
		}
	}
	return nil
}

// maxAdvance is the largest allowed text advance in pixels.
const maxAdvance = 240

var textIDRe = regexp.MustCompile(`^[a-z][a-z0-9_]{0,31}$`)

// validateID checks the id/advance fields of a text layer.
func (l *Layer) validateID(line int) error {
	if l.Advance < 0 || l.Advance > maxAdvance {
		return fmt.Errorf("line %d: advance must be between 0 and %d (got %d)", line, maxAdvance, l.Advance)
	}
	if l.ID == "" {
		if l.Advance != 0 {
			return fmt.Errorf("line %d: advance needs an id", line)
		}
		return nil
	}
	if l.Type != "text" {
		return fmt.Errorf("line %d: id is only allowed on type: text layers", line)
	}
	if !textIDRe.MatchString(l.ID) {
		return fmt.Errorf("line %d: id %q must match [a-z][a-z0-9_]{0,31}", line, l.ID)
	}
	if l.Align != "" && l.Align != "left" {
		return fmt.Errorf("line %d: id requires align: left", line)
	}
	if l.FitItems != nil {
		return fmt.Errorf("line %d: id cannot be combined with fit_items", line)
	}
	return nil
}

// checkDuplicateIDs reports a text id used more than once anywhere in the layer tree.
func checkDuplicateIDs(layers []Layer, seen map[string]bool) error {
	for i := range layers {
		if id := layers[i].ID; id != "" {
			if seen[id] {
				return fmt.Errorf("duplicate text id %q", id)
			}
			seen[id] = true
		}
		if err := checkDuplicateIDs(layers[i].Layers, seen); err != nil {
			return err
		}
	}
	return nil
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

// ElseMarker is a bool field that also accepts YAML null as true.
// This allows template authors to write either `else:` or `else: true`.
type ElseMarker bool

// UnmarshalYAML implements yaml.Unmarshaler.
// Null (from bare `else:` with no value) is treated as true.
func (e *ElseMarker) UnmarshalYAML(value *yaml.Node) error {
	if value.Tag == "!!null" {
		*e = true
		return nil
	}
	var b bool
	if err := value.Decode(&b); err != nil {
		return fmt.Errorf("renderer: ElseMarker: expected bool or null, got %q", value.Value)
	}
	*e = ElseMarker(b)
	return nil
}

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
