package mcp

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/font/sfnt"
	"gopkg.in/yaml.v3"

	"github.com/webfraggle/zza-generate-images/internal/editor"
	"github.com/webfraggle/zza-generate-images/internal/renderer"
)

func init() { registerToolGroup("fonts", registerFontTools) }

const (
	fontsKeyPrefix  = "@fonts/"
	maxLicenseChars = 200
	fontLicenseNote = "Font licences are the user's responsibility; check before sharing the template."
)

var (
	safeFontNameRe = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)
	fontIDRe       = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)
	nonIDCharRe    = regexp.MustCompile(`[^a-z0-9]+`)
)

type fontEntry struct {
	Key     string   `json:"key"`
	File    string   `json:"file"`
	Family  string   `json:"family"`
	Style   string   `json:"style"`
	Source  string   `json:"source"` // "fonts-dir" or "template"
	FoundIn []string `json:"found_in"`
	Bytes   int64    `json:"bytes"`
	License string   `json:"license,omitempty"`
}

type skippedFont struct {
	Key    string `json:"key"`
	Reason string `json:"reason"`
}

type listFontsIn struct {
	Query string `json:"query,omitempty" jsonschema:"case-insensitive substring of family, style or file name"`
}

type listFontsOut struct {
	FontsDir       string        `json:"fonts_dir"`
	FontsDirExists bool          `json:"fonts_dir_exists"`
	Fonts          []fontEntry   `json:"fonts"`
	Skipped        []skippedFont `json:"skipped"`
	Note           string        `json:"note,omitempty"`
}

type addFontIn struct {
	Template string `json:"template" jsonschema:"template name"`
	Font     string `json:"font" jsonschema:"font key from list_fonts, e.g. @fonts/MyFont.ttf or oebb-105-v1/Roboto-Bold.ttf"`
	ID       string `json:"id,omitempty" jsonschema:"font id for text layers; defaults to the lowercased file name without extension"`
}

type addFontOut struct {
	ID          string `json:"id"`
	File        string `json:"file"`
	Added       bool   `json:"added"`
	Copied      bool   `json:"copied"`
	LicenseNote string `json:"license_note"`
}

func registerFontTools(s *sdk.Server, e *env) {
	sdk.AddTool(s, &sdk.Tool{
		Name:        "list_fonts",
		Description: `Lists fonts you can add to a template: fonts already used in any template and fonts the user put into the "fonts" folder next to the templates folder. No system or web fonts. Returns key, family, style and licence info.`,
	}, func(ctx context.Context, _ *sdk.CallToolRequest, in listFontsIn) (*sdk.CallToolResult, listFontsOut, error) {
		return nil, e.listFonts(in), nil
	})

	sdk.AddTool(s, &sdk.Tool{
		Name:        "add_font",
		Description: "Copies a font (key from list_fonts) into the template AND registers it under fonts: in template.yaml in one step; returns the font id to use in text layers (font: <id>). Existing entries are reused, never duplicated.",
	}, func(ctx context.Context, _ *sdk.CallToolRequest, in addFontIn) (*sdk.CallToolResult, addFontOut, error) {
		out, err := e.addFont(in)
		return nil, out, err
	})
}

func isFontExt(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	return ext == ".ttf" || ext == ".otf"
}

// defaultFontID derives a font id from a file name: lowercase, base name only,
// every run of characters outside [a-z0-9] becomes "-".
func defaultFontID(file string) string {
	base := strings.TrimSuffix(file, filepath.Ext(file))
	id := strings.Trim(nonIDCharRe.ReplaceAllString(strings.ToLower(base), "-"), "-")
	if id == "" {
		return "font"
	}
	return id
}

// fontNames reads family, style and licence from the name table.
func fontNames(data []byte) (family, style, license string, err error) {
	f, err := sfnt.Parse(data)
	if err != nil {
		return "", "", "", err
	}
	var b sfnt.Buffer
	get := func(ids ...sfnt.NameID) string {
		for _, id := range ids {
			if s, err := f.Name(&b, id); err == nil && strings.TrimSpace(s) != "" {
				return strings.TrimSpace(s)
			}
		}
		return ""
	}
	family = get(16, sfnt.NameIDFamily)
	style = get(17, sfnt.NameIDSubfamily)
	license = get(sfnt.NameIDLicense)
	if r := []rune(license); len(r) > maxLicenseChars {
		license = string(r[:maxLicenseChars])
	}
	return family, style, license, nil
}

