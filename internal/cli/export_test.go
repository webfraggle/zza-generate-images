package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/webfraggle/zza-generate-images/internal/devexport"
)

func TestExportCmd(t *testing.T) {
	out := filepath.Join(t.TempDir(), "pkg")
	cmd := ExportCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"-t", "nederland-105-v1", "--templates-dir", "../../templates", "-o", out})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(out + ".zip")
	if err != nil {
		t.Fatal(err)
	}
	p, err := devexport.ReadZIP(b)
	if err != nil || p.Name != "nederland-105-v1" {
		t.Fatalf("ReadZIP: %v", err)
	}
}

func TestExportCmdIncompatible(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "t"), 0o755)
	y := "meta:\n  canvas: {width: 160, height: 80}\nlayers:\n  - type: rect\n    x: 0\n    y: 0\n    width: 1\n    height: 1\n    color: \"#000\"\n"
	os.WriteFile(filepath.Join(root, "t", "template.yaml"), []byte(y), 0o644)
	out := filepath.Join(root, "o.zip")
	var stderr bytes.Buffer
	cmd := ExportCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"-t", "t", "--templates-dir", root, "-o", out})
	if err := cmd.Execute(); err == nil {
		t.Fatal("want error (exit status != 0)")
	}
	if !strings.Contains(stderr.String(), "240 × 240") {
		t.Errorf("stderr = %q", stderr.String())
	}
	if _, err := os.Stat(out); err == nil {
		t.Error("zip written for incompatible template")
	}
}
