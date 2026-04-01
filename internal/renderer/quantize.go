package renderer

import (
	"image"
	"image/color"
	"sort"
)

// Quantize reduces src to a palette of at most n colors using the median-cut
// algorithm and returns an indexed *image.Paletted. n is clamped to [2, 256].
// No dithering is applied — each pixel is mapped to its nearest palette color.
func Quantize(src *image.NRGBA, n int) *image.Paletted {
	if n < 2 {
		n = 2
	}
	if n > 256 {
		n = 256
	}

	bounds := src.Bounds()
	pixels := make([]color.NRGBA, 0, bounds.Dx()*bounds.Dy())
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			pixels = append(pixels, src.NRGBAAt(x, y))
		}
	}

	palette := medianCut(pixels, n)

	p := make(color.Palette, len(palette))
	for i, c := range palette {
		p[i] = c
	}

	out := image.NewPaletted(bounds, p)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			out.SetColorIndex(x, y, uint8(p.Index(src.NRGBAAt(x, y))))
		}
	}
	return out
}

// medianCut partitions pixels into at most n buckets using the median-cut
// algorithm and returns the average color of each bucket as a palette.
func medianCut(pixels []color.NRGBA, n int) []color.NRGBA {
	if len(pixels) == 0 {
		return []color.NRGBA{{R: 0, G: 0, B: 0, A: 255}}
	}

	buckets := [][]color.NRGBA{append([]color.NRGBA(nil), pixels...)}

	for len(buckets) < n {
		idx := bucketWithLargestRange(buckets)
		b := buckets[idx]
		if len(b) <= 1 {
			break // cannot split further
		}
		axis := dominantAxis(b)
		sortByAxis(b, axis)
		mid := len(b) / 2
		buckets[idx] = b[:mid]
		buckets = append(buckets, b[mid:])
	}

	result := make([]color.NRGBA, len(buckets))
	for i, b := range buckets {
		result[i] = avgColor(b)
	}
	return result
}

// bucketWithLargestRange returns the index of the bucket with the greatest
// color range along any single axis.
func bucketWithLargestRange(buckets [][]color.NRGBA) int {
	best, bestRange := 0, -1
	for i, b := range buckets {
		_, r := dominantAxisAndRange(b)
		if r > bestRange {
			bestRange = r
			best = i
		}
	}
	return best
}

// dominantAxis returns the axis (0=R, 1=G, 2=B) with the largest value range in b.
func dominantAxis(b []color.NRGBA) int {
	ax, _ := dominantAxisAndRange(b)
	return ax
}

// dominantAxisAndRange returns the dominant axis and its value range for bucket b.
func dominantAxisAndRange(b []color.NRGBA) (axis int, rangeVal int) {
	if len(b) == 0 {
		return 0, 0
	}
	minR, maxR := int(b[0].R), int(b[0].R)
	minG, maxG := int(b[0].G), int(b[0].G)
	minB, maxB := int(b[0].B), int(b[0].B)
	for _, c := range b[1:] {
		if int(c.R) < minR {
			minR = int(c.R)
		}
		if int(c.R) > maxR {
			maxR = int(c.R)
		}
		if int(c.G) < minG {
			minG = int(c.G)
		}
		if int(c.G) > maxG {
			maxG = int(c.G)
		}
		if int(c.B) < minB {
			minB = int(c.B)
		}
		if int(c.B) > maxB {
			maxB = int(c.B)
		}
	}
	rR, rG, rB := maxR-minR, maxG-minG, maxB-minB
	if rR >= rG && rR >= rB {
		return 0, rR
	}
	if rG >= rB {
		return 1, rG
	}
	return 2, rB
}

// sortByAxis sorts b in-place by the given axis (0=R, 1=G, 2=B).
func sortByAxis(b []color.NRGBA, axis int) {
	sort.Slice(b, func(i, j int) bool {
		switch axis {
		case 0:
			return b[i].R < b[j].R
		case 1:
			return b[i].G < b[j].G
		default:
			return b[i].B < b[j].B
		}
	})
}

// avgColor returns the per-channel average of all pixels in b.
func avgColor(b []color.NRGBA) color.NRGBA {
	if len(b) == 0 {
		return color.NRGBA{A: 255}
	}
	var sumR, sumG, sumB int
	for _, c := range b {
		sumR += int(c.R)
		sumG += int(c.G)
		sumB += int(c.B)
	}
	n := len(b)
	return color.NRGBA{
		R: uint8(sumR / n),
		G: uint8(sumG / n),
		B: uint8(sumB / n),
		A: 255,
	}
}
