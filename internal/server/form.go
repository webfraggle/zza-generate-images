package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/webfraggle/zza-generate-images/internal/renderer"
)

// formField is one input on the detail page form.
type formField struct {
	Path        string // "zug1.vonnach" or "gleis" — used as input name
	Label       string
	Value       string // default rendered as text (numbers without trailing .0, bool "true"/"false", null "")
	Kind        string // "text" | "number" | "bool" — from JSON type of the default value
	Size        string // "s" short field, "l" long field
	Placeholder string
}

// formGroup is a titled block of fields (Bahnsteig, Zug 1, ...).
type formGroup struct {
	Title  string
	Fields []formField
	Open   bool // at least one field has a non-empty default value (bool false counts as empty)
}

// fieldLabels maps known default.json leaf keys to German labels.
var fieldLabels = map[string]string{
	"gleis":      "Gleis",
	"abschnitt":  "Abschnitt",
	"folgezuege": "Text „Folgezüge“",
	"zeit":       "Abfahrt",
	"abw":        "Verspätung (Min.)",
	"nr":         "Zug / Linie",
	"vonnach":    "Ziel",
	"via":        "Über",
	"hinweis":    "Hinweis",
	"fusszeile":  "Fußzeile",
	"abschnitte": "Abschnitte",
	"reihung":    "Wagenreihung",
	"mode":       "Modus",
	"mirrored":   "Gespiegelt",
}

// shortFields are rendered as narrow inputs.
var shortFields = map[string]bool{
	"zeit": true, "abw": true, "nr": true, "gleis": true, "abschnitt": true, "mode": true,
}

// fieldPlaceholders holds input placeholders for known keys.
var fieldPlaceholders = map[string]string{
	"zeit": "hh:mm",
	"abw":  "0",
}

// fieldOrder is the display order of known keys within a group, matching the
// old PHP generator (time, delay, train, destination, via, note). Unknown keys
// follow in default.json order.
var fieldOrder = []string{
	"gleis", "abschnitt", "folgezuege",
	"zeit", "abw", "nr", "vonnach", "via", "hinweis",
	"fusszeile", "abschnitte", "reihung", "mode", "mirrored",
}

// fieldRank returns the position of key in fieldOrder, or len(fieldOrder).
func fieldRank(key string) int {
	if i := slices.Index(fieldOrder, key); i >= 0 {
		return i
	}
	return len(fieldOrder)
}

var zugKeyRe = regexp.MustCompile(`^zug(\d+)$`)

// buildForm derives the detail-page form from a template's YAML source and its
// default.json. Only fields whose path is referenced by the template are
// included; group order follows default.json, fields are sorted by fieldOrder. Top-level scalars form
// the "Bahnsteig" group (always first), each top-level object becomes its own
// group. Deeper nesting and arrays are ignored. An empty defaultJSON yields nil.
func buildForm(yamlSrc, defaultJSON []byte) ([]formGroup, error) {
	if len(bytes.TrimSpace(defaultJSON)) == 0 {
		return nil, nil
	}
	refs, err := renderer.ReferencedPaths(yamlSrc)
	if err != nil {
		return nil, err
	}

	dec := json.NewDecoder(bytes.NewReader(defaultJSON))
	dec.UseNumber()
	if err := expectDelim(dec, '{'); err != nil {
		return nil, err
	}

	platform := formGroup{Title: "Bahnsteig"}
	var groups []formGroup
	for dec.More() {
		key, err := readKey(dec)
		if err != nil {
			return nil, err
		}
		tok, err := dec.Token()
		if err != nil {
			return nil, fmt.Errorf("default.json: %w", err)
		}
		switch tok {
		case json.Delim('{'):
			g := formGroup{Title: groupTitle(key)}
			if err := readObjectFields(dec, key, refs, &g); err != nil {
				return nil, err
			}
			groups = appendGroup(groups, g)
		case json.Delim('['):
			if err := skipRest(dec); err != nil {
				return nil, err
			}
		default:
			if refs[key] {
				addField(&platform, key, key, tok)
			}
		}
	}
	if _, err := dec.Token(); err != nil { // closing '}'
		return nil, fmt.Errorf("default.json: %w", err)
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return nil, errors.New("default.json: unexpected data after top-level object")
	}

	return appendGroup(appendGroup(nil, platform), groups...), nil
}

