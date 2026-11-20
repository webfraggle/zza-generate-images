package mcp

import (
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"path/filepath"
	"strings"
	"time"

	"github.com/webfraggle/zza-generate-images/internal/devexport"
	"github.com/webfraggle/zza-generate-images/internal/devicerender"
)

// Limits of the ZZA firmware (repo esp32-zugzielanzeiger, src/ProjectSetup.h).
// The QUAD is the tighter device: two tracks share the PSRAM.
const (
	deviceCoreBudget = 1536 << 10 // TPL_CORE_BUDGET_BYTES (QUAD, per track)
	deviceWorkLimit  = 15_000_000 // TPL_WORK_LIMIT, operations per core call
	// The device needs a little more than the host measures (faltblatt 1.17 MiB
	// on the host, up to 1.27 MB on the QUAD): warn from 85 % of the budget.
	deviceCoreWarn = deviceCoreBudget * 85 / 100
	// The probe runs with a generous budget so it can report the real need.
	deviceProbeBudget = 64 << 20
)

// deviceWarnings renders the template with the ZZA's own core (the C++
// renderer of the firmware) and default.json, and reports what would fail on
// the device: not exportable, more core memory than the QUAD gives a track,
// a render or animation frame above the firmware's work limit. Non-blocking.
func deviceWarnings(e *env, template string) []string {
	pkg, err := devexport.Export(e.templatesDir, template)
	if err != nil {
		var inc *devexport.Incompatible
		if errors.As(err, &inc) {
			return []string{"not ZZA-compatible (the device cannot show it, ZZA export fails): " + strings.ReplaceAll(strings.TrimPrefix(inc.Error(), "nicht ZZA-kompatibel:"), "\n", " ")}
		}
		return nil // other export errors: the regular validation reports them
	}
	r := devicerender.New(deviceProbeBudget)
	defer r.Close()
	if err := r.Load(pkg); err != nil {
		return []string{"ZZA core cannot load the template: " + e.scrubErr(err, maxErrLen)}
	}
	data, _ := json.Marshal(loadTestData(filepath.Join(e.templatesDir, template)))
	now := time.Now()
	var maxWork uint64
	if _, err := r.Render(data, now); err != nil {
		return []string{"ZZA core cannot render default.json: " + e.scrubErr(err, maxErrLen)}
	}
	maxWork = r.WorkUsed()
	_ = r.AnimFrames(data, now, func(_ int, _ *image.NRGBA, _ int, _ []image.Rectangle) error {
		maxWork = max(maxWork, r.WorkUsed())
		return nil
	})

	var w []string
	mib := func(n int) string { return fmt.Sprintf("%.2f MiB", float64(n)/(1<<20)) }
	switch peak := r.MemPeak(); {
	case peak > deviceCoreBudget:
		w = append(w, fmt.Sprintf("device: needs %s core memory, the QUAD allows 1.5 MiB per track — the display shows the built-in layout instead. Use fewer/smaller fonts and images (scale images to their drawn size).", mib(peak)))
	case peak > deviceCoreWarn:
		w = append(w, fmt.Sprintf("device: needs %s core memory, close to the QUAD limit of 1.5 MiB per track — longer texts may exceed it. Use fewer/smaller fonts and images.", mib(peak)))
	}
	if maxWork > deviceWorkLimit {
		w = append(w, fmt.Sprintf("device: one render step costs %.1f M operations, the ZZA allows %d M (about 5 s) — the display shows the built-in layout instead. Avoid scaling large images at runtime, deep loops and many layers.", float64(maxWork)/1e6, deviceWorkLimit/1_000_000))
	}
	return w
}
