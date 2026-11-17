package devicerender

import (
	"bytes"
	"errors"
	"image"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/webfraggle/zza-generate-images/internal/devexport"
	"github.com/webfraggle/zza-generate-images/internal/renderer"
)

// Budgets of the worst-case test: everything must fit into worstBudget; with
// worstSmallBudget the core must fail cleanly with ErrNoMem.
const (
	worstBudget      = 6 << 20
	worstSmallBudget = 1 << 20
)

// TestWorstCaseBudget runs the templates at the limits (spec §9 "Worst-Case",
// testdata/devtemplates/worst-*): export (or a rejection for size only), load,
// still image and the complete animation within 6 MB, near the Go renderer;
// MemPeak is logged (input for the manual test plan and Stage 3 sizing). With
// 1 MB the core returns ErrNoMem without crashing and renders the still image
// once the budget is large again.
func TestWorstCaseBudget(t *testing.T) {
	dirs, err := filepath.Glob(filepath.Join(covRoot, "worst-*"))
	if err != nil || len(dirs) != 5 {
		t.Fatalf("worst-case templates: %v %v", dirs, err)
	}
	for _, d := range dirs {
		name := filepath.Base(d)
		t.Run(name, func(t *testing.T) {
			p, err := devexport.Export(covRoot, name)
			var inc *devexport.Incompatible
			if errors.As(err, &inc) {
				for _, r := range inc.Reasons {
					if !strings.Contains(r, "Paket ist") {
						t.Fatalf("rejected for another reason than the package size: %v", inc)
					}
				}
				t.Logf("rejected for size: %v", inc)
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			size := 0
			for _, b := range p.Files {
				size += len(b)
			}
			tmpl, err := renderer.LoadTemplate(covRoot, name)
			if err != nil {
				t.Fatal(err)
			}
			now := fixedNow(t, tmpl)
			data := readDefault(t, covRoot, name)

			r := New(worstBudget)
			defer r.Close()
			if err := r.Load(p); err != nil {
				t.Fatalf("Load with %d bytes: %v", worstBudget, err)
			}
			loaded := r.MemUsed()
			start := time.Now()
			still, err := r.Render(data, now)
			if err != nil {
				t.Fatalf("Render: %v", err)
			}
			stillTime, stillPeak := time.Since(start), r.MemPeak()
			frames := 0
			start = time.Now()
			err = r.AnimFrames(data, now, func(int, *image.NRGBA, int, []image.Rectangle) error {
				frames++
				return nil
			})
			if err != nil {
				t.Fatalf("animation: %v", err)
			}
			t.Logf("package %d bytes; loaded %d; MemPeak still %d, with animation %d (budget %d); still %v, %d frames in %v",
				size, loaded, stillPeak, r.MemPeak(), worstBudget, stillTime.Round(time.Microsecond), frames, time.Since(start).Round(time.Millisecond))
			if r.MemPeak() > worstBudget {
				t.Errorf("MemPeak %d over the budget", r.MemPeak())
			}
			compareNear(t, r, covRoot, name, data)
			if frames > 1 {
				compareAnim(t, r, covRoot, name, data, func(n int) []int { return []int{0, 1, n / 2, n - 1} })
			}

			// Small budget: ErrNoMem at load, still image or animation, no crash.
			s := New(worstSmallBudget)
			defer s.Close()
			loadedSmall := false
			err = s.Load(p)
			if err == nil {
				loadedSmall = true
				if _, err = s.Render(data, now); err == nil {
					err = s.AnimFrames(data, now, func(int, *image.NRGBA, int, []image.Rectangle) error { return nil })
				}
			}
			t.Logf("budget %d: %v", worstSmallBudget, err)
			if !errors.Is(err, ErrNoMem) {
				t.Fatalf("budget %d: %v, want ErrNoMem", worstSmallBudget, err)
			}
			testSetLimit(s, worstBudget)
			if !loadedSmall {
				if err := s.Load(p); err != nil {
					t.Fatalf("Load after ErrNoMem: %v", err)
				}
			}
			again, err := s.Render(data, now)
			if err != nil {
				t.Fatalf("Render after ErrNoMem: %v", err)
			}
			if !bytes.Equal(again.Pix, still.Pix) {
				t.Error("still image after ErrNoMem differs")
			}
		})
	}
}
