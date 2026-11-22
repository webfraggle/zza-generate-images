//go:build unix

package mcp

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestReadRegularFile(t *testing.T) {
	dir := t.TempDir()
	ok := filepath.Join(dir, "a.txt")
	mustWrite(t, ok, "hello")
	if b, err := readRegularFile(ok, 10); err != nil || string(b) != "hello" {
		t.Fatalf("ok: %q %v", b, err)
	}
	if _, err := readRegularFile(ok, 4); !errors.Is(err, errFileTooLarge) {
		t.Errorf("too large: %v", err)
	}
	if _, err := readRegularFile(dir, 10); !errors.Is(err, errNotRegular) {
		t.Errorf("dir: %v", err)
	}
	if _, err := readRegularFile(filepath.Join(dir, "none"), 10); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("missing: %v", err)
	}
	link := filepath.Join(dir, "ln")
	if err := os.Symlink(ok, link); err != nil {
		t.Fatal(err)
	}
	if _, err := readRegularFile(link, 10); !errors.Is(err, errNotRegular) {
		t.Errorf("symlink: %v", err)
	}
	fifo := filepath.Join(dir, "ff")
	if err := syscall.Mkfifo(fifo, 0o644); err != nil {
		t.Skip("no fifo")
	}
	if _, err := readRegularFile(fifo, 10); !errors.Is(err, errNotRegular) {
		t.Errorf("fifo: %v", err)
	}
}
