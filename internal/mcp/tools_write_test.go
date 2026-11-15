package mcp

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/webfraggle/zza-generate-images/internal/editor"
	"github.com/webfraggle/zza-generate-images/internal/renderer"
)

func parseJSONResult(t *testing.T, _ any, text string, v any) {
	t.Helper()
	if i := strings.Index(text, "\n"); i >= 0 {
		text = text[:i] // first text content is the JSON result
	}
	if err := json.Unmarshal([]byte(text), v); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, text)
	}
}

func TestCreateTemplate(t *testing.T) {
	dir := copyTemplates(t)
	cs := newSession(t, dir)
	res := callTool(t, cs, "create_template", map[string]any{"name": "test-105-v1", "title": "Test", "description": "D"})
	requireOK(t, res)
	for _, f := range []string{"template.yaml", "default.json"} {
		if _, err := os.Stat(filepath.Join(dir, "test-105-v1", f)); err != nil {
			t.Errorf("missing %s", f)
		}
	}
	src, _ := os.ReadFile(filepath.Join(dir, "test-105-v1", "template.yaml"))
	tmpl, err := renderer.ParseTemplate(src, filepath.Join(dir, "test-105-v1"))
	if err != nil {
		t.Fatal(err)
	}
	if tmpl.Meta.Name != "Test" || tmpl.Meta.Description != "D" {
		t.Errorf("meta: %q %q", tmpl.Meta.Name, tmpl.Meta.Description)
	}
	if !strings.Contains(string(src), "Obere Hälfte") {
		t.Errorf("comments of the starter were lost")
	}
	img := resultImage(t, res)
	if img.Bounds().Dx() != 480 || img.Bounds().Dy() != 480 {
		t.Errorf("preview %v", img.Bounds())
	}
	var out struct {
		Versions map[string]string `json:"versions"`
	}
	parseJSONResult(t, res, resultText(res), &out)
	if out.Versions["template.yaml"] != editor.FileVersion(src) {
		t.Errorf("versions %v", out.Versions)
	}
}

func TestCreateTemplate_Rules(t *testing.T) {
	cs := newSession(t, copyTemplates(t, "sbb-105-v1"))
	requireToolError(t, callTool(t, cs, "create_template", map[string]any{"name": "test"}),
		`template names must end in "-105-v1" (only 1.05" displays are supported)`)
	requireToolError(t, callTool(t, cs, "create_template", map[string]any{"name": "Bad-105-v1"}), "invalid characters")
	requireToolError(t, callTool(t, cs, "create_template", map[string]any{"name": "sbb-105-v1"}), `template "sbb-105-v1" already exists`)
}

