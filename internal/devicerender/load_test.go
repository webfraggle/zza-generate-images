package devicerender

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"image"
	"os"
	"strings"
	"testing"

	"github.com/webfraggle/zza-generate-images/internal/devexport"
)

const templatesRoot = "../../templates"

// shippedPackages exports every shipped template the exporter accepts.
func shippedPackages(t testing.TB) map[string]*devexport.Package {
	t.Helper()
	ents, err := os.ReadDir(templatesRoot)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]*devexport.Package{}
	for _, e := range ents {
		if !e.IsDir() {
			continue
		}
		p, err := devexport.Export(templatesRoot, e.Name())
		if err != nil {
			var inc *devexport.Incompatible
			if errors.As(err, &inc) {
				continue
			}
			t.Fatalf("%s: %v", e.Name(), err)
		}
		out[e.Name()] = p
	}
	if len(out) == 0 {
		t.Fatal("no exportable template")
	}
	return out
}

// clonePkg copies a package's file map (data slices are shared; mutate via setFile).
func clonePkg(p *devexport.Package) *devexport.Package {
	q := &devexport.Package{Name: p.Name, Files: map[string][]byte{}}
	for n, d := range p.Files {
		q.Files[n] = d
	}
	return q
}

// setFile replaces a file and fixes size and hash in the manifest, so only the
// file's own validation can reject it.
func setFile(t testing.TB, p *devexport.Package, name string, data []byte) {
	t.Helper()
	p.Files[name] = data
	var m map[string]any
	if err := json.Unmarshal(p.Files["manifest.json"], &m); err != nil {
		t.Fatal(err)
	}
	for _, f := range m["files"].([]any) {
		fm := f.(map[string]any)
		if fm["n"] == name {
			h := sha256.Sum256(data)
			fm["s"] = len(data)
			fm["h"] = hex.EncodeToString(h[:])
		}
	}
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	p.Files["manifest.json"] = b
}

func cp(b []byte) []byte { return append([]byte(nil), b...) }

// firstFile returns the first file name (sorted) with the given prefix/suffix.
func firstFile(t testing.TB, p *devexport.Package, prefix, suffix string) string {
	t.Helper()
	best := ""
	for n := range p.Files {
		if strings.HasPrefix(n, prefix) && strings.HasSuffix(n, suffix) && (best == "" || n < best) {
			best = n
		}
	}
	if best == "" {
		t.Skipf("package has no %s*%s", prefix, suffix)
	}
	return best
}

func TestLoadShipped(t *testing.T) {
	for name, p := range shippedPackages(t) {
		t.Run(name, func(t *testing.T) {
			r := New(32 << 20)
			defer r.Close()
			if err := r.Load(p); err != nil {
				t.Fatal(err)
			}
			if r.MemUsed() <= 0 {
				t.Fatalf("MemUsed = %d after load", r.MemUsed())
			}
			// Loading again replaces the package.
			used := r.MemUsed()
			if err := r.Load(p); err != nil {
				t.Fatal(err)
			}
			if r.MemUsed() != used {
				t.Fatalf("MemUsed after reload = %d, want %d", r.MemUsed(), used)
			}
		})
	}
}

