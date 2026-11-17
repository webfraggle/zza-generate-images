package devicerender

import (
	"encoding/json"
	"errors"
	"image"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/webfraggle/zza-generate-images/internal/devexport"
	"github.com/webfraggle/zza-generate-images/internal/renderer"
)

// fuzzTarget is one shipped template loaded into the core for FuzzEvalData.
type fuzzTarget struct {
	name     string
	r        *Renderer
	tmpl     *renderer.Template
	animated bool
}

// jsonDepth returns the nesting depth of a valid JSON document.
func jsonDepth(v any) int {
	d := 0
	switch x := v.(type) {
	case map[string]any:
		for _, e := range x {
			d = max(d, jsonDepth(e))
		}
		return d + 1
	case []any:
		for _, e := range x {
			d = max(d, jsonDepth(e))
		}
		return d + 1
	}
	return 0
}

// FuzzEvalData feeds random train data to every exportable template in
// templates/: still image and animation must never crash. The data verdict is
// Go's (json.Unmarshal into a map, as the server does): invalid data gives
// ErrData; valid data renders, or fails with ErrTemplate only where Go's
// renderer fails too. Empty data means {} on the device (documented API) and
// device limits give ErrData: > 256 KB (R7) and nesting deeper than 32.
func FuzzEvalData(f *testing.F) {
	ents, err := os.ReadDir(templatesRoot)
	if err != nil {
		f.Fatal(err)
	}
	var targets []fuzzTarget
	for _, e := range ents {
		if !e.IsDir() {
			continue
		}
		p, err := devexport.Export(templatesRoot, e.Name())
		var inc *devexport.Incompatible
		if errors.As(err, &inc) {
			continue
		}
		if err != nil {
			f.Fatal(err)
		}
		tmpl, err := renderer.LoadTemplate(templatesRoot, e.Name())
		if err != nil {
			f.Fatal(err)
		}
		r := New(32 << 20)
		f.Cleanup(r.Close)
		if err := r.Load(p); err != nil {
			f.Fatal(err)
		}
		def, err := os.ReadFile(filepath.Join(templatesRoot, e.Name(), "default.json"))
		if err == nil {
			f.Add(def)
		}
		targets = append(targets, fuzzTarget{e.Name(), r, tmpl, hasFeature(p, "frames") || hasFeature(p, "splitflap")})
	}
	if len(targets) == 0 {
		f.Fatal("no exportable template")
	}
	for _, s := range []string{`{}`, `null`, `[]`, `{"zug1":{"zeit":"25:99","abw":"x"}}`, `{"zug1":{"vonnach":"€😀 Grüß","abw":1e308}}`,
		`{"gleis":[1,2,{"a":null}],"zug1":true}`, `{"a":1e999}`, `{"a":"x"} trailing`, `{a:1}`, `{"a":'x'}`, "{\"a\":\"\x01\"}", `{"zug1":{"zeit":"10:15","via":"` + "\xff\xfe" + `"}}`,
		`{"zug1":{"nr":"S1","zeit":"9:5","abw":-7.5,"hinweis":"*{x}|||"}}`, `{"a":[[[[[[[[[[[[[[[[[[[[[[[[[[[[[[[[[[1]]]]]]]]]]]]]]]]]]]]]]]]]]]]]]]]]]}`, `{"a":01}`, `{"a":NaN}`, `{"a":-}`} {
		f.Add([]byte(s))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		var m map[string]any
		goErr := json.Unmarshal(data, &m)
		var v any
		deep := json.Unmarshal(data, &v) == nil && jsonDepth(v) > 32
		limit := len(data) > 256<<10 || deep
		for _, tg := range targets {
			now := time.Date(2026, 1, 1, 10, 8, 37, 0, time.UTC)
			_, err := tg.r.Render(data, now)
			check := func(what string, err error) {
				t.Helper()
				switch {
				case len(data) == 0:
					if err != nil {
						t.Errorf("%s %s: empty data: %v", tg.name, what, err)
					}
				case goErr != nil:
					if !errors.Is(err, ErrData) {
						t.Errorf("%s %s: Go rejects the data (%v), C++: %v", tg.name, what, goErr, err)
					}
				case limit:
					if err != nil && !errors.Is(err, ErrData) {
						t.Errorf("%s %s: data at the device limit: %v", tg.name, what, err)
					}
				case err == nil:
				case errors.Is(err, ErrTemplate):
					if _, gerr := renderer.NewWithClock(templatesRoot, func() time.Time { return now }).Render(tg.tmpl, m); gerr == nil {
						t.Errorf("%s %s: C++ %v, Go renders", tg.name, what, err)
					}
				default:
					t.Errorf("%s %s: valid data: %v", tg.name, what, err)
				}
			}
			check("still", err)
			if tg.animated {
				err := tg.r.AnimFrames(data, now, func(int, *image.NRGBA, int, []image.Rectangle) error { return nil })
				check("animation", err)
			}
		}
	})
}

// hasFeature reports whether the package manifest lists feature.
func hasFeature(p *devexport.Package, feature string) bool {
	var man struct {
		Features []string `json:"features"`
	}
	if err := json.Unmarshal(p.Files["manifest.json"], &man); err != nil {
		return false
	}
	for _, f := range man.Features {
		if f == feature {
			return true
		}
	}
	return false
}
