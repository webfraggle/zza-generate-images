package devicerender

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/webfraggle/zza-generate-images/internal/devexport"
	"github.com/webfraggle/zza-generate-images/internal/renderer"
)

// workMargin: every legitimate template must stay below defaultWorkLimit / workMargin.
const workMargin = 4

// callWork is the largest work of one core call for a template and data set.
type callWork struct {
	still, begin, frame uint64
	frames              int
}

func (w callWork) max() uint64 { return max(w.still, w.begin, w.frame) }

// measureWork renders the still image and the complete animation (zza_anim_begin
// and every zza_anim_frame) and returns the work of the most expensive call of
// each kind. Errors are returned as they come (templates that Go rejects too).
func measureWork(r *Renderer, data []byte, now time.Time) (callWork, error) {
	var w callWork
	if _, err := r.Render(data, now); err != nil {
		return w, err
	}
	w.still = testWorkUsed(r)
	code, frames := testAnimBegin(r, data, toZza(now))
	if code != 0 {
		return w, testErr(r, code)
	}
	defer testAnimEnd(r)
	w.begin = testWorkUsed(r)
	w.frames = frames
	for i := 0; i < frames; i++ {
		if code := testAnimFrame(r, i); code != 0 {
			return w, fmt.Errorf("frame %d: %w", i, testErr(r, code))
		}
		w.frame = max(w.frame, testWorkUsed(r))
	}
	return w, nil
}

// genWork collects the largest work per call kind over the generated templates.
var genWork callWork

func recordGenWork(w callWork) {
	genWork.still = max(genWork.still, w.still)
	genWork.begin = max(genWork.begin, w.begin)
	genWork.frame = max(genWork.frame, w.frame)
}

// TestWorkLimitMargin: every shipped, cov-* and worst-* template (default,
// empty and long data) stays far below the default work limit. The numbers
// are logged; they are the basis of ZZA_DEFAULT_WORK_LIMIT (spec §13).
func TestWorkLimitMargin(t *testing.T) {
	type tc struct{ dir, name string }
	var cases []tc
	for _, root := range []string{templatesRoot, covRoot} {
		ents, err := os.ReadDir(root)
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range ents {
			if e.IsDir() {
				cases = append(cases, tc{root, e.Name()})
			}
		}
	}
	var lines []string
	var overall uint64
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tmpl, err := renderer.LoadTemplate(c.dir, c.name)
			if err != nil {
				t.Fatal(err)
			}
			r := New(32 << 20)
			defer r.Close()
			p, err := devexport.Export(c.dir, c.name)
			if err != nil {
				t.Fatalf("Export: %v", err)
			}
			if err := r.Load(p); err != nil {
				t.Fatal(err)
			}
			for ds, data := range covDatasets(t, readDefault(t, c.dir, c.name)) {
				w, err := measureWork(r, data, fixedNow(t, tmpl))
				if err != nil {
					t.Fatalf("%s: %v", ds, err)
				}
				lines = append(lines, fmt.Sprintf("%-22s %-7s still %11d  anim_begin %11d  frame %11d (%d frames)",
					c.name, ds, w.still, w.begin, w.frame, w.frames))
				overall = max(overall, w.max())
				if w.max()*workMargin > defaultWorkLimit {
					t.Errorf("%s: work %d, more than 1/%d of the default limit %d", ds, w.max(), workMargin, defaultWorkLimit)
				}
			}
		})
	}
	sort.Strings(lines)
	t.Logf("work per call (default limit %d):\n%s\nmaximum %d", defaultWorkLimit, strings.Join(lines, "\n"), overall)
}