func TestLoadCorrupt(t *testing.T) {
	// Pick a package that has fonts and images and a regex if possible.
	var base *devexport.Package
	var baseName string
	for name, p := range shippedPackages(t) {
		if _, ok := p.Files["f0.fnt"]; !ok {
			continue
		}
		if _, ok := p.Files["i0.img"]; !ok && base != nil {
			continue
		}
		if base == nil || strings.Contains(string(p.Files["manifest.json"]), `"images"`) {
			base, baseName = p, name
		}
	}
	if base == nil {
		t.Fatal("no package with a font")
	}
	t.Logf("base package %s", baseName)
	fnt := "f0.fnt"

	type tc struct {
		name string
		file string // must appear in the error text
		mod  func(p *devexport.Package)
	}
	setGlyph := func(p *devexport.Package, field int, v uint32, size int) {
		b := cp(p.Files[fnt])
		off := 16 + field
		if size == 2 {
			binary.LittleEndian.PutUint16(b[off:], uint16(v))
		} else {
			binary.LittleEndian.PutUint32(b[off:], v)
		}
		setFile(t, p, fnt, b)
	}
	cases := []tc{
		{"missing file", fnt, func(p *devexport.Package) { delete(p.Files, fnt) }},
		{"missing manifest", "manifest.json", func(p *devexport.Package) { delete(p.Files, "manifest.json") }},
		{"truncated by 1", fnt, func(p *devexport.Package) { p.Files[fnt] = cp(p.Files[fnt])[:len(p.Files[fnt])-1] }},
		{"flipped byte", fnt, func(p *devexport.Package) {
			b := cp(p.Files[fnt])
			b[len(b)/2] ^= 1
			p.Files[fnt] = b
		}},
		{"flipped t.json byte", "t.json", func(p *devexport.Package) {
			b := cp(p.Files["t.json"])
			b[len(b)/2] ^= 1
			p.Files["t.json"] = b
		}},
		{"format 2", "manifest.json", func(p *devexport.Package) {
			p.Files["manifest.json"] = bytes.Replace(p.Files["manifest.json"], []byte(`"format":1`), []byte(`"format":2`), 1)
		}},
		{"t.json {", "t.json", func(p *devexport.Package) { setFile(t, p, "t.json", []byte("{")) }},
		{"t.json array", "t.json", func(p *devexport.Package) { setFile(t, p, "t.json", []byte("[1]")) }},
		{"bad _re", "t.json", func(p *devexport.Package) { setFile(t, p, "t.json", []byte(`{"_re":{"a":[9,1,[]]}}`)) }},
		{"glyph offset beyond masks", fnt, func(p *devexport.Package) { setGlyph(p, 13, 0xFFFFFF, 4) }},
		{"glyph mask too big", fnt, func(p *devexport.Package) { setGlyph(p, 9, 0xFFFF, 2); setGlyph(p, 11, 0xFFFF, 2) }},
		{"glyph magic", fnt, func(p *devexport.Package) {
			b := cp(p.Files[fnt])
			b[0] = 'X'
			setFile(t, p, fnt, b)
		}},
		{"glyph order", fnt, func(p *devexport.Package) {
			b := cp(p.Files[fnt])
			b[16], b[16+17] = b[16+17], b[16]
			setFile(t, p, fnt, b)
		}},
		{"huge nGlyphs", fnt, func(p *devexport.Package) {
			b := cp(p.Files[fnt])
			binary.LittleEndian.PutUint16(b[10:], 0xFFFF)
			setFile(t, p, fnt, b)
		}},
		{"huge nKern", fnt, func(p *devexport.Package) {
			b := cp(p.Files[fnt])
			binary.LittleEndian.PutUint32(b[12:], 0xFFFFFFFF)
			setFile(t, p, fnt, b)
		}},
	}
	// Kerning order (only when the font has a kerning table with two entries).
	if b := base.Files[fnt]; binary.LittleEndian.Uint32(b[12:]) >= 2 {
		cases = append(cases, tc{"kerning order", fnt, func(p *devexport.Package) {
			b := cp(p.Files[fnt])
			ng := int(binary.LittleEndian.Uint16(b[10:]))
			k := 16 + ng*17
			var tmp [6]byte
			copy(tmp[:], b[k:k+6])
			copy(b[k:k+6], b[k+6:k+12])
			copy(b[k+6:k+12], tmp[:])
			setFile(t, p, fnt, b)
		}})
	}
	if _, ok := base.Files["i0.img"]; ok {
		img := "i0.img"
		cases = append(cases,
			tc{"image RLE run beyond end", img, func(p *devexport.Package) {
				// 1x1 image whose RLE stream repeats the pixel 129 times.
				b := []byte{1, 0, 1, 0, 1, 255, 1, 2, 3, 4}
				setFile(t, p, img, b)
			}},
			tc{"image raw size", img, func(p *devexport.Package) {
				b := cp(p.Files[img])
				b[4] = 0
				setFile(t, p, img, b)
			}},
			tc{"image huge dims", img, func(p *devexport.Package) {
				b := []byte{0xFF, 0xFF, 0xFF, 0xFF, 1, 0, 1, 2, 3, 4}
				setFile(t, p, img, b)
			}},
			tc{"image truncated header", img, func(p *devexport.Package) { setFile(t, p, img, []byte{1, 0}) }},
		)
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := clonePkg(base)
			c.mod(p)
			r := New(64 << 20)
			defer r.Close()
			err := r.Load(p)
			if !errors.Is(err, ErrFormat) {
				t.Fatalf("err = %v, want ErrFormat", err)
			}
			if !strings.Contains(err.Error(), c.file) {
				t.Fatalf("error %q does not name %q", err, c.file)
			}
			if r.MemUsed() != 0 {
				t.Fatalf("MemUsed = %d after failed load", r.MemUsed())
			}
		})
	}
}

