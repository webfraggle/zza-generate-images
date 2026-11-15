package mcp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/webfraggle/zza-generate-images/internal/renderer"
)

// fontFixture copies three templates and prepares <tmp>/fonts with a real
// font and a broken one.
func fontFixture(t *testing.T) (tdir, fdir string) {
	t.Helper()
	tdir = copyTemplates(t, "sbb-105-v1", "umuc-105-v1", "oebb-105-v1")
	fdir = filepath.Join(filepath.Dir(tdir), "fonts")
	if err := os.MkdirAll(fdir, 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join("..", "..", "templates", "rhb-105-v1", "LLPIXEL3.ttf"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fdir, "LLPIXEL3.ttf"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fdir, "broken.ttf"), []byte("this is plain text, not a font"), 0o644); err != nil {
		t.Fatal(err)
	}
	return tdir, fdir
}

func listFontsCall(t *testing.T, tdir string, args map[string]any) listFontsOut {
	t.Helper()
	cs := newSession(t, tdir)
	res := callTool(t, cs, "list_fonts", args)
	requireOK(t, res)
	var out listFontsOut
	if err := json.Unmarshal([]byte(resultText(res)), &out); err != nil {
		t.Fatalf("decoding result: %v\n%s", err, resultText(res))
	}
	return out
}

