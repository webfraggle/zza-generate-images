// Package editor provides filesystem operations for template editing:
// listing, reading, writing, uploading, and deleting files inside a
// template directory. It is the desktop build's data layer — the server
// build does not import it.
package editor

import (
	_ "embed"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// MaxUploadBytes is the maximum size for uploaded asset files (10 MiB).
const MaxUploadBytes = 10 << 20

//go:embed starter/template.yaml
var starterYAML []byte

//go:embed starter/default.json
var starterDefaultJSON []byte

var (
	ErrFileNotFound = errors.New("editor: file not found")
	ErrForbidden    = errors.New("editor: access denied")
	ErrInvalidName  = errors.New("editor: invalid filename")
)

// editableExts are the extensions that can be read and written as text.
var editableExts = map[string]bool{
	".yaml": true,
	".json": true,
}

// uploadExts are the extensions allowed for asset uploads.
var uploadExts = map[string]bool{
	".png":  true,
	".jpg":  true,
	".jpeg": true,
	".ttf":  true,
	".otf":  true,
}

// protectedFiles cannot be deleted.
var protectedFiles = map[string]bool{
	"template.yaml": true,
	"default.json":  true,
}

// safeNameRe restricts filenames to alphanumerics, dots, underscores and hyphens.
var safeNameRe = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

// FileInfo describes a single file in a template directory.
type FileInfo struct {
	Name      string `json:"name"`
	Size      int64  `json:"size"`
	Editable  bool   `json:"editable"`  // can be opened in the text editor
	Deletable bool   `json:"deletable"` // can be deleted via the UI
}

// templateDir returns the absolute path to the template directory and
// verifies it is inside templatesDir (path traversal guard).
func templateDir(templatesDir, templateName string) (string, error) {
	abs, err := filepath.Abs(filepath.Join(templatesDir, templateName))
	if err != nil {
		return "", fmt.Errorf("editor: resolving path: %w", err)
	}
	base, err := filepath.Abs(templatesDir)
	if err != nil {
		return "", fmt.Errorf("editor: resolving base: %w", err)
	}
	sep := string(filepath.Separator)
	if !strings.HasPrefix(abs+sep, base+sep) {
		return "", ErrForbidden
	}
	return abs, nil
}

// safeFilePath returns the absolute path to filename inside the template
// directory with path traversal protection.
func safeFilePath(templatesDir, templateName, filename string) (string, error) {
	if !safeNameRe.MatchString(filename) {
		return "", ErrInvalidName
	}
	dir, err := templateDir(templatesDir, templateName)
	if err != nil {
		return "", err
	}
	p := filepath.Join(dir, filename)
	sep := string(filepath.Separator)
	if !strings.HasPrefix(p+sep, dir+sep) {
		return "", ErrForbidden
	}
	return p, nil
}

// SanitizeFilename strips disallowed characters from a filename.
// Only [a-zA-Z0-9._-] are kept; everything else becomes "_".
// Any directory component is removed first.
func SanitizeFilename(name string) string {
	name = filepath.Base(name)
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9',
			r == '.', r == '_', r == '-':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	return b.String()
}

// InitTemplate creates the template directory and seeds it with a starter
// template.yaml if it does not already exist. It is a no-op if the directory
// is already present.
func InitTemplate(templatesDir, templateName string) error {
	dir, err := templateDir(templatesDir, templateName)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("editor: creating template dir: %w", err)
	}
	yamlPath := filepath.Join(dir, "template.yaml")
	if _, err := os.Stat(yamlPath); os.IsNotExist(err) {
		if err := os.WriteFile(yamlPath, starterYAML, 0o644); err != nil {
			return fmt.Errorf("editor: writing starter template.yaml: %w", err)
		}
	}
	jsonPath := filepath.Join(dir, "default.json")
	if _, err := os.Stat(jsonPath); os.IsNotExist(err) {
		if err := os.WriteFile(jsonPath, starterDefaultJSON, 0o644); err != nil {
			return fmt.Errorf("editor: writing starter default.json: %w", err)
		}
	}
	return nil
}

// ListFiles returns all files in the template directory.
func ListFiles(templatesDir, templateName string) ([]FileInfo, error) {
	dir, err := templateDir(templatesDir, templateName)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("editor: listing files: %w", err)
	}
	var files []FileInfo
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, statErr := e.Info()
		if statErr != nil {
			continue
		}
		ext := strings.ToLower(filepath.Ext(e.Name()))
		files = append(files, FileInfo{
			Name:      e.Name(),
			Size:      info.Size(),
			Editable:  editableExts[ext],
			Deletable: !protectedFiles[e.Name()],
		})
	}
	return files, nil
}

// ReadTextFile reads a text file (.yaml or .json) from the template directory.
func ReadTextFile(templatesDir, templateName, filename string) ([]byte, error) {
	ext := strings.ToLower(filepath.Ext(filename))
	if !editableExts[ext] {
		return nil, ErrForbidden
	}
	p, err := safeFilePath(templatesDir, templateName, filename)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p)
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrFileNotFound
	}
	return data, err
}

// WriteTextFile atomically writes a text file (.yaml or .json) in the template directory.
func WriteTextFile(templatesDir, templateName, filename string, data []byte) error {
	ext := strings.ToLower(filepath.Ext(filename))
	if !editableExts[ext] {
		return ErrForbidden
	}
	p, err := safeFilePath(templatesDir, templateName, filename)
	if err != nil {
		return err
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("editor: writing file: %w", err)
	}
	if err := os.Rename(tmp, p); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("editor: renaming file: %w", err)
	}
	return nil
}

// UploadFile writes an asset file (image or font) to the template directory.
// The filename is sanitized and must have an allowed upload extension.
func UploadFile(templatesDir, templateName, filename string, r io.Reader) error {
	filename = SanitizeFilename(filename)
	ext := strings.ToLower(filepath.Ext(filename))
	if !uploadExts[ext] {
		return ErrForbidden
	}
	p, err := safeFilePath(templatesDir, templateName, filename)
	if err != nil {
		return err
	}
	data, err := io.ReadAll(io.LimitReader(r, MaxUploadBytes+1))
	if err != nil {
		return fmt.Errorf("editor: reading upload: %w", err)
	}
	if int64(len(data)) > MaxUploadBytes {
		return fmt.Errorf("editor: upload exceeds %d bytes", MaxUploadBytes)
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("editor: writing upload: %w", err)
	}
	if err := os.Rename(tmp, p); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("editor: renaming upload: %w", err)
	}
	return nil
}

// DeleteFile removes a file from the template directory.
// template.yaml is protected and cannot be deleted.
func DeleteFile(templatesDir, templateName, filename string) error {
	if protectedFiles[filename] {
		return ErrForbidden
	}
	p, err := safeFilePath(templatesDir, templateName, filename)
	if err != nil {
		return err
	}
	if err := os.Remove(p); errors.Is(err, os.ErrNotExist) {
		return ErrFileNotFound
	} else if err != nil {
		return fmt.Errorf("editor: deleting file: %w", err)
	}
	return nil
}
