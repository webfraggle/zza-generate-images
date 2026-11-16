package devexport

import (
	"fmt"
	"image"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/webfraggle/zza-generate-images/internal/renderer"
)

// Limits of the device (spec §7.3).
const (
	maxPackageBytes = 3 << 20 // LittleFS
	maxRegexInst    = 2000
	maxRegexRunes   = 8000
	maxFontBytes    = 50 << 20

	reasonRegexDynamic = "Regex-Muster muss konstant sein"
	reasonRegexTooBig  = "Regex-Muster zu groß für das Gerät"
)

// Incompatible is returned by Export when a template cannot run on the device.
type Incompatible struct{ Reasons []string }

func (e *Incompatible) Error() string {
	return "nicht ZZA-kompatibel: " + strings.Join(e.Reasons, "; ")
}

var interpRe = regexp.MustCompile(`\{\{(?:'[^']*'|"[^"]*"|[^}])+\}\}`)

// layerRef is a JSON layer object that gets an annotation once names are assigned.
type layerRef struct {
	obj map[string]any
	key string // "_f", "_i" or "_io"
	f   fontKey
	img imgKey
}

// analysis is the result of walking a template: everything Export needs plus
// every reason why the template is not compatible.
type analysis struct {
	dir     string
	fontsBy map[string]string // font id -> file
	reasons []string
	seen    map[string]bool

	combos  map[fontKey]*glyphSet
	fontSrc map[string][]byte // font file -> bytes
	images  map[imgKey]bool
	srcImgs map[string]*imgSource // decoded originals by file name
	refs    []layerRef
	regexes map[string]bool
}

type imgSource struct {
	img *image.NRGBA
	err error
}

func (a *analysis) reason(format string, args ...any) {
	s := fmt.Sprintf(format, args...)
	if !a.seen[s] {
		a.seen[s] = true
		a.reasons = append(a.reasons, s)
	}
}

func newAnalysis(tmpl *renderer.Template) *analysis {
	a := &analysis{
		dir: tmpl.Dir, fontsBy: map[string]string{}, seen: map[string]bool{},
		combos: map[fontKey]*glyphSet{}, fontSrc: map[string][]byte{},
		images: map[imgKey]bool{}, srcImgs: map[string]*imgSource{}, regexes: map[string]bool{},
	}
	for _, f := range tmpl.Fonts {
		a.fontsBy[f.ID] = f.File
	}
	return a
}

// check collects all incompatibilities of tmpl; doc is the t.json tree.
func (a *analysis) check(tmpl *renderer.Template, doc map[string]any) {
	if c := tmpl.Meta.Canvas; c.Width != 240 || c.Height != 240 {
		a.reason("Canvas ist %d × %d, das Gerät unterstützt nur 240 × 240", c.Width, c.Height)
	}
	layers, _ := doc["layers"].([]any)
	a.walkLayers(tmpl.Layers, layers, "layers")
	a.scanMatches("layers", layers)
	a.checkRegexes()
}

func (a *analysis) walkLayers(layers []renderer.Layer, js []any, path string) {
	for i := range layers {
		if i >= len(js) {
			return
		}
		obj, _ := js[i].(map[string]any)
		if obj == nil {
			continue
		}
		l := &layers[i]
		lp := fmt.Sprintf("%s[%d]", path, i)
		switch l.Type {
		case "text":
			a.textLayer(l, obj, lp)
		case "image":
			a.imageLayer(l, obj)
		}
		for _, r := range l.Reel {
			for _, c := range r {
				if c > 0xFF {
					a.reason("Reel-Eintrag %q enthält Zeichen außerhalb von Latin-1", r)
					break
				}
			}
		}
		sub, _ := obj["layers"].([]any)
		a.walkLayers(l.Layers, sub, lp+".layers")
	}
}

// layerStrings returns all alternative texts of a StringOrCond JSON value.
func layerStrings(v any) []string {
	switch x := v.(type) {
	case string:
		return []string{x}
	case map[string]any:
		var out []string
		pairs, _ := x["_cond"].([]any)
		for _, p := range pairs {
			if kv, ok := p.([]any); ok && len(kv) == 2 {
				if k, _ := kv[0].(string); k == "then" || k == "else" {
					if s, ok := kv[1].(string); ok {
						out = append(out, s)
					}
				}
			}
		}
		return out
	}
	return nil
}

func (a *analysis) textLayer(l *renderer.Layer, obj map[string]any, path string) {
	file, ok := a.fontsBy[l.Font]
	if !ok {
		a.reason("Schrift %q ist im Template nicht definiert", l.Font)
		return
	}
	if _, ok := a.fontSrc[file]; !ok {
		a.loadFont(file)
	}
	k := fontKey{File: file, Size: l.Size, SX: l.ScaleX, SY: l.ScaleY}
	if k.Size <= 0 {
		k.Size = 12
	}
	if k.SX <= 0 {
		k.SX = 1
	}
	if k.SY <= 0 {
		k.SY = 1
	}
	g := a.combos[k]
	if g == nil {
		g = &glyphSet{chars: map[rune]bool{}}
		a.combos[k] = g
	}
	for _, s := range layerStrings(obj["value"]) {
		if strings.Contains(s, "{{") {
			g.dynamic = true // dynamic text: full set
		} else {
			for _, r := range s {
				g.chars[r] = true
			}
		}
		// Literal segments (outside {{ }}) are constants: the device would draw '?'.
		for _, r := range interpRe.ReplaceAllString(s, "") {
			if r > 0xFF {
				a.reason("%s (value): Zeichen U+%04X %q in %q liegt außerhalb von Latin-1", path, r, r, s)
			}
		}
	}
	a.refs = append(a.refs, layerRef{obj: obj, key: "_f", f: k})
}