func readDisk(t *testing.T, dir, tmpl, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, tmpl, name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestWriteFile_TemplateYAML(t *testing.T) {
	dir := copyTemplates(t, "sbb-105-v1")
	cs := newSession(t, dir)
	src := readDisk(t, dir, "sbb-105-v1", "template.yaml")
	changed := strings.Replace(src, "#", "#", 1) + "\n# edited\n"
	res := callTool(t, cs, "write_file", map[string]any{"template": "sbb-105-v1", "name": "template.yaml", "content": changed})
	requireOK(t, res)
	var out writeFileOut
	parseJSONResult(t, res, resultText(res), &out)
	if out.Version != editor.FileVersion([]byte(changed)) || readDisk(t, dir, "sbb-105-v1", "template.yaml") != changed {
		t.Errorf("write mismatch: %+v", out)
	}
}

func TestWriteFile_InvalidTemplateKeepsOldFile(t *testing.T) {
	dir := copyTemplates(t, "sbb-105-v1")
	cs := newSession(t, dir)
	before := readDisk(t, dir, "sbb-105-v1", "template.yaml")
	res := callTool(t, cs, "write_file", map[string]any{"template": "sbb-105-v1", "name": "template.yaml", "content": "layers: [::"})
	requireToolError(t, res, "template.yaml rejected: ")
	small := strings.Replace(before, "width: 240\n    height: 240", "width: 128\n    height: 64", 1)
	res = callTool(t, cs, "write_file", map[string]any{"template": "sbb-105-v1", "name": "template.yaml", "content": small})
	requireToolError(t, res, "canvas must be 240x240")
	if readDisk(t, dir, "sbb-105-v1", "template.yaml") != before {
		t.Errorf("file changed after rejected write")
	}
}

func TestWriteFile_JSON(t *testing.T) {
	dir := copyTemplates(t, "sbb-105-v1")
	cs := newSession(t, dir)
	requireOK(t, callTool(t, cs, "write_file", map[string]any{"template": "sbb-105-v1", "name": "test-long.json", "content": `{"zug1":{}}`}))
	requireToolError(t, callTool(t, cs, "write_file", map[string]any{"template": "sbb-105-v1", "name": "test-long.json", "content": `[1]`}), "invalid JSON: must be an object")
	if readDisk(t, dir, "sbb-105-v1", "test-long.json") != `{"zug1":{}}` {
		t.Errorf("rejected JSON was written")
	}
}

func TestWriteFile_DefaultJSONRenderWarning(t *testing.T) {
	dir := copyTemplates(t, "sbb-105-v1")
	mustWrite(t, filepath.Join(dir, "sbb-105-v1", "template.yaml"), "layers: [::")
	cs := newSession(t, dir)
	res := callTool(t, cs, "write_file", map[string]any{"template": "sbb-105-v1", "name": "default.json", "content": `{"a":1}`})
	requireOK(t, res)
	var out writeFileOut
	parseJSONResult(t, res, resultText(res), &out)
	if len(out.Warnings) == 0 || !strings.HasPrefix(out.Warnings[0], "probe render failed: ") {
		t.Errorf("warnings %v", out.Warnings)
	}
}

func TestWriteFile_ExpectedVersion(t *testing.T) {
	dir := copyTemplates(t, "sbb-105-v1")
	cs := newSession(t, dir)
	res := callTool(t, cs, "read_template", map[string]any{"template": "sbb-105-v1"})
	var rt readTemplateOut
	parseJSONResult(t, res, resultText(res), &rt)
	v := rt.Versions["template.yaml"]
	content := rt.TemplateYAML + "\n# one\n"
	requireOK(t, callTool(t, cs, "write_file", map[string]any{"template": "sbb-105-v1", "name": "template.yaml", "content": content, "expected_version": v}))
	res = callTool(t, cs, "write_file", map[string]any{"template": "sbb-105-v1", "name": "template.yaml", "content": rt.TemplateYAML + "\n# two\n", "expected_version": v})
	requireToolError(t, res, `conflict: template.yaml was changed since you read it (current version `)
	if readDisk(t, dir, "sbb-105-v1", "template.yaml") != content {
		t.Errorf("conflicting write changed the file")
	}
}

func TestWriteFile_ConcurrentSameVersion(t *testing.T) {
	dir := copyTemplates(t, "sbb-105-v1")
	cs := newSession(t, dir)
	src := readDisk(t, dir, "sbb-105-v1", "template.yaml")
	v := editor.FileVersion([]byte(src))
	var wg sync.WaitGroup
	results := make([]bool, 2)
	texts := make([]string, 2)
	for i := range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res := callTool(t, cs, "write_file", map[string]any{"template": "sbb-105-v1", "name": "template.yaml",
				"content": src + "\n# writer " + string(rune('A'+i)) + "\n", "expected_version": v})
			results[i] = !res.IsError
			texts[i] = resultText(res)
		}()
	}
	wg.Wait()
	ok := 0
	for i, r := range results {
		if r {
			ok++
		} else if !strings.HasPrefix(texts[i], "conflict: ") {
			t.Errorf("loser error: %q", texts[i])
		}
	}
	if ok != 1 {
		t.Errorf("%d writers succeeded, want exactly 1", ok)
	}
}

func TestWriteFile_Rejects(t *testing.T) {
	dir := copyTemplates(t, "sbb-105-v1")
	cs := newSession(t, dir)
	requireToolError(t, callTool(t, cs, "write_file", map[string]any{"template": "sbb-105-v1", "name": "bg.png", "content": "x"}), "only .yaml and .json files can be written")
	requireToolError(t, callTool(t, cs, "write_file", map[string]any{"template": "sbb-105-v1", "name": "../x.yaml", "content": "a: 1"}), "invalid file name")
	requireToolError(t, callTool(t, cs, "write_file", map[string]any{"template": "sbb-105-v1", "name": "big.json", "content": "{\"a\":\"" + strings.Repeat("x", 256<<10) + "\"}"}), "file too large (max 256 KiB)")
	if err := os.Symlink(filepath.Join(dir, "sbb-105-v1", "default.json"), filepath.Join(dir, "sbb-105-v1", "ln.json")); err == nil {
		requireToolError(t, callTool(t, cs, "write_file", map[string]any{"template": "sbb-105-v1", "name": "ln.json", "content": "{}"}), "refusing to overwrite non-regular file")
	}
}

func b64(b []byte) string { return base64.StdEncoding.EncodeToString(b) }

