package mcp

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	"math"
	"strconv"
)

var (
	errBadQuad  = errors.New("corners do not form a valid quadrilateral")
	errWrongDir = errors.New("corners are in the wrong order: use top-left, top-right, bottom-right, bottom-left (clockwise)")
)

// maxCornerCoord bounds corner coordinates (far beyond any real photo).
const maxCornerCoord = 1e6

type point struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// homography returns the projective transform h (row-major 3x3, h[8] = 1)
// that maps destination pixel coordinates of a w x h image (corners
// (0,0), (w-1,0), (w-1,h-1), (0,h-1)) onto the source points src
// (top-left, top-right, bottom-right, bottom-left).
func homography(src [4]point, w, h int) ([9]float64, error) {
	var zero [9]float64
	if w < 1 || h < 1 {
		return zero, errBadQuad
	}
	for _, p := range src {
		if math.IsNaN(p.X) || math.IsNaN(p.Y) || math.Abs(p.X) > maxCornerCoord || math.Abs(p.Y) > maxCornerCoord {
			return zero, errBadQuad
		}
	}
	if err := checkQuad(src); err != nil {
		return zero, err
	}
	fw, fh := math.Max(float64(w-1), 1), math.Max(float64(h-1), 1)
	dst := [4]point{{0, 0}, {fw, 0}, {fw, fh}, {0, fh}}

	var m [8][9]float64 // augmented matrix
	for i := 0; i < 4; i++ {
		u, v := dst[i].X, dst[i].Y
		x, y := src[i].X, src[i].Y
		m[2*i] = [9]float64{u, v, 1, 0, 0, 0, -x * u, -x * v, x}
		m[2*i+1] = [9]float64{0, 0, 0, u, v, 1, -y * u, -y * v, y}
	}
	// Gauss-Jordan elimination with partial pivoting.
	for col := 0; col < 8; col++ {
		p := col
		for r := col + 1; r < 8; r++ {
			if math.Abs(m[r][col]) > math.Abs(m[p][col]) {
				p = r
			}
		}
		if math.Abs(m[p][col]) < 1e-9 {
			return zero, errBadQuad
		}
		m[col], m[p] = m[p], m[col]
		for r := 0; r < 8; r++ {
			if r == col {
				continue
			}
			f := m[r][col] / m[col][col]
			for c := col; c < 9; c++ {
				m[r][c] -= f * m[col][c]
			}
		}
	}
	var out [9]float64
	for i := 0; i < 8; i++ {
		out[i] = m[i][8] / m[i][i]
		if math.IsNaN(out[i]) || math.IsInf(out[i], 0) {
			return zero, errBadQuad
		}
	}
	out[8] = 1
	return out, nil
}

// checkQuad requires a non-degenerate convex quadrilateral whose corners run
// clockwise on screen (y down): top-left, top-right, bottom-right, bottom-left.
func checkQuad(q [4]point) error {
	sign := 0
	for i := 0; i < 4; i++ {
		a, b, c := q[i], q[(i+1)%4], q[(i+2)%4]
		cr := (b.X-a.X)*(c.Y-b.Y) - (b.Y-a.Y)*(c.X-b.X)
		if math.Abs(cr) < 1e-9 {
			return errBadQuad
		}
		s := 1
		if cr < 0 {
			s = -1
		}
		if sign == 0 {
			sign = s
		} else if s != sign {
			return errBadQuad
		}
	}
	if sign < 0 {
		return errWrongDir
	}
	return nil
}

func applyH(h [9]float64, u, v float64) (float64, float64) {
	d := h[6]*u + h[7]*v + h[8]
	return (h[0]*u + h[1]*v + h[2]) / d, (h[3]*u + h[4]*v + h[5]) / d
}

