package devexport

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

const testFont = "../../templates/faltblatt-105-v1/NimbusSanL-Bol.otf"

// writeTemplate creates a template dir with the test font and the given YAML body
// (everything after the fonts block); png images are added by files.
func writeTemplate(t *testing.T, canvas, body string, files map[string]image.Image) (dir, name string) {
	t.Helper()
	dir = t.TempDir()
	name = "t1"
	td := filepath.Join(dir, name)
	if err := os.MkdirAll(td, 0o755); err != nil {
		t.Fatal(err)
	}
	font, err := os.ReadFile(testFont)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(td, "f.otf"), font, 0o644); err != nil {
		t.Fatal(err)
	}
	for n, img := range files {
		var b bytes.Buffer
		if err := png.Encode(&b, img); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(td, n), b.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	y := "meta:\n  name: t\n  canvas:\n" + canvas + "fonts:\n  - id: b\n    file: f.otf\nlayers:\n" + body
	if err := os.WriteFile(filepath.Join(td, "template.yaml"), []byte(y), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir, name
}

const canvas240 = "    width: 240\n    height: 240\n"

func gradient(w, h int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetNRGBA(x, y, color.NRGBA{uint8(x * 255 / w), uint8(y * 255 / h), 90, uint8(x * 255 / w)})
		}
	}
	return img
}

// knownIncompatible lists shipped templates the exporter rightly rejects
// (template name → expected reason fragment). Currently none.
var knownIncompatible = map[string]string{}

