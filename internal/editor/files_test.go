package editor

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// makeTestTemplate creates a temporary template directory with the given files.
func makeTestTemplate(t *testing.T) (templatesDir, templateName string) {
	t.Helper()
	dir := t.TempDir()
	templateName = "test-tmpl"
	tmplDir := filepath.Join(dir, templateName)
	if err := os.MkdirAll(tmplDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Seed files.
	files := map[string]string{
		"template.yaml": "meta:\n  name: Test\n",
		"default.json":  `{"key":"value"}`,
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(tmplDir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir, templateName
}

func TestSanitizeFilename(t *testing.T) {
	cases := []struct{ in, want string }{
		{"bg.png", "bg.png"},
		{"my file.png", "my_file.png"},
		{"../../../etc/passwd", "passwd"},
		{"font-bold.otf", "font-bold.otf"},
		{"Ä Ö Ü.png", "_____.png"},
	}
	for _, c := range cases {
		got := SanitizeFilename(c.in)
		if got != c.want {
			t.Errorf("SanitizeFilename(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestListFiles(t *testing.T) {
	dir, name := makeTestTemplate(t)
	files, err := ListFiles(dir, name)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(files))
	}
	found := map[string]bool{}
	for _, f := range files {
		found[f.Name] = true
	}
	if !found["template.yaml"] || !found["default.json"] {
		t.Error("expected template.yaml and default.json in listing")
	}
}

func TestListFiles_PathTraversal(t *testing.T) {
	dir, _ := makeTestTemplate(t)
	_, err := ListFiles(dir, "../other")
	if err == nil {
		t.Error("expected error for path traversal, got nil")
	}
}

func TestReadTextFile_OK(t *testing.T) {
	dir, name := makeTestTemplate(t)
	data, err := ReadTextFile(dir, name, "template.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(string(data), "name: Test") {
		t.Errorf("unexpected content: %s", data)
	}
}

func TestReadTextFile_Forbidden(t *testing.T) {
	dir, name := makeTestTemplate(t)
	// Write a PNG so it exists.
	_ = os.WriteFile(filepath.Join(dir, name, "bg.png"), []byte("PNG"), 0o644)
	_, err := ReadTextFile(dir, name, "bg.png")
	if !isErrForbidden(err) {
		t.Errorf("expected ErrForbidden for binary file, got %v", err)
	}
}

func TestReadTextFile_NotFound(t *testing.T) {
	dir, name := makeTestTemplate(t)
	_, err := ReadTextFile(dir, name, "missing.json")
	if !isErrNotFound(err) {
		t.Errorf("expected ErrFileNotFound, got %v", err)
	}
}

func TestWriteTextFile_OK(t *testing.T) {
	dir, name := makeTestTemplate(t)
	content := []byte("meta:\n  name: Updated\n")
	if err := WriteTextFile(dir, name, "template.yaml", content); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, _ := os.ReadFile(filepath.Join(dir, name, "template.yaml"))
	if string(got) != string(content) {
		t.Errorf("file content mismatch: %s", got)
	}
}

func TestWriteTextFile_Forbidden(t *testing.T) {
	dir, name := makeTestTemplate(t)
	err := WriteTextFile(dir, name, "bg.png", []byte("PNG"))
	if !isErrForbidden(err) {
		t.Errorf("expected ErrForbidden for .png write, got %v", err)
	}
}

func TestUploadFile_OK(t *testing.T) {
	dir, name := makeTestTemplate(t)
	data := []byte("fakepng")
	r := strings.NewReader(string(data))
	if err := UploadFile(dir, name, "bg.png", r); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, _ := os.ReadFile(filepath.Join(dir, name, "bg.png"))
	if string(got) != string(data) {
		t.Error("uploaded content mismatch")
	}
}

func TestUploadFile_DisallowedExt(t *testing.T) {
	dir, name := makeTestTemplate(t)
	r := strings.NewReader("content")
	err := UploadFile(dir, name, "script.js", r)
	if !isErrForbidden(err) {
		t.Errorf("expected ErrForbidden for .js upload, got %v", err)
	}
}

func TestUploadFile_TooLarge(t *testing.T) {
	dir, name := makeTestTemplate(t)
	// MaxUploadBytes+1 bytes of data.
	big := strings.NewReader(strings.Repeat("x", MaxUploadBytes+1))
	err := UploadFile(dir, name, "big.png", big)
	if err == nil {
		t.Error("expected error for oversized upload, got nil")
	}
}

func TestDeleteFile_OK(t *testing.T) {
	dir, name := makeTestTemplate(t)
	_ = os.WriteFile(filepath.Join(dir, name, "old.png"), []byte("PNG"), 0o644)
	if err := DeleteFile(dir, name, "old.png"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, name, "old.png")); !os.IsNotExist(err) {
		t.Error("file should be gone")
	}
}

func TestDeleteFile_Protected(t *testing.T) {
	dir, name := makeTestTemplate(t)
	err := DeleteFile(dir, name, "template.yaml")
	if !isErrForbidden(err) {
		t.Errorf("expected ErrForbidden for template.yaml, got %v", err)
	}
}

func TestDeleteFile_NotFound(t *testing.T) {
	dir, name := makeTestTemplate(t)
	err := DeleteFile(dir, name, "nonexistent.png")
	if !isErrNotFound(err) {
		t.Errorf("expected ErrFileNotFound, got %v", err)
	}
}

func isErrForbidden(err error) bool {
	return errors.Is(err, ErrForbidden) || errors.Is(err, ErrInvalidName)
}
func isErrNotFound(err error) bool { return errors.Is(err, ErrFileNotFound) }

func TestFileVersion(t *testing.T) {
	v := FileVersion([]byte("hello"))
	if v != "2cf24dba5fb0a30e" { // sha256("hello")[:8] hex
		t.Errorf("got %q", v)
	}
	if FileVersion(nil) != "e3b0c44298fc1c14" {
		t.Errorf("empty: got %q", FileVersion(nil))
	}
}

func TestWriteTextFileIfMatch(t *testing.T) {
	dir, name := makeTestTemplate(t)
	v0, err := CurrentVersion(dir, name, "template.yaml")
	if err != nil || v0 == "" {
		t.Fatalf("CurrentVersion = %q, %v", v0, err)
	}
	v1, err := WriteTextFileIfMatch(dir, name, "template.yaml", []byte("a: 1\n"), v0)
	if err != nil || v1 != FileVersion([]byte("a: 1\n")) {
		t.Fatalf("matching write: v=%q err=%v", v1, err)
	}
	_, err = WriteTextFileIfMatch(dir, name, "template.yaml", []byte("a: 2\n"), v0)
	if !errors.Is(err, ErrVersionConflict) || !strings.Contains(err.Error(), v1) {
		t.Fatalf("stale write error = %v", err)
	}
	got, _ := os.ReadFile(filepath.Join(dir, name, "template.yaml"))
	if string(got) != "a: 1\n" {
		t.Errorf("file changed despite conflict: %q", got)
	}
	for _, exp := range []string{"", "*"} {
		if _, err := WriteTextFileIfMatch(dir, name, "template.yaml", []byte("a: 3\n"), exp); err != nil {
			t.Errorf("unconditional write (%q): %v", exp, err)
		}
	}
	if _, err := WriteTextFileIfMatch(dir, name, "new.json", []byte("{}"), "abc"); !errors.Is(err, ErrVersionConflict) {
		t.Errorf("missing file with expected version: %v", err)
	}
	v, err := CurrentVersion(dir, name, "missing.json")
	if v != "" || err != nil {
		t.Errorf("CurrentVersion(missing) = %q, %v", v, err)
	}
}
