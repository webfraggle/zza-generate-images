package mcp

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// useBombDefs returns definitions l0..l<levels> where each level uses the
// previous one 10 times: l<levels> expands to 10^levels shapes. Tests use 5
// levels (100 000 shapes, far above maxSVGPaths but cheap for oksvg), so a
// red run cannot exhaust memory.
func useBombDefs(levels int) string {
	var b strings.Builder
	b.WriteString(`<g id="l0"><rect width="1" height="1"/></g>`)
	for i := 1; i <= levels; i++ {
		fmt.Fprintf(&b, `<g id="l%d">`, i)
		for j := 0; j < 10; j++ {
			fmt.Fprintf(&b, `<use href="#l%d"/>`, i-1)
		}
		b.WriteString(`</g>`)
	}
	return b.String()
}

// checkWithin runs checkSVGStructure with a time limit.
func checkWithin(t *testing.T, svg string) error {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- checkSVGStructure(svg) }()
	select {
	case err := <-done:
		return err
	case <-time.After(time.Second):
		t.Fatal("checkSVGStructure did not finish within 1s")
		return nil
	}
}

const bypassHead = `<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" viewBox="0 0 10 10">`

// Bypass B: an inner </defs> ends oksvg's defs mode; the following <use> is
// expanded immediately.
func TestCheckSVGStructure_NestedDefsRejected(t *testing.T) {
	svg := bypassHead + `<defs>` + useBombDefs(5) + `<g id="z"></g><defs></defs><use href="#l5"/></defs></svg>`
	err := checkWithin(t, svg)
	if err == nil || !strings.Contains(err.Error(), "svg contains nested <defs>") {
		t.Fatalf("got %v", err)
	}
}

// Bypass C: oksvg expands a <use> inside a gradient right away.
func TestCheckSVGStructure_UseInGradientRejected(t *testing.T) {
	for _, svg := range []string{
		bypassHead + `<defs>` + useBombDefs(5) + `<g id="flush"></g><linearGradient id="g"><use href="#l5"/></linearGradient></defs></svg>`,
		bypassHead + `<defs>` + useBombDefs(5) + `</defs><radialGradient id="g"><use href="#l5"/></radialGradient></svg>`,
		bypassHead + `<defs><g id="a"><rect width="1" height="1"/></g><pattern id="p"><use href="#a"/></pattern></defs></svg>`,
		bypassHead + `<defs><g id="a"><rect width="1" height="1"/></g><clipPath id="c"><use href="#a"/></clipPath></defs></svg>`,
		bypassHead + `<defs><g id="a"><rect width="1" height="1"/></g></defs><mask id="m"><use href="#a"/></mask></svg>`,
	} {
		err := checkWithin(t, svg)
		if err == nil || !strings.Contains(err.Error(), "<use> is not allowed inside gradients") {
			t.Errorf("got %v for %s", err, svg[len(bypassHead):])
		}
	}
}

// Every <use> counts against the limit, whatever its context.
func TestCheckSVGStructure_UseBombInAnyContext(t *testing.T) {
	defs := `<defs>` + useBombDefs(5) + `</defs>`
	cases := map[string]string{
		"g outside defs":     defs + `<g><use href="#l5"/></g>`,
		"a":                  defs + `<a><use href="#l5"/></a>`,
		"switch":             defs + `<switch><use href="#l5"/></switch>`,
		"marker":             defs + `<marker id="m"><use href="#l5"/></marker>`,
		"marker inside defs": `<defs>` + useBombDefs(5) + `<marker id="m"><use href="#l5"/></marker></defs>`,
		"unreferenced def":   `<defs>` + useBombDefs(5) + `<g id="x"><use href="#l5"/></g></defs>`,
		"two hrefs":          defs + `<defs><g id="a"><rect width="1" height="1"/></g></defs><use href="#a" xlink:href="#l5"/>`,
	}
	for name, body := range cases {
		err := checkWithin(t, bypassHead+body+`</svg>`)
		if err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

// Legitimate reuse keeps working.
func TestCheckSVGStructure_LegitUse(t *testing.T) {
	svg := bypassHead + `<defs><g id="dot"><rect width="2" height="2" fill="#ff0000"/></g>` +
		`<linearGradient id="gr"><stop offset="0" stop-color="#000"/></linearGradient></defs>` +
		`<use href="#dot"/><use href="#dot"/><use href="#dot"/><use xlink:href="#dot"/><use xlink:href="#dot"/></svg>`
	if err := checkWithin(t, svg); err != nil {
		t.Fatal(err)
	}
	if _, err := rasterizeSVG(svg, 10, 10, true); err != nil {
		t.Fatal(err)
	}
}
