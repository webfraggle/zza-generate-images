package mcp

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	xdraw "golang.org/x/image/draw"

	"github.com/webfraggle/zza-generate-images/internal/editor"
)

var (
	errNotImage        = errors.New("not a PNG or JPEG image")
	errReadFailed      = errors.New("cannot read")
	errCannotReadImage = errors.New("cannot read image file")
)

// loadImageFile reads and decodes a PNG or JPEG file. The size is checked
// before reading, the dimensions before decoding. Errors never contain file
// content. format is "png" or "jpeg".
func loadImageFile(path string, maxBytes int64, maxSide, maxPixels int) (image.Image, string, error) {
	img, format, _, err := loadImageBytes(path, maxBytes, maxSide, maxPixels)
	return img, format, err
}

// loadImageBytes is loadImageFile that also returns the raw file bytes.
func loadImageBytes(path string, maxBytes int64, maxSide, maxPixels int) (image.Image, string, []byte, error) {
	data, err := readRegularFile(path, maxBytes)
	switch {
	case errors.Is(err, errNotRegular):
		return nil, "", nil, errNotRegular
	case errors.Is(err, errFileTooLarge):
		return nil, "", nil, fmt.Errorf("image file too large (max %d MiB)", maxBytes>>20)
	case err != nil:
		return nil, "", nil, fmt.Errorf("%w %s", errReadFailed, path)
	}
	img, format, err := decodeImage(data, maxSide, maxPixels)
	if err != nil {
		return nil, "", nil, err
	}
	return img, format, data, nil
}

// decodeImage decodes PNG/JPEG bytes after checking the dimensions.
func decodeImage(data []byte, maxSide, maxPixels int) (image.Image, string, error) {
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || (format != "png" && format != "jpeg") {
		return nil, "", errNotImage
	}
	if cfg.Width <= 0 || cfg.Height <= 0 {
		return nil, "", errNotImage
	}
	if maxSide > 0 && (cfg.Width > maxSide || cfg.Height > maxSide) {
		return nil, "", fmt.Errorf("image too large: %dx%d (max %d px per side)", cfg.Width, cfg.Height, maxSide)
	}
	if maxPixels > 0 && cfg.Width*cfg.Height > maxPixels {
		if maxPixels%1_000_000 == 0 {
			return nil, "", fmt.Errorf("image too large: %dx%d (max %d megapixels)", cfg.Width, cfg.Height, maxPixels/1_000_000)
		}
		return nil, "", fmt.Errorf("image too large: %dx%d (max %d pixels)", cfg.Width, cfg.Height, maxPixels)
	}
	img, format, err := image.Decode(bytes.NewReader(data))
	if err != nil || (format != "png" && format != "jpeg") {
		return nil, "", errNotImage
	}
	return img, format, nil
}

// toNRGBA converts any image to a zero-based NRGBA copy.
func toNRGBA(img image.Image) *image.NRGBA {
	b := img.Bounds()
	dst := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(dst, dst.Bounds(), img, b.Min, draw.Src)
	return dst
}

// scaleNearest enlarges img by an integer factor without smoothing.
func scaleNearest(img image.Image, factor int) *image.NRGBA {
	src := toNRGBA(img)
	if factor < 1 {
		factor = 1
	}
	w, h := src.Bounds().Dx(), src.Bounds().Dy()
	dst := image.NewNRGBA(image.Rect(0, 0, w*factor, h*factor))
	for y := 0; y < h*factor; y++ {
		so := (y / factor) * src.Stride
		do := y * dst.Stride
		for x := 0; x < w*factor; x++ {
			copy(dst.Pix[do+x*4:do+x*4+4], src.Pix[so+(x/factor)*4:so+(x/factor)*4+4])
		}
	}
	return dst
}

// fitWithin shrinks img (CatmullRom) so that its longest side is at most
// maxSide. It never enlarges; smaller images are returned unchanged.
func fitWithin(img image.Image, maxSide int) image.Image {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	long := max(w, h)
	if long <= maxSide || maxSide < 1 {
		return img
	}
	nw, nh := max(1, (w*maxSide+long/2)/long), max(1, (h*maxSide+long/2)/long)
	dst := image.NewNRGBA(image.Rect(0, 0, nw, nh))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), img, b, xdraw.Src, nil)
	return dst
}

