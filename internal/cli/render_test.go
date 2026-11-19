package cli

import (
	"bytes"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/webfraggle/zza-generate-images/internal/devexport"
	"github.com/webfraggle/zza-generate-images/internal/devicerender"
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

// TestRenderZZA: --zza renders through the device core, pixel-identical to
// devicerender.Render at the same time; GIF and incompatible templates fail.
func TestRenderZZA(t *testing.T) {
	const root = "../../testdata/devtemplates"
	data, err := os.ReadFile(filepath.Join(root, "cov-text", "default.json"))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	in := filepath.Join(dir, "in.json")
	if err := os.WriteFile(in, data, 0o644); err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) error {
		cmd := RenderCmd()
		cmd.SetOut(&bytes.Buffer{})
		cmd.SetErr(&bytes.Buffer{})
		cmd.SetArgs(append([]string{"-t", "cov-text", "-i", in, "--templates-dir", root}, args...))
		return cmd.Execute()
	}

	out := filepath.Join(dir, "out.png")
	if err := run("-o", out, "--zza", "--time", "2026-01-01T10:08:37"); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(out)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	got, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}

	pkg, err := devexport.Export(root, "cov-text")
	if err != nil {
		t.Fatal(err)
	}
	r := devicerender.New(32 << 20)
	defer r.Close()
	if err := r.Load(pkg); err != nil {
		t.Fatal(err)
	}
	loc, err := time.LoadLocation("Europe/Berlin") // cov-text sets no meta.timezone
	if err != nil {
		t.Fatal(err)
	}
	want, err := r.Render(data, time.Date(2026, 1, 1, 10, 8, 37, 0, loc))
	if err != nil {
		t.Fatal(err)
	}
	if got.Bounds() != want.Bounds() {
		t.Fatalf("bounds %v, want %v", got.Bounds(), want.Bounds())
	}
	diff := 0
	for y := 0; y < 240; y++ {
		for x := 0; x < 240; x++ {
			gr, gg, gb, ga := got.At(x, y).RGBA()
			wr, wg, wb, wa := want.At(x, y).RGBA()
			if gr != wr || gg != wg || gb != wb || ga != wa {
				diff++
			}
		}
	}
	if diff != 0 {
		t.Errorf("%d pixels differ from devicerender.Render", diff)
	}

	if err := run("-o", filepath.Join(dir, "o.gif"), "--zza"); err == nil || !strings.Contains(err.Error(), "--zza kann nur PNG") {
		t.Errorf("--zza with a gif: %v", err)
	}
	if err := run("-o", filepath.Join(dir, "o.png"), "--zza", "--format", "gif"); err == nil || !strings.Contains(err.Error(), "--zza kann nur PNG") {
		t.Errorf("--zza --format gif: %v", err)
	}
	if err := run("-o", filepath.Join(dir, "o.png"), "--time", "gestern"); err == nil {
		t.Error("invalid --time accepted")
	}
	// cliFlapYAML has a 20×10 canvas: not ZZA compatible.
	if _, err := runRender(t, "out.png", "--zza"); err == nil || !strings.Contains(err.Error(), "nicht ZZA-kompatibel") {
		t.Errorf("incompatible template: %v", err)
	}
}

// TestRenderTimeFlag: --time sets the clock of the Go renderer too (template zone).
func TestRenderTimeFlag(t *testing.T) {
	a, err := runRender(t, "a.png", "--time", "2026-01-01T10:08:37")
	if err != nil {
		t.Fatal(err)
	}
	if len(a) == 0 {
		t.Fatal("empty output")
	}
	if _, err := runRender(t, "b.png", "--time", "nonsense"); err == nil {
		t.Error("invalid --time accepted")
	}
}