// hostilePackage builds the PoC of the security review by hand: one opaque
// 240×240 '?' glyph with advance 0 and 256 text layers of 4096 '?' each,
// all behind if: isEmpty(cheap). One render of it used to take minutes.
func hostilePackage(t *testing.T) *devexport.Package {
	t.Helper()
	const side = 240
	fnt := []byte("ZF1\x00")
	fnt = binary.LittleEndian.AppendUint16(fnt, 200)    // ascent
	fnt = binary.LittleEndian.AppendUint16(fnt, 40)     // descent
	fnt = binary.LittleEndian.AppendUint16(fnt, 240)    // height
	fnt = binary.LittleEndian.AppendUint16(fnt, 1)      // glyphs
	fnt = binary.LittleEndian.AppendUint32(fnt, 0)      // kerning pairs
	fnt = append(fnt, '?')                              // cp
	fnt = binary.LittleEndian.AppendUint32(fnt, 0)      // advance 0: every glyph on the same spot
	fnt = binary.LittleEndian.AppendUint16(fnt, 0)      // bx
	fnt = binary.LittleEndian.AppendUint16(fnt, 0xFF38) // by = -200
	fnt = binary.LittleEndian.AppendUint16(fnt, side)
	fnt = binary.LittleEndian.AppendUint16(fnt, side)
	fnt = binary.LittleEndian.AppendUint32(fnt, 0)
	fnt = append(fnt, []byte(strings.Repeat("\xff", side*side))...)

	layer := map[string]any{"type": "text", "_f": "f0", "font": "f", "value": strings.Repeat("?", 4096),
		"color": "#ffffff", "x": 0, "y": 0, "if": "isEmpty(cheap)"}
	layers := make([]any, 256)
	for i := range layers {
		layers[i] = layer
	}
	tj, err := json.Marshal(map[string]any{"meta": map[string]any{"name": "poc"}, "layers": layers})
	if err != nil {
		t.Fatal(err)
	}
	return rawPackage(t, map[string][]byte{"f0.fnt": fnt, "t.json": tj})
}

// rawPackage builds a package from the given files plus a matching manifest.
func rawPackage(t *testing.T, files map[string][]byte) *devexport.Package {
	t.Helper()
	type entry struct {
		N string `json:"n"`
		S int    `json:"s"`
		H string `json:"h"`
	}
	names := make([]string, 0, len(files))
	for n := range files {
		names = append(names, n)
	}
	sort.Strings(names)
	var list []entry
	for _, n := range names {
		h := sha256.Sum256(files[n])
		list = append(list, entry{n, len(files[n]), hex.EncodeToString(h[:])})
	}
	man, err := json.Marshal(map[string]any{"format": 1, "name": "raw", "canvas": []int{240, 240}, "files": list})
	if err != nil {
		t.Fatal(err)
	}
	files["manifest.json"] = man
	return &devexport.Package{Name: "raw", Files: files}
}

// TestWorkLimitHostile: the PoC package fails with ErrTemplate "Rechenaufwand
// zu hoch" in bounded time (still image and animation), leaks nothing, and the
// renderer stays usable.
func TestWorkLimitHostile(t *testing.T) {
	p := hostilePackage(t)
	if n := len(p.Files["t.json"]) + len(p.Files["f0.fnt"]); n > 3<<20 {
		t.Fatalf("PoC package %d bytes, over the package limit", n)
	}
	r := loadPkg(t, p)
	loaded := r.MemUsed()
	now := time.Date(2026, 1, 1, 10, 8, 37, 0, time.UTC)

	start := time.Now()
	_, err := r.Render([]byte(`{}`), now)
	d := time.Since(start)
	t.Logf("hostile render: %v after %v (work %d)", err, d, testWorkUsed(r))
	if !errors.Is(err, ErrTemplate) || !strings.Contains(err.Error(), "Rechenaufwand zu hoch") {
		t.Fatalf("err = %v, want ErrTemplate Rechenaufwand zu hoch", err)
	}
	if d > 5*time.Second {
		t.Errorf("hostile render took %v", d)
	}
	if testWorkUsed(r) > defaultWorkLimit {
		t.Errorf("work %d over the limit", testWorkUsed(r))
	}
	err = r.AnimFrames([]byte(`{}`), now, func(int, *image.NRGBA, int, []image.Rectangle) error { return nil })
	if !errors.Is(err, ErrTemplate) || !strings.Contains(err.Error(), "Rechenaufwand zu hoch") {
		t.Fatalf("animation: %v, want Rechenaufwand zu hoch", err)
	}
	if r.MemUsed() != loaded {
		t.Errorf("MemUsed %d after the errors, %d after load", r.MemUsed(), loaded)
	}
	// Still usable: the layers are skipped with this data.
	if _, err := r.Render([]byte(`{"cheap":"x"}`), now); err != nil {
		t.Fatalf("render after the limit: %v", err)
	}
}

