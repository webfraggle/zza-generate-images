package devicerender

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// bigArray is a JSON array of n zeros.
func bigArray(n int) string {
	return "[" + strings.TrimSuffix(strings.Repeat("0,", n), ",") + "]"
}

// TestJSONLargeContainers: containers with more than 32 767 members (the
// limit of ArduinoJson's 2-byte slot offsets, its default on 32-bit targets)
// in the train data and in t.json keep the members after them intact. The
// core pins ARDUINOJSON_SLOT_OFFSET_SIZE 4 so the device and the preview share
// one layout.
func TestJSONLargeContainers(t *testing.T) {
	const n = 40000

	t.Run("data", func(t *testing.T) {
		r := loadDevice(t, covRoot, "cov-text")
		def := strings.TrimSpace(string(readDefault(t, covRoot, "cov-text")))
		// The big array comes first, so every member after it is linked across it.
		data := []byte(`{"a_big":` + bigArray(n) + `,` + strings.TrimPrefix(def, "{"))
		if len(data) > 256<<10 {
			t.Fatalf("data %d bytes, over the data limit", len(data))
		}
		compareNear(t, r, covRoot, "cov-text", data)
	})

	t.Run("t.json", func(t *testing.T) {
		dir := t.TempDir()
		copyTemplate(t, covRoot, "cov-text", dir, "big", "meta:", "a_big: "+bigArray(n)+"\nmeta:")
		b, err := os.ReadFile(filepath.Join(dir, "big", "template.yaml"))
		if err != nil || !strings.HasPrefix(string(b), "a_big: [0,0") {
			t.Fatalf("template not patched: %v", err)
		}
		r := loadDevice(t, dir, "big")
		compareNear(t, r, dir, "big", readDefault(t, dir, "big"))
	})
}
