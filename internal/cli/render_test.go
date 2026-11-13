package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

const cliFlapYAML = `meta:
  canvas: {width: 20, height: 10}
layers:
  - type: splitflap
    x: 0
    y: 0
    width: 10
    height: 10
    cells: 2
    split: chars
    value: "AB"
    reel: " AB"
    var: ch
    layers:
      - if: "eq(ch, 'A')"
        layers: [{type: rect, x: 0, y: 0, width: 10, height: 10, color: "#ff0000"}]
`

func runRender(t *testing.T, args ...string) ([]byte, error) {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "t"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "t", "template.yaml"), []byte(cliFlapYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	in := filepath.Join(root, "in.json")
	if err := os.WriteFile(in, []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(root, args[0])
	cmd := RenderCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs(append([]string{"-t", "t", "-i", in, "--templates-dir", root, "-o", out}, args[1:]...))
	if err := cmd.Execute(); err != nil {
		return nil, err
	}
	return os.ReadFile(out)
}

func TestRender_OutputFormats(t *testing.T) {
	cases := []struct {
		name  string
		args  []string
		magic string
	}{
		{"gif by extension", []string{"out.gif"}, "GIF89a"},
		{"upper-case extension", []string{"out.GIF"}, "GIF89a"},
		{"png by extension", []string{"out.png"}, "\x89PNG"},
		{"no extension defaults to png", []string{"out"}, "\x89PNG"},
		{"format wins over extension", []string{"out.png", "--format", "gif"}, "GIF89a"},
		{"format png wins over gif extension", []string{"out.gif", "--format", "png"}, "\x89PNG"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := runRender(t, c.args...)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.HasPrefix(got, []byte(c.magic)) {
				t.Errorf("output starts with %q, want %q", got[:min(len(got), 6)], c.magic)
			}
		})
	}
}

func TestRender_InvalidFormat(t *testing.T) {
	if _, err := runRender(t, "out.png", "--format", "bmp"); err == nil {
		t.Error("expected error for --format bmp")
	}
}
