package editor

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// ErrNotRegular is returned by AtomicWrite when the target exists but is not
// a regular file (symlink, directory, device, ...). It matches ErrForbidden
// via errors.Is, so HTTP handlers answer 403.
var ErrNotRegular = fmt.Errorf("%w: target is not a regular file", ErrForbidden)

// AtomicWrite writes data to dir/name atomically. The content goes to a fresh
// temp file ".<name>.<random>.tmp" in dir, created with O_EXCL by
// os.CreateTemp — it never follows a symlink planted under a predictable
// temp name. The temp file is then renamed over the target. A target that
// exists but is not a regular file (e.g. a symlink) is refused. On any error
// the temp file is removed. The file mode is 0644.
//
// Temp names start with "." and end in ".tmp"; the file watcher and the
// template capacity check ignore such names.
func AtomicWrite(dir, name string, data []byte) (err error) {
	if err := ValidateWriteName(name); err != nil {
		return err
	}
	target := filepath.Join(dir, name)
	if err := checkRegularOrMissing(target); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, "."+name+".*.tmp")
	if err != nil {
		return fmt.Errorf("editor: creating temp file: %w", err)
	}
	tmp := f.Name()
	defer func() {
		if err != nil {
			_ = f.Close()
			_ = os.Remove(tmp)
		}
	}()
	if _, err = f.Write(data); err != nil {
		return fmt.Errorf("editor: writing temp file: %w", err)
	}
	if err = f.Chmod(0o644); err != nil {
		return fmt.Errorf("editor: chmod temp file: %w", err)
	}
	if err = f.Close(); err != nil {
		return fmt.Errorf("editor: closing temp file: %w", err)
	}
	// Re-check right before the rename: replacing a symlink would be harmless
	// (rename replaces the link itself), but a symlinked target signals a
	// tampered template folder — refuse rather than silently replace it.
	if err = checkRegularOrMissing(target); err != nil {
		return err
	}
	if err = os.Rename(tmp, target); err != nil {
		return fmt.Errorf("editor: renaming temp file: %w", err)
	}
	return nil
}

func checkRegularOrMissing(p string) error {
	fi, err := os.Lstat(p)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return nil
	case err != nil:
		return fmt.Errorf("editor: stat target: %w", err)
	case !fi.Mode().IsRegular():
		return ErrNotRegular
	}
	return nil
}
