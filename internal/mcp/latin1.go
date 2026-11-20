package mcp

import (
	"bytes"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// The ZZA (on-device renderer) only knows Latin-1 (U+0000-U+00FF): a template
// or train data with other characters cannot be shown on the device. The MCP
// therefore refuses every non-Latin-1 character the device could see — all
// YAML keys and values, all of default.json, create_template's title and
// description. YAML comments are free (they never leave the file).

const maxLatin1Reports = 10

// latin1Hints suggests a Latin-1 replacement for common typographic characters.
var latin1Hints = map[rune]string{
	'–': "-", '—': "-", '‐': "-", '‑': "-", '−': "-",
	'‘': "'", '’': "'", '‚': "'", '′': "'",
	'“': `"`, '”': `"`, '„': `"`, '″': `"`,
	'…': "...", '•': "·", '€': "EUR", '→': "->", '←': "<-",
	'\u2009': " ", '\u202f': " ", '\u200b': "",
}

const latin1Rule = "only Latin-1 characters (U+0000-U+00FF: ASCII plus e.g. äöüß éàç ° ½ · « ») are allowed, because the ZZA renders Latin-1 only — replace them"

type latin1Issue struct {
	line int
	r    rune
}

func (i latin1Issue) String() string {
	return fmt.Sprintf("line %d: %s", i.line, describeRune(i.r))
}

// describeRune names a rune and its Latin-1 replacement, if one is known.
func describeRune(r rune) string {
	s := fmt.Sprintf("%q (U+%04X)", r, r)
	if h, ok := latin1Hints[r]; ok {
		q := `"`
		if strings.Contains(h, `"`) {
			q = "'"
		}
		s += " -> use " + q + h + q
	}
	return s
}

// nonLatin1 returns the non-Latin-1 runes of s with their line (first = line).
func nonLatin1(s string, line int) []latin1Issue {
	var out []latin1Issue
	for _, r := range s {
		if r == '\n' {
			line++
		}
		if r > 0xFF {
			out = append(out, latin1Issue{line, r})
		}
	}
	return out
}

func latin1Error(what string, issues []latin1Issue) error {
	if len(issues) == 0 {
		return nil
	}
	parts := make([]string, 0, maxLatin1Reports+1)
	for i, is := range issues {
		if i == maxLatin1Reports {
			parts = append(parts, fmt.Sprintf("and %d more", len(issues)-i))
			break
		}
		parts = append(parts, is.String())
	}
	return fmt.Errorf("%s: %s: %s", what, latin1Rule, strings.Join(parts, "; "))
}

// checkLatin1YAML checks all keys and scalar values of a YAML document;
// comments are ignored. Unparsable YAML passes (the YAML check reports it).
func checkLatin1YAML(content []byte) error {
	var root yaml.Node
	if yaml.Unmarshal(content, &root) != nil {
		return nil
	}
	var issues []latin1Issue
	var walk func(n *yaml.Node)
	walk = func(n *yaml.Node) {
		if n.Kind == yaml.ScalarNode {
			issues = append(issues, nonLatin1(n.Value, n.Line)...)
		}
		for _, c := range n.Content {
			walk(c)
		}
	}
	walk(&root)
	return latin1Error("non-Latin-1 characters", issues)
}

// checkLatin1JSON checks the whole JSON text (JSON has no comments).
func checkLatin1JSON(content []byte) error {
	if !bytes.ContainsFunc(content, func(r rune) bool { return r > 0xFF }) {
		return nil
	}
	return latin1Error("non-Latin-1 characters", nonLatin1(string(content), 1))
}

// checkLatin1Text checks a single-line input field (create_template).
func checkLatin1Text(field, s string) error {
	issues := nonLatin1(s, 1)
	if len(issues) == 0 {
		return nil
	}
	parts := make([]string, 0, len(issues))
	for i, is := range issues {
		if i == maxLatin1Reports {
			break
		}
		parts = append(parts, describeRune(is.r))
	}
	return fmt.Errorf("%s: %s: %s", field, latin1Rule, strings.Join(parts, "; "))
}