func TestLoadNoMem(t *testing.T) {
	for name, p := range shippedPackages(t) {
		t.Run(name, func(t *testing.T) {
			r := New(64 << 20)
			if err := r.Load(p); err != nil {
				t.Fatal(err)
			}
			peak := r.MemPeak()
			r.Close()
			for _, budget := range []int{peak / 2, peak - 1, 1} {
				r := New(budget)
				err := r.Load(p)
				if !errors.Is(err, ErrNoMem) {
					t.Fatalf("budget %d (peak %d): err = %v, want ErrNoMem", budget, peak, err)
				}
				if r.MemUsed() != 0 {
					t.Fatalf("MemUsed = %d after failed load", r.MemUsed())
				}
				r.Close()
			}
		})
	}
}

func TestRendererUnusable(t *testing.T) {
	p := shippedPackages(t)
	for _, pk := range p {
		r := New(1 << 20)
		r.Close()
		if err := r.Load(pk); !errors.Is(err, ErrState) {
			t.Fatalf("Load after Close: %v, want ErrState", err)
		}
		// A renderer whose core allocation failed.
		r2 := &Renderer{}
		if err := r2.Load(pk); !errors.Is(err, ErrNoMem) {
			t.Fatalf("Load on nil core: %v, want ErrNoMem", err)
		}
		return
	}
}

