package mcp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckLatin1YAML(t *testing.T) {
	ok := "# Kommentar – darf alles: → … „x“\nmeta:\n  name: \"Größe ÄÖÜ ß é ½ ·\"\nlayers:\n  - type: text\n    value: \"{{zug1.via}}\"  # – egal\n"
	if err := checkLatin1YAML([]byte(ok)); err != nil {
		t.Fatalf("Latin-1 values and non-Latin-1 comments must pass: %v", err)
	}

	bad := "meta:\n  name: \"x\"\nlayers:\n  - type: text\n    value: \"{{zug1.via | replace(' - ', ' – ')}}\"\n  - type: text\n    value: \"+5’\"\n"
	err := checkLatin1YAML([]byte(bad))
	if err == nil {
		t.Fatal("non-Latin-1 values must be rejected")
	}
	msg := err.Error()
	for _, want := range []string{"Latin-1", "line 5", "U+2013", "line 7", "U+2019", `"-"`, `"'"`} {
		if !strings.Contains(msg, want) {
			t.Errorf("error lacks %q:\n%s", want, msg)
		}
	}

	if err := checkLatin1YAML([]byte("layers:\n  - {type: text, value: \"x\", \"•\": 1}\n")); err == nil {
		t.Error("non-Latin-1 keys must be rejected")
	}
}

func TestCheckLatin1JSON(t *testing.T) {
	if err := checkLatin1JSON([]byte(`{"zug1": {"vonnach": "Zürich HB", "hinweis": "½ Std. später"}}`)); err != nil {
		t.Fatalf("Latin-1 JSON must pass: %v", err)
	}
	err := checkLatin1JSON([]byte("{\n  \"zug1\": {\n    \"via\": \"Bern – Thun …\"\n  }\n}"))
	if err == nil {
		t.Fatal("non-Latin-1 JSON must be rejected")
	}
	for _, want := range []string{"line 3", "U+2013", "U+2026", `"..."`} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error lacks %q:\n%s", want, err.Error())
		}
	}
}

func TestCheckLatin1_ListIsCapped(t *testing.T) {
	err := checkLatin1JSON([]byte(`{"a": "` + strings.Repeat("€", 50) + `"}`))
	if err == nil {
		t.Fatal("want error")
	}
	if n := strings.Count(err.Error(), "U+20AC"); n > maxLatin1Reports {
		t.Errorf("%d reports, want at most %d", n, maxLatin1Reports)
	}
	if !strings.Contains(err.Error(), "more") {
		t.Errorf("capped list must say there are more:\n%s", err.Error())
	}
}

func TestWriteFile_RejectsNonLatin1(t *testing.T) {
	dir := copyTemplates(t)
	cs := newSession(t, dir)
	requireOK(t, callTool(t, cs, "create_template", map[string]any{"name": "l1-105-v1"}))
	p := filepath.Join(dir, "l1-105-v1", "template.yaml")
	before, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	src := strings.Replace(string(before), "name:", "name: \"Zug – Ziel\" #", 1)
	res := callTool(t, cs, "write_file", map[string]any{"template": "l1-105-v1", "name": "template.yaml", "content": src})
	requireToolError(t, res, "Latin-1")
	after, _ := os.ReadFile(p)
	if string(after) != string(before) {
		t.Error("rejected template.yaml must not be written")
	}

	res = callTool(t, cs, "write_file", map[string]any{"template": "l1-105-v1", "name": "default.json", "content": `{"zug1": {"via": "A … B"}}`})
	requireToolError(t, res, "Latin-1")
}

func TestCreateTemplate_RejectsNonLatin1Meta(t *testing.T) {
	cs := newSession(t, copyTemplates(t))
	requireToolError(t, callTool(t, cs, "create_template", map[string]any{"name": "l2-105-v1", "title": "Wien – Mitte"}), "Latin-1")
	requireToolError(t, callTool(t, cs, "create_template", map[string]any{"name": "l3-105-v1", "description": "Tafel → rechts"}), "Latin-1")
}
