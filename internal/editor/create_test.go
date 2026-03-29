package editor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCreateTemplate_CreatesFiles(t *testing.T) {
	dir := t.TempDir()
	err := CreateTemplate(dir, "my-tmpl", "Mein Template", "Eine Beschreibung", `1.05"`, 240, 240)
	if err != nil {
		t.Fatalf("CreateTemplate: %v", err)
	}

	// Directory must exist.
	info, err := os.Stat(filepath.Join(dir, "my-tmpl"))
	if err != nil || !info.IsDir() {
		t.Fatal("template directory not created")
	}

	// template.yaml must exist and contain meta fields.
	yaml, err := os.ReadFile(filepath.Join(dir, "my-tmpl", "template.yaml"))
	if err != nil {
		t.Fatalf("template.yaml not written: %v", err)
	}
	yamlStr := string(yaml)
	for _, want := range []string{
		`name: "Mein Template"`,
		`description: "Eine Beschreibung"`,
		`display: "1.05\""`,
		`width: 240`,
		`height: 240`,
	} {
		if !strings.Contains(yamlStr, want) {
			t.Errorf("template.yaml missing %q\ngot:\n%s", want, yamlStr)
		}
	}

	// default.json must exist.
	if _, err := os.Stat(filepath.Join(dir, "my-tmpl", "default.json")); err != nil {
		t.Fatal("default.json not written")
	}
}

func TestCreateTemplate_SmallDisplay(t *testing.T) {
	dir := t.TempDir()
	err := CreateTemplate(dir, "small", "Klein", "", `0.96"`, 160, 160)
	if err != nil {
		t.Fatalf("CreateTemplate: %v", err)
	}
	yaml, _ := os.ReadFile(filepath.Join(dir, "small", "template.yaml"))
	yamlStr := string(yaml)
	for _, want := range []string{`display: "0.96\""`, `width: 160`, `height: 160`} {
		if !strings.Contains(yamlStr, want) {
			t.Errorf("missing %q in yaml:\n%s", want, yamlStr)
		}
	}
}

func TestCreateTemplate_FailsIfExists(t *testing.T) {
	dir := t.TempDir()
	// Create the dir manually.
	if err := os.Mkdir(filepath.Join(dir, "exists"), 0o755); err != nil {
		t.Fatal(err)
	}
	err := CreateTemplate(dir, "exists", "X", "", `1.05"`, 240, 240)
	if err == nil {
		t.Fatal("expected error when directory already exists, got nil")
	}
}

func TestCreateTemplate_EscapesYAMLSpecialChars(t *testing.T) {
	dir := t.TempDir()
	err := CreateTemplate(dir, "esc", `Say "hello"`, `Back\slash`, `1.05"`, 240, 240)
	if err != nil {
		t.Fatalf("CreateTemplate: %v", err)
	}
	yaml, _ := os.ReadFile(filepath.Join(dir, "esc", "template.yaml"))
	yamlStr := string(yaml)
	if !strings.Contains(yamlStr, `Say \"hello\"`) {
		t.Errorf("double quotes not escaped:\n%s", yamlStr)
	}
	if !strings.Contains(yamlStr, `Back\\slash`) {
		t.Errorf("backslash not escaped:\n%s", yamlStr)
	}
}
