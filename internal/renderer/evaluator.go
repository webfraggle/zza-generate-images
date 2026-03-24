package renderer

import (
	"fmt"
	"regexp"
	"strings"
)

var interpolateRe = regexp.MustCompile(`\{\{([^}]+)\}\}`)

// Evaluator resolves template variables from a data map.
// Phase 1: variable interpolation only — no filters, no conditionals.
type Evaluator struct {
	data map[string]interface{}
}

// NewEvaluator creates a new Evaluator with the given data map.
func NewEvaluator(data map[string]interface{}) *Evaluator {
	return &Evaluator{data: data}
}

// Interpolate replaces all {{path.to.key}} placeholders in s with values from the data map.
// Missing keys resolve to an empty string. Numbers are converted to their string representation.
func (e *Evaluator) Interpolate(s string) string {
	return interpolateRe.ReplaceAllStringFunc(s, func(match string) string {
		// Strip {{ and }}
		inner := strings.TrimSpace(match[2 : len(match)-2])
		val := e.resolvePath(inner)
		return val
	})
}

// resolvePath resolves a dot-separated path like "zug1.zeit" against the data map.
func (e *Evaluator) resolvePath(path string) string {
	parts := strings.Split(path, ".")
	var current interface{} = map[string]interface{}(e.data)

	for _, part := range parts {
		part = strings.TrimSpace(part)
		switch m := current.(type) {
		case map[string]interface{}:
			val, ok := m[part]
			if !ok {
				return ""
			}
			current = val
		case map[interface{}]interface{}:
			val, ok := m[part]
			if !ok {
				return ""
			}
			current = val
		default:
			return ""
		}
	}

	if current == nil {
		return ""
	}
	return fmt.Sprintf("%v", current)
}