// readObjectFields consumes the rest of a JSON object (after its '{') and adds
// its referenced scalar members to g. Nested objects and arrays are skipped.
func readObjectFields(dec *json.Decoder, prefix string, refs map[string]bool, g *formGroup) error {
	for dec.More() {
		key, err := readKey(dec)
		if err != nil {
			return err
		}
		tok, err := dec.Token()
		if err != nil {
			return fmt.Errorf("default.json: %w", err)
		}
		if d, ok := tok.(json.Delim); ok && (d == '{' || d == '[') {
			if err := skipRest(dec); err != nil {
				return err
			}
			continue
		}
		path := prefix + "." + key
		if refs[path] {
			addField(g, path, key, tok)
		}
	}
	_, err := dec.Token() // closing '}'
	if err != nil {
		return fmt.Errorf("default.json: %w", err)
	}
	return nil
}

// addField appends a field built from a scalar JSON token to g.
func addField(g *formGroup, path, key string, tok json.Token) {
	f := formField{
		Path:        path,
		Label:       key,
		Kind:        "text",
		Size:        "l",
		Placeholder: fieldPlaceholders[key],
	}
	if l, ok := fieldLabels[key]; ok {
		f.Label = l
	}
	if shortFields[key] {
		f.Size = "s"
	}
	nonEmpty := false
	switch v := tok.(type) {
	case string:
		f.Value = v
		nonEmpty = v != ""
	case json.Number:
		f.Kind = "number"
		f.Value = formatNumber(v)
		nonEmpty = true
	case bool:
		f.Kind = "bool"
		f.Size = "s"
		f.Value = strconv.FormatBool(v)
		nonEmpty = v
	case nil:
		// null: empty text field
	}
	g.Fields = append(g.Fields, f)
	if nonEmpty {
		g.Open = true
	}
}

// formatNumber renders a JSON number without a trailing ".0" (7.0 -> "7").
func formatNumber(n json.Number) string {
	if i, err := n.Int64(); err == nil {
		return strconv.FormatInt(i, 10)
	}
	if f, err := n.Float64(); err == nil {
		return strconv.FormatFloat(f, 'f', -1, 64)
	}
	return n.String()
}

// groupTitle maps "zugN" to "Zug N"; any other key is used verbatim.
func groupTitle(key string) string {
	if m := zugKeyRe.FindStringSubmatch(key); m != nil {
		return "Zug " + m[1]
	}
	return key
}

// appendGroup appends the given groups to dst, dropping those without fields.
func appendGroup(dst []formGroup, gs ...formGroup) []formGroup {
	for _, g := range gs {
		if len(g.Fields) > 0 {
			slices.SortStableFunc(g.Fields, func(a, b formField) int {
				return fieldRank(leafKey(a.Path)) - fieldRank(leafKey(b.Path))
			})
			dst = append(dst, g)
		}
	}
	return dst
}

// leafKey returns the last dot-separated segment of a field path.
func leafKey(path string) string {
	return path[strings.LastIndexByte(path, '.')+1:]
}

// expectDelim reads the next token and checks that it is the delimiter d.
func expectDelim(dec *json.Decoder, d json.Delim) error {
	tok, err := dec.Token()
	if err != nil {
		return fmt.Errorf("default.json: %w", err)
	}
	if tok != d {
		return fmt.Errorf("default.json: expected %q, got %v", d, tok)
	}
	return nil
}

// readKey reads an object key token.
func readKey(dec *json.Decoder) (string, error) {
	tok, err := dec.Token()
	if err != nil {
		return "", fmt.Errorf("default.json: %w", err)
	}
	key, ok := tok.(string)
	if !ok {
		return "", fmt.Errorf("default.json: expected object key, got %v", tok)
	}
	return key, nil
}

// skipRest consumes the remainder of an object or array whose opening
// delimiter has already been read, including any nested values.
func skipRest(dec *json.Decoder) error {
	for depth := 1; depth > 0; {
		tok, err := dec.Token()
		if err != nil {
			return fmt.Errorf("default.json: %w", err)
		}
		switch tok {
		case json.Delim('{'), json.Delim('['):
			depth++
		case json.Delim('}'), json.Delim(']'):
			depth--
		}
	}
	return nil
}