// readFontFile reads a regular font file with the size limit applied.
func readFontFile(path string) ([]byte, error) {
	fi, err := os.Lstat(path)
	if err != nil || !fi.Mode().IsRegular() {
		return nil, fmt.Errorf("cannot read font file")
	}
	if fi.Size() > maxFontBytes {
		return nil, fmt.Errorf("font file too large (max %d MiB)", maxFontBytes>>20)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read font file")
	}
	return data, nil
}

func shortErr(err error) string {
	s := err.Error()
	if r := []rune(s); len(r) > 100 {
		s = string(r[:100])
	}
	return s
}

// scanFonts collects all fonts from the fonts folder and from the template
// folders. Unreadable fonts are reported in skipped.
func scanFonts(e *env) (fonts []fontEntry, skipped []skippedFont) {
	fonts, skipped = []fontEntry{}, []skippedFont{}

	build := func(key, file, source string, found []string, data []byte) (fontEntry, *skippedFont) {
		family, style, license, err := fontNames(data)
		if err != nil {
			return fontEntry{}, &skippedFont{Key: key, Reason: "not a font: " + shortErr(err)}
		}
		if family == "" {
			family = defaultFontID(file)
		}
		return fontEntry{Key: key, File: file, Family: family, Style: style, Source: source,
			FoundIn: found, Bytes: int64(len(data)), License: license}, nil
	}
	candidates := func(dir string) (names []string) {
		ents, err := os.ReadDir(dir)
		if err != nil {
			return nil
		}
		for _, ent := range ents {
			if ent.Type().IsRegular() && isFontExt(ent.Name()) {
				names = append(names, ent.Name())
			}
		}
		return names
	}
	load := func(key, path string) ([]byte, *skippedFont) {
		data, err := readFontFile(path)
		if err != nil {
			return nil, &skippedFont{Key: key, Reason: err.Error()}
		}
		return data, nil
	}

	// Fonts folder.
	for _, name := range candidates(e.fontsDir) {
		key := fontsKeyPrefix + name
		if !safeFontNameRe.MatchString(name) {
			skipped = append(skipped, skippedFont{Key: key, Reason: "unsupported file name (allowed: letters, digits, . _ -)"})
			continue
		}
		data, sk := load(key, filepath.Join(e.fontsDir, name))
		if sk != nil {
			skipped = append(skipped, *sk)
			continue
		}
		if fe, sk := build(key, name, "fonts-dir", []string{}, data); sk != nil {
			skipped = append(skipped, *sk)
		} else {
			fonts = append(fonts, fe)
		}
	}

	// Template folders, grouped by content hash.
	tents, _ := os.ReadDir(e.templatesDir)
	byHash := map[[32]byte]int{}
	for _, t := range tents {
		if !t.IsDir() || renderer.ValidateTemplateName(t.Name()) != nil {
			continue
		}
		tdir := filepath.Join(e.templatesDir, t.Name())
		for _, name := range candidates(tdir) {
			key := t.Name() + "/" + name
			if !safeFontNameRe.MatchString(name) {
				skipped = append(skipped, skippedFont{Key: key, Reason: "unsupported file name (allowed: letters, digits, . _ -)"})
				continue
			}
			data, sk := load(key, filepath.Join(tdir, name))
			if sk != nil {
				skipped = append(skipped, *sk)
				continue
			}
			h := sha256.Sum256(data)
			if i, ok := byHash[h]; ok {
				fonts[i].FoundIn = append(fonts[i].FoundIn, t.Name())
				continue
			}
			fe, sk := build(key, name, "template", []string{t.Name()}, data)
			if sk != nil {
				skipped = append(skipped, *sk)
				continue
			}
			byHash[h] = len(fonts)
			fonts = append(fonts, fe)
		}
	}
	for i := range fonts {
		sort.Strings(fonts[i].FoundIn)
	}
	sort.SliceStable(fonts, func(i, j int) bool {
		a, b := fonts[i], fonts[j]
		if (a.Source == "fonts-dir") != (b.Source == "fonts-dir") {
			return a.Source == "fonts-dir"
		}
		if x, y := strings.ToLower(a.Family), strings.ToLower(b.Family); x != y {
			return x < y
		}
		if x, y := strings.ToLower(a.Style), strings.ToLower(b.Style); x != y {
			return x < y
		}
		return a.Key < b.Key
	})
	sort.SliceStable(skipped, func(i, j int) bool { return skipped[i].Key < skipped[j].Key })
	return fonts, skipped
}

