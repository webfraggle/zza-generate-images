package devicerender

import (
	"bytes"
	"encoding/json"
	"errors"
	"image"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/webfraggle/zza-generate-images/internal/devexport"
	"github.com/webfraggle/zza-generate-images/internal/renderer"
)

// goAnim renders the animation with the Go reference renderer at the fixed
// time, seeded like the server (AnimationSeed of the template name and body).
func goAnim(t *testing.T, dir, name string, data []byte) (*renderer.Animation, error) {
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
	r := renderer.NewWithClock(dir, func() time.Time { return now })
	return r.RenderAnimation(tmpl, m, renderer.AnimationSeed(name, data))
}

// deviceAnim renders the animation with the C++ core at the fixed time.
func deviceAnim(t *testing.T, r *Renderer, dir, name string, data []byte) (*renderer.Animation, error) {
	t.Helper()
	tmpl, err := renderer.LoadTemplate(dir, name)
	if err != nil {
		t.Fatalf("LoadTemplate: %v", err)
	}
	return r.RenderAnimation(data, fixedNow(t, tmpl))
}

// withData returns def (a JSON object) with the given fields replaced.
func withData(t *testing.T, def []byte, kv map[string]any) []byte {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(def, &m); err != nil {
		t.Fatal(err)
	}
	for k, v := range kv {
		m[k] = v
	}
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// compareAnim asserts that Go and C++ give the same frame count, FPS, Delay
// and Loop, and near frames (the given indices; nil = all). The C++ frames are
// returned.
func compareAnim(t *testing.T, r *Renderer, dir, name string, data []byte, idx func(n int) []int) *renderer.Animation {
	t.Helper()
	g, err := goAnim(t, dir, name, data)
	if err != nil {
		t.Fatalf("Go: %v", err)
	}
	c, err := deviceAnim(t, r, dir, name, data)
	if err != nil {
		t.Fatalf("C++: %v", err)
	}
	if len(c.Frames) != len(g.Frames) || c.FPS != g.FPS || c.Delay != g.Delay || c.Loop != g.Loop {
		t.Fatalf("C++ %d frames FPS %d Delay %d Loop %v; Go %d frames FPS %d Delay %d Loop %v",
			len(c.Frames), c.FPS, c.Delay, c.Loop, len(g.Frames), g.FPS, g.Delay, g.Loop)
	}
	var list []int
	if idx == nil {
		for i := range g.Frames {
			list = append(list, i)
		}
	} else {
		list = idx(len(g.Frames))
	}
	for _, i := range list {
		if msg := nearCheck(t, g.Frames[i], c.Frames[i]); msg != "" {
			t.Errorf("frame %d/%d: %s", i, len(g.Frames), msg)
		}
	}
	return c
}

type animCase struct{ dir, name string }

var flapCases = []animCase{{templatesRoot, "faltblatt-105-v1"}, {covRoot, "cov-flap"}}

func TestFlapFramesNear(t *testing.T) {
	for _, tc := range flapCases {
		t.Run(tc.name, func(t *testing.T) {
			r := loadDevice(t, tc.dir, tc.name)
			def := readDefault(t, tc.dir, tc.name)
			sets := map[string][]byte{"default": def}
			if tc.name == "cov-flap" {
				sets = covDatasets(t, def)
				sets["other"] = withData(t, def, map[string]any{"zug": "S", "gleis": "12", "ziel": "Basel SBB"})
			}
			for ds, data := range sets {
				t.Run(ds, func(t *testing.T) {
					c := compareAnim(t, r, tc.dir, tc.name, data, nil)
					if len(c.Frames) < 3 {
						t.Fatalf("only %d frames", len(c.Frames))
					}
					still, err := goRender(t, tc.dir, tc.name, data)
					if err != nil {
						t.Fatal(err)
					}
					last := c.Frames[len(c.Frames)-1]
					assertNear(t, still, last)
					dstill, err := deviceRender(t, r, tc.dir, tc.name, data)
					if err != nil {
						t.Fatal(err)
					}
					if !bytes.Equal(dstill.Pix, last.Pix) {
						t.Error("last frame differs from the C++ still image")
					}
				})
			}
		})
	}
}

func TestFlapDeterministic(t *testing.T) {
	for _, tc := range flapCases {
		t.Run(tc.name, func(t *testing.T) {
			r := loadDevice(t, tc.dir, tc.name)
			data := readDefault(t, tc.dir, tc.name)
			a, err := deviceAnim(t, r, tc.dir, tc.name, data)
			if err != nil {
				t.Fatal(err)
			}
			b, err := deviceAnim(t, loadDevice(t, tc.dir, tc.name), tc.dir, tc.name, data)
			if err != nil {
				t.Fatal(err)
			}
			if len(a.Frames) != len(b.Frames) {
				t.Fatalf("%d vs %d frames", len(a.Frames), len(b.Frames))
			}
			for i := range a.Frames {
				if !bytes.Equal(a.Frames[i].Pix, b.Frames[i].Pix) {
					t.Fatalf("frame %d differs", i)
				}
			}
			// The seed covers the body bytes: other whitespace, other run.
			c, err := deviceAnim(t, r, tc.dir, tc.name, append([]byte(" "), data...))
			if err != nil {
				t.Fatal(err)
			}
			g, err := goAnim(t, tc.dir, tc.name, append([]byte(" "), data...))
			if err != nil {
				t.Fatal(err)
			}
			if len(c.Frames) != len(g.Frames) {
				t.Fatalf("other body: C++ %d frames, Go %d", len(c.Frames), len(g.Frames))
			}
		})
	}
}

// copyTemplate copies a template directory to dir/name, applying the
// replacements to template.yaml.
func copyTemplate(t *testing.T, srcDir, srcName, dir, name string, repl ...string) {
	t.Helper()
	src := filepath.Join(srcDir, srcName)
	dst := filepath.Join(dir, name)
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	ents, err := os.ReadDir(src)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range ents {
		b, err := os.ReadFile(filepath.Join(src, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if e.Name() == "template.yaml" {
			s := string(b)
			for i := 0; i+1 < len(repl); i += 2 {
				if !strings.Contains(s, repl[i]) {
					t.Fatalf("template.yaml has no %q", repl[i])
				}
				s = strings.Replace(s, repl[i], repl[i+1], 1)
			}
			b = []byte(s)
		}
		if err := os.WriteFile(filepath.Join(dst, e.Name()), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestFrameModeNear(t *testing.T) {
	tmp := t.TempDir()
	copyTemplate(t, covRoot, "cov-frames", tmp, "cov-frames-slow", "frame_delay: 0.02", "frame_delay: 10", "loop: false", "loop: true")
	cases := []animCase{
		{templatesRoot, "umuc-105-v1"}, {templatesRoot, "oebb-105-v1"}, {templatesRoot, "nederland-105-v1"},
		{covRoot, "cov-frames"}, {tmp, "cov-frames-slow"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := loadDevice(t, tc.dir, tc.name)
			data := readDefault(t, tc.dir, tc.name)
			c := compareAnim(t, r, tc.dir, tc.name, data, func(n int) []int { return []int{0, 1, n / 2, n - 1} })
			still, err := deviceRender(t, r, tc.dir, tc.name, data)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(still.Pix, c.Frames[0].Pix) {
				t.Error("frame 0 differs from the C++ still image")
			}
			if bytes.Equal(c.Frames[0].Pix, c.Frames[1].Pix) {
				t.Error("frames 0 and 1 are equal")
			}
		})
	}
}

// checkDirty renders the animation with maxRects rectangles per frame and
// checks: frame 0 reports the whole canvas; for every later frame the pixels
// outside the reported rectangles equal the previous frame. It returns the
// changed area summed over frames 1… (in pixels) and the frames.
func checkDirty(t *testing.T, r *Renderer, data []byte, now time.Time, maxRects int) (int, []*image.NRGBA) {
	t.Helper()
	full := image.Rect(0, 0, canvasW, canvasH)
	var frames []*image.NRGBA
	area := 0
	_, err := r.animFrames(data, now, maxRects, func(i int, img *image.NRGBA, _ int, dirty []image.Rectangle) error {
		if len(dirty) > maxRects {
			t.Errorf("frame %d: %d rectangles, max %d", i, len(dirty), maxRects)
		}
		for _, d := range dirty {
			if d.Empty() || !d.In(full) {
				t.Errorf("frame %d: rectangle %v empty or outside the canvas", i, d)
			}
		}
		if i == 0 {
			if maxRects > 0 && (len(dirty) != 1 || dirty[0] != full) {
				t.Errorf("frame 0: dirty %v, want the whole canvas", dirty)
			}
		} else if maxRects > 0 { // without rectangles only the frames are checked
			prev := frames[i-1]
			for _, d := range dirty {
				area += d.Dx() * d.Dy()
			}
			for y := 0; y < canvasH; y++ {
				for x := 0; x < canvasW; x++ {
					in := false
					for _, d := range dirty {
						if (image.Point{x, y}).In(d) {
							in = true
							break
						}
					}
					o := img.PixOffset(x, y)
					if !in && !bytes.Equal(img.Pix[o:o+4], prev.Pix[o:o+4]) {
						t.Fatalf("frame %d: pixel (%d,%d) changed outside %v", i, x, y, dirty)
					}
				}
			}
		}
		frames = append(frames, img)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return area, frames
}

func TestDirtyRects(t *testing.T) {
	cases := []animCase{
		{templatesRoot, "faltblatt-105-v1"}, {covRoot, "cov-flap"}, {covRoot, "cov-flap-elif"},
		{covRoot, "cov-frames"}, {templatesRoot, "umuc-105-v1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tmpl, err := renderer.LoadTemplate(tc.dir, tc.name)
			if err != nil {
				t.Fatal(err)
			}
			r := loadDevice(t, tc.dir, tc.name)
			data := readDefault(t, tc.dir, tc.name)
			now := fixedNow(t, tmpl)
			area, frames := checkDirty(t, r, data, now, maxDirtyRects)
			t.Logf("%d frames, changed area per frame %d px", len(frames), area/max(1, len(frames)-1))
			if n := len(frames) - 1; n > 0 && area >= n*canvasW*canvasH/2 {
				t.Errorf("dirty area %d over %d frames: not smaller than half the canvas per frame", area, n)
			}
			for _, m := range []int{1, 0} {
				_, fr := checkDirty(t, r, data, now, m)
				for i := range fr {
					if !bytes.Equal(fr[i].Pix, frames[i].Pix) {
						t.Fatalf("max_rects %d: frame %d differs", m, i)
					}
				}
			}
		})
	}
}

// TestPrefixRuleElif: the static prefix must not change the result — neither
// when the first splitflap continues an if chain (no prefix) nor when every
// splitflap is in an untaken branch (one frame, like Go).
func TestPrefixRuleElif(t *testing.T) {
	r := loadDevice(t, covRoot, "cov-flap-elif")
	def := readDefault(t, covRoot, "cov-flap-elif")
	for _, sel := range []string{"flap", "other", "text"} {
		t.Run(sel, func(t *testing.T) {
			compareAnim(t, r, covRoot, "cov-flap-elif", withData(t, def, map[string]any{"sel": sel}), nil)
		})
	}
	// The same template with the prefix disabled (the first splitflap block
	// continues an if chain) gives byte-equal frames.
	t.Run("prefix-off", func(t *testing.T) {
		tmp := t.TempDir()
		copyTemplate(t, covRoot, "cov-flap", tmp, "cov-flap-noprefix",
			`  - if: "not(equals(mode, 'none'))"`,
			"  - {if: \"false\", type: rect, color: \"#ffffff\", x: 0, y: 0, width: 5, height: 5}\n  - elif: \"not(equals(mode, 'none'))\"")
		data := readDefault(t, covRoot, "cov-flap")
		on, err := deviceAnim(t, loadDevice(t, covRoot, "cov-flap"), covRoot, "cov-flap", data)
		if err != nil {
			t.Fatal(err)
		}
		// Same seed: the package name must stay "cov-flap".
		p, err := devexport.Export(tmp, "cov-flap-noprefix")
		if err != nil {
			t.Fatal(err)
		}
		q := clonePkg(p)
		man := bytes.Replace(q.Files["manifest.json"], []byte(`"name":"cov-flap-noprefix"`), []byte(`"name":"cov-flap"`), 1)
		if bytes.Equal(man, q.Files["manifest.json"]) {
			t.Fatalf("manifest has no name: %s", man)
		}
		q.Files["manifest.json"] = man // the manifest is not hashed
		off, err := deviceAnim(t, loadPkg(t, q), tmp, "cov-flap-noprefix", data)
		if err != nil {
			t.Fatal(err)
		}
		if len(on.Frames) != len(off.Frames) {
			t.Fatalf("%d frames with prefix, %d without", len(on.Frames), len(off.Frames))
		}
		for i := range on.Frames {
			if !bytes.Equal(on.Frames[i].Pix, off.Frames[i].Pix) {
				t.Fatalf("frame %d differs with and without prefix", i)
			}
		}
	})
	t.Run("untaken", func(t *testing.T) {
		r := loadDevice(t, covRoot, "cov-flap")
		data := withData(t, readDefault(t, covRoot, "cov-flap"), map[string]any{"mode": "none"})
		c := compareAnim(t, r, covRoot, "cov-flap", data, nil)
		if len(c.Frames) != 1 {
			t.Fatalf("%d frames, want 1", len(c.Frames))
		}
	})
}

// TestAnimNoMem: too little memory for the animation gives ErrNoMem without
// leaking; the renderer stays usable for still images and, with enough
// memory, for the animation.
func TestAnimNoMem(t *testing.T) {
	for _, tc := range flapCases {
		t.Run(tc.name, func(t *testing.T) {
			p, err := devexport.Export(tc.dir, tc.name)
			if err != nil {
				t.Fatal(err)
			}
			data := readDefault(t, tc.dir, tc.name)
			probe := loadPkg(t, p)
			loaded := probe.MemUsed()
			want, err := deviceAnim(t, probe, tc.dir, tc.name, data)
			if err != nil {
				t.Fatal(err)
			}
			if probe.MemUsed() != loaded {
				t.Errorf("MemUsed after the animation %d, want %d (loaded)", probe.MemUsed(), loaded)
			}
			peak := probe.MemPeak()
			t.Logf("loaded %d, animation peak %d", loaded, peak)

			try := func(budget int) error {
				r := New(budget)
				defer r.Close()
				if err := r.Load(p); err != nil {
					t.Fatalf("Load with %d: %v", budget, err)
				}
				got, err := deviceAnim(t, r, tc.dir, tc.name, data)
				t.Logf("budget loaded+%d: %v", budget-loaded, err)
				if err != nil {
					if !errors.Is(err, ErrNoMem) {
						t.Fatalf("budget %d: %v, want ErrNoMem", budget, err)
					}
					if r.MemUsed() != loaded {
						t.Fatalf("budget %d: MemUsed after ErrNoMem %d, want %d", budget, r.MemUsed(), loaded)
					}
					if _, err := deviceRender(t, r, tc.dir, tc.name, data); err != nil && !errors.Is(err, ErrNoMem) {
						t.Fatalf("budget %d: Render after ErrNoMem: %v", budget, err)
					}
					return err
				}
				for i := range want.Frames {
					if !bytes.Equal(got.Frames[i].Pix, want.Frames[i].Pix) {
						t.Fatalf("budget %d: frame %d differs", budget, i)
					}
				}
				return nil
			}

			// Loading plus 50 KB: the animation does not fit, a still image does.
			r := New(loaded + 50<<10)
			defer r.Close()
			if err := r.Load(p); err != nil {
				t.Fatal(err)
			}
			if _, err := deviceAnim(t, r, tc.dir, tc.name, data); !errors.Is(err, ErrNoMem) {
				t.Fatalf("animation with loaded+50 KB: %v, want ErrNoMem", err)
			}
			if r.MemUsed() != loaded {
				t.Fatalf("MemUsed after ErrNoMem %d, want %d", r.MemUsed(), loaded)
			}
			compareNear(t, r, tc.dir, tc.name, data)

			// Budgets between loading and the peak fail cleanly or give the same frames.
			fails := 0
			for k := 1; k <= 12; k++ {
				if try(loaded+(peak-loaded)*k/13) != nil {
					fails++
				}
			}
			if fails == 0 {
				t.Error("no budget below the peak failed")
			}
			if err := try(peak); err != nil {
				t.Fatalf("animation with the peak budget: %v", err)
			}
		})
	}
}

// TestAnimFrameNoMem: ErrNoMem inside zza_anim_frame (the frame's own memory
// does not fit) ends the animation, frees all of it and leaves the renderer
// usable.
func TestAnimFrameNoMem(t *testing.T) {
	for _, name := range []string{"cov-flap", "cov-frames"} {
		t.Run(name, func(t *testing.T) {
			r := loadDevice(t, covRoot, name)
			loaded := r.MemUsed()
			data := readDefault(t, covRoot, name)
			now := zzaTime{2026, 1, 1, 10, 8, 37, 4}
			code, frames := testAnimBegin(r, data, now)
			if code != 0 || frames < 2 {
				t.Fatalf("begin: code %d, %d frames", code, frames)
			}
			testSetLimit(r, r.MemUsed()) // nothing more fits
			if code := testAnimFrame(r, 1); code != noMemCode {
				t.Fatalf("frame with a full budget: code %d, want ZZA_ERR_NOMEM", code)
			}
			if r.MemUsed() != loaded {
				t.Fatalf("MemUsed after ErrNoMem %d, want %d", r.MemUsed(), loaded)
			}
			if code := testAnimFrame(r, 1); code != errStateCode {
				t.Fatalf("frame after ErrNoMem: code %d, want ZZA_ERR_STATE (animation ended)", code)
			}
			testSetLimit(r, 32<<20)
			compareNear(t, r, covRoot, name, data)
			if _, err := deviceAnim(t, r, covRoot, name, data); err != nil {
				t.Fatalf("animation after ErrNoMem: %v", err)
			}
		})
	}
}

func TestAnimDataErrors(t *testing.T) {
	r := loadDevice(t, covRoot, "cov-flap")
	now := time.Now()
	for _, data := range []string{`{`, `[1,2]`, `"x"`} {
		if _, err := r.RenderAnimation([]byte(data), now); !errors.Is(err, ErrData) {
			t.Errorf("%q: %v, want ErrData", data, err)
		}
	}
	big := []byte(`{"a":"` + strings.Repeat("x", 256<<10) + `"}`)
	if _, err := r.RenderAnimation(big, now); !errors.Is(err, ErrData) {
		t.Errorf("big data: %v, want ErrData", err)
	}
	// No session: frames are refused; out-of-range frames too.
	if code := testAnimFrame(r, 0); code != errStateCode {
		t.Errorf("frame without begin: %d, want ZZA_ERR_STATE", code)
	}
	if _, err := r.RenderAnimation(readDefault(t, covRoot, "cov-flap"), now); err != nil {
		t.Fatal(err)
	}
	if code := testAnimFrame(r, 1000); code != errStateCode {
		t.Errorf("frame 1000: %d, want ZZA_ERR_STATE", code)
	}
	if code := testAnimFrame(r, -1); code != errStateCode {
		t.Errorf("frame -1: %d, want ZZA_ERR_STATE", code)
	}
	if _, err := New(1<<20).RenderAnimation([]byte(`{}`), now); !errors.Is(err, ErrState) {
		t.Errorf("no package: %v, want ErrState", err)
	}
	// A template without animation has one frame, held 1 s.
	s := loadDevice(t, covRoot, "cov-text")
	n := 0
	err := s.AnimFrames(readDefault(t, covRoot, "cov-text"), now, func(i int, _ *image.NRGBA, d int, _ []image.Rectangle) error {
		n++
		if d != 100 {
			t.Errorf("delay %d, want 100", d)
		}
		return nil
	})
	if err != nil || n != 1 {
		t.Errorf("cov-text: %d frames, %v", n, err)
	}
}

// TestFlapErrorsLikeGo: splitflap errors give ErrTemplate for the still image
// and the animation exactly when Go fails; otherwise the results are near.
func TestFlapErrorsLikeGo(t *testing.T) {
	dir := t.TempDir()
	withKids := func(attrs, kids string) string {
		return "  - {type: rect, color: \"#203040\", x: 0, y: 0, width: 240, height: 240}\n" +
			"  - type: splitflap\n    var: v\n" + attrs + "    layers:\n" + kids
	}
	kid := `      - {type: text, value: "{{v}}", font: reg, size: 12, color: "#ffffff", x: 2, y: 2}
`
	cases := map[string]string{
		"ok":             withKids("    x: 10\n    y: 10\n    width: 50\n    height: 20\n    value: \"{{a}}\"\n    reel: \"ABC\"\n", kid),
		"size-zero":      withKids("    x: 10\n    y: 10\n    width: 0\n    height: 20\n    value: \"A\"\n", kid),
		"cells-neg":      withKids("    x: 10\n    y: 10\n    width: 5\n    height: 20\n    cells: -1\n    split: chars\n    value: \"A\"\n", kid),
		"cells-65":       withKids("    x: 0\n    y: 10\n    width: 3\n    height: 20\n    cells: 65\n    split: chars\n    value: \"A\"\n", kid),
		"cells-no-split": withKids("    x: 10\n    y: 10\n    width: 5\n    height: 20\n    cells: 2\n    value: \"A\"\n", kid),
		"split-bad":      withKids("    x: 10\n    y: 10\n    width: 5\n    height: 20\n    split: words\n    value: \"A\"\n", kid),
		"reel-65":        withKids("    x: 10\n    y: 10\n    width: 5\n    height: 20\n    value: \"A\"\n    reel: \""+strings.Repeat("x", 65)+"\"\n", kid),
		"flip-7":         withKids("    x: 10\n    y: 10\n    width: 5\n    height: 20\n    value: \"A\"\n    flip_frames: 7\n", kid),
		"split-y":        withKids("    x: 10\n    y: 10\n    width: 5\n    height: 20\n    split_y: 20\n    value: \"A\"\n", kid),
		"outside":        withKids("    x: 230\n    y: 10\n    width: 20\n    height: 20\n    value: \"A\"\n", kid),
		"copy-child":     withKids("    x: 10\n    y: 10\n    width: 20\n    height: 20\n    value: \"A\"\n", "      - if: \"false\"\n        layers:\n          - {type: copy, src_x: 0, src_y: 0, src_width: 2, src_height: 2, x: 0, y: 0}\n"),
		"overlap": withKids("    x: 10\n    y: 10\n    width: 20\n    height: 20\n    value: \"A\"\n", kid) +
			"  - {type: splitflap, x: 25, y: 25, width: 20, height: 20, value: \"B\"}\n",
		"modules-total": withKids("    x: 0\n    y: 10\n    width: 3\n    height: 20\n    cells: 40\n    split: chars\n    value: \"A\"\n", kid) +
			"  - {type: splitflap, x: 0, y: 50, width: 3, height: 20, cells: 30, split: chars, value: \"B\"}\n",
		"in-loop": "  - type: loop\n    value: \"a|b\"\n    layers:\n      - {type: splitflap, x: 10, y: 10, width: 20, height: 20, value: \"B\"}\n",
		// Only a reel value's leaf fails: the still image renders, the animation does not.
		"reel-leaf-error": withKids("    x: 10\n    y: 10\n    width: 50\n    height: 20\n    value: \"A\"\n    reel: \"ABC\"\n",
			"      - type: rect\n        color: {if: \"equals(v, 'A')\", then: \"#ffffff\", else: \"bad\"}\n        x: 0\n        y: 0\n        width: 5\n        height: 5\n"),
	}
	for name, layers := range cases {
		t.Run(name, func(t *testing.T) {
			writeTemplate(t, dir, name, layers)
			data := []byte(`{"a":"B"}`)
			r := loadDevice(t, dir, name)
			_, gerr := goRender(t, dir, name, data)
			_, cerr := deviceRender(t, r, dir, name, data)
			if (gerr != nil) != (cerr != nil) || (cerr != nil && !errors.Is(cerr, ErrTemplate)) {
				t.Fatalf("still: Go %v; C++ %v", gerr, cerr)
			}
			if (name == "ok" || name == "reel-leaf-error") && gerr != nil {
				t.Fatalf("case is wrong: Go still fails: %v", gerr)
			}
			if gerr == nil {
				compareNear(t, r, dir, name, data)
			}
			_, gerr = goAnim(t, dir, name, data)
			_, cerr = deviceAnim(t, r, dir, name, data)
			if (gerr != nil) != (cerr != nil) || (cerr != nil && !errors.Is(cerr, ErrTemplate)) {
				t.Fatalf("animation: Go %v; C++ %v", gerr, cerr)
			}
			if name == "reel-leaf-error" && gerr == nil {
				t.Fatal("case is wrong: Go animation renders")
			}
			if gerr == nil {
				compareAnim(t, r, dir, name, data, nil)
			}
		})
	}
}

// clockFrames writes a frame-mode template with a seconds clock (60 frames of
// one second) into dir/name and returns it; the font comes from cov-frames.
func clockFrames(t *testing.T, dir, name string) {
	t.Helper()
	copyTemplate(t, covRoot, "cov-frames", dir, name)
	yaml := `meta:
  name: "Uhr"
  canvas: {width: 240, height: 240}
  animation:
    frames: 60
    frame_delay: 1
    loop: true

fonts:
  - id: reg
    file: NimbusSanL-Reg.otf

layers:
  - {type: rect, color: "#102030", x: 0, y: 0, width: 240, height: 240}
  - {type: text, value: "{{now | format('HH:mm:ss')}}", font: reg, size: 16, color: "#ffffff", x: 8, y: 100}
`
	if err := os.WriteFile(filepath.Join(dir, name, "template.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestAnimFrameNowParity: without now the core advances the clock per frame
// like the Go renderer (start + f*frame_delay, across midnight).
func TestAnimFrameNowParity(t *testing.T) {
	tmp := t.TempDir()
	clockFrames(t, tmp, "clock")
	start := time.Date(2026, 1, 1, 23, 59, 30, 0, tzBerlin(t)) // the template zone, see the fractional test
	tmpl, err := renderer.LoadTemplate(tmp, "clock")
	if err != nil {
		t.Fatal(err)
	}
	data := []byte("{}")
	g, err := renderer.NewWithClock(tmp, func() time.Time { return start }).RenderAnimation(tmpl, map[string]any{}, renderer.AnimationSeed("clock", data))
	if err != nil {
		t.Fatal(err)
	}
	c, err := loadDevice(t, tmp, "clock").RenderAnimation(data, start)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Frames) != 60 || len(g.Frames) != 60 {
		t.Fatalf("frames: C++ %d, Go %d", len(c.Frames), len(g.Frames))
	}
	for i := range g.Frames {
		if msg := nearCheck(t, g.Frames[i], c.Frames[i]); msg != "" {
			t.Errorf("frame %d: %s", i, msg)
		}
		if i > 0 && bytes.Equal(c.Frames[i].Pix, c.Frames[i-1].Pix) {
			t.Errorf("C++ frame %d equals frame %d (clock did not advance)", i, i-1)
		}
	}
}

func tzBerlin(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Fatal(err)
	}
	return loc
}

// TestAnimFrameNowParityFractionalDelay: with frame_delay 0.3 the core's rule
// floor(i*delay_cs/100) matches Go, including the carry into the date.
func TestAnimFrameNowParityFractionalDelay(t *testing.T) {
	tmp := t.TempDir()
	clockFrames(t, tmp, "clock")
	b, err := os.ReadFile(filepath.Join(tmp, "clock", "template.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	s := strings.Replace(string(b), "frame_delay: 1", "frame_delay: 0.3", 1)
	s = strings.Replace(s, "HH:mm:ss", "dd.MM. HH:mm:ss", 1)
	if err := os.WriteFile(filepath.Join(tmp, "clock", "template.yaml"), []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
	// In the template's zone (Berlin), like fixedNow: Go converts the start time
	// into it, the core takes wall-clock time as given.
	start := time.Date(2026, 1, 1, 23, 59, 50, 0, tzBerlin(t))
	tmpl, err := renderer.LoadTemplate(tmp, "clock")
	if err != nil {
		t.Fatal(err)
	}
	data := []byte("{}")
	g, err := renderer.NewWithClock(tmp, func() time.Time { return start }).RenderAnimation(tmpl, map[string]any{}, renderer.AnimationSeed("clock", data))
	if err != nil {
		t.Fatal(err)
	}
	c, err := loadDevice(t, tmp, "clock").RenderAnimation(data, start)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Frames) != 60 || len(g.Frames) != 60 {
		t.Fatalf("frames: C++ %d, Go %d", len(c.Frames), len(g.Frames))
	}
	for i := range g.Frames {
		if msg := nearCheck(t, g.Frames[i], c.Frames[i]); msg != "" {
			t.Errorf("frame %d: %s", i, msg)
		}
	}
	// nearCheck is loose for small text; pin the rule exactly: frame i is the
	// core's own still image at start + floor(i*30/100) seconds.
	r := loadDevice(t, tmp, "clock")
	for i := range c.Frames {
		want, err := r.Render(data, start.Add(time.Duration(i*30/100)*time.Second))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(c.Frames[i].Pix, want.Pix) {
			t.Errorf("frame %d is not the still image at start + floor(%d*0.3) s", i, i)
		}
	}
}

// TestAnimFrameNowOverride: a now passed to zza_anim_frame is the time of that
// frame, in frame mode and for a still.
func TestAnimFrameNowOverride(t *testing.T) {
	tmp := t.TempDir()
	clockFrames(t, tmp, "clock")
	copyTemplate(t, covRoot, "cov-frames", tmp, "still")
	still := `meta:
  name: "Standbild"
  canvas: {width: 240, height: 240}

fonts:
  - id: reg
    file: NimbusSanL-Reg.otf

layers:
  - {type: rect, color: "#102030", x: 0, y: 0, width: 240, height: 240}
  - {type: text, value: "{{now | format('HH:mm:ss')}}", font: reg, size: 16, color: "#ffffff", x: 8, y: 100}
`
	if err := os.WriteFile(filepath.Join(tmp, "still", "template.yaml"), []byte(still), 0o644); err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, 1, 1, 23, 59, 30, 0, time.UTC)
	noon := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	data := []byte("{}")
	for _, tc := range []struct {
		name string
		i    int
	}{{"clock", 5}, {"still", 0}} {
		t.Run(tc.name, func(t *testing.T) {
			r := loadDevice(t, tmp, tc.name)
			want, err := r.Render(data, noon)
			if err != nil {
				t.Fatal(err)
			}
			if code, _ := testAnimBegin(r, data, toZza(start)); code != 0 {
				t.Fatalf("anim begin: %d", code)
			}
			defer testAnimEnd(r)
			code, got := testAnimFrameAt(r, tc.i, &noon)
			if code != 0 {
				t.Fatalf("anim frame: %d", code)
			}
			if !bytes.Equal(got.Pix, want.Pix) {
				t.Error("frame with now differs from Render at that time")
			}
			_, without := testAnimFrameAt(r, tc.i, nil)
			if bytes.Equal(without.Pix, got.Pix) {
				t.Error("now had no effect")
			}
		})
	}
}

// TestAnimFrameNowIgnoredForSplitflap: split-flap stays frozen at the start time.
func TestAnimFrameNowIgnoredForSplitflap(t *testing.T) {
	r := loadDevice(t, covRoot, "cov-flap")
	data := readDefault(t, covRoot, "cov-flap")
	start := time.Date(2026, 1, 1, 10, 8, 37, 0, time.UTC)
	other := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	code, frames := testAnimBegin(r, data, toZza(start))
	if code != 0 || frames < 3 {
		t.Fatalf("anim begin: code %d, %d frames", code, frames)
	}
	defer testAnimEnd(r)
	for _, k := range []int{0, 1, frames / 2, frames - 1} {
		_, a := testAnimFrameAt(r, k, nil)
		code, b := testAnimFrameAt(r, k, &other)
		if code != 0 {
			t.Fatalf("frame %d: %d", k, code)
		}
		if !bytes.Equal(a.Pix, b.Pix) {
			t.Errorf("frame %d differs with now", k)
		}
	}
}