// TestWorkLimitSetting: zza_set_work_limit applies per call, survives Load and
// 0 restores the default.
func TestWorkLimitSetting(t *testing.T) {
	p, err := devexport.Export(templatesRoot, "sbb-105-v1")
	if err != nil {
		t.Fatal(err)
	}
	tmpl, err := renderer.LoadTemplate(templatesRoot, "sbb-105-v1")
	if err != nil {
		t.Fatal(err)
	}
	now := fixedNow(t, tmpl)
	data := readDefault(t, templatesRoot, "sbb-105-v1")
	r := New(32 << 20)
	defer r.Close()
	testSetWorkLimit(r, 1000) // before Load: kept
	if err := r.Load(p); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Render(data, now); !errors.Is(err, ErrTemplate) || !strings.Contains(err.Error(), "Rechenaufwand zu hoch") {
		t.Fatalf("limit 1000: %v", err)
	}
	if testWorkUsed(r) > 1000 {
		t.Errorf("work %d over the limit 1000", testWorkUsed(r))
	}
	testSetWorkLimit(r, 0)
	if _, err := r.Render(data, now); err != nil {
		t.Fatalf("default limit: %v", err)
	}
	used := testWorkUsed(r)
	if used == 0 {
		t.Fatal("no work counted")
	}
	testSetWorkLimit(r, used) // exactly enough
	if _, err := r.Render(data, now); err != nil {
		t.Fatalf("limit = work: %v", err)
	}
	testSetWorkLimit(r, used-1)
	if _, err := r.Render(data, now); !errors.Is(err, ErrTemplate) {
		t.Fatalf("limit = work-1: %v", err)
	}
}

// TestInterpolateCap: one interpolation produces at most 1 MiB (Go has no
// such cap, spec §13); four references to 256 KiB still fit.
func TestInterpolateCap(t *testing.T) {
	a := strings.Repeat("x", 256<<10)
	data := `{"a":"` + a + `"}`
	now := toZza(time.Date(2026, 1, 1, 10, 8, 37, 0, time.UTC))
	four := strings.Repeat("{{a}}", 4)
	if got, rc := testEval(data, "", now, 0, 1, 0, four, false); rc != 0 || len(got) != 1<<20 {
		t.Fatalf("4 × 256 KiB: %d bytes, code %d", len(got), rc)
	}
	for _, expr := range []string{four + "y", "y" + four, four + "{{a | prefix('y')}}", strings.Repeat("{{a}}", 1000)} {
		if _, rc := testEval(data, "", now, 0, 1, 0, expr, false); rc != templateCode {
			t.Errorf("%.30s…: code %d, want %d", expr, rc, templateCode)
		}
	}

	// In a render: ErrTemplate.
	dir := t.TempDir()
	writeTemplate(t, dir, "cap", "  - {type: text, font: reg, x: 0, y: 0, color: \"#fff\", value: \""+strings.Repeat("{{a}}", 5)+"\"}\n")
	r := loadDevice(t, dir, "cap")
	d := []byte(`{"a":"` + strings.Repeat("x", 250<<10) + `"}`)
	if _, err := r.Render(d, time.Date(2026, 1, 1, 10, 8, 37, 0, time.UTC)); !errors.Is(err, ErrTemplate) ||
		!strings.Contains(err.Error(), "1048576") {
		t.Errorf("render: %v", err)
	}
	if b, err := os.ReadFile(filepath.Join(dir, "cap", "template.yaml")); err != nil || len(b) == 0 {
		t.Fatal(err)
	}
}