func (e *env) listFonts(in listFontsIn) listFontsOut {
	out := listFontsOut{FontsDir: e.fontsDir}
	if fi, err := os.Stat(e.fontsDir); err == nil && fi.IsDir() {
		out.FontsDirExists = true
	}
	all, skipped := scanFonts(e)
	inFontsDir := 0
	for _, f := range all {
		if f.Source == "fonts-dir" {
			inFontsDir++
		}
	}
	for _, s := range skipped {
		if strings.HasPrefix(s.Key, fontsKeyPrefix) {
			inFontsDir++
		}
	}
	q := strings.ToLower(strings.TrimSpace(in.Query))
	out.Fonts, out.Skipped = []fontEntry{}, []skippedFont{}
	for _, f := range all {
		if q == "" || strings.Contains(strings.ToLower(f.Family+"\x00"+f.Style+"\x00"+f.File), q) {
			if len(out.Fonts) < maxFontResults {
				out.Fonts = append(out.Fonts, f)
			}
		}
	}
	for _, s := range skipped {
		if q == "" || strings.Contains(strings.ToLower(s.Key), q) {
			out.Skipped = append(out.Skipped, s)
		}
	}
	if inFontsDir == 0 {
		out.Note = fmt.Sprintf("Put .ttf/.otf files into %s to make more fonts available.", e.fontsDir)
	}
	return out
}

// resolveFontKey maps a font key from list_fonts to an absolute file path. It
// re-validates the key form and that the target is a regular file.
func resolveFontKey(e *env, key string) (path string, err error) {
	bad := fmt.Errorf("unknown font key %q — use a key from list_fonts", key)
	var p, file string
	if rest, ok := strings.CutPrefix(key, fontsKeyPrefix); ok {
		if !safeFontNameRe.MatchString(rest) {
			return "", bad
		}
		p, file = filepath.Join(e.fontsDir, rest), rest
	} else {
		tmpl, f, ok := strings.Cut(key, "/")
		if !ok || !safeFontNameRe.MatchString(f) || renderer.ValidateTemplateName(tmpl) != nil {
			return "", bad
		}
		if fi, err := os.Lstat(filepath.Join(e.templatesDir, tmpl)); err != nil || !fi.IsDir() {
			return "", bad
		}
		p, file = filepath.Join(e.templatesDir, tmpl, f), f
	}
	if !isFontExt(file) {
		return "", bad
	}
	if fi, err := os.Lstat(p); err != nil || !fi.Mode().IsRegular() {
		return "", bad
	}
	return p, nil
}

