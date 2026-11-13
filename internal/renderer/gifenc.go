package renderer

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	"io"
	"sort"
)

const (
	gifMaxSampleFrames = 8   // first + last + up to 6 in between
	gifFinalDelay      = 100 // hundredths of a second: the still image stays 1 s
)

// EncodeGIF writes anim as a GIF that plays exactly once and ends on the last
// frame. colors is the number of real colours in the global palette (0 means
// 256, otherwise 2–256), capped at 255; the transparent entry is added on top,
// so colors: 2 gives two real colours plus transparency (palette length 3) and
// 256 gives 255 + 1.
//
// All frames share one global colour table and no local tables. If all frames
// together use at most that many distinct colours, exactly those form the
// palette; otherwise a mosaic of sample frames is quantized with Quantize.
// Frame 0 is stored in full. Later frames are delta rectangles (DisposalNone)
// covering the bounding box of changed pixels; inside it, pixels equal to the
// previous composited frame are written as the transparent index, which
// compresses far better. Consecutive identical frames are merged by adding
// their delay. The output is deterministic for identical input.
func EncodeGIF(w io.Writer, anim *Animation, colors int) error {
	if anim == nil || len(anim.Frames) == 0 {
		return fmt.Errorf("renderer: EncodeGIF: animation has no frames")
	}
	if anim.FPS <= 0 {
		return fmt.Errorf("renderer: EncodeGIF: fps must be positive (got %d)", anim.FPS)
	}
	if colors == 0 {
		colors = 256
	}
	if colors < 2 || colors > 256 {
		return fmt.Errorf("renderer: EncodeGIF: colors must be between 2 and 256 (got %d)", colors)
	}
	bounds := image.Rect(0, 0, 0, 0)
	for i, f := range anim.Frames {
		if f == nil {
			return fmt.Errorf("renderer: EncodeGIF: frame %d is nil", i)
		}
		if i == 0 {
			bounds = f.Bounds()
		} else if f.Bounds() != bounds {
			return fmt.Errorf("renderer: EncodeGIF: frame %d has bounds %v, want %v", i, f.Bounds(), bounds)
		}
	}
	if bounds.Empty() {
		return fmt.Errorf("renderer: EncodeGIF: frames are empty")
	}
	if bounds.Min != (image.Point{}) {
		return fmt.Errorf("renderer: EncodeGIF: frame bounds %v must start at (0,0)", bounds)
	}

	pal := gifPalette(anim.Frames, colors)
	transparent := uint8(len(pal) - 1)
	realPal := pal[:transparent]          // the transparent entry is never a nearest match
	lookup := make(map[color.NRGBA]uint8) // only used for lookups, never iterated

	delay := (100 + anim.FPS/2) / anim.FPS
	if delay < 1 {
		delay = 1
	}

	out := &gif.GIF{LoopCount: -1, Config: image.Config{ColorModel: pal, Width: bounds.Dx(), Height: bounds.Dy()}}
	var prev *image.Paletted
	for i, f := range anim.Frames {
		cur := image.NewPaletted(bounds, pal)
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			for x := bounds.Min.X; x < bounds.Max.X; x++ {
				c := opaque(f.NRGBAAt(x, y))
				idx, ok := lookup[c]
				if !ok {
					idx = uint8(realPal.Index(c))
					lookup[c] = idx
				}
				cur.SetColorIndex(x, y, idx)
			}
		}

		d := delay
		if i == len(anim.Frames)-1 {
			d = gifFinalDelay
		}

		if prev == nil {
			out.Image = append(out.Image, cur)
			out.Delay = append(out.Delay, d)
			out.Disposal = append(out.Disposal, gif.DisposalNone)
		} else if r := changedRect(prev, cur); r.Empty() {
			out.Delay[len(out.Delay)-1] += d
		} else {
			sub := image.NewPaletted(r, pal)
			for y := r.Min.Y; y < r.Max.Y; y++ {
				for x := r.Min.X; x < r.Max.X; x++ {
					if v := cur.ColorIndexAt(x, y); v != prev.ColorIndexAt(x, y) {
						sub.SetColorIndex(x, y, v)
					} else {
						sub.SetColorIndex(x, y, transparent)
					}
				}
			}
			out.Image = append(out.Image, sub)
			out.Delay = append(out.Delay, d)
			out.Disposal = append(out.Disposal, gif.DisposalNone)
		}
		prev = cur
	}

	if err := gif.EncodeAll(w, out); err != nil {
		return fmt.Errorf("renderer: EncodeGIF: %w", err)
	}
	return nil
}

