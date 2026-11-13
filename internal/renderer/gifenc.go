package renderer

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	"io"
)

const (
	gifMaxSampleFrames = 8   // first + last + up to 6 in between
	gifFinalDelay      = 100 // hundredths of a second: the still image stays 1 s
)

// EncodeGIF writes anim as a GIF that plays exactly once and ends on the last
// frame. colors is the global palette size (0 means 256, otherwise 2–256).
//
// All frames share one global palette, built from a mosaic of sample frames.
// Frames after the first are stored as delta rectangles (DisposalNone) covering
// only the pixels that changed; consecutive identical frames are merged by
// adding their delay. The palette is always opaque (Quantize never emits
// transparency), so no index is ever a transparent "keep previous pixel" one.
// The output is deterministic for identical input.
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

	pal := gifPalette(anim.Frames, colors)
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
				c := f.NRGBAAt(x, y)
				idx, ok := lookup[c]
				if !ok {
					idx = uint8(pal.Index(c))
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
			sub := cur.SubImage(r).(*image.Paletted)
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

// gifPalette quantizes a side-by-side mosaic of sample frames (first, last and
// up to six evenly spaced in between) into the global palette.
func gifPalette(frames []*image.NRGBA, colors int) color.Palette {
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

	pal := Quantize(mosaic, colors).Palette
	for len(pal) < 2 { // a GIF palette needs at least two entries
		pal = append(pal, color.NRGBA{A: 255})
	}
	return pal
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
