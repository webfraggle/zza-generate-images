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
	Truncated      bool          `json:"truncated,omitempty"`
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
	data, err := readRegularFile(path, maxFontBytes)
	switch {
	case errors.Is(err, errFileTooLarge):
		return nil, fmt.Errorf("font file too large (max %d MiB)", maxFontBytes>>20)
	case err != nil:
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
			} else {
				out.Truncated = true
			}
		}
	}
	for _, s := range skipped {
		if q == "" || strings.Contains(strings.ToLower(s.Key), q) {
			if len(out.Skipped) < maxFontResults {
				out.Skipped = append(out.Skipped, s)
			} else {
				out.Truncated = true
			}
		}
	}
	if out.Truncated {
		out.Note = fmt.Sprintf("Results are limited to %d entries; narrow the list with query. ", maxFontResults)
	}
	if inFontsDir == 0 {
		out.Note += fmt.Sprintf("Put .ttf/.otf files into %s to make more fonts available.", e.fontsDir)
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

// maxNodeLine returns the last source line covered by n and its children.
func maxNodeLine(n *yaml.Node) int {
	m := n.Line
	for _, c := range n.Content {
		if l := maxNodeLine(c); l > m {
			m = l
		}
	}
	return m
}

func yamlScalar(v string) string {
	b, err := yaml.Marshal(v)
	if err != nil {
		return v
	}
	return strings.TrimRight(string(b), "\r\n")
}

// addFontDef registers {id, file} under fonts: in the template YAML src.
// The entry is inserted as text next to the existing entries, so the rest of
// the file (comments, blank lines, line endings) stays byte-identical. If an
// entry for file exists, nothing is added and its id is returned in
// existingID. A missing fonts: key is inserted before layers:. Flow-style or
// empty fonts lists fall back to re-encoding the document.
func addFontDef(src []byte, id, file string) (out []byte, existingID string, err error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(src, &doc); err != nil {
		return nil, "", fmt.Errorf("decoding template YAML: %w", err)
	}
	if doc.Kind != yaml.DocumentNode || len(doc.Content) != 1 || doc.Content[0].Kind != yaml.MappingNode {
		return nil, "", fmt.Errorf("template YAML is not a mapping")
	}
	root := doc.Content[0]
	var seq, layersKey *yaml.Node
	layersAt := -1
	for i := 0; i+1 < len(root.Content); i += 2 {
		switch root.Content[i].Value {
		case "fonts":
			seq = root.Content[i+1]
		case "layers":
			layersAt, layersKey = i, root.Content[i]
		}
	}
	if seq != nil && seq.Kind != yaml.SequenceNode && !(seq.Kind == yaml.ScalarNode && seq.Tag == "!!null") {
		return nil, "", fmt.Errorf("fonts is not a list")
	}
	if seq != nil && seq.Kind == yaml.SequenceNode {
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
			if strings.EqualFold(efile, file) {
				return src, eid, nil
			}
			if eid == id {
				return nil, "", fmt.Errorf("font id %q is already used for %s", id, efile)
			}
		}
	}

	before, err := renderer.ParseTemplate(src, "")
	if err != nil {
		return nil, "", fmt.Errorf("template.yaml is invalid: %w", err)
	}

	eol := "\n"
	if bytes.Contains(src, []byte("\r\n")) {
		eol = "\r\n"
	}
	lines := strings.Split(string(src), "\n")
	cr := ""
	if eol == "\r\n" {
		cr = "\r"
	}
	insertAt := -1 // index in lines before which new lines go
	var add []string
	switch {
	case seq != nil && seq.Kind == yaml.SequenceNode && seq.Style&yaml.FlowStyle == 0 && len(seq.Content) > 0:
		last := seq.Content[len(seq.Content)-1]
		if last.Kind == yaml.MappingNode && last.Line-1 < len(lines) {
			first := lines[last.Line-1]
			indent := first[:len(first)-len(strings.TrimLeft(first, " "))]
			inner := strings.Repeat(" ", last.Column-1)
			if strings.HasPrefix(strings.TrimLeft(first, " "), "- ") {
				insertAt = maxNodeLine(last)
				add = []string{indent + "- id: " + yamlScalar(id) + cr, inner + "file: " + yamlScalar(file) + cr}
			}
		}
	case seq == nil && layersKey != nil:
		at := layersKey.Line - 1
		for at > 0 && strings.HasPrefix(strings.TrimSpace(lines[at-1]), "#") {
			at--
		}
		insertAt = at
		add = []string{"fonts:" + cr, "  - id: " + yamlScalar(id) + cr, "    file: " + yamlScalar(file) + cr, cr}
	}
	_ = layersAt
	if insertAt >= 0 && insertAt <= len(lines) {
		nl := append([]string{}, lines[:insertAt]...)
		nl = append(nl, add...)
		nl = append(nl, lines[insertAt:]...)
		out = []byte(strings.Join(nl, "\n"))
	} else {
		out, err = reencodeWithFont(&doc, root, seq, layersAt, id, file)
		if err != nil {
			return nil, "", err
		}
	}

	after, err := renderer.ParseTemplate(out, "")
	if err != nil {
		return nil, "", fmt.Errorf("template.yaml would become invalid: %w", err)
	}
	if len(after.Fonts) != len(before.Fonts)+1 {
		return nil, "", fmt.Errorf("internal error: unexpected fonts list after edit")
	}
	for i, f := range before.Fonts {
		if after.Fonts[i] != f {
			return nil, "", fmt.Errorf("internal error: fonts list changed unexpectedly")
		}
	}
	if n := after.Fonts[len(after.Fonts)-1]; n.ID != id || n.File != file {
		return nil, "", fmt.Errorf("internal error: new font entry not as expected")
	}
	return out, "", nil
}

