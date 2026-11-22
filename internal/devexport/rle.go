package devexport

import "errors"

// EncodeRLE compresses data made of bpp-byte pixels. Control byte c < 128:
// c+1 literal pixels follow; c >= 128: the next pixel repeats c-126 times
// (2..129). len(data) must be a multiple of bpp.
func EncodeRLE(data []byte, bpp int) []byte {
	n := len(data) / bpp
	same := func(i, j int) bool {
		for k := 0; k < bpp; k++ {
			if data[i*bpp+k] != data[j*bpp+k] {
				return false
			}
		}
		return true
	}
	runAt := func(i int) int {
		run := 1
		for i+run < n && run < 129 && same(i, i+run) {
			run++
		}
		return run
	}
	// A run of two bytes (bpp 1) costs as much as two literals and would only
	// split the surrounding literal segment.
	minRun := 2
	if bpp == 1 {
		minRun = 3
	}
	var out []byte
	for i := 0; i < n; {
		run := runAt(i)
		if run >= minRun {
			out = append(out, byte(run+126))
			out = append(out, data[i*bpp:(i+1)*bpp]...)
			i += run
			continue
		}
		j := i + 1
		for j < n && j-i < 128 && runAt(j) < minRun {
			j++
		}
		out = append(out, byte(j-i-1))
		out = append(out, data[i*bpp:j*bpp]...)
		i = j
	}
	return out
}

// DecodeRLE expands RLE data to exactly npix pixels of bpp bytes. It is the
// reference for the device decoder. Trailing bytes after npix pixels are an error.
func DecodeRLE(data []byte, bpp, npix int) ([]byte, error) {
	out, used, err := DecodeRLEPrefix(data, bpp, npix)
	if err == nil && used != len(data) {
		return nil, errors.New("rle: trailing data")
	}
	return out, err
}

// DecodeRLEPrefix expands the first npix pixels of data and reports how many
// input bytes they used (the glyph masks of a .fnt are stored back to back).
func DecodeRLEPrefix(data []byte, bpp, npix int) ([]byte, int, error) {
	out := make([]byte, 0, npix*bpp)
	for p := 0; len(out) < npix*bpp; {
		if p >= len(data) {
			return nil, 0, errors.New("rle: truncated")
		}
		c := int(data[p])
		p++
		if c < 128 {
			l := (c + 1) * bpp
			if p+l > len(data) || len(out)+l > npix*bpp {
				return nil, 0, errors.New("rle: literal overruns")
			}
			out = append(out, data[p:p+l]...)
			p += l
		} else {
			if p+bpp > len(data) || len(out)+(c-126)*bpp > npix*bpp {
				return nil, 0, errors.New("rle: run overruns")
			}
			for k := 0; k < c-126; k++ {
				out = append(out, data[p:p+bpp]...)
			}
			p += bpp
		}
		if len(out) == npix*bpp {
			return out, p, nil
		}
	}
	return out, 0, nil
}
