package mcp

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// All shipped templates must be free of unknown-key warnings; otherwise the
// list of known keys (derived from the renderer types) is incomplete.
func TestUnknownKeys_ShippedTemplatesClean(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("..", "..", "templates", "*", "template.yaml"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no templates found: %v", err)
	}
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if w := unknownKeyWarnings(b); len(w) != 0 {
			t.Errorf("%s: %q", f, w)
		}
	}
}

func TestUnknownKeys_Typos(t *testing.T) {
	src := `meta:
  canvas: {width: 240, height: 240}
layers:
  - type: rect
    iff: "isEmpty(gleis)"
    x: 0
    y: 0
    width: 240
    height: 120
    color: "#123456"
  - type: copy
    src_x: 0
    src_y: 0
    src_widht: 240
    src_height: 120
    x: 0
    y: 120
`
	want := []string{
		`line 5: unknown key "iff"`,
		`line 14: unknown key "src_widht"`,
	}
	if got := unknownKeyWarnings([]byte(src)); !reflect.DeepEqual(got, want) {
		t.Errorf("got %q\nwant %q", got, want)
	}
}

func TestUnknownKeys_NestedAndKnownForms(t *testing.T) {
	src := `meta:
  name: x
  canvs: {width: 240, height: 240}
  animation: {frames: 2, frame_dely: 1}
fonts:
  - {id: bold, file: a.ttf, size: 3}
layers:
  - if: "isEmpty(gleis)"
    layers:
      - type: text
        value: "a"
        color:
          if: "isEmpty(gleis)"
          then: "#ffffff"
          else: "#000000"
        fit_items: {split_by: "-", gapp: 2}
  - else:
    layers: []
  - type: splitflap
    reel: ["a", "b"]
    layers: []
`
	want := []string{
		`line 3: unknown key "canvs"`,
		`line 4: unknown key "frame_dely"`,
		`line 6: unknown key "size"`,
		`line 16: unknown key "gapp"`,
	}
	if got := unknownKeyWarnings([]byte(src)); !reflect.DeepEqual(got, want) {
		t.Errorf("got %q\nwant %q", got, want)
	}
}

func TestUnknownKeys_TopLevelAndCap(t *testing.T) {
	var b strings.Builder
	b.WriteString("meta:\n  canvas: {width: 240, height: 240}\nlayer: []\nlayers:\n")
	for i := 0; i < 15; i++ {
		fmt.Fprintf(&b, "  - {type: rect, colour: \"#fff\"}\n")
	}
	got := unknownKeyWarnings([]byte(b.String()))
	if len(got) != maxUnknownKeyWarnings {
		t.Fatalf("got %d warnings, want %d: %q", len(got), maxUnknownKeyWarnings, got)
	}
	if got[0] != `line 3: unknown key "layer"` {
		t.Errorf("first warning %q", got[0])
	}
}

func TestValidateTemplateYAML_RejectionNamesTypo(t *testing.T) {
	e, _ := sbbEnv(t)
	src := "meta:\n  canvas: {width: 240, height: 240}\nlayers:\n  - {type: copy, src_x: 0, src_y: 0, src_widht: 240, src_height: 120, x: 0, y: 120}\n"
	_, err := validateTemplateYAML(e, "sbb-105-v1", []byte(src))
	if err == nil || !strings.Contains(err.Error(), `unknown keys, maybe typos: line 4: unknown key "src_widht"`) {
		t.Fatalf("err = %v", err)
	}
}

func TestWriteFile_UnknownKeyWarning(t *testing.T) {
	dir := copyTemplates(t, "sbb-105-v1")
	cs := newSession(t, dir)
	src := readDisk(t, dir, "sbb-105-v1", "template.yaml")
	changed := strings.Replace(src, "max_width:", "max_widht:", 1)
	if changed == src {
		t.Fatal("sbb template has no max_width")
	}
	res := callTool(t, cs, "write_file", map[string]any{"template": "sbb-105-v1", "name": "template.yaml", "content": changed})
	requireOK(t, res)
	var out writeFileOut
	parseJSONResult(t, res, resultText(res), &out)
	found := false
	for _, w := range out.Warnings {
		if strings.HasSuffix(w, `unknown key "max_widht"`) && strings.HasPrefix(w, "line ") {
			found = true
		}
	}
	if !found {
		t.Errorf("warnings %q", out.Warnings)
	}
	if readDisk(t, dir, "sbb-105-v1", "template.yaml") != changed {
		t.Error("template with unknown key was not saved")
	}
}