// reencodeWithFont is the fallback for flow-style or empty fonts lists and
// templates without layers: it re-encodes the whole document.
func reencodeWithFont(doc, root, seq *yaml.Node, layersAt int, id, file string) ([]byte, error) {
	if seq == nil {
		seq = &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
		kn := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "fonts"}
		if layersAt < 0 {
			root.Content = append(root.Content, kn, seq)
		} else {
			rest := append([]*yaml.Node{kn, seq}, root.Content[layersAt:]...)
			root.Content = append(root.Content[:layersAt:layersAt], rest...)
		}
	} else if seq.Kind == yaml.ScalarNode {
		*seq = yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
	}
	str := func(v string) *yaml.Node { return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: v} }
	seq.Content = append(seq.Content, &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map",
		Content: []*yaml.Node{str("id"), str(id), str("file"), str(file)}})
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(doc); err != nil {
		return nil, fmt.Errorf("encoding template YAML: %w", err)
	}
	if err := enc.Close(); err != nil {
		return nil, fmt.Errorf("encoding template YAML: %w", err)
	}
	return buf.Bytes(), nil
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
		return zero, fmt.Errorf("%s", e.scrubErr(err, maxErrLen))
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
	if _, err := os.Lstat(target); err == nil {
		differs := fmt.Errorf("%s already exists in %s with different content", file, in.Template)
		cur, err := readRegularFile(target, maxFontBytes)
		switch {
		case errors.Is(err, errNotRegular):
			return zero, fmt.Errorf("refusing to overwrite non-regular file %q", file)
		case errors.Is(err, errFileTooLarge):
			return zero, differs
		case err != nil:
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
		hint := ""
		if copied {
			hint = "; font file was copied, retry add_font"
		}
		switch {
		case errors.Is(err, editor.ErrVersionConflict):
			return zero, fmt.Errorf("conflict: template.yaml was changed while adding the font%s. Call read_template again and retry.", hint)
		case err != nil:
			return zero, fmt.Errorf("cannot write template.yaml%s", hint)
		}
	}
	return addFontOut{ID: id, File: file, Added: added, Copied: copied, LicenseNote: fontLicenseNote}, nil
}