func FuzzLoad(f *testing.F) {
	pkgs := shippedPackages(f)
	var names []string
	for n := range pkgs {
		names = append(names, n)
	}
	// Seeds: (package index, file selector, op, position, value)
	for i := range names {
		f.Add(uint8(i), uint8(0), uint8(0), uint32(7), uint8(1))
		f.Add(uint8(i), uint8(1), uint8(1), uint32(5), uint8(0))
		f.Add(uint8(i), uint8(2), uint8(2), uint32(100), uint8(0xFF))
	}
	f.Fuzz(func(t *testing.T, pi, fi, op uint8, pos uint32, val uint8) {
		base := pkgs[names[int(pi)%len(names)]]
		p := clonePkg(base)
		fn := make([]string, 0, len(p.Files))
		for n := range p.Files {
			fn = append(fn, n)
		}
		sortStrings(fn)
		name := fn[int(fi)%len(fn)]
		b := cp(p.Files[name])
		if len(b) == 0 {
			return
		}
		at := int(pos) % len(b)
		switch op % 4 {
		case 0: // flip a byte, keep the manifest (hash must catch it)
			b[at] ^= val | 1
			p.Files[name] = b
		case 1: // truncate, keep the manifest
			p.Files[name] = b[:at]
		case 2: // flip a byte and fix the manifest (the file's own validation must hold)
			b[at] = val
			if name != "manifest.json" {
				setFile(t, p, name, b)
			} else {
				p.Files[name] = b
			}
		case 3: // truncate and fix the manifest
			if name != "manifest.json" {
				setFile(t, p, name, b[:at])
			} else {
				p.Files[name] = b[:at]
			}
		}
		r := New(8 << 20)
		defer r.Close()
		err := r.Load(p)
		if err != nil && !errors.Is(err, ErrFormat) && !errors.Is(err, ErrNoMem) {
			t.Fatalf("unexpected error class: %v", err)
		}
		if err != nil && r.MemUsed() != 0 {
			t.Fatalf("MemUsed = %d after failed load", r.MemUsed())
		}
	})
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

func fnv32(b []byte) uint32 {
	h := uint32(2166136261)
	for _, c := range b {
		h = (h ^ uint32(c)) * 16777619
	}
	return h
}

// expectedImage decodes an .img the way the format says (Go reference).
func expectedImage(t *testing.T, b []byte) []byte {
	t.Helper()
	w, h := int(binary.LittleEndian.Uint16(b)), int(binary.LittleEndian.Uint16(b[2:]))
	if b[4] == 0 {
		return b[5:]
	}
	out, err := devexport.DecodeRLE(b[5:], 4, w*h)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// expectedMasks concatenates the glyph masks in table order (Go reference).
func expectedMasks(t *testing.T, b []byte) []byte {
	t.Helper()
	ng, nk := int(binary.LittleEndian.Uint16(b[10:])), int(binary.LittleEndian.Uint32(b[12:]))
	area := b[16+ng*17+nk*6:]
	var out []byte
	for i := 0; i < ng; i++ {
		e := b[16+i*17:]
		n := int(binary.LittleEndian.Uint16(e[9:])) * int(binary.LittleEndian.Uint16(e[11:]))
		off := int(binary.LittleEndian.Uint32(e[13:]))
		if n == 0 {
			continue
		}
		if b[3]&1 == 0 {
			out = append(out, area[off:off+n]...)
			continue
		}
		m, _, err := devexport.DecodeRLEPrefix(area[off:], 1, n)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, m...)
	}
	return out
}

// TestLoadDecodedData checks the decoded images and masks against the Go
// reference decoders, for raw and RLE encodings.
func TestLoadDecodedData(t *testing.T) {
	rawImg, rleImg, rawFnt, rleFnt := 0, 0, 0, 0
	for name, p := range shippedPackages(t) {
		r := New(64 << 20)
		if err := r.Load(p); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		for fn, b := range p.Files {
			var want []byte
			switch {
			case strings.HasSuffix(fn, ".img"):
				want = expectedImage(t, b)
				if b[4] == 0 {
					rawImg++
				} else {
					rleImg++
				}
			case strings.HasSuffix(fn, ".fnt"):
				want = expectedMasks(t, b)
				if b[3]&1 == 0 {
					rawFnt++
				} else {
					rleFnt++
				}
			default:
				continue
			}
			got, rc := testHash(r, strings.TrimSuffix(strings.TrimSuffix(fn, ".img"), ".fnt"))
			if rc != zzaOK || got != fnv32(want) {
				t.Errorf("%s/%s: hash %08x rc %d, want %08x", name, fn, got, rc, fnv32(want))
			}
		}
		r.Close()
	}
	t.Logf("covered: raw img %d, RLE img %d, raw fnt %d, RLE fnt %d", rawImg, rleImg, rawFnt, rleFnt)
	if rleImg == 0 || rleFnt == 0 || rawFnt == 0 {
		t.Errorf("shipped templates do not cover every font/RLE encoding")
	}
}

// TestLoadRawImage covers enc 0 (the exporter only writes it when RLE is not smaller).
func TestLoadRawImage(t *testing.T) {
	for _, base := range shippedPackages(t) {
		if _, ok := base.Files["i0.img"]; !ok {
			continue
		}
		p := clonePkg(base)
		raw := []byte{2, 0, 1, 0, 0, 1, 2, 3, 4, 5, 6, 7, 8}
		setFile(t, p, "i0.img", raw)
		r := New(64 << 20)
		defer r.Close()
		if err := r.Load(p); err != nil {
			t.Fatal(err)
		}
		if got, rc := testHash(r, "i0"); rc != zzaOK || got != fnv32(raw[5:]) {
			t.Fatalf("hash %08x rc %d", got, rc)
		}
		return
	}
	t.Skip("no package with images")
}

// TestLoadPlausibility: the loader enforces the exporter's limits on images
// (at most 6 291 456 pixels) and glyph masks (sides up to 1024).
func TestLoadPlausibility(t *testing.T) {
	font := func(w, h int) []byte {
		b := []byte("ZF1\x00")
		b = binary.LittleEndian.AppendUint16(b, 10)
		b = binary.LittleEndian.AppendUint16(b, 2)
		b = binary.LittleEndian.AppendUint16(b, 12)
		b = binary.LittleEndian.AppendUint16(b, 1)
		b = binary.LittleEndian.AppendUint32(b, 0)
		b = append(b, '?')
		b = binary.LittleEndian.AppendUint32(b, 640)
		b = binary.LittleEndian.AppendUint16(b, 0)
		b = binary.LittleEndian.AppendUint16(b, 0xFFF6)
		b = binary.LittleEndian.AppendUint16(b, uint16(w))
		b = binary.LittleEndian.AppendUint16(b, uint16(h))
		b = binary.LittleEndian.AppendUint32(b, 0)
		return append(b, make([]byte, w*h)...)
	}
	imgFile := func(w, h int) []byte {
		return devexport.EncodeImage(image.NewNRGBA(image.Rect(0, 0, w, h))) // transparent: RLE
	}
	tj := []byte(`{"layers":[]}`)
	for _, c := range []struct {
		name, file string
		data       []byte
		ok         bool
	}{
		{"glyph 1024", "f0.fnt", font(1024, 1), true},
		{"glyph 1025 wide", "f0.fnt", font(1025, 1), false},
		{"glyph 1025 high", "f0.fnt", font(1, 1025), false},
		{"image 2048x3072 = 6291456 px", "i0.img", imgFile(2048, 3072), true},
		{"image 2049x3072", "i0.img", imgFile(2049, 3072), false},
		{"image 65535x97", "i0.img", imgFile(65535, 97), false},
	} {
		t.Run(c.name, func(t *testing.T) {
			r := New(64 << 20)
			defer r.Close()
			err := r.Load(rawPackage(t, map[string][]byte{c.file: c.data, "t.json": tj}))
			if c.ok && err != nil {
				t.Fatalf("Load: %v", err)
			}
			if !c.ok && (!errors.Is(err, ErrFormat) || !strings.Contains(err.Error(), "too large")) {
				t.Fatalf("Load: %v, want ErrFormat … too large", err)
			}
		})
	}
}

// TestClockFromManifest: zza_clock reads manifest "clock"; a missing field is
// none, any other value than none|minute|second is a format error.
func TestClockFromManifest(t *testing.T) {
	base := shippedPackages(t)["umuc-105-v1"]
	if base == nil {
		t.Skip("umuc-105-v1 not exportable")
	}
	withClock := func(v string) *devexport.Package {
		p := clonePkg(base)
		var m map[string]any
		if err := json.Unmarshal(p.Files["manifest.json"], &m); err != nil {
			t.Fatal(err)
		}
		if v == "" {
			delete(m, "clock")
		} else {
			m["clock"] = v
		}
		b, err := json.Marshal(m)
		if err != nil {
			t.Fatal(err)
		}
		p.Files["manifest.json"] = b
		return p
	}
	for _, tc := range []struct {
		clock string
		want  int
	}{{"", 0}, {"none", 0}, {"minute", 1}, {"second", 2}} {
		r := loadPkg(t, withClock(tc.clock))
		if got := testClock(r); got != tc.want {
			t.Errorf("clock %q: zza_clock = %d, want %d", tc.clock, got, tc.want)
		}
	}
	r := New(32 << 20)
	defer r.Close()
	if got := testClock(r); got != 0 {
		t.Errorf("without package: %d, want 0", got)
	}
	err := r.Load(withClock("hourly"))
	if !errors.Is(err, ErrFormat) || !strings.Contains(err.Error(), "manifest.json: invalid clock") {
		t.Errorf("clock hourly: %v, want ErrFormat \"manifest.json: invalid clock\"", err)
	}
}
