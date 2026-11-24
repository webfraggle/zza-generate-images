package editor

import (
	"os"
	"path/filepath"

	"github.com/webfraggle/zza-generate-images/internal/devexport"
	"github.com/webfraggle/zza-generate-images/internal/renderer"
)

// MaxTemplateNameLen is the longest name a template may be renamed to: longer
// names cannot be installed on the controller (see devexport.MaxNameLen).
const MaxTemplateNameLen = devexport.MaxNameLen

// RenameTemplate renames the folder of a template; the folder name is the
// template's name (URL, ZZA package, folder on the controller), meta.name in
// template.yaml is only its title.
//
// Errors: ErrInvalidName (characters), ErrNameTooLong, ErrTemplateNotFound
// (no such folder, or a symlink), ErrTemplateExists (also when both names
// are the same).
func RenameTemplate(templatesDir, oldName, newName string) error {
	if renderer.ValidateTemplateName(oldName) != nil {
		return ErrTemplateNotFound
	}
	if renderer.ValidateTemplateName(newName) != nil {
		return ErrInvalidName
	}
	if len(newName) > MaxTemplateNameLen {
		return ErrNameTooLong
	}
	from, to := filepath.Join(templatesDir, oldName), filepath.Join(templatesDir, newName)
	// Lstat: a symlinked folder may point outside the templates folder.
	if info, err := os.Lstat(from); err != nil || !info.IsDir() {
		return ErrTemplateNotFound
	}
	if _, err := os.Lstat(to); err == nil {
		return ErrTemplateExists
	}
	return os.Rename(from, to)
}