func (a *analysis) loadFont(file string) {
	a.fontSrc[file] = nil
	if strings.ContainsAny(file, "/\\") || filepath.Base(file) != file {
		a.reason("Schriftdatei %q ist ungültig", file)
		return
	}
	p := filepath.Join(a.dir, file)
	st, err := os.Stat(p)
	if err != nil || st.Size() > maxFontBytes {
		a.reason("Schriftdatei %q nicht lesbar", file)
		return
	}
	b, err := os.ReadFile(p)
	if err != nil {
		a.reason("Schriftdatei %q nicht lesbar", file)
		return
	}
	a.fontSrc[file] = b
}

// dim classifies a width/height JSON value: constant n (0 = absent) or dynamic.
func dim(v any) (n int, dynamic bool) {
	switch x := v.(type) {
	case nil:
		return 0, false
	case int:
		return x, false
	}
	return 0, true
}

func (a *analysis) imageLayer(l *renderer.Layer, obj map[string]any) {
	file := l.File
	lower := strings.ToLower(file)
	if strings.ContainsAny(file, "/\\") || filepath.Base(file) != file ||
		!(strings.HasSuffix(lower, ".png") || strings.HasSuffix(lower, ".jpg") || strings.HasSuffix(lower, ".jpeg")) {
		a.reason("Bilddatei %q ist ungültig (nur .png/.jpg ohne Pfad)", file)
		return
	}
	src := a.srcImgs[file]
	if src == nil {
		src = &imgSource{}
		src.img, src.err = loadImage(filepath.Join(a.dir, file))
		if src.err == nil && (src.img.Bounds().Dx() > 0xFFFF || src.img.Bounds().Dy() > 0xFFFF) {
			src.err = fmt.Errorf("zu groß")
		}
		a.srcImgs[file] = src
		if src.err != nil {
			a.reason("Bild %q nicht lesbar: %v", file, src.err)
		}
	}
	if src.err != nil {
		return
	}
	w, wd := dim(obj["width"])
	h, hd := dim(obj["height"])
	if wd || hd {
		a.images[imgKey{File: file}] = true
		a.refs = append(a.refs, layerRef{obj: obj, key: "_io", img: imgKey{File: file}})
		return
	}
	k := imgKey{File: file}
	if w > 0 || h > 0 {
		if w > maxImageDim || h > maxImageDim {
			a.reason("Bild %q: Zielgröße %d × %d zu groß", file, w, h)
			return
		}
		k.W, k.H = scaledSize(src.img, w, h)
	}
	if px := k.W * k.H; px*4 > maxPackageBytes || src.img.Bounds().Dx()*src.img.Bounds().Dy()*4 > maxPackageBytes*8 {
		a.reason("Bild %q ist zu groß für das Gerät", file)
		return
	}
	a.images[k] = true
	a.refs = append(a.refs, layerRef{obj: obj, key: "_i", img: k})
}

// scanMatches walks the JSON tree and collects the match filter patterns of
// every interpolated field and condition.
func (a *analysis) scanMatches(key string, v any) {
	visit := func(p string, literal bool) {
		if literal {
			a.regexes[p] = true
		} else {
			a.reason(reasonRegexDynamic)
		}
	}
	switch x := v.(type) {
	case string:
		if key == "if" || key == "elif" {
			renderer.ScanMatchCondition(x, visit)
		} else {
			renderer.ScanMatchInterpolated(x, visit)
		}
	case []any:
		for _, e := range x {
			a.scanMatches(key, e)
		}
	case map[string]any:
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if k == "_cond" {
				pairs, _ := x[k].([]any)
				for _, p := range pairs {
					if kv, ok := p.([]any); ok && len(kv) == 2 {
						a.scanMatches(kv[0].(string), kv[1])
					}
				}
				continue
			}
			a.scanMatches(k, x[k])
		}
	}
}

// checkRegexes compiles every constant pattern; failures and oversize programs are reasons.
func (a *analysis) checkRegexes() {
	for _, p := range a.sortedPatterns() {
		prog, err := CompileRegex(p)
		if err != nil {
			a.reason("Regex-Muster %q ist ungültig: %v", p, err)
			continue
		}
		runes := 0
		for _, in := range prog.Inst {
			runes += len(in.Runes)
		}
		if len(prog.Inst) > maxRegexInst || runes > maxRegexRunes {
			a.reason(reasonRegexTooBig)
		}
	}
}

func (a *analysis) sortedPatterns() []string {
	out := make([]string, 0, len(a.regexes))
	for p := range a.regexes {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}
