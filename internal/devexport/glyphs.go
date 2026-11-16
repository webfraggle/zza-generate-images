package devexport

import (
	"encoding/binary"
	"fmt"
	"image"
	"image/draw"
	"math"
	"slices"
	"sort"

	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

// fontKey is one font combination: font file plus size and the scales that are
// baked into the rasterised masks.
type fontKey struct {
	File         string
	Size, SX, SY float64
}

func (k fontKey) less(o fontKey) bool {
	if k.File != o.File {
		return k.File < o.File
	}
	if k.Size != o.Size {
		return k.Size < o.Size
	}
	if k.SX != o.SX {
		return k.SX < o.SX
	}
	return k.SY < o.SY
}

// glyphSet is the set of code points a font combination needs.
type glyphSet struct {
	chars   map[rune]bool
	dynamic bool // some text of the combination contains {{ }}: full set
}

// runes returns the sorted code points to rasterise: the constant characters
// (0x20–0xFF) plus '?', or 0x20–0xFF for dynamic texts.
func (g *glyphSet) runes() []rune {
	var out []rune
	for r := rune(0x20); r <= 0xFF; r++ {
		if g.dynamic || g.chars[r] || r == '?' {
			out = append(out, r)
		}
	}
	return out
}

const (
	fntHeaderSize = 16
	fntEntrySize  = 17
	fntKernSize   = 6
)

// BuildFont rasterises the given code points (0x20–0xFF, sorted and de-duplicated
// here) of one font combination into the .fnt format.
func BuildFont(fontData []byte, k fontKey, runes []rune) ([]byte, error) {
	runes = slices.Clone(runes)
	slices.Sort(runes)
	runes = slices.Compact(runes)
	otf, err := opentype.Parse(fontData)
	if err != nil {
		return nil, err
	}
	face, err := opentype.NewFace(otf, &opentype.FaceOptions{Size: k.Size, DPI: 72})
	if err != nil {
		return nil, err
	}
	defer face.Close()
	m := face.Metrics()
	asc, desc, h := m.Ascent.Round(), m.Descent.Round(), m.Height.Round()
	if k.SY != 1 {
		asc = int(math.Round(float64(asc) * k.SY))
		h = int(math.Round(float64(h) * k.SY))
	}

	type entry struct {
		r      rune
		adv    int32
		bx, by int16
		w, h   uint16
		mask   []byte
		rle    []byte
	}
	ents := make([]entry, 0, len(runes))
	for _, r := range runes {
		adv, _ := face.GlyphAdvance(r)
		e := entry{r: r, adv: int32(adv)}
		dr, mask, mp, _, ok := face.Glyph(fixed.Point26_6{}, r)
		if ok && !dr.Empty() {
			a := image.NewAlpha(image.Rect(0, 0, dr.Dx(), dr.Dy()))
			draw.Draw(a, a.Bounds(), mask, mp, draw.Src)
			bx, by := dr.Min.X, dr.Min.Y
			if k.SX != 1 || k.SY != 1 {
				w := int(math.Ceil(float64(dr.Dx()) * k.SX))
				hh := int(math.Ceil(float64(dr.Dy()) * k.SY))
				s := image.NewAlpha(image.Rect(0, 0, w, hh))
				xdraw.CatmullRom.Scale(s, s.Bounds(), a, a.Bounds(), xdraw.Src, nil)
				a = s
				bx = int(math.Round(float64(bx) * k.SX))
				by = int(math.Round(float64(by) * k.SY))
			}
			e.bx, e.by = int16(bx), int16(by)
			e.w, e.h = uint16(a.Bounds().Dx()), uint16(a.Bounds().Dy())
			e.mask = a.Pix
			e.rle = EncodeRLE(a.Pix, 1)
		}
		ents = append(ents, e)
	}

	rawLen, rleLen := 0, 0
	for _, e := range ents {
		rawLen += len(e.mask)
		rleLen += len(e.rle)
	}
	useRLE := rleLen < rawLen

	var kern []byte
	nKern := 0
	for _, l := range runes {
		for _, r := range runes {
			if kv := face.Kern(l, r); kv != 0 {
				kern = binary.LittleEndian.AppendUint16(kern, uint16(l)<<8|uint16(r))
				kern = binary.LittleEndian.AppendUint32(kern, uint32(int32(kv)))
				nKern++
			}
		}
	}

	flags := byte(0)
	if useRLE {
		flags = 1
	}
	out := make([]byte, 0, fntHeaderSize+len(ents)*fntEntrySize+len(kern)+rawLen)
	out = append(out, 'Z', 'F', '1', flags)
	out = binary.LittleEndian.AppendUint16(out, uint16(int16(asc)))
	out = binary.LittleEndian.AppendUint16(out, uint16(int16(desc)))
	out = binary.LittleEndian.AppendUint16(out, uint16(int16(h)))
	out = binary.LittleEndian.AppendUint16(out, uint16(len(ents)))
	out = binary.LittleEndian.AppendUint32(out, uint32(nKern))
	var masks []byte
	for _, e := range ents {
		out = append(out, byte(e.r))
		out = binary.LittleEndian.AppendUint32(out, uint32(e.adv))
		out = binary.LittleEndian.AppendUint16(out, uint16(e.bx))
		out = binary.LittleEndian.AppendUint16(out, uint16(e.by))
		out = binary.LittleEndian.AppendUint16(out, e.w)
		out = binary.LittleEndian.AppendUint16(out, e.h)
		out = binary.LittleEndian.AppendUint32(out, uint32(len(masks)))
		if useRLE {
			masks = append(masks, e.rle...)
		} else {
			masks = append(masks, e.mask...)
		}
	}
	out = append(out, kern...)
	out = append(out, masks...)
	if len(out) > math.MaxUint32 {
		return nil, fmt.Errorf("font too large")
	}
	return out, nil
}

// sortedFontKeys returns the keys of m in a deterministic order.
func sortedFontKeys[V any](m map[fontKey]V) []fontKey {
	keys := make([]fontKey, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i].less(keys[j]) })
	return keys
}
