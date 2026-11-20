//go:build unix

package mcp

import (
	"os"
	"path/filepath"
	"testing"
)

// A symlink "<name>.tmp" planted in a (foreign) template folder must not make
// write_file write outside the templates folder.
func TestWriteFileTmpSymlinkNotFollowed(t *testing.T) {
	tdir := copyTemplates(t, "sbb-105-v1")
	outside := filepath.Join(t.TempDir(), "victim-rc")
	if err := os.WriteFile(outside, []byte("original\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(tdir, "sbb-105-v1", "notes.yaml.tmp")); err != nil {
		t.Fatal(err)
	}
	cs := newSession(t, tdir)
	res := callTool(t, cs, "write_file", map[string]any{"template": "sbb-105-v1", "name": "notes.yaml", "content": "a: 1\n"})
	requireOK(t, res)
	if got, _ := os.ReadFile(outside); string(got) != "original\n" {
		t.Fatalf("write_file wrote outside the templates folder: %q", got)
	}
}

// Same for asset writes (svg_to_png -> writeAssetBytes).
func TestAssetWriteTmpSymlinkNotFollowed(t *testing.T) {
	tdir := copyTemplates(t, "sbb-105-v1")
	outside := filepath.Join(t.TempDir(), "victim.bin")
	if err := os.WriteFile(outside, []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(tdir, "sbb-105-v1", "x.png.tmp")); err != nil {
		t.Fatal(err)
	}
	cs := newSession(t, tdir)
	res := callTool(t, cs, "svg_to_png", map[string]any{"template": "sbb-105-v1", "name": "x.png", "width": 8, "height": 8,
		"svg": `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 8 8"><rect width="8" height="8" fill="red"/></svg>`})
	requireOK(t, res)
	if got, _ := os.ReadFile(outside); string(got) != "original" {
		t.Fatalf("asset write went outside the templates folder (%d bytes)", len(got))
	}
	fi, err := os.Lstat(filepath.Join(tdir, "sbb-105-v1", "x.png"))
	if err != nil || !fi.Mode().IsRegular() {
		t.Fatalf("x.png not a regular file: %v %v", fi, err)
	}
}
