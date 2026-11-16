package devexport

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
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

// knownIncompatible lists shipped templates the exporter rightly rejects.
// rhb-105-v1 draws a constant "•" (U+2022), which is outside Latin-1.
var knownIncompatible = map[string]string{"rhb-105-v1": "Latin-1"}

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
	if g2[0].W != (g1[0].W+1)/2 || g2[0].H != 2*g1[0].H || h2[0] != 2*hdr[0] {
		t.Errorf("scaled glyph %+v / %v vs %+v / %v", g2[0], h2, g1[0], hdr)
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