func TestUploadImage(t *testing.T) {
	dir := copyTemplates(t, "sbb-105-v1")
	cs := newSession(t, dir)
	pb := pngBytes(t, solid(10, 10, color.NRGBA{200, 0, 0, 255}))
	res := callTool(t, cs, "upload_image", map[string]any{"template": "sbb-105-v1", "name": "logo.png", "data_base64": b64(pb)})
	requireOK(t, res)
	var out uploadImageOut
	parseJSONResult(t, res, resultText(res), &out)
	if out != (uploadImageOut{Name: "logo.png", Width: 10, Height: 10, Bytes: len(pb), Replaced: false}) {
		t.Errorf("out %+v", out)
	}
	res = callTool(t, cs, "upload_image", map[string]any{"template": "sbb-105-v1", "name": "logo.png", "data_base64": b64(pb)})
	requireOK(t, res)
	parseJSONResult(t, res, resultText(res), &out)
	if !out.Replaced {
		t.Errorf("second upload not replaced")
	}
	requireOK(t, callTool(t, cs, "upload_image", map[string]any{"template": "sbb-105-v1", "name": "d.png", "data_base64": "data:image/png;base64," + b64(pb)}))
	requireOK(t, callTool(t, cs, "upload_image", map[string]any{"template": "sbb-105-v1", "name": "my logo.PNG", "data_base64": b64(pb)}))
	if _, err := os.Stat(filepath.Join(dir, "sbb-105-v1", "my_logo.PNG")); err != nil {
		t.Errorf("sanitized name missing: %v", err)
	}
	jb := jpegBytes(t, solid(8, 8, color.NRGBA{0, 0, 200, 255}))
	requireToolError(t, callTool(t, cs, "upload_image", map[string]any{"template": "sbb-105-v1", "name": "x.png", "data_base64": b64(jb)}), "file content is jpeg but name ends in .png")
	requireOK(t, callTool(t, cs, "upload_image", map[string]any{"template": "sbb-105-v1", "name": "x.jpg", "data_base64": b64(jb)}))
	requireToolError(t, callTool(t, cs, "upload_image", map[string]any{"template": "sbb-105-v1", "name": "x.gif", "data_base64": b64(pb)}), "only .png, .jpg and .jpeg images are allowed")
	requireToolError(t, callTool(t, cs, "upload_image", map[string]any{"template": "sbb-105-v1", "name": "x.png", "data_base64": "!!!not base64"}), "invalid base64")
	requireToolError(t, callTool(t, cs, "upload_image", map[string]any{"template": "sbb-105-v1", "name": "x.png", "data_base64": b64(make([]byte, maxUploadBytes+1))}), "image too large (max 5 MiB)")
	requireToolError(t, callTool(t, cs, "upload_image", map[string]any{"template": "sbb-105-v1", "name": "x.png", "data_base64": b64([]byte("TOPSECRET"))}), "not a PNG or JPEG image")
}

