package mcp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/webfraggle/zza-generate-images/docs"
	"github.com/webfraggle/zza-generate-images/internal/renderer"
)

// codeBlocks returns the bodies of all fenced code blocks with the given info
// string (```lang … ```) in the design guide.
func codeBlocks(t *testing.T, lang string) []string {
	t.Helper()
	re := regexp.MustCompile("(?ms)^```" + lang + "\\n(.*?)^```\\s*$")
	var out []string
	for _, m := range re.FindAllStringSubmatch(docs.DesignGuide, -1) {
		out = append(out, m[1])
	}
	return out
}

func TestDesignGuide_MandatoryContent(t *testing.T) {
	g := docs.DesignGuide
	for _, s := range []string{
		"240×120", "240×240", "-105-v1",
		"y 0–119", "y 120–239", "außen",
		"umuc-105-v1", "nederland-105-v1", "faltblatt-105-v1", "rhb-105-v1",
		"copy", "src_x: 120", "x: 162", // umuc swap example
		"Minimum 10 px",
		"{{zug1.vonnach}}", "{{zug1.abw}}", "{{gleis}}",
		"isEmpty", "Frankfurt (Main) Flughafen Fernbahnhof", `"abw": 25`,
		"render_preview", "get_template_spec", "svg_to_png", "add_font", "perspective_correct",
		"keine Verläufe",
		// physical reading of the two sides
		"Wer vor Seite A steht, sieht das Gleis links",
		// corner order for perspective_correct
		"oben-links, oben-rechts, unten-rechts, unten-links",
	} {
		if !strings.Contains(g, s) {
			t.Errorf("guide lacks %q", s)
		}
	}
	if strings.Contains(g, "Erste Fassung (Gerüst)") || strings.Contains(g, "(folgt)") {
		t.Error("guide still contains placeholder text from the first draft")
	}
	if n := strings.Count(g, "\n- [ ] "); n < 8 {
		t.Errorf("closing checklist has %d items, want at least 8", n)
	}
}

// starterKeys returns the key set of internal/editor/starter/default.json
// (nested keys as "zug1.vonnach").
func starterKeys(t *testing.T) []string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "editor", "starter", "default.json"))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	var keys []string
	for k, v := range m {
		keys = append(keys, k)
		if sub, ok := v.(map[string]any); ok {
			for sk := range sub {
				keys = append(keys, k+"."+sk)
			}
		}
	}
	return keys
}

func hasKey(m map[string]any, key string) bool {
	parts := strings.SplitN(key, ".", 2)
	v, ok := m[parts[0]]
	if !ok || len(parts) == 1 {
		return ok
	}
	sub, ok := v.(map[string]any)
	if !ok {
		return false
	}
	_, ok = sub[parts[1]]
	return ok
}

// guideTestData returns the JSON code blocks of the guide that are complete
// train data sets (contain zug1).
func guideTestData(t *testing.T) []map[string]any {
	t.Helper()
	var sets []map[string]any
	for i, b := range codeBlocks(t, "json") {
		var m map[string]any
		if err := json.Unmarshal([]byte(b), &m); err != nil {
			t.Errorf("json block %d does not parse: %v", i, err)
			continue
		}
		if _, ok := m["zug1"]; ok {
			sets = append(sets, m)
		}
	}
	return sets
}

func TestDesignGuide_JSONTestDataComplete(t *testing.T) {
	sets := guideTestData(t)
	if len(sets) < 4 {
		t.Fatalf("guide has %d complete test data sets, want 4", len(sets))
	}
	keys := starterKeys(t)
	for i, m := range sets {
		for _, k := range keys {
			if !hasKey(m, k) {
				t.Errorf("test data set %d lacks field %q", i, k)
			}
		}
	}
}

// fontDir returns a temp dir holding every font file of the repo templates,
// so that guide examples referencing e.g. Roboto-Bold.ttf can be rendered.
func fontDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	files, _ := filepath.Glob(filepath.Join("..", "..", "templates", "*", "*"))
	for _, f := range files {
		ext := strings.ToLower(filepath.Ext(f))
		if ext != ".ttf" && ext != ".otf" {
			continue
		}
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, filepath.Base(f)), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestDesignGuide_YAMLExamplesParse(t *testing.T) {
	var full, fragments int
	for i, b := range codeBlocks(t, "yaml") {
		if !strings.HasPrefix(b, "meta:") {
			// Layer fragments (quoted copy layers etc.) must parse as the
			// layers of a template.
			src := "meta:\n  canvas: {width: 240, height: 240}\nlayers:\n" + indent(b, "  ")
			if _, err := renderer.ParseTemplate([]byte(src), "/tmp"); err != nil {
				t.Errorf("yaml fragment %d does not parse as layers: %v", i, err)
			}
			fragments++
			continue
		}
		full++
		tmpl, err := renderer.ParseTemplate([]byte(b), "/tmp")
		if err != nil {
			t.Errorf("yaml block %d does not parse: %v", i, err)
			continue
		}
		if c := tmpl.Meta.Canvas; c.Width != 240 || c.Height != 240 {
			t.Errorf("yaml block %d: canvas %dx%d, want 240x240", i, c.Width, c.Height)
		}
	}
	if full == 0 {
		t.Fatal("guide has no complete yaml example starting with meta:")
	}
	if fragments < 4 { // umuc, faltblatt, nederland, rhb copy layers at least
		t.Errorf("guide has %d yaml layer fragments, want at least 4", fragments)
	}
}

// TestDesignGuide_YAMLExamplesRender renders every complete example with all
// four test data sets of the guide: the examples must really work.
func TestDesignGuide_YAMLExamplesRender(t *testing.T) {
	dir := fontDir(t)
	sets := guideTestData(t)
	for i, b := range codeBlocks(t, "yaml") {
		if !strings.HasPrefix(b, "meta:") {
			continue
		}
		tmpl, err := renderer.ParseTemplate([]byte(b), dir)
		if err != nil {
			t.Errorf("yaml block %d does not parse: %v", i, err)
			continue
		}
		for j, data := range sets {
			img, err := renderer.New(filepath.Dir(dir)).Render(tmpl, data)
			if err != nil {
				t.Errorf("yaml block %d, data set %d: render failed: %v", i, j, err)
				continue
			}
			if bottomHalfEmpty(img) {
				t.Errorf("yaml block %d: bottom half is empty", i)
			}
		}
	}
}

func indent(s, prefix string) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	for i, l := range lines {
		if l != "" {
			lines[i] = prefix + l
		}
	}
	return strings.Join(lines, "\n") + "\n"
}