func TestDefaultFontID(t *testing.T) {
	cases := map[string]string{
		"NimbusSanL-Bol.otf": "nimbussanl-bol",
		"04B_03__.TTF":       "04b-03",
		"Roboto Bold.ttf":    "roboto-bold",
	}
	for in, want := range cases {
		if got := defaultFontID(in); got != want {
			t.Errorf("defaultFontID(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestListFonts(t *testing.T) {
	tdir, fdir := fontFixture(t)
	out := listFontsCall(t, tdir, nil)
	if out.FontsDir != fdir || !out.FontsDirExists {
		t.Fatalf("fonts_dir = %q exists=%v, want %q true", out.FontsDir, out.FontsDirExists, fdir)
	}
	if len(out.Fonts) == 0 || out.Fonts[0].Key != "@fonts/LLPIXEL3.ttf" || out.Fonts[0].Source != "fonts-dir" {
		t.Fatalf("first entry = %+v", out.Fonts)
	}
	n := 0
	for _, f := range out.Fonts {
		if f.Family == "" {
			t.Errorf("empty family: %+v", f)
		}
		if f.File == "Roboto-Bold.ttf" {
			n++
			if f.Key != "oebb-105-v1/Roboto-Bold.ttf" || strings.Join(f.FoundIn, ",") != "oebb-105-v1,umuc-105-v1" {
				t.Errorf("Roboto-Bold = %+v", f)
			}
		}
	}
	if n != 1 {
		t.Errorf("Roboto-Bold.ttf listed %d times, want 1", n)
	}
	found := false
	for _, s := range out.Skipped {
		if s.Key == "@fonts/broken.ttf" && strings.HasPrefix(s.Reason, "not a font: ") {
			found = true
		}
	}
	if !found {
		t.Errorf("broken.ttf not in skipped: %+v", out.Skipped)
	}

	q := listFontsCall(t, tdir, map[string]any{"query": "nimbus"})
	if len(q.Fonts) == 0 {
		t.Fatal("query nimbus: no fonts")
	}
	for _, f := range q.Fonts {
		if !strings.HasPrefix(f.File, "NimbusSanL-") {
			t.Errorf("query nimbus returned %q", f.File)
		}
	}
}

func TestListFonts_NoFontsDir(t *testing.T) {
	tdir, fdir := fontFixture(t)
	if err := os.RemoveAll(fdir); err != nil {
		t.Fatal(err)
	}
	cs := newSession(t, tdir)
	res := callTool(t, cs, "list_fonts", nil)
	requireOK(t, res)
	var out listFontsOut
	if err := json.Unmarshal([]byte(resultText(res)), &out); err != nil {
		t.Fatal(err)
	}
	if out.FontsDirExists {
		t.Error("fonts_dir_exists should be false")
	}
	if !strings.Contains(resultText(res), "Put .ttf/.otf files into") {
		t.Errorf("missing hint: %s", resultText(res))
	}
}

func addFontCall(t *testing.T, tdir string, args map[string]any) (addFontOut, string) {
	t.Helper()
	cs := newSession(t, tdir)
	res := callTool(t, cs, "add_font", args)
	if res.IsError {
		return addFontOut{}, resultText(res)
	}
	var out addFontOut
	if err := json.Unmarshal([]byte(resultText(res)), &out); err != nil {
		t.Fatalf("decoding: %v\n%s", err, resultText(res))
	}
	return out, ""
}

func TestAddFont_FromFontsDir(t *testing.T) {
	tdir, _ := fontFixture(t)
	yamlPath := filepath.Join(tdir, "sbb-105-v1", "template.yaml")
	before, _ := os.ReadFile(yamlPath)
	cs := newSession(t, tdir)
	args := map[string]any{"template": "sbb-105-v1", "font": "@fonts/LLPIXEL3.ttf"}
	res := callTool(t, cs, "add_font", args)
	requireOK(t, res)
	var out addFontOut
	if err := json.Unmarshal([]byte(resultText(res)), &out); err != nil {
		t.Fatal(err)
	}
	if out.ID != "llpixel3" || out.File != "LLPIXEL3.ttf" || !out.Added || !out.Copied || out.LicenseNote == "" {
		t.Fatalf("out = %+v", out)
	}
	if _, err := os.Stat(filepath.Join(tdir, "sbb-105-v1", "LLPIXEL3.ttf")); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(yamlPath)
	for _, line := range strings.Split(string(before), "\n") {
		if strings.HasPrefix(line, "# ") && !strings.Contains(string(after), line) {
			t.Fatalf("comment line lost: %q", line)
		}
	}
	tmpl, err := renderer.LoadTemplate(tdir, "sbb-105-v1")
	if err != nil {
		t.Fatal(err)
	}
	last := tmpl.Fonts[len(tmpl.Fonts)-1]
	if last.ID != "llpixel3" || last.File != "LLPIXEL3.ttf" {
		t.Errorf("last font = %+v", last)
	}
	count := len(tmpl.Fonts)

	res = callTool(t, cs, "add_font", args)
	requireOK(t, res)
	out = addFontOut{}
	_ = json.Unmarshal([]byte(resultText(res)), &out)
	if out.Added || out.Copied || out.ID != "llpixel3" {
		t.Errorf("second call = %+v", out)
	}
	tmpl, _ = renderer.LoadTemplate(tdir, "sbb-105-v1")
	if len(tmpl.Fonts) != count {
		t.Errorf("fonts count %d -> %d", count, len(tmpl.Fonts))
	}
}

func TestAddFont_FromTemplate(t *testing.T) {
	tdir, _ := fontFixture(t)
	out, errText := addFontCall(t, tdir, map[string]any{"template": "sbb-105-v1", "font": "oebb-105-v1/Roboto-Light.ttf", "id": "light"})
	if errText != "" {
		t.Fatal(errText)
	}
	if out.ID != "light" || !out.Added || !out.Copied {
		t.Errorf("out = %+v", out)
	}
	if _, err := os.Stat(filepath.Join(tdir, "sbb-105-v1", "Roboto-Light.ttf")); err != nil {
		t.Error(err)
	}
}

func TestAddFont_Conflicts(t *testing.T) {
	tdir, _ := fontFixture(t)
	_, errText := addFontCall(t, tdir, map[string]any{"template": "sbb-105-v1", "font": "oebb-105-v1/Roboto-Light.ttf", "id": "bold"})
	if want := `font id "bold" is already used for NimbusSanL-Bol.otf`; !strings.Contains(errText, want) {
		t.Errorf("error = %q, want %q", errText, want)
	}
	junk := filepath.Join(tdir, "sbb-105-v1", "LLPIXEL3.ttf")
	if err := os.WriteFile(junk, []byte("junk"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, errText = addFontCall(t, tdir, map[string]any{"template": "sbb-105-v1", "font": "@fonts/LLPIXEL3.ttf"})
	if want := "LLPIXEL3.ttf already exists in sbb-105-v1 with different content"; !strings.Contains(errText, want) {
		t.Errorf("error = %q, want %q", errText, want)
	}
	if b, _ := os.ReadFile(junk); string(b) != "junk" {
		t.Error("existing file was overwritten")
	}
}

func TestAddFont_NoFontsKey(t *testing.T) {
	tdir, _ := fontFixture(t)
	yamlPath := filepath.Join(tdir, "sbb-105-v1", "template.yaml")
	src := "meta:\n  name: x\n  canvas:\n    width: 240\n    height: 240\n\n# layers follow\nlayers:\n  - type: rect\n    x: 0\n    y: 0\n    width: 10\n    height: 10\n    color: \"#fff\"\n"
	if err := os.WriteFile(yamlPath, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, errText := addFontCall(t, tdir, map[string]any{"template": "sbb-105-v1", "font": "@fonts/LLPIXEL3.ttf"}); errText != "" {
		t.Fatal(errText)
	}
	got, _ := os.ReadFile(yamlPath)
	s := string(got)
	if fi, li := strings.Index(s, "fonts:"), strings.Index(s, "layers:"); fi < 0 || fi > li {
		t.Errorf("fonts: not inserted before layers:\n%s", s)
	}
	tmpl, err := renderer.LoadTemplate(tdir, "sbb-105-v1")
	if err != nil {
		t.Fatal(err)
	}
	if len(tmpl.Fonts) != 1 || tmpl.Fonts[0].ID != "llpixel3" {
		t.Errorf("fonts = %+v", tmpl.Fonts)
	}
}

func TestAddFont_BadKeys(t *testing.T) {
	tdir, fdir := fontFixture(t)
	if err := os.Symlink(filepath.Join(fdir, "LLPIXEL3.ttf"), filepath.Join(fdir, "link.ttf")); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"@fonts/../x.ttf", "@fonts/sub/x.ttf", "/etc/x.ttf", "sbb-105-v1/bg.png", "nope-105-v1/x.ttf", "@fonts/link.ttf"} {
		_, errText := addFontCall(t, tdir, map[string]any{"template": "sbb-105-v1", "font": key})
		if !strings.Contains(errText, "unknown font key") {
			t.Errorf("key %q: error = %q", key, errText)
		}
	}
	_, errText := addFontCall(t, tdir, map[string]any{"template": "sbb-105-v1", "font": "@fonts/broken.ttf"})
	if !strings.Contains(errText, "not a font: ") {
		t.Errorf("broken: error = %q", errText)
	}
	if strings.Contains(errText, "plain text") {
		t.Error("error leaks file content")
	}
}