func TestExportAllShippedTemplates(t *testing.T) {
	root := "../../templates"
	ents, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, e := range ents {
		if !e.IsDir() {
			continue
		}
		n++
		t.Run(e.Name(), func(t *testing.T) {
			p, err := Export(root, e.Name())
			if why, known := knownIncompatible[e.Name()]; known {
				inc, ok := err.(*Incompatible)
				if !ok || !strings.Contains(inc.Error(), why) {
					t.Fatalf("err = %v, want incompatible (%s)", err, why)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			var mf struct {
				Format int    `json:"format"`
				Name   string `json:"name"`
				Files  []struct {
					N string `json:"n"`
					S int    `json:"s"`
					H string `json:"h"`
				} `json:"files"`
			}
			if err := json.Unmarshal(p.Files["manifest.json"], &mf); err != nil {
				t.Fatal(err)
			}
			if mf.Format != FormatVersion || mf.Name != e.Name() || len(mf.Files) != len(p.Files)-1 {
				t.Fatalf("manifest %+v vs %d files", mf, len(p.Files))
			}
			for _, f := range mf.Files {
				h := sha256.Sum256(p.Files[f.N])
				if len(f.N) > 12 || f.S != len(p.Files[f.N]) || f.H != hex.EncodeToString(h[:]) {
					t.Errorf("manifest entry %+v does not match file", f)
				}
			}
			z1, err := p.ZIP()
			if err != nil {
				t.Fatal(err)
			}
			p2, err := Export(root, e.Name())
			if err != nil {
				t.Fatal(err)
			}
			z2, _ := p2.ZIP()
			if !bytes.Equal(z1, z2) {
				t.Error("ZIP is not reproducible")
			}
			back, err := ReadZIP(z1)
			if err != nil || back.Name != e.Name() || len(back.Files) != len(p.Files) {
				t.Fatalf("ReadZIP: %v", err)
			}
			var tj map[string]any
			if err := json.Unmarshal(p.Files["t.json"], &tj); err != nil {
				t.Fatalf("t.json: %v", err)
			}
		})
	}
	if n == 0 {
		t.Fatal("no templates")
	}
}

func TestIncompatibleReasons(t *testing.T) {
	check := func(canvas, body string, want ...string) {
		t.Helper()
		dir, name := writeTemplate(t, canvas, body, nil)
		_, err := Export(dir, name)
		inc, ok := err.(*Incompatible)
		if !ok {
			t.Fatalf("err = %v, want *Incompatible", err)
		}
		if !strings.HasPrefix(inc.Error(), "nicht ZZA-kompatibel: ") {
			t.Errorf("Error() = %q", inc.Error())
		}
		all := strings.Join(inc.Reasons, "\n")
		for _, w := range want {
			if !strings.Contains(all, w) {
				t.Errorf("reasons %q lack %q", all, w)
			}
		}
	}
	text := func(v string) string {
		return "  - type: text\n    font: b\n    size: 12\n    x: 0\n    y: 0\n    color: \"#fff\"\n    value: " + v + "\n"
	}
	check("    width: 160\n    height: 80\n", text(`"a"`), "240 × 240")
	check(canvas240, text(`"€"`), "Latin-1")
	check(canvas240, text(`"{{x | match('(', 0)}}"`), "Regex")
	check(canvas240, text(`"{{x | match(pat, 0)}}"`), "Regex-Muster muss konstant sein")
	check(canvas240, text(`"{{x | match('a|b', 0)}}"`+"\n")+text(`"€"`)+
		"  - if: \"equals(x | match('[', 0), 'a')\"\n    type: rect\n    x: 0\n    y: 0\n    width: 1\n    height: 1\n    color: \"#000\"\n",
		"Latin-1", "Regex")

	// several errors at once: all reasons
	dir, name := writeTemplate(t, "    width: 160\n    height: 80\n", text(`"€"`)+text(`"{{x | match('(', 0)}}"`), nil)
	_, err := Export(dir, name)
	inc := err.(*Incompatible)
	if len(inc.Reasons) != 3 {
		t.Errorf("reasons = %q, want 3", inc.Reasons)
	}
}

func TestRegexTooBig(t *testing.T) {
	pat := strings.Repeat("(ab|cd)", 600)
	dir, name := writeTemplate(t, canvas240,
		"  - type: text\n    font: b\n    x: 0\n    y: 0\n    color: \"#fff\"\n    value: \"{{x | match('"+pat+"', 0)}}\"\n", nil)
	_, err := Export(dir, name)
	inc, ok := err.(*Incompatible)
	if !ok || !strings.Contains(strings.Join(inc.Reasons, "|"), "Regex-Muster zu groß für das Gerät") {
		t.Fatalf("err = %v", err)
	}
}

// parseFnt reads the .fnt format (see doc.go) for the tests.
type fntGlyph struct {
	CP     byte
	Adv    int32
	BX, BY int16
	W, H   int
	Mask   []byte
}

func parseFnt(t *testing.T, b []byte) (hdr [3]int16, glyphs []fntGlyph, nKern int) {
	t.Helper()
	if string(b[:3]) != "ZF1" {
		t.Fatal("magic")
	}
	flags := b[3]
	hdr = [3]int16{int16(binary.LittleEndian.Uint16(b[4:])), int16(binary.LittleEndian.Uint16(b[6:])), int16(binary.LittleEndian.Uint16(b[8:]))}
	ng := int(binary.LittleEndian.Uint16(b[10:]))
	nKern = int(binary.LittleEndian.Uint32(b[12:]))
	masks := b[fntHeaderSize+ng*fntEntrySize+nKern*fntKernSize:]
	for i := 0; i < ng; i++ {
		e := b[fntHeaderSize+i*fntEntrySize:]
		g := fntGlyph{CP: e[0], Adv: int32(binary.LittleEndian.Uint32(e[1:])),
			BX: int16(binary.LittleEndian.Uint16(e[5:])), BY: int16(binary.LittleEndian.Uint16(e[7:])),
			W: int(binary.LittleEndian.Uint16(e[9:])), H: int(binary.LittleEndian.Uint16(e[11:]))}
		off := int(binary.LittleEndian.Uint32(e[13:]))
		if n := g.W * g.H; n > 0 {
			if flags&1 == 0 {
				g.Mask = masks[off : off+n]
			} else {
				m, _, err := DecodeRLEPrefix(masks[off:], 1, n)
				if err != nil {
					t.Fatalf("glyph %d: %v", g.CP, err)
				}
				g.Mask = m
			}
		}
		glyphs = append(glyphs, g)
	}
	return
}

func glyphSetOf(t *testing.T, p *Package) []string {
	t.Helper()
	var out []string
	for n, b := range p.Files {
		if !strings.HasSuffix(n, ".fnt") {
			continue
		}
		_, gs, _ := parseFnt(t, b)
		var s []byte
		for _, g := range gs {
			s = append(s, g.CP)
		}
		out = append(out, string(s))
	}
	sort.Strings(out)
	return out
}

func TestGlyphSubset(t *testing.T) {
	layer := func(v string) string {
		return "  - type: text\n    font: b\n    size: 12\n    x: 0\n    y: 0\n    color: \"#fff\"\n    value: " + v + "\n"
	}
	dir, name := writeTemplate(t, canvas240, layer(`"Gleis"`), nil)
	p, err := Export(dir, name)
	if err != nil {
		t.Fatal(err)
	}
	got := glyphSetOf(t, p)
	if len(got) != 1 || got[0] != "?Geils" { // sorted by code point: ? (3F) G (47) e i l s
		t.Fatalf("glyphs = %q, want ?Geils", got)
	}
	dir, name = writeTemplate(t, canvas240, layer(`"{{gleis}}"`), nil)
	p, err = Export(dir, name)
	if err != nil {
		t.Fatal(err)
	}
	if got := glyphSetOf(t, p); len(got) != 1 || len(got[0]) != 224 {
		t.Fatalf("dynamic glyph count = %d, want 224", len(got[0]))
	}
	// a second combination (other size) is separate, and a conditional value counts all branches
	dir, name = writeTemplate(t, canvas240, layer(`"ab"`)+
		"  - type: text\n    font: b\n    size: 14\n    x: 0\n    y: 0\n    color: \"#fff\"\n    value:\n      if: \"isEmpty(x)\"\n      then: \"c\"\n      else: \"d\"\n", nil)
	p, _ = Export(dir, name)
	if got := glyphSetOf(t, p); len(got) != 2 || got[0] != "?ab" || got[1] != "?cd" {
		t.Fatalf("glyphs = %q", got)
	}
}

func TestFontMasksMatchRaster(t *testing.T) {
	// Masks of the RLE and raw variants decode to the same pixels; scaling changes size.
	font, _ := os.ReadFile(testFont)
	k := fontKey{File: "f", Size: 20, SX: 1, SY: 1}
	b1, err := BuildFont(font, k, []rune("AV?"))
	if err != nil {
		t.Fatal(err)
	}
	hdr, gs, nk := parseFnt(t, b1)
	if hdr[0] <= 0 || len(gs) != 3 || gs[0].CP != '?' || gs[1].CP != 'A' || gs[2].CP != 'V' {
		t.Fatalf("hdr %v glyphs %v", hdr, gs)
	}
	if nk <= 0 { // Nimbus Sans Bold kerns A-V
		t.Fatalf("nKern = %d", nk)
	}
	k2 := fontKey{File: "f", Size: 20, SX: 0.5, SY: 2}
	b2, err := BuildFont(font, k2, []rune("A"))
	if err != nil {
		t.Fatal(err)
	}
	h2, g2, _ := parseFnt(t, b2)
	_, g1, _ := parseFnt(t, mustBuild(t, font, k, "A"))
	// Resampled by exactly (0.5, 2): size within the kernel spill of w*sx,
	// h*sy, bearings floored, coverage scales with the area (0.5 × 2 = 1).
	cov := func(m []byte) (s int) {
		for _, v := range m {
			s += int(v)
		}
		return s
	}
	c1, c2 := cov(g1[0].Mask), cov(g2[0].Mask)
	if d := g2[0].W - g1[0].W/2; d < 0 || d > 2 || g2[0].H < 2*g1[0].H || g2[0].H > 2*g1[0].H+3 || h2[0] != 2*hdr[0] ||
		g2[0].BX > g1[0].BX/2 || g2[0].BY > 2*g1[0].BY || c2 < c1*95/100 || c2 > c1*105/100 {
		t.Errorf("scaled glyph %+v / %v (coverage %d) vs %+v / %v (coverage %d)", g2[0], h2, c2, g1[0], hdr, c1)
	}
	sum := 0
	for _, v := range gs[1].Mask {
		sum += int(v)
	}
	if len(gs[1].Mask) != gs[1].W*gs[1].H || sum == 0 {
		t.Errorf("mask of A missing (%d bytes, sum %d)", len(gs[1].Mask), sum)
	}
	if g2[0].Adv != g1[0].Adv {
		t.Errorf("advance must stay natural")
	}
}

func mustBuild(t *testing.T, font []byte, k fontKey, s string) []byte {
	b, err := BuildFont(font, k, []rune(s))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestImageScaledWhenConstant(t *testing.T) {
	dir, name := writeTemplate(t, canvas240, "  - type: image\n    file: a.png\n    x: 0\n    y: 0\n    width: 50\n",
		map[string]image.Image{"a.png": gradient(100, 40)})
	p, err := Export(dir, name)
	if err != nil {
		t.Fatal(err)
	}
	var tj struct {
		Layers []map[string]any `json:"layers"`
	}
	json.Unmarshal(p.Files["t.json"], &tj)
	l := tj.Layers[0]
	if _, has := l["_io"]; has {
		t.Fatal("_io present for constant size")
	}
	img, err := DecodeImage(p.Files[l["_i"].(string)+".img"])
	if err != nil {
		t.Fatal(err)
	}
	if img.Bounds().Dx() != 50 || img.Bounds().Dy() != 20 {
		t.Fatalf("size %v, want 50x20", img.Bounds())
	}
}

func TestImageOriginalWhenDynamic(t *testing.T) {
	dir, name := writeTemplate(t, canvas240,
		"  - type: text\n    id: a\n    font: b\n    x: 0\n    y: 0\n    color: \"#fff\"\n    value: \"x\"\n"+
			"  - type: image\n    file: a.png\n    x: 0\n    y: 0\n    width: \"{{a.end}}\"\n",
		map[string]image.Image{"a.png": gradient(30, 10)})
	p, err := Export(dir, name)
	if err != nil {
		t.Fatal(err)
	}
	var tj struct {
		Layers []map[string]any `json:"layers"`
	}
	json.Unmarshal(p.Files["t.json"], &tj)
	l := tj.Layers[1]
	if _, has := l["_i"]; has {
		t.Fatal("_i present for dynamic size")
	}
	img, err := DecodeImage(p.Files[l["_io"].(string)+".img"])
	if err != nil || img.Bounds().Dx() != 30 || img.Bounds().Dy() != 10 {
		t.Fatalf("original image: %v %v", img, err)
	}
	if string(tj.Layers[0]["_f"].(string)[0]) != "f" {
		t.Error("text layer lacks _f")
	}
}

func TestReadZIPWithTopFolder(t *testing.T) {
	dir, name := writeTemplate(t, canvas240, "  - type: rect\n    x: 0\n    y: 0\n    width: 1\n    height: 1\n    color: \"#000\"\n", nil)
	p, err := Export(dir, name)
	if err != nil {
		t.Fatal(err)
	}
	flat, _ := p.ZIP()
	want, err := ReadZIP(flat)
	if err != nil {
		t.Fatal(err)
	}
	wrapped := &Package{Files: map[string][]byte{}}
	for n, b := range p.Files {
		wrapped.Files["x/"+n] = b
	}
	// build the wrapped ZIP by hand (Package.ZIP writes names as given)
	wrapped.Files["x/manifest.json"] = p.Files["manifest.json"]
	z, err := wrapped.ZIP()
	if err != nil {
		t.Fatal(err)
	}
	got, err := ReadZIP(z)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Files) != len(want.Files) || got.Name != want.Name {
		t.Fatalf("got %d files %q", len(got.Files), got.Name)
	}
	for n, b := range want.Files {
		if !bytes.Equal(got.Files[n], b) {
			t.Errorf("%s differs", n)
		}
	}
}

func TestCondAndConstantRegexInJSON(t *testing.T) {
	dir, name := writeTemplate(t, canvas240,
		"  - type: text\n    font: b\n    x: 0\n    y: 0\n    color: 112233\n    value:\n      if: \"isEmpty(x)\"\n      then: \"a\"\n      elif: \"contains(x, 'q')\"\n      then: \"b\"\n      else: \"{{x | match('[0-9]+', 0)}}\"\n"+
			"  - else:\n    type: rect\n    x: 0\n    y: 0\n    width: 1\n    height: 1\n    color: \"#000\"\n", nil)
	p, err := Export(dir, name)
	if err != nil {
		t.Fatal(err)
	}
	var tj map[string]any
	json.Unmarshal(p.Files["t.json"], &tj)
	if _, ok := tj["_re"].(map[string]any)["[0-9]+"]; !ok {
		t.Errorf("_re = %v", tj["_re"])
	}
	l0 := tj["layers"].([]any)[0].(map[string]any)
	if l0["color"] != "112233" {
		t.Errorf("color = %v (%T), want string", l0["color"], l0["color"])
	}
	if c := l0["value"].(map[string]any)["_cond"].([]any); len(c) != 5 {
		t.Errorf("_cond = %v", c)
	}
	if tj["layers"].([]any)[1].(map[string]any)["else"] != true {
		t.Error("bare else not true")
	}
}

func imgFlat(w, h int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for i := 0; i < len(img.Pix); i += 4 {
		copy(img.Pix[i:], []byte{10, 20, 30, 255})
	}
	return img
}

func TestLiteralPartsOfInterpolatedTextAreChecked(t *testing.T) {
	layer := func(v string) string {
		return "  - type: text\n    font: b\n    x: 0\n    y: 0\n    color: \"#fff\"\n    value: " + v + "\n"
	}
	dir, name := writeTemplate(t, canvas240, layer(`"x{{a}}€"`), nil)
	_, err := Export(dir, name)
	inc, ok := err.(*Incompatible)
	if !ok {
		t.Fatalf("err = %v", err)
	}
	r := strings.Join(inc.Reasons, "|")
	if !strings.Contains(r, "Latin-1") || !strings.Contains(r, "U+20AC") || !strings.Contains(r, "layers[0]") {
		t.Errorf("reasons = %q", r)
	}
	dir, name = writeTemplate(t, canvas240, layer(`"x{{a}}é"`), nil)
	if _, err := Export(dir, name); err != nil {
		t.Fatal(err)
	}
}

// TestZIPLocalHeadersComplete reads the ZIP front to back using only local
// file headers (no central directory), as a streaming firmware reader would.
func TestZIPLocalHeadersComplete(t *testing.T) {
	p := &Package{Name: "x", Files: map[string][]byte{
		"manifest.json": []byte(`{"name":"x"}`), "a.img": bytes.Repeat([]byte{1, 2, 3}, 100), "t.json": []byte("{}"), "empty": {},
	}}
	z, err := p.ZIP()
	if err != nil {
		t.Fatal(err)
	}
	got := map[string][]byte{}
	for pos := 0; pos+30 <= len(z) && binary.LittleEndian.Uint32(z[pos:]) == 0x04034b50; {
		flags := binary.LittleEndian.Uint16(z[pos+6:])
		method := binary.LittleEndian.Uint16(z[pos+8:])
		csize := int(binary.LittleEndian.Uint32(z[pos+18:]))
		usize := int(binary.LittleEndian.Uint32(z[pos+22:]))
		nl, el := int(binary.LittleEndian.Uint16(z[pos+26:])), int(binary.LittleEndian.Uint16(z[pos+28:]))
		if flags&8 != 0 || method != 0 || csize != usize {
			t.Fatalf("entry at %d: flags %#x method %d sizes %d/%d", pos, flags, method, csize, usize)
		}
		name := string(z[pos+30 : pos+30+nl])
		start := pos + 30 + nl + el
		got[name] = z[start : start+csize]
		pos = start + csize
	}
	if len(got) != len(p.Files) {
		t.Fatalf("recovered %d files, want %d", len(got), len(p.Files))
	}
	for n, b := range p.Files {
		if !bytes.Equal(got[n], b) {
			t.Errorf("%s differs", n)
		}
	}
}

// TestYAMLAliasLimits: aliases are expanded with cycle detection and a node
// cap, so hostile YAML is rejected instead of recursing forever (stack
// overflow) or allocating without bound.
func TestYAMLAliasLimits(t *testing.T) {
	const layer = "  - type: rect\n    x: 0\n    y: 0\n    width: 1\n    height: 1\n    color: \"#000\"\n"
	bomb := func(merge bool) string {
		var b strings.Builder
		b.WriteString("zz_l0: &l0 [\"xxxxxxxx\",\"xxxxxxxx\",\"xxxxxxxx\",\"xxxxxxxx\",\"xxxxxxxx\",\"xxxxxxxx\",\"xxxxxxxx\",\"xxxxxxxx\",\"xxxxxxxx\",\"xxxxxxxx\"]\n")
		for i := 1; i <= 9; i++ {
			if merge {
				fmt.Fprintf(&b, "zz_l%d: &l%d {<<: {k: *l%d}", i, i, i-1)
				for j := 0; j < 10; j++ {
					fmt.Fprintf(&b, ", a%d: *l%d", j, i-1)
				}
				b.WriteString("}\n")
				continue
			}
			fmt.Fprintf(&b, "zz_l%d: &l%d [", i, i)
			for j := 0; j < 10; j++ {
				if j > 0 {
					b.WriteString(",")
				}
				fmt.Fprintf(&b, "*l%d", i-1)
			}
			b.WriteString("]\n")
		}
		return b.String()
	}
	for name, extra := range map[string]string{
		"cycle":       "zz_extra: &a [*a]\n",
		"cycle-map":   "zz_extra: &a {k: *a}\n",
		"cycle-merge": "zz_extra: &a {<<: *a}\n",
		"bomb":        bomb(false),
		"bomb-merge":  bomb(true),
	} {
		t.Run(name, func(t *testing.T) {
			dir, tn := writeTemplate(t, canvas240, layer+extra, nil)
			start := time.Now()
			_, err := Export(dir, tn)
			inc, ok := err.(*Incompatible)
			if !ok {
				t.Fatalf("err = %v, want *Incompatible", err)
			}
			if r := strings.Join(inc.Reasons, "|"); !strings.Contains(r, "YAML") {
				t.Errorf("reasons = %q", r)
			}
			if d := time.Since(start); d > 5*time.Second {
				t.Errorf("took %v", d)
			}
		})
	}

	// Ordinary aliases and merge keys still work.
	dir, tn := writeTemplate(t, canvas240, "  - &r {type: rect, x: 0, y: 0, width: 1, height: 1, color: \"#000\"}\n  - *r\n  - {<<: *r, x: 5}\n", nil)
	p, err := Export(dir, tn)
	if err != nil {
		t.Fatal(err)
	}
	var tj map[string]any
	if err := json.Unmarshal(p.Files["t.json"], &tj); err != nil {
		t.Fatal(err)
	}
	ls := tj["layers"].([]any)
	if len(ls) != 3 || ls[1].(map[string]any)["x"] != float64(0) || ls[2].(map[string]any)["x"] != float64(5) || ls[2].(map[string]any)["width"] != float64(1) {
		t.Errorf("layers = %v", ls)
	}
}

// pngHeaderOnly returns a PNG that declares w×h pixels but has no image data:
// image.DecodeConfig accepts it, image.Decode fails.
func pngHeaderOnly(w, h uint32) []byte {
	var b bytes.Buffer
	b.WriteString("\x89PNG\r\n\x1a\n")
	ihdr := make([]byte, 13)
	binary.BigEndian.PutUint32(ihdr[0:], w)
	binary.BigEndian.PutUint32(ihdr[4:], h)
	ihdr[8], ihdr[9] = 8, 6 // 8-bit RGBA
	chunk := append([]byte("IHDR"), ihdr...)
	binary.Write(&b, binary.BigEndian, uint32(len(ihdr)))
	b.Write(chunk)
	binary.Write(&b, binary.BigEndian, crc32.ChecksumIEEE(chunk))
	return b.Bytes()
}

// TestImageDimensionsCheckedBeforeDecoding: the size limits apply to the
// declared dimensions, before the image is decoded (no huge allocation).
func TestImageDimensionsCheckedBeforeDecoding(t *testing.T) {
	for _, c := range []struct {
		w, h uint32
		big  bool
	}{
		{30000, 30000, true}, // 900 M pixels
		{4000, 2000, true},   // 8 M pixels: over the device's 6 291 456
		{70000, 1, true},     // wider than the format's u16
		{2000, 3000, false},  // 6 M pixels: within the limit (then fails decoding)
	} {
		dir, name := writeTemplate(t, canvas240, "  - type: image\n    file: big.png\n    x: 0\n    y: 0\n", nil)
		if err := os.WriteFile(filepath.Join(dir, name, "big.png"), pngHeaderOnly(c.w, c.h), 0o644); err != nil {
			t.Fatal(err)
		}
		_, err := Export(dir, name)
		inc, ok := err.(*Incompatible)
		if !ok {
			t.Fatalf("%dx%d: err = %v", c.w, c.h, err)
		}
		r := strings.Join(inc.Reasons, "|")
		if got := strings.Contains(r, "zu groß für das Gerät"); got != c.big {
			t.Errorf("%dx%d: reasons %q, want too big = %v", c.w, c.h, r, c.big)
		}
	}
}
