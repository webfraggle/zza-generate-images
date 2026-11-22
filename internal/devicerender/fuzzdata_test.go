package devicerender

import (
	"encoding/json"
	"errors"
	"image"
	"math/big"
	"os"
	"path/filepath"
	"strings"
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
	for _, n := range longNumbers() {
		f.Add([]byte(`{"zug1":{"abw":` + n + `,"zeit":"10:15"},"gleis":` + n + `}`))
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

// exactDecimal returns the exact decimal expansion of m × 2^-e (e > 0).
func exactDecimal(m int64, e uint) string {
	// m × 2^-e = m × 5^e / 10^e
	n := new(big.Int).Mul(big.NewInt(m), new(big.Int).Exp(big.NewInt(5), big.NewInt(int64(e)), nil))
	d := n.String()
	if len(d) <= int(e) {
		d = strings.Repeat("0", int(e)-len(d)+1) + d
	}
	return d[:len(d)-int(e)] + "." + d[len(d)-int(e):]
}

// longNumbers returns valid JSON numbers longer than ArduinoJson's 63-character
// buffer: long mantissas, many leading zeros, long exponents, halfway cases
// whose rounding depends on a digit beyond 800 significant digits, and values
// at the float64 overflow threshold.
func longNumbers() []string {
	threshold := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 1024), new(big.Int).Lsh(big.NewInt(1), 970)) // between DBL_MAX and 2^1024
	one := big.NewInt(1)
	tie := exactDecimal(5, 1075) // 2.5 × 2^-1074: halfway between two subnormals
	return []string{
		"0.0000000000000000000000000000000000000000000000000000000000000000001",
		"-0." + strings.Repeat("0", 400) + "1234567890123456789",
		"123456789012345678901234567890123456789012345678901234567890123456789.5",
		"1e00000000000000000000000000000000000000000000000000000000000000000000000308",
		"1E-0000000000000000000000000000000000000000000000000000000000000000000000005",
		"1e+000000000000000000000000000000000000000000000000000000000000000000000309",
		"9007199254740993." + strings.Repeat("0", 100) + "1", // just above a tie: rounds up
		"9007199254740993." + strings.Repeat("0", 100),       // the tie: rounds to even
		tie,
		tie + strings.Repeat("0", 1200) + "1", // sticky digit far beyond 800 significant digits
		"-" + tie + strings.Repeat("0", 600) + "7",
		strings.Repeat("9", 70) + "e-60",
		new(big.Int).Sub(threshold, one).String(),                                  // largest finite value
		threshold.String(),                                                         // tie: rounds to 2^1024, out of range
		new(big.Int).Sub(threshold, one).String() + "." + strings.Repeat("9", 500), // still below
		"0." + strings.Repeat("0", 330) + "1e639",
		strings.Repeat("1", 400) + "e-400",
	}
}

// TestDataLongNumbers: numbers longer than 63 characters are accepted or
// rejected like Go's json.Unmarshal and have exactly Go's float64 value.
func TestDataLongNumbers(t *testing.T) {
	r := loadDevice(t, covRoot, "cov-text")
	now := time.Date(2026, 1, 1, 10, 8, 37, 0, time.UTC)
	for i, n := range longNumbers() {
		data := `{"a":` + n + `}`
		var m map[string]any
		goErr := json.Unmarshal([]byte(data), &m)
		_, err := r.Render([]byte(data), now)
		if (goErr != nil) != errors.Is(err, ErrData) || (goErr == nil && err != nil) {
			t.Errorf("case %d (%.40s…): Go %v, C++ %v", i, n, goErr, err)
			continue
		}
		if goErr != nil {
			continue
		}
		for _, expr := range []string{"{{a}}", "{{a | mul(1)}}"} {
			want := renderer.EvalForTest(m, now, 0, 1, 0, expr, false)
			got, rc := testEval(data, "", toZza(now), 0, 1, 0, expr, false)
			if rc != 0 || got != want {
				t.Errorf("case %d (%.40s…) %s: go %q, c++ %q (code %d)", i, n, expr, want, got, rc)
			}
		}
	}
}

