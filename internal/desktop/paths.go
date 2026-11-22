// Package desktop provides desktop-build entrypoints: templates directory
// resolution, Wails bootstrap, and browser-fallback when no webview is available.
package desktop

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/webfraggle/zza-generate-images/internal/editor"
)

// ResolveTemplatesDir returns the absolute path to the templates folder.
// Priority: override (--templates-dir flag) → sibling-of-app-bundle (macOS .app)
// → sibling-of-executable (Windows and bare macOS/Linux binaries).
func ResolveTemplatesDir(override, exePath string) (string, error) {
	if override != "" {
		abs, err := filepath.Abs(override)
		if err != nil {
			return "", fmt.Errorf("desktop: resolving override: %w", err)
		}
		return abs, nil
	}

	exeAbs, err := filepath.Abs(exePath)
	if err != nil {
		return "", fmt.Errorf("desktop: resolving exe path: %w", err)
	}
	exeDir := filepath.Dir(exeAbs)

	// macOS .app bundle: binary lives at <Bundle>.app/Contents/MacOS/<exe>
	// Walk up from the executable dir; if any ancestor ends in ".app", place
	// the templates dir as that ancestor's sibling.
	if strings.Contains(filepath.ToSlash(exeDir), ".app/Contents/MacOS") {
		// Walk up via filepath.Dir until we stop making progress (Dir of a
		// root path returns itself on both Unix and Windows).
		for cur := exeDir; ; cur = filepath.Dir(cur) {
			if strings.HasSuffix(cur, ".app") {
				return filepath.Join(filepath.Dir(cur), "templates"), nil
			}
			if parent := filepath.Dir(cur); parent == cur {
				break
			}
		}
	}

	return filepath.Join(exeDir, "templates"), nil
}

// EnsureTemplatesDir creates dir if it doesn't exist and seeds a minimal
// starter template (default/) when the dir contains no template sub-directories.
func EnsureTemplatesDir(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("desktop: creating templates dir: %w", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("desktop: reading templates dir: %w", err)
	}
	for _, e := range entries {
		if e.IsDir() {
			return nil
		}
	}
	// Empty (or file-only) templates dir → seed a starter template.
	return editor.InitTemplate(dir, "default")
}