// warpPerspective cuts the quadrilateral corners (in source pixel
// coordinates) out of src and maps it onto a w x h image, sampling
// bilinearly. Areas outside the source are transparent.
func warpPerspective(src *image.NRGBA, corners [4]point, w, h int) (*image.NRGBA, error) {
	hm, err := homography(corners, w, h)
	if err != nil {
		return nil, err
	}
	out := image.NewNRGBA(image.Rect(0, 0, w, h))
	sw, sh := src.Bounds().Dx(), src.Bounds().Dy()
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			// Pixel centre (x+0.5) mapped into the homography's [0,w-1] range,
			// so edge pixels sample inside the quadrilateral.
			u := (float64(x) + 0.5) * math.Max(float64(w-1), 1) / float64(w)
			v := (float64(y) + 0.5) * math.Max(float64(h-1), 1) / float64(h)
			sx, sy := applyH(hm, u, v)
			if math.IsNaN(sx) || math.IsNaN(sy) || sx < -0.5 || sy < -0.5 || sx > float64(sw)-0.5 || sy > float64(sh)-0.5 {
				continue
			}
			out.SetNRGBA(x, y, bilinear(src, sx, sy))
		}
	}
	return out, nil
}

// bilinear samples src at (x, y) where integer coordinates are pixel centres;
// edge pixels are clamped. Colours are weighted by alpha.
func bilinear(src *image.NRGBA, x, y float64) color.NRGBA {
	sw, sh := src.Bounds().Dx(), src.Bounds().Dy()
	x0, y0 := int(math.Floor(x)), int(math.Floor(y))
	fx, fy := x-float64(x0), y-float64(y0)
	var r, g, b, a float64
	for dy := 0; dy < 2; dy++ {
		for dx := 0; dx < 2; dx++ {
			wgt := 1 - fx
			if dx == 1 {
				wgt = fx
			}
			if dy == 1 {
				wgt *= fy
			} else {
				wgt *= 1 - fy
			}
			px := min(max(x0+dx, 0), sw-1)
			py := min(max(y0+dy, 0), sh-1)
			c := src.NRGBAAt(px, py)
			wa := wgt * float64(c.A)
			r += wa * float64(c.R)
			g += wa * float64(c.G)
			b += wa * float64(c.B)
			a += wa
		}
	}
	if a <= 0 {
		return color.NRGBA{}
	}
	return color.NRGBA{
		R: uint8(math.Round(r / a)), G: uint8(math.Round(g / a)), B: uint8(math.Round(b / a)),
		A: uint8(math.Round(a)),
	}
}

// colorToAlpha returns a copy of img in which every pixel within tol (per
// channel) of c is fully transparent.
func colorToAlpha(img *image.NRGBA, c color.NRGBA, tol int) *image.NRGBA {
	out := image.NewNRGBA(img.Bounds())
	copy(out.Pix, img.Pix)
	for i := 0; i+3 < len(out.Pix); i += 4 {
		d := max(absInt(int(out.Pix[i])-int(c.R)), absInt(int(out.Pix[i+1])-int(c.G)), absInt(int(out.Pix[i+2])-int(c.B)))
		if d <= tol {
			out.Pix[i+3] = 0
		}
	}
	return out
}

func absInt(i int) int {
	if i < 0 {
		return -i
	}
	return i
}

// sampleAt returns the mean colour over the square (2r+1)^2 around (x, y),
// clipped to the image. Colour channels are weighted by alpha; the alpha of
// the result is the plain mean.
func sampleAt(img *image.NRGBA, x, y, r int) color.NRGBA {
	b := img.Bounds()
	var sr, sg, sb, sa, n int
	for yy := max(y-r, b.Min.Y); yy <= min(y+r, b.Max.Y-1); yy++ {
		for xx := max(x-r, b.Min.X); xx <= min(x+r, b.Max.X-1); xx++ {
			c := img.NRGBAAt(xx, yy)
			a := int(c.A)
			sr += int(c.R) * a
			sg += int(c.G) * a
			sb += int(c.B) * a
			sa += a
			n++
		}
	}
	if n == 0 || sa == 0 {
		return color.NRGBA{}
	}
	return color.NRGBA{uint8((sr + sa/2) / sa), uint8((sg + sa/2) / sa), uint8((sb + sa/2) / sa), uint8((sa + n/2) / n)}
}

// parseHexColor parses "#rrggbb".
func parseHexColor(s string) (color.NRGBA, error) {
	if len(s) != 7 || s[0] != '#' {
		return color.NRGBA{}, fmt.Errorf("color must be #rrggbb")
	}
	v, err := strconv.ParseUint(s[1:], 16, 32)
	if err != nil {
		return color.NRGBA{}, fmt.Errorf("color must be #rrggbb")
	}
	return color.NRGBA{uint8(v >> 16), uint8(v >> 8), uint8(v), 255}, nil
}
