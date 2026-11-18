package devexport

import (
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/draw"
	_ "image/jpeg" // decoders for the formats renderImage permits
	_ "image/png"
	"io"
	"math"
	"os"

	xdraw "golang.org/x/image/draw"
)

const (
	maxImageFileBytes = 16 << 20
	maxImageDim       = 16384 // renderer.maxCanvasDimension
	// maxImagePixels: the most pixels of a source image (and of every .img in a
	// package); the device core enforces the same limit when loading.
	maxImagePixels = maxPackageBytes * 2 // 6 291 456 (= 8 × package size / 4 bytes)
	maxImageSide   = 0xFFFF              // .img stores u16 sides
)

// errImageTooBig: the image's declared size exceeds what the device accepts.
var errImageTooBig = errors.New("zu groß für das Gerät")

// imgKey identifies one exported image: the template file and the constant
// target size (w = h = 0: the original size).
type imgKey struct {
	File string
	W, H int
}

// loadImage reads and decodes a template image as non-premultiplied NRGBA.
func loadImage(path string) (*image.NRGBA, error) {
	st, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if st.Size() > maxImageFileBytes {
		return nil, fmt.Errorf("Datei größer als %d MB", maxImageFileBytes>>20)
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	// Check the declared size before decoding, so a small file declaring a huge
	// image is rejected without allocating it.
	cfg, _, err := image.DecodeConfig(f)
	if err != nil {
		return nil, err
	}
	if cfg.Width > maxImageSide || cfg.Height > maxImageSide || cfg.Width*cfg.Height > maxImagePixels {
		return nil, errImageTooBig
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	src, _, err := image.Decode(f)
	if err != nil {
		return nil, err
	}
	b := src.Bounds()
	n := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(n, n.Bounds(), src, b.Min, draw.Src)
	return n, nil
}

// scaleLike scales src exactly like renderer.renderImage: a missing side keeps
// the aspect ratio (rounded, at least 1), CatmullRom with Over.
func scaleLike(src *image.NRGBA, w, h int) *image.NRGBA {
	w, h = scaledSize(src, w, h)
	dst := image.NewNRGBA(image.Rect(0, 0, w, h))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), xdraw.Over, nil)
	return dst
}

// EncodeImage serialises an NRGBA image as .img (see doc.go); RLE only if smaller than raw.
func EncodeImage(img *image.NRGBA) []byte {
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	raw := img.Pix
	if img.Stride != w*4 { // compact
		raw = make([]byte, 0, w*h*4)
		for y := 0; y < h; y++ {
			raw = append(raw, img.Pix[y*img.Stride:y*img.Stride+w*4]...)
		}
	}
	enc, data := byte(0), raw
	if r := EncodeRLE(raw, 4); len(r) < len(raw) {
		enc, data = 1, r
	}
	out := make([]byte, 5, 5+len(data))
	binary.LittleEndian.PutUint16(out[0:], uint16(w))
	binary.LittleEndian.PutUint16(out[2:], uint16(h))
	out[4] = enc
	return append(out, data...)
}

// DecodeImage is the inverse of EncodeImage.
func DecodeImage(b []byte) (*image.NRGBA, error) {
	if len(b) < 5 {
		return nil, fmt.Errorf("img: too short")
	}
	w, h := int(binary.LittleEndian.Uint16(b[0:])), int(binary.LittleEndian.Uint16(b[2:]))
	pix := b[5:]
	switch b[4] {
	case 0:
		if len(pix) != w*h*4 {
			return nil, fmt.Errorf("img: raw size %d, want %d", len(pix), w*h*4)
		}
	case 1:
		var err error
		if pix, err = DecodeRLE(pix, 4, w*h); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("img: unknown encoding %d", b[4])
	}
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	copy(img.Pix, pix)
	return img, nil
}

// scaledSize is the size renderer.renderImage scales to; (0,0) stays original.
func scaledSize(src *image.NRGBA, w, h int) (int, int) {
	if w <= 0 && h <= 0 {
		return 0, 0
	}
	sw, sh := src.Bounds().Dx(), src.Bounds().Dy()
	if w <= 0 {
		w = max(1, int(math.Round(float64(sw)*float64(h)/float64(sh))))
	}
	if h <= 0 {
		h = max(1, int(math.Round(float64(sh)*float64(w)/float64(sw))))
	}
	return w, h
}
