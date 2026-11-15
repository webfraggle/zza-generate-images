//go:build unix

package editor

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// plantTmpSymlink places "<name>.tmp" -> outside file in the template folder,
// as a foreign template (ZIP/Git) could bring it along.
func plantTmpSymlink(t *testing.T, dir, tmpl, name string) string {
	t.Helper()
	outside := filepath.Join(t.TempDir(), "victim")
	if err := os.WriteFile(outside, []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, tmpl, name+".tmp")); err != nil {
		t.Fatal(err)
	}
	return outside
}

func TestWriteTextFile_TmpSymlinkNotFollowed(t *testing.T) {
	dir, tmpl := makeTestTemplate(t)
	outside := plantTmpSymlink(t, dir, tmpl, "notes.yaml")
	if err := WriteTextFile(dir, tmpl, "notes.yaml", []byte("evil: 1\n")); err != nil {
		t.Fatalf("WriteTextFile: %v", err)
	}
	if got, _ := os.ReadFile(outside); string(got) != "original" {
		t.Fatalf("write escaped the template folder: outside file now %q", got)
	}
	fi, err := os.Lstat(filepath.Join(dir, tmpl, "notes.yaml"))
	if err != nil || !fi.Mode().IsRegular() {
		t.Fatalf("target is not a regular file: %v %v", fi, err)
	}
}

func TestWriteTextFileIfMatch_TmpSymlinkNotFollowed(t *testing.T) {
	dir, tmpl := makeTestTemplate(t)
	outside := plantTmpSymlink(t, dir, tmpl, "template.yaml")
	if _, err := WriteTextFileIfMatch(dir, tmpl, "template.yaml", []byte("x: 1\n"), ""); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(outside); string(got) != "original" {
		t.Fatalf("write escaped: %q", got)
	}
}

func TestUploadFile_TmpSymlinkNotFollowed(t *testing.T) {
	dir, tmpl := makeTestTemplate(t)
	outside := plantTmpSymlink(t, dir, tmpl, "bg.png")
	if err := UploadFile(dir, tmpl, "bg.png", bytes.NewReader([]byte("\x89PNGdata"))); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(outside); string(got) != "original" {
		t.Fatalf("upload escaped: %q", got)
	}
}

func TestAtomicWrite_TargetSymlinkRejected(t *testing.T) {
	dir := t.TempDir()
	outside := filepath.Join(t.TempDir(), "victim")
	if err := os.WriteFile(outside, []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, "a.json")); err != nil {
		t.Fatal(err)
	}
	if err := AtomicWrite(dir, "a.json", []byte("{}")); err == nil {
		t.Fatal("expected error for symlinked target")
	}
	if got, _ := os.ReadFile(outside); string(got) != "original" {
		t.Fatalf("outside changed: %q", got)
	}
	fi, err := os.Lstat(filepath.Join(dir, "a.json"))
	if err != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("symlink should be left alone: %v %v", fi, err)
	}
	assertNoTempLeft(t, dir)
}

func TestAtomicWrite_OK(t *testing.T) {
	dir := t.TempDir()
	if err := AtomicWrite(dir, "a.json", []byte("{}")); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(filepath.Join(dir, "a.json"))
	if err != nil || fi.Mode().Perm() != 0o644 {
		t.Fatalf("mode: %v %v", fi, err)
	}
	if err := AtomicWrite(dir, "a.json", []byte(`{"a":1}`)); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "a.json")); string(got) != `{"a":1}` {
		t.Fatalf("content %q", got)
	}
	assertNoTempLeft(t, dir)
}

func TestAtomicWrite_RejectsBadName(t *testing.T) {
	dir := t.TempDir()
	for _, n := range []string{"", ".", "..", "a/b", "../x"} {
		if err := AtomicWrite(dir, n, []byte("x")); err == nil {
			t.Errorf("AtomicWrite(%q) succeeded", n)
		}
	}
}

// InitTemplate on an existing (foreign) template whose template.yaml is a
// dangling symlink must not create the symlink's target.
func TestInitTemplate_DanglingSymlinkNotFollowed(t *testing.T) {
	dir := t.TempDir()
	tmpl := filepath.Join(dir, "x-105-v1")
	if err := os.MkdirAll(tmpl, 0o755); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "created-outside")
	if err := os.Symlink(outside, filepath.Join(tmpl, "template.yaml")); err != nil {
		t.Fatal(err)
	}
	_ = InitTemplate(dir, "x-105-v1")
	if _, err := os.Lstat(outside); err == nil {
		t.Fatal("InitTemplate followed a dangling symlink and wrote outside the template")
	}
}

// A template folder that is a symlink (templates/x-105-v1 -> elsewhere) must
// not be used: editor operations would read and write in the link target.
func TestTemplateDir_SymlinkRejected(t *testing.T) {
	dir := t.TempDir()
	target := t.TempDir()
	if err := os.WriteFile(filepath.Join(target, "template.yaml"), []byte("x: 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(dir, "x-105-v1")); err != nil {
		t.Fatal(err)
	}
	if err := InitTemplate(dir, "x-105-v1"); !errors.Is(err, ErrForbidden) {
		t.Errorf("InitTemplate: %v", err)
	}
	if err := WriteTextFile(dir, "x-105-v1", "a.json", []byte("{}")); !errors.Is(err, ErrForbidden) {
		t.Errorf("WriteTextFile: %v", err)
	}
	if err := UploadFile(dir, "x-105-v1", "a.png", bytes.NewReader([]byte("x"))); !errors.Is(err, ErrForbidden) {
		t.Errorf("UploadFile: %v", err)
	}
	if _, err := ReadTextFile(dir, "x-105-v1", "template.yaml"); !errors.Is(err, ErrForbidden) {
		t.Errorf("ReadTextFile: %v", err)
	}
	if _, err := ListFiles(dir, "x-105-v1"); !errors.Is(err, ErrForbidden) {
		t.Errorf("ListFiles: %v", err)
	}
	if err := DeleteFile(dir, "x-105-v1", "template.yaml"); err == nil {
		t.Errorf("DeleteFile accepted")
	}
	for _, n := range []string{"a.json", "a.png", "default.json"} {
		if _, err := os.Lstat(filepath.Join(target, n)); err == nil {
			t.Errorf("%s was written into the symlink target", n)
		}
	}
	// A new (not yet existing) template folder is still fine.
	if err := InitTemplate(dir, "new-105-v1"); err != nil {
		t.Errorf("InitTemplate new: %v", err)
	}
}

func assertNoTempLeft(t *testing.T, dir string) {
	t.Helper()
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".tmp") {
			t.Errorf("temp file left behind: %s", e.Name())
		}
	}
}
