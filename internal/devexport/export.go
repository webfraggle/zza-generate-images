package devexport

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"

	"github.com/webfraggle/zza-generate-images/internal/renderer"
)

// FormatVersion is the package format version (manifest "format").
const FormatVersion = 1

// Package is a flat set of files (names <= 12 characters) for the device.
type Package struct {
	Name  string
	Files map[string][]byte
}

// Export checks the template for device compatibility and builds its package.
// An incompatible template yields *Incompatible listing every reason. A file
// that cannot be read (permissions, I/O) is a plain error, not a verdict on
// the template: callers must not treat it as "incompatible".
// MaxNameLen is the longest template name the controller can install. Its file
// system allows 32 characters per name and the installation goes through the
// folder "tpl_<name>.new" (8 characters more than the name).
const MaxNameLen = 24

func Export(templatesDir, name string) (*Package, error) {
	tmpl, err := renderer.LoadTemplate(templatesDir, name)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) || isReadError(err) {
			return nil, err
		}
		return nil, &Incompatible{Reasons: []string{"Template ungültig: " + err.Error()}}
	}
	doc, err := templateJSON(filepath.Join(tmpl.Dir, "template.yaml"))
	if err != nil {
		if isReadError(err) {
			return nil, err
		}
		return nil, &Incompatible{Reasons: []string{"Template ungültig: " + err.Error()}}
	}
	a := newAnalysis(tmpl)
	if len(name) > MaxNameLen {
		a.reason("Template-Name %q hat %d Zeichen, das Gerät erlaubt höchstens %d Zeichen (Template umbenennen: Knopf „Umbenennen“ im Editor)", name, len(name), MaxNameLen)
	}
	a.check(tmpl, doc)

	files := map[string][]byte{}
	if len(a.reasons) == 0 {
		a.build(doc, files)
	}
	if len(a.reasons) == 0 {
		total := 0
		for _, b := range files {
			total += len(b)
		}
		if total > maxPackageBytes {
			a.reason("Paket ist %d Bytes groß, das Gerät fasst höchstens %d Bytes", total, maxPackageBytes)
		}
	}
	if len(a.reasons) > 0 {
		return nil, &Incompatible{Reasons: a.reasons}
	}

	features := []string{}
	if tmpl.HasSplitflap() {
		features = append(features, "splitflap")
	}
	if tmpl.Meta.Animation.Frames > 0 {
		features = append(features, "frames")
	}
	if len(a.regexes) > 0 {
		features = append(features, "regex")
	}
	if len(a.images) > 0 {
		features = append(features, "images")
	}
	type fileInfo struct {
		N string `json:"n"`
		S int    `json:"s"`
		H string `json:"h"`
	}
	list := []fileInfo{}
	names := make([]string, 0, len(files))
	for n := range files {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		h := sha256.Sum256(files[n])
		list = append(list, fileInfo{n, len(files[n]), hex.EncodeToString(h[:])})
	}
	manifest, err := marshalJSON(struct {
		Format   int        `json:"format"`
		Name     string     `json:"name"`
		Canvas   [2]int     `json:"canvas"`
		Files    []fileInfo `json:"files"`
		Features []string   `json:"features"`
		Clock    string     `json:"clock"`
	}{FormatVersion, name, [2]int{240, 240}, list, features, tmpl.ClockGranularity()})
	if err != nil {
		return nil, err
	}
	files["manifest.json"] = manifest
	return &Package{Name: name, Files: files}, nil
}

// build creates image, font and t.json files and annotates the template tree.
func (a *analysis) build(doc map[string]any, files map[string][]byte) {
	imgNames := map[imgKey]string{}
	keys := make([]imgKey, 0, len(a.images))
	for k := range a.images {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		x, y := keys[i], keys[j]
		if x.File != y.File {
			return x.File < y.File
		}
		if x.W != y.W {
			return x.W < y.W
		}
		return x.H < y.H
	})
	for i, k := range keys {
		n := fmt.Sprintf("i%d", i)
		imgNames[k] = n
		img := a.srcImgs[k.File].img
		if k.W > 0 {
			img = scaleLike(img, k.W, k.H)
		}
		files[n+".img"] = EncodeImage(img)
	}

	fontNames := map[fontKey]string{}
	for i, k := range sortedFontKeys(a.combos) {
		n := fmt.Sprintf("f%d", i)
		fontNames[k] = n
		data, err := BuildFont(a.fontSrc[k.File], k, a.combos[k].runes())
		if err != nil {
			a.reason("Schrift %q nicht verwendbar: %v", k.File, err)
			return
		}
		files[n+".fnt"] = data
	}

	for _, r := range a.refs {
		if r.key == "_f" {
			r.obj[r.key] = fontNames[r.f]
		} else {
			r.obj[r.key] = imgNames[r.img]
		}
	}
	if len(a.regexes) > 0 {
		re := map[string]Prog{}
		for p := range a.regexes {
			re[p], _ = CompileRegex(p) // checked in check()
		}
		doc["_re"] = re
	}
	tj, err := marshalJSON(doc)
	if err != nil {
		a.reason("Template nicht als JSON darstellbar: %v", err)
		return
	}
	files["t.json"] = tj
}

// isReadError reports whether err comes from the file system (the file could
// not be opened or read) rather than from the template's content.
func isReadError(err error) bool {
	var pe *fs.PathError
	return errors.As(err, &pe)
}