// addFontDef registers {id, file} under fonts: in the template YAML src,
// keeping comments. If an entry for file exists, nothing is added and its id
// is returned in existingID. A missing fonts: key is inserted before layers:.
func addFontDef(src []byte, id, file string) (out []byte, existingID string, err error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(src, &doc); err != nil {
		return nil, "", fmt.Errorf("decoding template YAML: %w", err)
	}
	if doc.Kind != yaml.DocumentNode || len(doc.Content) != 1 || doc.Content[0].Kind != yaml.MappingNode {
		return nil, "", fmt.Errorf("template YAML is not a mapping")
	}
	root := doc.Content[0]
	var seq *yaml.Node
	layersAt := -1
	for i := 0; i+1 < len(root.Content); i += 2 {
		switch root.Content[i].Value {
		case "fonts":
			seq = root.Content[i+1]
			if seq.Kind == yaml.ScalarNode && seq.Tag == "!!null" {
				*seq = yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
			}
		case "layers":
			layersAt = i
		}
	}
	if seq == nil {
		seq = &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
		kn := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "fonts"}
		if layersAt < 0 {
			root.Content = append(root.Content, kn, seq)
		} else {
			rest := append([]*yaml.Node{kn, seq}, root.Content[layersAt:]...)
			root.Content = append(root.Content[:layersAt:layersAt], rest...)
		}
	}
	if seq.Kind != yaml.SequenceNode {
		return nil, "", fmt.Errorf("fonts is not a list")
	}
	for _, item := range seq.Content {
		var eid, efile string
		if item.Kind == yaml.MappingNode {
			for i := 0; i+1 < len(item.Content); i += 2 {
				switch item.Content[i].Value {
				case "id":
					eid = item.Content[i+1].Value
				case "file":
					efile = item.Content[i+1].Value
				}
			}
		}
		if efile == file {
			return src, eid, nil
		}
		if eid == id {
			return nil, "", fmt.Errorf("font id %q is already used for %s", id, efile)
		}
	}
	str := func(v string) *yaml.Node { return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: v} }
	seq.Content = append(seq.Content, &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map",
		Content: []*yaml.Node{str("id"), str(id), str("file"), str(file)}})
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(&doc); err != nil {
		return nil, "", fmt.Errorf("encoding template YAML: %w", err)
	}
	if err := enc.Close(); err != nil {
		return nil, "", fmt.Errorf("encoding template YAML: %w", err)
	}
	return buf.Bytes(), "", nil
}

func (e *env) addFont(in addFontIn) (addFontOut, error) {
	var zero addFontOut
	e.writeMu.Lock()
	defer e.writeMu.Unlock()

	dir, err := e.templateDir(in.Template)
	if err != nil {
		return zero, err
	}
	srcPath, err := resolveFontKey(e, in.Font)
	if err != nil {
		return zero, err
	}
	data, err := readFontFile(srcPath)
	if err != nil {
		return zero, err
	}
	if _, err := opentype.Parse(data); err != nil {
		return zero, fmt.Errorf("not a font: %s", shortErr(err))
	}
	file := editor.SanitizeFilename(filepath.Base(srcPath))

	id := in.ID
	if id == "" {
		id = defaultFontID(file)
	} else if !fontIDRe.MatchString(id) {
		return zero, fmt.Errorf("invalid font id %q (allowed: letters, digits, _ -, max 64)", id)
	}

	yamlSrc, err := editor.ReadTextFile(e.templatesDir, in.Template, "template.yaml")
	if err != nil {
		return zero, fmt.Errorf("cannot read template.yaml")
	}
	newYAML, existingID, err := addFontDef(yamlSrc, id, file)
	if err != nil {
		return zero, err
	}
	added := existingID == ""
	if added {
		if _, err := renderer.ParseTemplate(newYAML, dir); err != nil {
			return zero, fmt.Errorf("template.yaml would become invalid: %s", e.scrubErr(err, maxErrLen))
		}
	} else {
		id = existingID
	}

	copied := true
	target := filepath.Join(dir, file)
	if fi, err := os.Lstat(target); err == nil {
		if !fi.Mode().IsRegular() {
			return zero, fmt.Errorf("refusing to overwrite non-regular file %q", file)
		}
		differs := fmt.Errorf("%s already exists in %s with different content", file, in.Template)
		if fi.Size() != int64(len(data)) {
			return zero, differs
		}
		cur, err := os.ReadFile(target)
		if err != nil {
			return zero, fmt.Errorf("cannot read %q", file)
		}
		if !bytes.Equal(cur, data) {
			return zero, differs
		}
		copied = false
	}
	if copied {
		if _, err := writeAssetBytes(e, in.Template, file, data); err != nil {
			return zero, err
		}
	}
	if added {
		_, err := editor.WriteTextFileIfMatch(e.templatesDir, in.Template, "template.yaml", newYAML, editor.FileVersion(yamlSrc))
		switch {
		case errors.Is(err, editor.ErrVersionConflict):
			return zero, fmt.Errorf("conflict: template.yaml was changed while adding the font. Call read_template again and retry.")
		case err != nil:
			return zero, fmt.Errorf("cannot write template.yaml")
		}
	}
	return addFontOut{ID: id, File: file, Added: added, Copied: copied, LicenseNote: fontLicenseNote}, nil
}
