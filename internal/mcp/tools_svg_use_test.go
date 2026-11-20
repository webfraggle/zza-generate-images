package mcp

import (
	"fmt"
	"image/color"
	"strings"
	"testing"
	"time"
)

const svgHead = `<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" viewBox="0 0 10 10">`

// A self-referencing <use> made oksvg recurse until the stack overflowed,
// which kills the whole process (not recoverable).
func TestRasterizeSVG_UseSelfReference(t *testing.T) {
	svg := svgHead + `<defs><g id="a"><rect width="1" height="1"/><use xlink:href="#a"/></g></defs><use xlink:href="#a"/></svg>`
	_, err := rasterizeSVG(svg, 10, 10, true)
	if err == nil || !strings.Contains(err.Error(), "svg contains a recursive <use> reference") {
		t.Fatalf("got %v", err)
	}
}

func TestRasterizeSVG_UseIndirectCycle(t *testing.T) {
	svg := svgHead + `<defs><g id="a"><rect width="1" height="1"/><use href="#b"/></g><g id="b"><use xlink:href="#a"/></g></defs><use href="#a"/></svg>`
	_, err := rasterizeSVG(svg, 10, 10, true)
	if err == nil || !strings.Contains(err.Error(), "svg contains a recursive <use> reference") {
		t.Fatalf("got %v", err)
	}
}

// 7 levels of 10x <use> each expand to 10^7 shapes while parsing — before
// the maxSVGPaths check ran (1.9 KB SVG -> 13.6 GiB allocations).
func TestRasterizeSVG_UseBomb(t *testing.T) {
	var b strings.Builder
	b.WriteString(svgHead + `<defs><g id="l0"><rect width="1" height="1"/></g>`)
	for i := 1; i <= 7; i++ {
		fmt.Fprintf(&b, `<g id="l%d">`, i)
		for j := 0; j < 10; j++ {
			fmt.Fprintf(&b, `<use xlink:href="#l%d"/>`, i-1)
		}
		b.WriteString(`</g>`)
	}
	b.WriteString(`</defs><use xlink:href="#l7"/></svg>`)
	done := make(chan error, 1)
	start := time.Now()
	go func() {
		_, err := rasterizeSVG(b.String(), 10, 10, true)
		done <- err
	}()
	select {
	case err := <-done:
		if err == nil || !strings.Contains(err.Error(), "svg has too many shapes after <use> expansion") {
			t.Fatalf("got %v", err)
		}
		if d := time.Since(start); d > time.Second {
			t.Fatalf("took %v", d)
		}
	case <-time.After(time.Second):
		t.Fatal("use bomb not rejected within 1s")
	}
}

// Saturating arithmetic: very deep fan-out must not overflow into a small number.
func TestCheckSVGUse_NoOverflow(t *testing.T) {
	var b strings.Builder
	b.WriteString(svgHead + `<defs><g id="l0"><rect width="1" height="1"/></g>`)
	for i := 1; i <= 40; i++ {
		fmt.Fprintf(&b, `<g id="l%d">`, i)
		for j := 0; j < 16; j++ {
			fmt.Fprintf(&b, `<use href="#l%d"/>`, i-1)
		}
		b.WriteString(`</g>`)
	}
	b.WriteString(`</defs><use href="#l40"/></svg>`)
	if err := checkSVGStructure(b.String()); err == nil || !strings.Contains(err.Error(), "after <use> expansion") {
		t.Fatalf("got %v", err)
	}
}

func TestRasterizeSVG_UseUnknownID(t *testing.T) {
	svg := svgHead + `<defs><g id="a"><rect width="1" height="1"/></g></defs><use href="#nope"/></svg>`
	_, err := rasterizeSVG(svg, 10, 10, true)
	if err == nil || !strings.Contains(err.Error(), "svg <use> references unknown id") {
		t.Fatalf("got %v", err)
	}
}

func TestRasterizeSVG_TooDeeplyNested(t *testing.T) {
	svg := svgHead + strings.Repeat("<g>", 100) + `<rect width="1" height="1"/>` + strings.Repeat("</g>", 100) + `</svg>`
	_, err := rasterizeSVG(svg, 10, 10, true)
	if err == nil || !strings.Contains(err.Error(), "svg is nested too deeply") {
		t.Fatalf("got %v", err)
	}
}

// A symbol used five times is legitimate and renders.
func TestRasterizeSVG_UseLegit(t *testing.T) {
	svg := svgHead + `<defs><g id="dot"><rect width="2" height="2" fill="#ff0000"/></g></defs>` +
		`<use href="#dot" x="0"/><use href="#dot" x="2"/><use href="#dot" x="4"/><use xlink:href="#dot" x="6"/><use xlink:href="#dot" x="8"/></svg>`
	img, err := rasterizeSVG(svg, 10, 10, false)
	if err != nil {
		t.Fatal(err)
	}
	if c := img.NRGBAAt(0, 0); c != (color.NRGBA{R: 255, A: 255}) {
		t.Errorf("pixel (0,0) = %v, want red", c)
	}
}