// TestDataHugeNumber: a 200 KB number is decided like Go in bounded time
// (linear scan, bounded strtod).
func TestDataHugeNumber(t *testing.T) {
	r := loadDevice(t, covRoot, "cov-text")
	now := time.Date(2026, 1, 1, 10, 8, 37, 0, time.UTC)
	digits := strings.Repeat("7", 200<<10)
	for name, n := range map[string]string{
		"tiny":       "0." + strings.Repeat("0", 200<<10) + "1",
		"huge":       digits,
		"huge-scale": "0." + digits + "e2000",
		"near-max":   "1." + digits + "e308",
		"fraction":   "1." + digits,
	} {
		data := []byte(`{"a":` + n + `}`)
		var m map[string]any
		goErr := json.Unmarshal(data, &m)
		start := time.Now()
		_, err := r.Render(data, now)
		d := time.Since(start)
		t.Logf("%s: %d bytes, Go %v, C++ %v in %v", name, len(data), goErr != nil, err, d.Round(time.Microsecond))
		if (goErr != nil) != errors.Is(err, ErrData) || (goErr == nil && err != nil) {
			t.Errorf("%s: Go %v, C++ %v", name, goErr, err)
		}
		if goErr == nil {
			want := renderer.EvalForTest(m, now, 0, 1, 0, "{{a}}", false)
			got, rc := testEval(string(data), "", toZza(now), 0, 1, 0, "{{a}}", false)
			if rc != 0 || got != want {
				t.Errorf("%s: go %q, c++ %q (code %d)", name, want, got, rc)
			}
		}
	}

}

// TestParseFloatLongStrings: numeric strings (not JSON numbers) of any length
// that reach a filter (mul, greaterThan, …) parse like Go's strconv.ParseFloat:
// long decimal strings go through the same bounded canonical form as the data
// parser, in Go syntax (sign, leading/trailing point, leading zeros).
func TestParseFloatLongStrings(t *testing.T) {
	now := time.Date(2026, 1, 1, 10, 8, 37, 0, time.UTC)
	digits := strings.Repeat("7", 200<<10)
	cases := append(longNumbers(),
		"+"+strings.Repeat("1", 70),
		"."+strings.Repeat("0", 100)+"5",
		strings.Repeat("3", 80)+".",
		strings.Repeat("3", 80)+".e-70",
		"000"+strings.Repeat("0", 80)+"42",
		"-"+strings.Repeat("0", 100)+".0",
		"0x1"+strings.Repeat("0", 70)+"p-200",
		strings.Repeat("1", 70)+"x", // invalid
		strings.Repeat("1", 70)+"e", // invalid
		digits, "0."+digits+"e2000", "1."+digits+"e308", "0."+strings.Repeat("0", 200<<10)+"1",
	)
	for i, n := range cases {
		data := `{"a":"` + n + `"}`
		var m map[string]any
		if err := json.Unmarshal([]byte(data), &m); err != nil {
			t.Fatal(err)
		}
		for _, expr := range []string{"{{a | mul(1)}}", "{{a | add(0)}}"} {
			start := time.Now()
			want := renderer.EvalForTest(m, now, 0, 1, 0, expr, false)
			got, rc := testEval(data, "", toZza(now), 0, 1, 0, expr, false)
			if rc != 0 || got != want {
				t.Errorf("case %d (%.40s…) %s: go %.60q, c++ %.60q (code %d)", i, n, expr, want, got, rc)
			}
			if d := time.Since(start); d > time.Second {
				t.Errorf("case %d: %v", i, d)
			}
		}
		cond := "greaterThan(a, '0')"
		want := renderer.EvalForTest(m, now, 0, 1, 0, cond, true)
		if got, rc := testEval(data, "", toZza(now), 0, 1, 0, cond, true); rc != 0 || got != want {
			t.Errorf("case %d (%.40s…) %s: go %q, c++ %q", i, n, cond, want, got)
		}
	}
}
