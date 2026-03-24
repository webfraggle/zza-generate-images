package renderer

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

var validTemplateName = regexp.MustCompile(`^[a-z0-9-]+$`)

// ValidateTemplateName checks that name contains only lowercase letters, digits and hyphens,
// and is at most 64 characters long.
func ValidateTemplateName(name string) error {
	if len(name) == 0 {
		return fmt.Errorf("renderer: template name must not be empty")
	}
	if len(name) > 64 {
		return fmt.Errorf("renderer: template name too long (max 64 characters)")
	}
	if !validTemplateName.MatchString(name) {
		return fmt.Errorf("renderer: template name %q contains invalid characters (only a-z, 0-9 and - are allowed)", name)
	}
	return nil
}

// SafeTemplatePath joins templatesDir and name, then verifies that the result
// is actually located under templatesDir to prevent path traversal attacks.
// It also validates the name via ValidateTemplateName, so callers do not need
// to call both functions separately.
func SafeTemplatePath(templatesDir, name string) (string, error) {
	if err := ValidateTemplateName(name); err != nil {
		return "", err
	}
	base, err := filepath.Abs(templatesDir)
	if err != nil {
		return "", fmt.Errorf("renderer: resolving templates dir: %w", err)
	}

	joined := filepath.Join(base, name)
	clean := filepath.Clean(joined)

	// Ensure the cleaned path starts with base + separator to prevent escaping.
	prefix := base + string(filepath.Separator)
	if clean != base && !strings.HasPrefix(clean, prefix) {
		return "", fmt.Errorf("renderer: path traversal detected for template name %q", name)
	}

	return clean, nil
}
