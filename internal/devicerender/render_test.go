package devicerender

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/webfraggle/zza-generate-images/internal/devexport"
	"github.com/webfraggle/zza-generate-images/internal/renderer"
)

const covRoot = "../../testdata/devtemplates"

// templateLoc returns the template's time zone (Berlin if unset), like the Go renderer.
func templateLoc(t *testing.T, tmpl *renderer.Template) *time.Location {
	t.Helper()
	name := tmpl.Meta.Timezone
	if name == "" {
		name = "Europe/Berlin"
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		t.Fatal(err)
	}
	return loc
}

// fixedNow is the fixed render time 2026-01-01 10:08:37 in the template's zone.
func fixedNow(t *testing.T, tmpl *renderer.Template) time.Time {
	return time.Date(2026, 1, 1, 10, 8, 37, 0, templateLoc(t, tmpl))
}

// goRender renders with the Go reference renderer at the fixed time.
func goRender(t *testing.T, dir, name string, data []byte) (*image.NRGBA, error) {
	t.Helper()
	tmpl, err := renderer.LoadTemplate(dir, name)
	if err != nil {
		t.Fatalf("LoadTemplate: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("data: %v", err)
	}
	now := fixedNow(t, tmpl)
	return renderer.NewWithClock(dir, func() time.Time { return now }).Render(tmpl, m)
}

// loadDevice exports and loads a template into a new device renderer.
func loadDevice(t *testing.T, dir, name string) *Renderer {
	t.Helper()
	p, err := devexport.Export(dir, name)
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	return loadPkg(t, p)
}

func loadPkg(t *testing.T, p *devexport.Package) *Renderer {
	t.Helper()
	r := New(32 << 20)
	t.Cleanup(r.Close)
	if err := r.Load(p); err != nil {
		t.Fatalf("Load: %v", err)
	}
	return r
}

// deviceRender renders with the C++ core at the fixed time (in the template's zone).
func deviceRender(t *testing.T, r *Renderer, dir, name string, data []byte) (*image.NRGBA, error) {
	t.Helper()
	tmpl, err := renderer.LoadTemplate(dir, name)
	if err != nil {
		t.Fatalf("LoadTemplate: %v", err)
	}
	return r.Render(data, fixedNow(t, tmpl))
}

func readDefault(t *testing.T, dir, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, name, "default.json"))
	if os.IsNotExist(err) {
		return []byte("{}")
	}
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// compareNear renders data with Go and C++ and asserts the images are near.
func compareNear(t *testing.T, r *Renderer, dir, name string, data []byte) {
	t.Helper()
	g, err := goRender(t, dir, name, data)
	if err != nil {
		t.Fatalf("Go render: %v", err)
	}
	c, err := deviceRender(t, r, dir, name, data)
	if err != nil {
		t.Fatalf("C++ render: %v", err)
	}
	assertNear(t, g, c)
}

func TestRenderShippedNear(t *testing.T) {
	ents, err := os.ReadDir(templatesRoot)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, e := range ents {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		t.Run(name, func(t *testing.T) {
			tmpl, err := renderer.LoadTemplate(templatesRoot, name)
			if err != nil {
				t.Fatal(err)
			}
			p, err := devexport.Export(templatesRoot, name)
			var inc *devexport.Incompatible
			if errors.As(err, &inc) {
				t.Skipf("not exportable: %v", inc)
			}
			if err != nil {
				t.Fatal(err)
			}
			r := loadPkg(t, p)
			data := readDefault(t, templatesRoot, name)
			n++
			if _, err := r.Render(data, fixedNow(t, tmpl)); err != nil {
				t.Fatal(err)
			}
			used := r.MemUsed() // second render: no leak
			if _, err := r.Render(data, fixedNow(t, tmpl)); err != nil {
				t.Fatal(err)
			}
			if r.MemUsed() != used {
				t.Errorf("MemUsed after second render %d, want %d", r.MemUsed(), used)
			}
			compareNear(t, r, templatesRoot, name, data)
		})
	}
	if n == 0 {
		t.Fatal("no shipped template compared")
	}
}