// opaque drops the alpha channel: GIF palettes here are always opaque and
// Quantize ignores alpha as well.
func opaque(c color.NRGBA) color.NRGBA {
	c.A = 255
	return c
}

// distinctColors returns the distinct (opaque) colours over all frames sorted
// by R,G,B, or ok=false as soon as there are more than limit of them.
func distinctColors(frames []*image.NRGBA, limit int) (cols []color.NRGBA, ok bool) {
	seen := make(map[color.NRGBA]struct{}) // collection only, never iterated
	for _, f := range frames {
		b := f.Bounds()
		for y := b.Min.Y; y < b.Max.Y; y++ {
			for x := b.Min.X; x < b.Max.X; x++ {
				c := opaque(f.NRGBAAt(x, y))
				if _, dup := seen[c]; dup {
					continue
				}
				seen[c] = struct{}{}
				cols = append(cols, c)
				if len(cols) > limit {
					return nil, false
				}
			}
		}
	}
	sort.Slice(cols, func(i, j int) bool {
		a, b := cols[i], cols[j]
		if a.R != b.R {
			return a.R < b.R
		}
		if a.G != b.G {
			return a.G < b.G
		}
		return a.B < b.B
	})
	return cols, true
}

// gifPalette returns the global palette: real colours followed by one fully
// transparent entry (always last, never used for real pixels). With at most
// min(colors, 255) distinct colours in total those are used exactly; otherwise a
// side-by-side mosaic of sample frames (first, last and up to six evenly
// spaced in between) is quantized.
func gifPalette(frames []*image.NRGBA, colors int) color.Palette {
	realColors := min(colors, 255) // the transparent entry comes on top
	var pal color.Palette
	if cols, ok := distinctColors(frames, realColors); ok {
		for _, c := range cols {
			pal = append(pal, c)
		}
		return append(pal, color.NRGBA{})
	}
	n := len(frames)
	idxs := []int{0}
	if n > 1 {
		mid := n - 2 // frames strictly between first and last
		if mid > gifMaxSampleFrames-2 {
			mid = gifMaxSampleFrames - 2
		}
		for k := 1; k <= mid; k++ {
			idxs = append(idxs, k*(n-1)/(mid+1))
		}
		idxs = append(idxs, n-1)
	}
	// Drop duplicates (possible for very short sequences); order stays ascending.
	uniq := idxs[:0]
	for _, v := range idxs {
		if len(uniq) == 0 || uniq[len(uniq)-1] != v {
			uniq = append(uniq, v)
		}
	}

	b := frames[0].Bounds()
	mosaic := image.NewNRGBA(image.Rect(0, 0, b.Dx()*len(uniq), b.Dy()))
	for k, fi := range uniq {
		dst := image.Rect(k*b.Dx(), 0, (k+1)*b.Dx(), b.Dy())
		draw.Draw(mosaic, dst, frames[fi], b.Min, draw.Src)
	}

	pal = Quantize(mosaic, realColors).Palette
	return append(pal, color.NRGBA{})
}

// changedRect returns the bounding rectangle of pixels that differ between a
// and b (same bounds), or an empty rectangle if they are identical.
func changedRect(a, b *image.Paletted) image.Rectangle {
	bd := a.Bounds()
	minX, minY, maxX, maxY := bd.Max.X, bd.Max.Y, bd.Min.X-1, bd.Min.Y-1
	for y := bd.Min.Y; y < bd.Max.Y; y++ {
		ra := a.Pix[a.PixOffset(bd.Min.X, y):][:bd.Dx()]
		rb := b.Pix[b.PixOffset(bd.Min.X, y):][:bd.Dx()]
		for i := range ra {
			if ra[i] == rb[i] {
				continue
			}
			x := bd.Min.X + i
			if x < minX {
				minX = x
			}
			if x > maxX {
				maxX = x
			}
			if y < minY {
				minY = y
			}
			maxY = y
		}
	}
	if maxX < minX {
		return image.Rectangle{}
	}
	return image.Rect(minX, minY, maxX+1, maxY+1)
}
