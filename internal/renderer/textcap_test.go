package renderer

import (
	"image"
	"strings"
	"testing"
	"time"
)

func TestCapTextRunes(t *testing.T) {
	if got := capText("äöü"); got != "äöü" {
		t.Errorf("short text changed: %q", got)
	}
	long := strings.Repeat("ä", maxTextRunes+10)
	got := capText(long)
	if n := len([]rune(got)); n != maxTextRunes {
		t.Errorf("capped to %d runes, want %d", n, maxTextRunes)
	}
}

// A huge wrapped text must not cost more than its visible part: lines below
// the canvas are neither wrapped endlessly nor drawn.
func TestSbb105_HugeHinweisIsCheap(t *testing.T) {
	r, tmpl, data := loadSbb(t)
	z := data["zug1"].(map[string]interface{})
	short := "Abweichende Wagenreihung Zug Zug Zug Zug Zug Zug Zug Zug Zug Zug"
	z["hinweis"] = short
	want, err := r.Render(tmpl, data)
	if err != nil {
		t.Fatal(err)
	}
	z["hinweis"] = short + strings.Repeat(" Zug", 250_000) // ~1 MiB
	start := time.Now()
	got, err := r.Render(tmpl, data)
	if err != nil {
		t.Fatal(err)
	}
	if d := time.Since(start); d > 500*time.Millisecond {
		t.Errorf("render took %v, want < 500ms", d)
	}
	if !framesEqual([]*image.NRGBA{want}, []*image.NRGBA{got}) {
		t.Error("visible output changed by text beyond the canvas")
	}
}