// imageContent encodes img as a PNG image content for a tool result.
func imageContent(img image.Image) (*sdk.ImageContent, error) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, fmt.Errorf("cannot encode image")
	}
	return &sdk.ImageContent{Data: buf.Bytes(), MIMEType: "image/png"}, nil
}

// encodeImageFor encodes img for storing under a file name: PNG for .png,
// JPEG (quality 90) for .jpg/.jpeg.
func encodeImageFor(name string, img image.Image) ([]byte, error) {
	var buf bytes.Buffer
	switch strings.ToLower(filepath.Ext(name)) {
	case ".png":
		if err := png.Encode(&buf, img); err != nil {
			return nil, fmt.Errorf("cannot encode image")
		}
	case ".jpg", ".jpeg":
		if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
			return nil, fmt.Errorf("cannot encode image")
		}
	default:
		return nil, fmt.Errorf("only .png, .jpg and .jpeg images are allowed")
	}
	return buf.Bytes(), nil
}

// writePNGFile stores img as PNG in the template (atomic, name must end in
// .png). replaced reports whether a file existed.
func writePNGFile(e *env, template, name string, img image.Image) (replaced bool, err error) {
	if strings.ToLower(filepath.Ext(name)) != ".png" {
		return false, fmt.Errorf("file name must end in .png")
	}
	data, err := encodeImageFor(name, img)
	if err != nil {
		return false, err
	}
	return writeAssetBytes(e, template, name, data)
}

var assetExts = map[string]bool{".png": true, ".jpg": true, ".jpeg": true, ".ttf": true, ".otf": true}

// writeAssetBytes atomically stores an asset file (image or font) in the
// template folder. It never overwrites non-regular files and enforces the
// per-template file and size limits. replaced reports whether a file existed.
func writeAssetBytes(e *env, template, name string, data []byte) (replaced bool, err error) {
	if !assetExts[strings.ToLower(filepath.Ext(name))] {
		return false, fmt.Errorf("file type of %q is not allowed", name)
	}
	p, err := editor.SafeFilePath(e.templatesDir, template, name)
	if err == nil {
		err = editor.ValidateWriteName(name)
	}
	if err != nil {
		return false, errInvalidWriteName(name)
	}
	if fi, err := os.Lstat(p); err == nil {
		if !fi.Mode().IsRegular() {
			return false, fmt.Errorf("refusing to overwrite non-regular file %q", name)
		}
		replaced = true
	} else if !errors.Is(err, os.ErrNotExist) {
		return false, fmt.Errorf("cannot write %q", name)
	}
	if err := checkTemplateCapacity(filepath.Dir(p), name, int64(len(data))); err != nil {
		return false, err
	}
	if err := editor.AtomicWrite(filepath.Dir(p), filepath.Base(p), data); err != nil {
		if errors.Is(err, editor.ErrNotRegular) {
			return false, fmt.Errorf("refusing to overwrite non-regular file %q", name)
		}
		return false, fmt.Errorf("cannot write %q", name)
	}
	return replaced, nil
}

// errInvalidWriteName is the error for names editor.ValidateWriteName rejects.
func errInvalidWriteName(name string) error {
	return fmt.Errorf("invalid file name %q (allowed: letters, digits, . _ -; not starting with a dot; no Windows device names like CON or NUL)", name)
}

// checkTemplateCapacity verifies that adding or replacing newName with
// newSize bytes keeps the template within the file-count and size limits.
func checkTemplateCapacity(dir string, newName string, newSize int64) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("cannot read template folder")
	}
	count, total, exists := 0, int64(0), false
	for _, ent := range entries {
		if ent.IsDir() || strings.HasSuffix(ent.Name(), ".tmp") {
			continue
		}
		if ent.Name() == newName {
			exists = true
			continue
		}
		count++
		if info, err := ent.Info(); err == nil {
			total += info.Size()
		}
	}
	if !exists && count >= maxFilesPerTemplate {
		return fmt.Errorf("too many files in template (max %d)", maxFilesPerTemplate)
	}
	if total+newSize > maxTemplateBytes {
		return fmt.Errorf("template would exceed %d MiB", maxTemplateBytes>>20)
	}
	return nil
}