// mapStrings returns a copy of the JSON value with every string replaced by f(s).
func mapStrings(v any, f func(string) string) any {
	switch x := v.(type) {
	case string:
		return f(x)
	case map[string]any:
		out := map[string]any{}
		for k, e := range x {
			out[k] = mapStrings(e, f)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, e := range x {
			out[i] = mapStrings(e, f)
		}
		return out
	}
	return v
}

// covDatasets returns default.json, the same with empty strings and with very long strings.
func covDatasets(t *testing.T, def []byte) map[string][]byte {
	t.Helper()
	var v any
	if err := json.Unmarshal(def, &v); err != nil {
		t.Fatal(err)
	}
	enc := func(v any) []byte {
		b, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	return map[string][]byte{
		"default": def,
		"empty":   enc(mapStrings(v, func(string) string { return "" })),
		"long": enc(mapStrings(v, func(s string) string {
			if s == "" {
				s = "Leer"
			}
			return strings.Repeat(s+" ", 300/len(s)+2)
		})),
	}
}

func TestRenderCoverageNear(t *testing.T) {
	dirs, err := filepath.Glob(filepath.Join(covRoot, "cov-*"))
	if err != nil || len(dirs) == 0 {
		t.Fatalf("no coverage templates: %v", err)
	}
	for _, d := range dirs {
		name := filepath.Base(d)
		t.Run(name, func(t *testing.T) {
			if _, err := renderer.LoadTemplate(covRoot, name); err != nil {
				t.Fatal(err)
			}
			r := loadDevice(t, covRoot, name)
			for ds, data := range covDatasets(t, readDefault(t, covRoot, name)) {
				t.Run(ds, func(t *testing.T) { compareNear(t, r, covRoot, name, data) })
			}
		})
	}
}

// ---- generated test templates ----

const tmplHead = `meta:
  name: test
  canvas: {width: 240, height: 240}
fonts:
  - id: reg
    file: font.otf
layers:
`

// writeTemplate creates <dir>/<name>/template.yaml with tmplHead + layers, a
// font and an image img.png (w×h, half transparent at the right).
func writeTemplate(t *testing.T, dir, name, layers string) {
	t.Helper()
	td := filepath.Join(dir, name)
	if err := os.MkdirAll(td, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(td, "template.yaml"), []byte(tmplHead+layers), 0o644); err != nil {
		t.Fatal(err)
	}
	font, err := os.ReadFile(filepath.Join(templatesRoot, "sbb-105-v1", "NimbusSanL-Reg.otf"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(td, "font.otf"), font, 0o644); err != nil {
		t.Fatal(err)
	}
	img := image.NewNRGBA(image.Rect(0, 0, 40, 8))
	for y := 0; y < 8; y++ {
		for x := 0; x < 40; x++ {
			a := uint8(255)
			if x >= 20 {
				a = 128
			}
			img.SetNRGBA(x, y, color.NRGBA{uint8(6 * x), uint8(30 * y), 200, a})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(td, "img.png"), buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRenderErrorsLikeGo(t *testing.T) {
	dir := t.TempDir()
	cases := map[string]string{
		"text-color": `  - {type: text, value: "x", font: reg, color: "#12345", x: 1, y: 1}
`,
		"rect-color": `  - {type: rect, color: "red", x: 1, y: 1, width: 5, height: 5}
`,
		"rotate-abc": `  - {type: image, file: img.png, x: 50, y: 50, rotate: "abc"}
`,
		"rotate-inf": `  - {type: image, file: img.png, x: 50, y: 50, rotate: "Inf"}
`,
		"copy-zero": `  - {type: copy, src_x: 0, src_y: 0, src_width: 0, src_height: 5, x: 5, y: 5}
`,
		"unknown-type": `  - {type: circle, x: 1, y: 1}
`,
		"elif-first": `  - {type: rect, elif: "true", color: "#ffffff", x: 1, y: 1, width: 5, height: 5}
`,
		"nested-loop": `  - type: loop
    value: "a|b"
    layers:
      - type: loop
        value: "c"
        layers:
          - {type: rect, color: "#ffffff", x: 1, y: 1, width: 5, height: 5}
`,
		"cond-color-bad": `  - type: rect
    color: {if: "equals(a, 'b')", then: "#ffffff", else: "#zzzzzz"}
    x: 1
    y: 1
    width: 5
    height: 5
`,
	}
	for name, layers := range cases {
		t.Run(name, func(t *testing.T) {
			writeTemplate(t, dir, name, layers)
			if _, err := goRender(t, dir, name, []byte("{}")); err == nil {
				t.Fatal("Go renders without error; case is wrong")
			}
			r := loadDevice(t, dir, name)
			_, err := deviceRender(t, r, dir, name, []byte("{}"))
			if !errors.Is(err, ErrTemplate) {
				t.Fatalf("C++: %v, want ErrTemplate", err)
			}
		})
	}

	// Unknown font id: the exporter rejects it, so the package is patched.
	t.Run("unknown-font", func(t *testing.T) {
		writeTemplate(t, dir, "font-bad", `  - {type: text, value: "x", font: nope, color: "#ffffff", x: 1, y: 1}
`)
		if _, err := goRender(t, dir, "font-bad", []byte("{}")); err == nil {
			t.Fatal("Go renders without error")
		}
		writeTemplate(t, dir, "font-ok", `  - {type: text, value: "x", font: reg, color: "#ffffff", x: 1, y: 1}
`)
		p, err := devexport.Export(dir, "font-ok")
		if err != nil {
			t.Fatal(err)
		}
		for _, patch := range []struct{ from, to string }{{`"_f":"f0"`, `"_f":"f7"`}, {`"_f":"f0",`, ``}} {
			q := clonePkg(p)
			tj := string(q.Files["t.json"])
			if !strings.Contains(tj, patch.from) {
				t.Fatalf("t.json has no %s: %s", patch.from, tj)
			}
			setFile(t, q, "t.json", []byte(strings.Replace(tj, patch.from, patch.to, 1)))
			r := loadPkg(t, q)
			if _, err := r.Render([]byte("{}"), time.Now()); !errors.Is(err, ErrTemplate) {
				t.Fatalf("patch %q: %v, want ErrTemplate", patch.to, err)
			}
		}
	})
}

func TestRenderDataErrors(t *testing.T) {
	dir := t.TempDir()
	writeTemplate(t, dir, "d", `  - {type: text, value: "{{a}}", font: reg, color: "#ffffff", x: 1, y: 20}
`)
	r := loadDevice(t, dir, "d")
	for _, data := range []string{`{`, `[1,2]`, `"x"`, `{"a":}`} {
		if _, err := r.Render([]byte(data), time.Now()); !errors.Is(err, ErrData) {
			t.Errorf("%q: %v, want ErrData", data, err)
		}
	}
	// R7: data larger than 256 KB is rejected.
	big := []byte(`{"a":"` + strings.Repeat("x", 256<<10) + `"}`)
	if _, err := r.Render(big, time.Now()); !errors.Is(err, ErrData) {
		t.Errorf("big data: %v, want ErrData", err)
	}
	for _, data := range []string{``, `{}`, `null`} {
		if _, err := r.Render([]byte(data), time.Now()); err != nil {
			t.Errorf("%q: %v", data, err)
		}
	}
	// Still usable after errors; no package → ErrState.
	if _, err := New(1<<20).Render([]byte(`{}`), time.Now()); !errors.Is(err, ErrState) {
		t.Errorf("no package: %v, want ErrState", err)
	}
}

// TestRenderHostileData: non-Latin-1 runes, invalid UTF-8 and a huge field in
// a wrapped text layer render without error, fast, and draw '?' for each
// non-Latin-1 rune / invalid byte.
func TestRenderHostileData(t *testing.T) {
	dir := t.TempDir()
	writeTemplate(t, dir, "h", `  - {type: text, value: "{{a}}", font: reg, size: 16, color: "#ffffff", x: 4, y: 4, max_width: 230}
  - {type: text, value: "{{b}}", font: reg, size: 12, color: "#ffff00", x: 4, y: 200, max_width: 100}
`)
	r := loadDevice(t, dir, "h")
	hostile := "Zug 🚄 nach Groß ẞtadt \xff\xfe Ende 😀😀 ő x"
	asQ := "Zug ? nach Groß ?tadt ?? Ende ?? ? x"
	huge := strings.Repeat("Lang-Wort ", 25000) // 250 KB, just below the data limit
	mk := func(a string) []byte {
		// Hand-built JSON so invalid UTF-8 bytes reach the core unchanged.
		return []byte(`{"a":"` + a + `","b":"` + huge + `"}`)
	}
	start := time.Now()
	got, err := r.Render(mk(hostile), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if d := time.Since(start); d > 200*time.Millisecond {
		t.Errorf("render took %v, want < 200 ms", d)
	}
	want, err := r.Render(mk(asQ), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got.Pix, want.Pix) {
		t.Error("hostile data does not render like '?' substitutes")
	}
	blank, err := r.Render([]byte(`{"a":"","b":""}`), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(got.Pix, blank.Pix) {
		t.Error("nothing drawn")
	}
}

// TestDynamicSizeEdge: dynamic image sizes 0, -5, 10 000 and beyond the
// limit behave like Go (error ↔ ErrTemplate, same picture otherwise).
func TestDynamicSizeEdge(t *testing.T) {
	dir := t.TempDir()
	cases := map[string]string{
		"w0":       `width: "{{0}}"`,
		"w-5":      `width: "{{0-5}}"`,
		"w-5h-5":   `width: "{{0-5}}", height: "{{0-5}}"`,
		"h-only":   `height: "{{30}}"`,
		"w10000":   `width: "{{10000}}", height: 30`,
		"h10000":   `height: "{{10000}}", width: 30`,
		"w20000":   `width: "{{20000}}"`,
		"w-5h20":   `width: "{{0-5}}", height: "{{20}}"`,
		"w1h1":     `width: "{{1}}", height: "{{1}}"`,
		"rot-big":  `width: "{{2000}}", height: 300, rotate: "30"`,
		"rot-tiny": `width: "{{3}}", rotate: "45"`,
	}
	for name, attrs := range cases {
		t.Run(name, func(t *testing.T) {
			writeTemplate(t, dir, name, fmt.Sprintf(`  - {type: rect, color: "#203040", x: 0, y: 0, width: 240, height: 240}
  - {type: image, file: img.png, x: 100, y: 100, %s}
`, attrs))
			g, gerr := goRender(t, dir, name, []byte("{}"))
			r := loadDevice(t, dir, name)
			c, cerr := deviceRender(t, r, dir, name, []byte("{}"))
			if gerr != nil {
				if !errors.Is(cerr, ErrTemplate) {
					t.Fatalf("Go: %v; C++: %v, want ErrTemplate", gerr, cerr)
				}
				return
			}
			if cerr != nil {
				t.Fatalf("C++: %v", cerr)
			}
			assertNear(t, g, c)
		})
	}
}

// TestRenderNoMem: a budget that fits the package but not the render memory
// (data document, scaled images) gives ErrNoMem without leaking; the renderer
// stays usable.
func TestRenderNoMem(t *testing.T) {
	const name = "cov-image"
	p, err := devexport.Export(covRoot, name)
	if err != nil {
		t.Fatal(err)
	}
	probe := loadPkg(t, p)
	loaded, loadPeak := probe.MemUsed(), probe.MemPeak()
	data := readDefault(t, covRoot, name)
	if _, err := deviceRender(t, probe, covRoot, name, data); err != nil {
		t.Fatal(err)
	}
	need := probe.MemPeak()
	budget := max(loadPeak, loaded+(need-loaded)/2)
	if budget >= need {
		t.Fatalf("render needs no more than loading (%d >= %d)", budget, need)
	}

	r := New(budget)
	defer r.Close()
	if err := r.Load(p); err != nil {
		t.Fatal(err)
	}
	if _, err := deviceRender(t, r, covRoot, name, data); !errors.Is(err, ErrNoMem) {
		t.Fatalf("render: %v, want ErrNoMem", err)
	}
	if r.MemUsed() != loaded {
		t.Fatalf("MemUsed after failed render %d, want %d", r.MemUsed(), loaded)
	}
	r2 := New(need)
	defer r2.Close()
	if err := r2.Load(p); err != nil {
		t.Fatal(err)
	}
	if _, err := deviceRender(t, r2, covRoot, name, data); err != nil {
		t.Fatalf("render with peak budget %d (loaded %d, used %d peak %d): %v", need, loaded, r2.MemUsed(), r2.MemPeak(), err)
	}
}

// TestRenderHostileValues: out-of-range numbers in a (hand-made) package's
// text layer give ErrTemplate (what Go rejects at load) or render, always in
// bounded time — never a hang or undefined behaviour.
func TestRenderHostileValues(t *testing.T) {
	dir := t.TempDir()
	writeTemplate(t, dir, "hv", `  - {type: text, id: tx, advance: 3, value: "{{a}}", font: reg, size: 12, scale_x: 0.5, scale_y: 0.5, line_height: 1.5, max_width: 100, color: "#ffffff", x: 4, y: 4}
  - {type: text, value: "{{a}}", font: reg, size: 12, line_height: 1.5, max_width: 100, color: "#ffffff", x: 4, y: 120}
`)
	p, err := devexport.Export(dir, "hv")
	if err != nil {
		t.Fatal(err)
	}
	data := []byte(`{"a":"` + strings.Repeat("Wort-Wort Hallo ", 250) + `"}`)
	tj := string(p.Files["t.json"])
	patch := func(from, to string) *devexport.Package {
		if strings.Count(tj, from) == 0 {
			t.Fatalf("t.json has no %s: %s", from, tj)
		}
		q := clonePkg(p)
		setFile(t, q, "t.json", []byte(strings.ReplaceAll(tj, from, to)))
		return q
	}
	var cases []struct{ from, to string }
	for _, v := range []string{"1e-9", "1e999", "-1e999", "1e300", "-0.01", "NaN", "8.0001", "0.0499"} {
		cases = append(cases, struct{ from, to string }{`"scale_x":0.5`, `"scale_x":` + v})
		cases = append(cases, struct{ from, to string }{`"scale_y":0.5`, `"scale_y":` + v})
		cases = append(cases, struct{ from, to string }{`"line_height":1.5`, `"line_height":` + v})
	}
	for _, v := range []string{"2147483647", "-2147483648", "9223372036854775807", "1e300"} {
		cases = append(cases, struct{ from, to string }{`"max_width":100`, `"max_width":` + v})
	}
	for _, v := range []string{"-1", "241", "2147483647"} {
		cases = append(cases, struct{ from, to string }{`"advance":3`, `"advance":` + v})
	}
	for _, c := range cases {
		q := patch(c.from, c.to)
		r := New(32 << 20)
		if err := r.Load(q); err != nil {
			r.Close()
			if !errors.Is(err, ErrFormat) {
				t.Errorf("%s: load: %v", c.to, err)
			}
			continue // not representable as JSON for the core: rejected at load
		}
		start := time.Now()
		_, err := r.Render(data, time.Now())
		if d := time.Since(start); d > 200*time.Millisecond {
			t.Errorf("%s: render took %v", c.to, d)
		}
		if err != nil && !errors.Is(err, ErrTemplate) {
			t.Errorf("%s: %v, want nil or ErrTemplate", c.to, err)
		}
		t.Logf("%s: %v", c.to, err)
		r.Close()
	}
}

// TestRenderHugeWrapWidthLikeGo: a valid template whose wrap width divided by
// scale_x (4e9) exceeds int32 renders like Go.
func TestRenderHugeWrapWidthLikeGo(t *testing.T) {
	dir := t.TempDir()
	writeTemplate(t, dir, "hw", `  - {type: text, value: "{{a}}", font: reg, size: 14, scale_x: 0.5, max_width: 2000000000, color: "#ffffff", x: 4, y: 4}
  - {type: text, value: "{{a}}", font: reg, size: 14, scale_x: 0.5, max_width: 2000000000, align: right, width: 230, color: "#ffff00", x: 4, y: 40}
`)
	r := loadDevice(t, dir, "hw")
	compareNear(t, r, dir, "hw", []byte(`{"a":"`+strings.Repeat("Langer Text ", 300)+`"}`))
}

// TestRenderTextMemoryBudget: the layout memory of a wrapped 4096-rune text
// is budget memory: it raises MemPeak, and a budget without room for it gives
// ErrNoMem instead of succeeding.
func TestRenderTextMemoryBudget(t *testing.T) {
	dir := t.TempDir()
	writeTemplate(t, dir, "tm", `  - {type: text, if: "isEmpty(nowrap)", value: "{{t}}", font: reg, size: 10, max_width: 100, color: "#ffffff", x: 4, y: 4}
  - {type: text, else: true, value: "{{t}}", font: reg, size: 10, color: "#ffffff", x: 4, y: 4}
`)
	text := strings.Repeat("Wort ", 900) // > 4096 runes, capped
	wrap := []byte(`{"t":"` + text + `","nowrap":""}`)
	nowrap := []byte(`{"t":"` + text + `","nowrap":"x"}`)
	peak := func(data []byte) int {
		r := loadDevice(t, dir, "tm")
		if _, err := deviceRender(t, r, dir, "tm", data); err != nil {
			t.Fatal(err)
		}
		return r.MemPeak()
	}
	pw, pn := peak(wrap), peak(nowrap)
	t.Logf("MemPeak wrapped %d, unwrapped %d", pw, pn)
	if pw-pn < 16<<10 {
		t.Fatalf("wrapping a 4096-rune text raised MemPeak by only %d bytes", pw-pn)
	}
	p, err := devexport.Export(dir, "tm")
	if err != nil {
		t.Fatal(err)
	}
	r := New(pn + 2048)
	defer r.Close()
	if err := r.Load(p); err != nil {
		t.Fatal(err)
	}
	if _, err := deviceRender(t, r, dir, "tm", wrap); !errors.Is(err, ErrNoMem) {
		t.Fatalf("wrapped text with small budget: %v, want ErrNoMem", err)
	}
	if _, err := deviceRender(t, r, dir, "tm", nowrap); err != nil {
		t.Fatalf("unwrapped text after ErrNoMem: %v", err)
	}
}

// TestRenderLimitsLikeGo: render-time limits behave like Go (error ↔
// ErrTemplate, otherwise a near image).
func TestRenderLimitsLikeGo(t *testing.T) {
	dir := t.TempDir()
	nested := ""
	for i := 0; i < 18; i++ {
		nested += strings.Repeat("  ", i+1) + "- layers:\n"
	}
	nested += strings.Repeat("  ", 19) + `- {type: rect, color: "#ffffff", x: 1, y: 1, width: 5, height: 5}` + "\n"
	ok := nested[:0]
	for i := 0; i < 16; i++ {
		ok += strings.Repeat("  ", i+1) + "- layers:\n"
	}
	ok += strings.Repeat("  ", 17) + `- {type: rect, color: "#ffffff", x: 1, y: 1, width: 5, height: 5}` + "\n"
	var children strings.Builder
	for k := 0; k < 9; k++ {
		fmt.Fprintf(&children, "      - {type: rect, color: \"#ff%02x00\", x: \"{{i * 8}}\", y: %d, width: 6, height: 6}\n", k*20, k*8)
	}
	items := func(n int) string {
		s := make([]string, n)
		for i := range s {
			s[i] = fmt.Sprintf("x%d", i)
		}
		return strings.Join(s, "|")
	}
	padded := strings.Repeat(" ", 4090) + "Zürich Hauptbahnhof Gleis 7"
	cases := []struct {
		name, layers, data string
	}{
		{"loop-layers-over-limit", "  - type: loop\n    value: \"{{v}}\"\n    max_items: 30\n    layers:\n" + children.String(),
			`{"v":"` + items(30) + `"}`},
		{"loop-layers-at-limit", "  - type: loop\n    value: \"{{v}}\"\n    max_items: 30\n    layers:\n" + children.String(),
			`{"v":"` + items(28) + `"}`},
		{"block-depth-over", nested, `{}`},
		{"block-depth-ok", ok, `{}`},
		{"max-items-capped", "  - type: loop\n    value: \"{{v}}\"\n    max_items: 500\n    layers:\n      - {type: rect, color: \"#ffffff\", x: \"{{i}}\", y: \"{{i / 2}}\", width: 1, height: 3}\n",
			`{"v":"` + items(250) + `"}`},
		{"rune-cap-wrap", `  - {type: text, value: "{{t}}", font: reg, size: 14, max_width: 60, color: "#ffffff", x: 4, y: 4}
`, `{"t":"` + padded + `"}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			writeTemplate(t, dir, c.name, c.layers)
			_, gerr := goRender(t, dir, c.name, []byte(c.data))
			r := loadDevice(t, dir, c.name)
			_, cerr := deviceRender(t, r, dir, c.name, []byte(c.data))
			t.Logf("go: %v; c++: %v", gerr, cerr)
			if gerr != nil {
				if !errors.Is(cerr, ErrTemplate) {
					t.Fatalf("Go: %v; C++: %v, want ErrTemplate", gerr, cerr)
				}
				return
			}
			compareNear(t, r, dir, c.name, []byte(c.data))
		})
	}
	// The cap really changes the wrapped result: without the padding the full text shows.
	r := loadDevice(t, dir, "rune-cap-wrap")
	capped, err := deviceRender(t, r, dir, "rune-cap-wrap", []byte(`{"t":"`+padded+`"}`))
	if err != nil {
		t.Fatal(err)
	}
	full, err := deviceRender(t, r, dir, "rune-cap-wrap", []byte(`{"t":"Zürich Hauptbahnhof Gleis 7"}`))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(capped.Pix, full.Pix) {
		t.Error("4096-rune cap had no effect on the wrapped text")
	}
}