func TestImportImage(t *testing.T) {
	dir := copyTemplates(t, "sbb-105-v1")
	cs := newSession(t, dir)
	src := t.TempDir()

	big := filepath.Join(src, "photo.jpg")
	if err := os.WriteFile(big, jpegBytes(t, solid(3000, 1000, color.NRGBA{10, 120, 30, 255})), 0o644); err != nil {
		t.Fatal(err)
	}
	res := callTool(t, cs, "import_image", map[string]any{"template": "sbb-105-v1", "path": big})
	requireOK(t, res)
	var out importImageOut
	parseJSONResult(t, res, resultText(res), &out)
	if out.Name != "photo.jpg" || out.Width != 2048 || out.Height != 683 || out.ScaledFrom == nil || *out.ScaledFrom != (dims{3000, 1000}) {
		t.Errorf("out %+v", out)
	}
	if im := resultImage(t, res); max(im.Bounds().Dx(), im.Bounds().Dy()) > 768 {
		t.Errorf("preview too large: %v", im.Bounds())
	}

	smallBytes := pngBytes(t, solid(20, 10, color.NRGBA{1, 2, 3, 255}))
	small := filepath.Join(src, "orig.png")
	if err := os.WriteFile(small, smallBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	requireOK(t, callTool(t, cs, "import_image", map[string]any{"template": "sbb-105-v1", "path": small, "name": "ref.png"}))
	if readDisk(t, dir, "sbb-105-v1", "ref.png") != string(smallBytes) {
		t.Errorf("small image is not a byte copy")
	}

	requireToolError(t, callTool(t, cs, "import_image", map[string]any{"template": "sbb-105-v1", "path": "photo.jpg"}), "path must be absolute")
	link := filepath.Join(src, "link.png")
	if err := os.Symlink(small, link); err == nil {
		requireToolError(t, callTool(t, cs, "import_image", map[string]any{"template": "sbb-105-v1", "path": link}), "cannot read image file")
	}
	sub := filepath.Join(src, "dir.png")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	requireToolError(t, callTool(t, cs, "import_image", map[string]any{"template": "sbb-105-v1", "path": sub}), "cannot read image file")
	secret := filepath.Join(src, "secret.png")
	if err := os.WriteFile(secret, []byte("TOPSECRET"), 0o644); err != nil {
		t.Fatal(err)
	}
	res = callTool(t, cs, "import_image", map[string]any{"template": "sbb-105-v1", "path": secret})
	requireToolError(t, res, "not a PNG or JPEG image")
	if strings.Contains(resultText(res), "TOPSECRET") {
		t.Errorf("error leaks file content")
	}
	missing := filepath.Join(src, "nope.png")
	requireToolError(t, callTool(t, cs, "import_image", map[string]any{"template": "sbb-105-v1", "path": missing}), "cannot read image file")
	// Extension is checked before the file is looked at: no existence oracle
	// for arbitrary paths.
	for _, p := range []string{filepath.Join(src, "nope.txt"), filepath.Join(src, "orig.png", "x"), small + ".bak"} {
		res := callTool(t, cs, "import_image", map[string]any{"template": "sbb-105-v1", "path": p})
		requireToolError(t, res, "only .png, .jpg and .jpeg images can be imported")
	}
	// Missing, directory, symlink and unreadable all give the same text.
	texts := map[string]bool{}
	for _, p := range []string{missing, sub, link, filepath.Join(src, "nodir", "x.png")} {
		texts[resultText(callTool(t, cs, "import_image", map[string]any{"template": "sbb-105-v1", "path": p}))] = true
	}
	if len(texts) != 1 {
		t.Errorf("import_image errors differ by path state: %v", texts)
	}
}

// A 16-bit PNG 8192x8192 is only ~0.5 MiB compressed but decodes to 1 GiB:
// import_image must enforce the pixel limit before decoding.
func TestImportImage_PixelLimit(t *testing.T) {
	dir := copyTemplates(t, "sbb-105-v1")
	cs := newSession(t, dir)
	p := filepath.Join(t.TempDir(), "big.png")
	// 6000x6000 = 36 M pixels: within maxImportSide, above maxToolInputPixels.
	img := image.NewNRGBA64(image.Rect(0, 0, 6000, 6000))
	var buf bytes.Buffer
	if err := (&png.Encoder{CompressionLevel: png.BestSpeed}).Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	res := callTool(t, cs, "import_image", map[string]any{"template": "sbb-105-v1", "path": p})
	requireToolError(t, res, "max 25 megapixels")
}

func TestWriteFile_CaseVariantsRejected(t *testing.T) {
	dir := copyTemplates(t, "sbb-105-v1")
	cs := newSession(t, dir)
	before := readDisk(t, dir, "sbb-105-v1", "template.yaml")
	for _, name := range []string{"Template.yaml", "TEMPLATE.yaml", "template.YAML", "DEFAULT.json", "Default.json"} {
		res := callTool(t, cs, "write_file", map[string]any{"template": "sbb-105-v1", "name": name, "content": "layers: [::"})
		requireToolError(t, res, "(lowercase)")
	}
	if readDisk(t, dir, "sbb-105-v1", "template.yaml") != before {
		t.Errorf("template.yaml changed")
	}
}

// withEXIF inserts an APP1 "Exif" segment right after the SOI marker.
func withEXIF(t *testing.T, jpg []byte) []byte {
	t.Helper()
	payload := []byte("Exif\x00\x00GPS-SECRET")
	seg := append([]byte{0xFF, 0xE1, byte((len(payload) + 2) >> 8), byte(len(payload) + 2)}, payload...)
	out := append([]byte{}, jpg[:2]...)
	out = append(out, seg...)
	return append(out, jpg[2:]...)
}

func TestJPEGMetadataStripped(t *testing.T) {
	dir := copyTemplates(t, "sbb-105-v1")
	cs := newSession(t, dir)
	jb := withEXIF(t, jpegBytes(t, solid(16, 16, color.NRGBA{9, 99, 9, 255})))
	if !strings.Contains(string(jb), "Exif") {
		t.Fatal("test setup: no EXIF")
	}
	requireOK(t, callTool(t, cs, "upload_image", map[string]any{"template": "sbb-105-v1", "name": "u.jpg", "data_base64": b64(jb)}))
	if got := readDisk(t, dir, "sbb-105-v1", "u.jpg"); strings.Contains(got, "Exif") || strings.Contains(got, "GPS-SECRET") {
		t.Errorf("upload_image kept EXIF")
	}
	src := filepath.Join(t.TempDir(), "p.jpg")
	if err := os.WriteFile(src, jb, 0o644); err != nil {
		t.Fatal(err)
	}
	requireOK(t, callTool(t, cs, "import_image", map[string]any{"template": "sbb-105-v1", "path": src}))
	if got := readDisk(t, dir, "sbb-105-v1", "p.jpg"); strings.Contains(got, "Exif") || strings.Contains(got, "GPS-SECRET") {
		t.Errorf("import_image kept EXIF")
	}
}
