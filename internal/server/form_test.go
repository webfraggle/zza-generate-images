package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func readTemplateFiles(t *testing.T, name string) (yamlSrc, defaultJSON []byte) {
	t.Helper()
	dir := filepath.Join("..", "..", "templates", name)
	y, err := os.ReadFile(filepath.Join(dir, "template.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	j, err := os.ReadFile(filepath.Join(dir, "default.json"))
	if err != nil {
		t.Fatal(err)
	}
	return y, j
}

// groupPaths flattens groups into title -> ordered field paths.
func groupPaths(groups []formGroup) (titles []string, paths [][]string) {
	for _, g := range groups {
		titles = append(titles, g.Title)
		var p []string
		for _, f := range g.Fields {
			p = append(p, f.Path)
		}
		paths = append(paths, p)
	}
	return titles, paths
}

func TestBuildForm_Faltblatt(t *testing.T) {
	y, j := readTemplateFiles(t, "faltblatt-105-v1")
	groups, err := buildForm(y, j)
	if err != nil {
		t.Fatal(err)
	}
	titles, paths := groupPaths(groups)
	wantTitles := []string{"Bahnsteig", "Zug 1"}
	wantPaths := [][]string{
		{"gleis"},
		{"zug1.zeit", "zug1.nr", "zug1.vonnach", "zug1.via"},
	}
	if !reflect.DeepEqual(titles, wantTitles) || !reflect.DeepEqual(paths, wantPaths) {
		t.Errorf("got %v %v, want %v %v", titles, paths, wantTitles, wantPaths)
	}
	gleis := groups[0].Fields[0]
	if gleis.Label != "Gleis" || gleis.Value != "6" || gleis.Kind != "number" || gleis.Size != "s" {
		t.Errorf("gleis field: %+v", gleis)
	}
	zeit := groups[1].Fields[0]
	if zeit.Label != "Abfahrt" || zeit.Placeholder != "hh:mm" || zeit.Kind != "text" || zeit.Size != "s" {
		t.Errorf("zeit field: %+v", zeit)
	}
	vonnach := groups[1].Fields[2]
	if vonnach.Label != "Ziel" || vonnach.Value != "Frankfurt" || vonnach.Size != "l" {
		t.Errorf("vonnach field: %+v", vonnach)
	}
	if !groups[0].Open || !groups[1].Open {
		t.Error("groups with defaults should be open")
	}
}

func TestBuildForm_Umuc(t *testing.T) {
	y, j := readTemplateFiles(t, "umuc-105-v1")
	groups, err := buildForm(y, j)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, g := range groups {
		seen[g.Title] = true
		if (g.Title == "Zug 1" || g.Title == "Zug 2" || g.Title == "Zug 3") && !g.Open {
			t.Errorf("%s should be open", g.Title)
		}
	}
	for _, want := range []string{"Zug 1", "Zug 2", "Zug 3"} {
		if !seen[want] {
			t.Errorf("missing group %q in %v", want, groups)
		}
	}
}

func TestBuildForm_Synthetic(t *testing.T) {
	y := []byte(`
layers:
  - value: "{{zug2.vonnach}} {{zug1.abw}} {{gleis}} {{mirrored}} {{extra.foo}} {{zug1.zeit}}"
    if: "zug3.unused"
`)
	j := []byte(`{
		"mirrored": false,
		"gleis": 7.0,
		"stationNr": 1,
		"zug1": {"zeit": "", "abw": 5, "deep": {"x": 1}, "list": [1, {"a": 2}]},
		"extra": {"foo": null},
		"zug2": {"vonnach": ""},
		"zug3": {"other": "x"},
		"arr": [1, 2]
	}`)
	groups, err := buildForm(y, j)
	if err != nil {
		t.Fatal(err)
	}
	titles, paths := groupPaths(groups)
	wantTitles := []string{"Bahnsteig", "Zug 1", "extra", "Zug 2"}
	wantPaths := [][]string{
		{"gleis", "mirrored"},
		{"zug1.zeit", "zug1.abw"},
		{"extra.foo"},
		{"zug2.vonnach"},
	}
	if !reflect.DeepEqual(titles, wantTitles) || !reflect.DeepEqual(paths, wantPaths) {
		t.Fatalf("got %v %v, want %v %v", titles, paths, wantTitles, wantPaths)
	}

	mirrored := groups[0].Fields[1]
	if mirrored.Kind != "bool" || mirrored.Value != "false" || mirrored.Size != "s" || mirrored.Label != "Gespiegelt" {
		t.Errorf("mirrored: %+v", mirrored)
	}
	gleis := groups[0].Fields[0]
	if gleis.Kind != "number" || gleis.Value != "7" {
		t.Errorf("gleis: %+v", gleis)
	}
	abw := groups[1].Fields[1]
	if abw.Kind != "number" || abw.Value != "5" || abw.Placeholder != "0" || abw.Label != "Verspätung (Min.)" {
		t.Errorf("abw: %+v", abw)
	}
	foo := groups[2].Fields[0]
	if foo.Kind != "text" || foo.Value != "" || foo.Label != "foo" || foo.Size != "l" {
		t.Errorf("foo: %+v", foo)
	}

	wantOpen := []bool{true, true, false, false}
	for i, g := range groups {
		if g.Open != wantOpen[i] {
			t.Errorf("group %q Open=%v, want %v", g.Title, g.Open, wantOpen[i])
		}
	}
}

func TestBuildForm_BoolFalseIsEmpty(t *testing.T) {
	groups, err := buildForm([]byte(`v: "{{mirrored}}"`), []byte(`{"mirrored": false}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 1 || groups[0].Open {
		t.Errorf("bool false should not open group: %+v", groups)
	}
	groups, _ = buildForm([]byte(`v: "{{mirrored}}"`), []byte(`{"mirrored": true}`))
	if len(groups) != 1 || !groups[0].Open {
		t.Errorf("bool true should open group: %+v", groups)
	}
}

func TestBuildForm_EmptyAndInvalid(t *testing.T) {
	groups, err := buildForm([]byte(`v: "{{gleis}}"`), nil)
	if err != nil || groups != nil {
		t.Errorf("empty json: got %v, %v", groups, err)
	}
	if _, err := buildForm([]byte(`v: "{{gleis}}"`), []byte(`{"gleis": `)); err == nil {
		t.Error("invalid json: expected error")
	}
	if _, err := buildForm([]byte(`v: "{{gleis}}"`), []byte(`[1, 2]`)); err == nil {
		t.Error("non-object json: expected error")
	}
	if _, err := buildForm([]byte("a: [unclosed"), []byte(`{"gleis": 1}`)); err == nil {
		t.Error("invalid yaml: expected error")
	}
}

func TestLoadDetailData_Faltblatt(t *testing.T) {
	srv := newTestServer(t)
	d, err := srv.loadDetailData(context.Background(), "faltblatt-105-v1")
	if err != nil {
		t.Fatal(err)
	}
	if !d.HasDefault || d.DefaultJSON == "" {
		t.Error("expected default.json to be loaded")
	}
	paths := map[string]bool{}
	for _, g := range d.Form {
		for _, f := range g.Fields {
			paths[f.Path] = true
		}
	}
	if !paths["zug1.vonnach"] {
		t.Errorf("form missing zug1.vonnach: %+v", d.Form)
	}
	if paths["zug1.reihung"] {
		t.Error("form must not contain zug1.reihung")
	}
}

func TestLoadDetailData_NotFound(t *testing.T) {
	srv := newTestServer(t)
	if _, err := srv.loadDetailData(context.Background(), "does-not-exist"); err == nil {
		t.Error("expected error for unknown template")
	}
}

func TestServer_Detail_OK(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/faltblatt-105-v1", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("detail: got %d, want 200", rr.Code)
	}
}
