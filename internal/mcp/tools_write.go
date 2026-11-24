package mcp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"strings"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"gopkg.in/yaml.v3"

	"github.com/webfraggle/zza-generate-images/internal/devexport"
	"github.com/webfraggle/zza-generate-images/internal/editor"
	"github.com/webfraggle/zza-generate-images/internal/renderer"
)

func init() { registerToolGroup("write", registerWriteTools) }

const (
	maxMetaTitleLen = 200
	maxMetaDescLen  = 1000
	maxErrLen       = 1000
)

type createTemplateIn struct {
	Name        string `json:"name" jsonschema:"template name, lowercase letters, digits and hyphens, ending in -105-v1, at most 24 characters in total"`
	Title       string `json:"title,omitempty" jsonschema:"display title (meta.name); defaults to the name"`
	Description string `json:"description,omitempty" jsonschema:"short description (meta.description)"`
}

type renameTemplateIn struct {
	Template string `json:"template" jsonschema:"current template name"`
	Name     string `json:"name" jsonschema:"new template name, lowercase letters, digits and hyphens, ending in -105-v1, at most 24 characters in total"`
}

type renameTemplateOut struct {
	Template string `json:"template"`
}

type createTemplateOut struct {
	Template string            `json:"template"`
	Files    []editor.FileInfo `json:"files"`
	Versions map[string]string `json:"versions"`
}

type writeFileIn struct {
	Template        string `json:"template" jsonschema:"template name"`
	Name            string `json:"name" jsonschema:"file name, e.g. template.yaml or default.json"`
	Content         string `json:"content" jsonschema:"complete new file content"`
	ExpectedVersion string `json:"expected_version,omitempty" jsonschema:"version from read_template/read_file; the write is refused if the file changed since"`
}

type writeFileOut struct {
	Name     string   `json:"name"`
	Version  string   `json:"version"`
	Warnings []string `json:"warnings"`
}

type uploadImageIn struct {
	Template   string `json:"template" jsonschema:"template name"`
	Name       string `json:"name" jsonschema:"file name ending in .png, .jpg or .jpeg"`
	DataBase64 string `json:"data_base64" jsonschema:"image bytes, base64 encoded (a data: URL is accepted)"`
}

type uploadImageOut struct {
	Name     string `json:"name"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
	Bytes    int    `json:"bytes"`
	Replaced bool   `json:"replaced"`
}

type importImageIn struct {
	Template string `json:"template" jsonschema:"template name"`
	Path     string `json:"path" jsonschema:"absolute path of a PNG/JPEG file on the user's computer (~/ is allowed)"`
	Name     string `json:"name,omitempty" jsonschema:"file name in the template; defaults to the source file name"`
}

type dims struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

type importImageOut struct {
	Name       string `json:"name"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`
	ScaledFrom *dims  `json:"scaled_from,omitempty"`
	Replaced   bool   `json:"replaced"`
}

func registerWriteTools(s *sdk.Server, e *env) {
	sdk.AddTool(s, &sdk.Tool{
		Name:        "create_template",
		Description: `Creates a new template from the starter (canvas 240x240, mirrored front/back layout). Name: lowercase letters, digits and hyphens, must end in "-105-v1" and be at most 24 characters long in total (17 before the suffix), e.g. "berlin-sbahn-105-v1". Title and description: Latin-1 characters only. Returns a preview image.`,
	}, func(ctx context.Context, _ *sdk.CallToolRequest, in createTemplateIn) (*sdk.CallToolResult, createTemplateOut, error) {
		return e.createTemplate(in)
	})

	sdk.AddTool(s, &sdk.Tool{
		Name:        "rename_template",
		Description: `Renames a template (its folder; the title meta.name stays). Use it when a name is longer than 24 characters: the display cannot install such a template. New name: lowercase letters, digits and hyphens, must end in "-105-v1", at most 24 characters in total, not taken yet. Afterwards use the new name in all tools; tell the user that an editor or preview page of the old name must be reopened.`,
	}, func(ctx context.Context, _ *sdk.CallToolRequest, in renameTemplateIn) (*sdk.CallToolResult, renameTemplateOut, error) {
		out, err := e.renameTemplate(in)
		return nil, out, err
	})

	sdk.AddTool(s, &sdk.Tool{
		Name:        "write_file",
		Description: "Writes template.yaml, default.json or another .json/.yaml file of a template. Only Latin-1 characters (U+0000-U+00FF) are allowed in YAML keys/values and in JSON (the ZZA renders Latin-1 only; YAML comments are free): use - instead of en/em dashes, ' and \" instead of typographic quotes, ... instead of an ellipsis. template.yaml is validated (YAML, canvas must be 240x240, test render with default.json) and NOT saved if invalid — the error explains why. Pass expected_version from read_template/read_file to avoid overwriting edits made in the editor meanwhile. Returns the new version and warnings (design issues, unknown keys = likely typos, \"device:\" = would fail on the ZZA: too much memory for the QUAD (1.5 MiB per track) or a render step above the work limit). Fix device warnings before finishing. Call render_preview afterwards.",
	}, func(ctx context.Context, _ *sdk.CallToolRequest, in writeFileIn) (*sdk.CallToolResult, writeFileOut, error) {
		out, err := e.writeFile(in)
		return nil, out, err
	})

	sdk.AddTool(s, &sdk.Tool{
		Name:        "upload_image",
		Description: "Stores a PNG or JPEG (base64) in the template, e.g. a logo you generated. For photos the user attached in the chat, prefer import_image with the file path — base64 copies of attachments are unreliable. JPEG metadata is stripped; PNGs with text/EXIF chunks are re-encoded, other PNGs are stored unchanged.",
	}, func(ctx context.Context, _ *sdk.CallToolRequest, in uploadImageIn) (*sdk.CallToolResult, uploadImageOut, error) {
		out, err := e.uploadImage(in)
		return nil, out, err
	})

	sdk.AddTool(s, &sdk.Tool{
		Name:        "import_image",
		Description: "Copies a PNG/JPEG from the user's computer (absolute path, ask the user for it) into the template, scaling it down to max 2048 px. Use it for reference photos of real displays; then use perspective_correct, sample_colors and crop_image. Returns the stored size and a preview. JPEG metadata is stripped; PNGs with text/EXIF chunks are re-encoded, other PNGs are stored unchanged.",
	}, func(ctx context.Context, _ *sdk.CallToolRequest, in importImageIn) (*sdk.CallToolResult, importImageOut, error) {
		return e.importImage(in)
	})
}

// scrubErr removes the templates path from an error text, shortens renderer
// error chains (tidyRenderMsg) and limits the length.
func (e *env) scrubErr(err error, limit int) string {
	s := strings.ReplaceAll(err.Error(), e.templatesDir+string(filepath.Separator), "")
	s = strings.ReplaceAll(s, e.templatesDir, "")
	s = tidyRenderMsg(s)
	if r := []rune(s); len(r) > limit {
		s = string(r[:limit])
	}
	return s
}

// checkNewTemplateName applies the rules for the name of a new or renamed template.
func checkNewTemplateName(name string) error {
	if err := renderer.ValidateTemplateName(name); err != nil {
		return fmt.Errorf("invalid template name: %s", strings.TrimPrefix(err.Error(), "renderer: "))
	}
	if !strings.HasSuffix(name, templateSuffix) {
		return fmt.Errorf(`template names must end in "%s" (only 1.05" displays are supported)`, templateSuffix)
	}
	if len(name) > devexport.MaxNameLen {
		return fmt.Errorf(`template name too long: %q has %d characters, the display allows at most %d (%d before "%s")`,
			name, len(name), devexport.MaxNameLen, devexport.MaxNameLen-len(templateSuffix), templateSuffix)
	}
	return nil
}

func (e *env) renameTemplate(in renameTemplateIn) (renameTemplateOut, error) {
	var zero renameTemplateOut
	if err := checkNewTemplateName(in.Name); err != nil {
		return zero, err
	}
	e.writeMu.Lock()
	defer e.writeMu.Unlock()
	switch err := editor.RenameTemplate(e.templatesDir, in.Template, in.Name); {
	case err == nil:
		return renameTemplateOut{Template: in.Name}, nil
	case errors.Is(err, editor.ErrTemplateNotFound):
		return zero, fmt.Errorf("template %q not found", in.Template)
	case errors.Is(err, editor.ErrTemplateExists):
		return zero, fmt.Errorf("template %q already exists", in.Name)
	default:
		return zero, fmt.Errorf("cannot rename template %q", in.Template)
	}
}

func (e *env) createTemplate(in createTemplateIn) (*sdk.CallToolResult, createTemplateOut, error) {
	var zero createTemplateOut
	if err := checkNewTemplateName(in.Name); err != nil {
		return nil, zero, err
	}
	if len([]rune(in.Title)) > maxMetaTitleLen {
		return nil, zero, fmt.Errorf("title too long (max %d characters)", maxMetaTitleLen)
	}
	if len([]rune(in.Description)) > maxMetaDescLen {
		return nil, zero, fmt.Errorf("description too long (max %d characters)", maxMetaDescLen)
	}
	if err := checkLatin1Text("title", in.Title); err != nil {
		return nil, zero, err
	}
	if err := checkLatin1Text("description", in.Description); err != nil {
		return nil, zero, err
	}
	e.writeMu.Lock()
	defer e.writeMu.Unlock()

	dir := filepath.Join(e.templatesDir, in.Name)
	if _, err := os.Lstat(dir); err == nil {
		return nil, zero, fmt.Errorf("template %q already exists — use read_template / write_file", in.Name)
	}
	if err := editor.InitTemplate(e.templatesDir, in.Name); err != nil {
		return nil, zero, fmt.Errorf("cannot create template %q", in.Name)
	}
	src, err := editor.ReadTextFile(e.templatesDir, in.Name, "template.yaml")
	if err != nil {
		return nil, zero, fmt.Errorf("cannot read the starter template.yaml")
	}
	title := in.Title
	if title == "" {
		title = in.Name
	}
	kv := map[string]string{"name": title}
	if in.Description != "" {
		kv["description"] = in.Description
	}
	edited, err := setMetaScalars(src, kv)
	if err != nil {
		return nil, zero, fmt.Errorf("cannot set template metadata")
	}
	yamlVersion, err := editor.WriteTextFileIfMatch(e.templatesDir, in.Name, "template.yaml", edited, "")
	if err != nil {
		return nil, zero, fmt.Errorf("cannot write template.yaml")
	}
	out := createTemplateOut{Template: in.Name, Versions: map[string]string{"template.yaml": yamlVersion}}
	if def, err := editor.ReadTextFile(e.templatesDir, in.Name, "default.json"); err == nil {
		out.Versions["default.json"] = editor.FileVersion(def)
	}
	out.Files, err = editor.ListFiles(e.templatesDir, in.Name)
	if err != nil || out.Files == nil {
		out.Files = []editor.FileInfo{}
	}
	still, _, err := renderStill(e, in.Name, loadTestData(dir))
	if err != nil {
		return nil, zero, err
	}
	res, err := jsonImageResult(out, []image.Image{scaleNearest(still, 2)})
	if err != nil {
		return nil, zero, err
	}
	return res, out, nil
}

func (e *env) writeFile(in writeFileIn) (writeFileOut, error) {
	var zero writeFileOut
	e.writeMu.Lock()
	defer e.writeMu.Unlock()

	dir, err := e.templateDir(in.Template)
	if err != nil {
		return zero, err
	}
	ext := strings.ToLower(filepath.Ext(in.Name))
	if ext != ".yaml" && ext != ".json" {
		return zero, fmt.Errorf("only .yaml and .json files can be written")
	}
	// Other spellings would bypass validation on case-insensitive file systems.
	for _, special := range []string{"template.yaml", "default.json"} {
		if strings.EqualFold(in.Name, special) && in.Name != special {
			return zero, fmt.Errorf("file name must be %q (lowercase)", special)
		}
	}
	p, err := editor.SafeFilePath(e.templatesDir, in.Template, in.Name)
	if err == nil {
		err = editor.ValidateWriteName(in.Name)
	}
	if err != nil {
		return zero, errInvalidWriteName(in.Name)
	}
	content := []byte(in.Content)
	if len(content) > maxTextFileBytes {
		return zero, fmt.Errorf("file too large (max %d KiB)", maxTextFileBytes>>10)
	}
	if fi, err := os.Lstat(p); err == nil && !fi.Mode().IsRegular() {
		return zero, fmt.Errorf("refusing to overwrite non-regular file %q", in.Name)
	}
	if err := checkTemplateCapacity(dir, in.Name, int64(len(content))); err != nil {
		return zero, err
	}

	latin1 := checkLatin1JSON
	if ext == ".yaml" {
		latin1 = checkLatin1YAML
	}
	if err := latin1(content); err != nil {
		return zero, fmt.Errorf("%s rejected: %v", in.Name, err)
	}

	warnings := []string{}
	switch {
	case in.Name == "template.yaml":
		w, err := validateTemplateYAML(e, in.Template, content)
		if err != nil {
			return zero, fmt.Errorf("template.yaml rejected: %s", e.scrubErr(err, maxErrLen))
		}
		warnings = append(warnings, w...)
	case ext == ".yaml":
		var v any
		if err := yaml.Unmarshal(content, &v); err != nil {
			return zero, fmt.Errorf("invalid YAML: %s", e.scrubErr(err, maxErrLen))
		}
	default:
		if err := validateJSONObject(content); err != nil {
			return zero, err
		}
	}

	version, err := editor.WriteTextFileIfMatch(e.templatesDir, in.Template, in.Name, content, in.ExpectedVersion)
	switch {
	case errors.Is(err, editor.ErrVersionConflict):
		var vc *editor.VersionConflictError
		cur := ""
		if errors.As(err, &vc) {
			cur = vc.Current
		}
		if cur == "" {
			cur = "none, the file does not exist"
		}
		return zero, fmt.Errorf("conflict: %s was changed since you read it (current version %s). Call read_file/read_template again and re-apply your change.", in.Name, cur)
	case errors.Is(err, editor.ErrInvalidName):
		return zero, fmt.Errorf("invalid file name %q (allowed: letters, digits, . _ -)", in.Name)
	case err != nil:
		return zero, fmt.Errorf("cannot write %q", in.Name)
	}

	probeOK := true
	if in.Name == "default.json" {
		var data map[string]any
		if err := json.Unmarshal(content, &data); err == nil {
			if _, _, err := renderStill(e, in.Template, data); err != nil {
				probeOK = false
				warnings = append(warnings, "probe render failed: "+strings.TrimPrefix(e.scrubErr(err, maxErrLen), "render failed: "))
			}
		}
	}
	// What would fail on the ZZA itself (memory, work limit, compatibility).
	if probeOK && (in.Name == "template.yaml" || in.Name == "default.json") {
		warnings = append(warnings, deviceWarnings(e, in.Template)...)
	}
	return writeFileOut{Name: in.Name, Version: version, Warnings: warnings}, nil
}

// decodeBase64 accepts a data: URL prefix, standard and URL alphabets, with
// or without padding, and ignores whitespace.
func decodeBase64(s string) ([]byte, error) {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "data:") {
		if i := strings.Index(s, "base64,"); i >= 0 {
			s = s[i+len("base64,"):]
		}
	}
	s = strings.Map(func(r rune) rune {
		if r == ' ' || r == '\n' || r == '\r' || r == '\t' {
			return -1
		}
		return r
	}, s)
	for _, enc := range []*base64.Encoding{base64.StdEncoding, base64.RawStdEncoding, base64.URLEncoding, base64.RawURLEncoding} {
		if b, err := enc.DecodeString(s); err == nil {
			return b, nil
		}
	}
	return nil, fmt.Errorf("invalid base64")
}

func imageExt(name string) (string, bool) {
	ext := strings.ToLower(filepath.Ext(name))
	return ext, ext == ".png" || ext == ".jpg" || ext == ".jpeg"
}

// formatMatchesExt reports whether a decoded format ("png"/"jpeg") fits the extension.
func formatMatchesExt(format, ext string) bool {
	if format == "png" {
		return ext == ".png"
	}
	return ext == ".jpg" || ext == ".jpeg"
}

func (e *env) uploadImage(in uploadImageIn) (uploadImageOut, error) {
	var zero uploadImageOut
	e.writeMu.Lock()
	defer e.writeMu.Unlock()

	if _, err := e.templateDir(in.Template); err != nil {
		return zero, err
	}
	name := editor.SanitizeFilename(in.Name)
	ext, ok := imageExt(name)
	if !ok {
		return zero, fmt.Errorf("only .png, .jpg and .jpeg images are allowed")
	}
	if len(in.DataBase64) > maxUploadBase64Chars {
		return zero, fmt.Errorf("image too large (max %d MiB)", maxUploadBytes>>20)
	}
	data, err := decodeBase64(in.DataBase64)
	if err != nil {
		return zero, err
	}
	if len(data) > maxUploadBytes {
		return zero, fmt.Errorf("image too large (max %d MiB)", maxUploadBytes>>20)
	}
	img, format, err := decodeImage(data, maxUploadSide, 0)
	if err != nil {
		return zero, err
	}
	if !formatMatchesExt(format, ext) {
		return zero, fmt.Errorf("file content is %s but name ends in %s", format, ext)
	}
	if format == "jpeg" || pngHasMetadata(data) {
		// Re-encode JPEGs and PNGs with text/EXIF chunks: drops metadata
		// such as GPS positions (templates get shared as ZIP).
		if data, err = encodeImageFor(name, img); err != nil {
			return zero, err
		}
	}
	replaced, err := writeAssetBytes(e, in.Template, name, data)
	if err != nil {
		return zero, err
	}
	b := img.Bounds()
	return uploadImageOut{Name: name, Width: b.Dx(), Height: b.Dy(), Bytes: len(data), Replaced: replaced}, nil
}

func (e *env) importImage(in importImageIn) (*sdk.CallToolResult, importImageOut, error) {
	var zero importImageOut
	e.writeMu.Lock()
	defer e.writeMu.Unlock()

	if _, err := e.templateDir(in.Template); err != nil {
		return nil, zero, err
	}
	path := in.Path
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, zero, fmt.Errorf("cannot resolve ~ in path")
		}
		path = filepath.Join(home, path[2:])
	}
	if !filepath.IsAbs(path) {
		return nil, zero, fmt.Errorf("path must be absolute")
	}
	// Extension first, then one generic text for missing / not regular /
	// unreadable: the tool must not reveal whether arbitrary paths exist.
	if _, ok := imageExt(path); !ok {
		return nil, zero, fmt.Errorf("only .png, .jpg and .jpeg images can be imported")
	}
	if fi, err := os.Lstat(path); err != nil || !fi.Mode().IsRegular() {
		return nil, zero, errCannotReadImage
	}
	name := editor.SanitizeFilename(filepath.Base(path))
	if in.Name != "" {
		name = editor.SanitizeFilename(in.Name)
	}
	targetExt, ok := imageExt(name)
	if !ok {
		return nil, zero, fmt.Errorf("only .png, .jpg and .jpeg images are allowed")
	}
	img, format, raw, err := loadImageBytes(path, maxImportBytes, maxImportSide, maxToolInputPixels)
	switch {
	case errors.Is(err, errNotRegular), errors.Is(err, errReadFailed):
		return nil, zero, errCannotReadImage
	case err != nil:
		return nil, zero, err
	}
	b := img.Bounds()
	out := importImageOut{Name: name, Width: b.Dx(), Height: b.Dy()}
	data := raw
	if max(b.Dx(), b.Dy()) > maxStoredImportSide {
		scaled := toNRGBA(fitWithin(img, maxStoredImportSide))
		out.ScaledFrom = &dims{Width: b.Dx(), Height: b.Dy()}
		out.Width, out.Height = scaled.Bounds().Dx(), scaled.Bounds().Dy()
		img = scaled
		data, err = encodeImageFor(name, scaled)
		if err != nil {
			return nil, zero, err
		}
	} else if format == "jpeg" || pngHasMetadata(raw) || !formatMatchesExt(format, targetExt) {
		// JPEGs and PNGs with text/EXIF chunks are re-encoded: drops metadata.
		data, err = encodeImageFor(name, img)
		if err != nil {
			return nil, zero, err
		}
	}
	if out.Replaced, err = writeAssetBytes(e, in.Template, name, data); err != nil {
		return nil, zero, err
	}
	hint := fmt.Sprintf("Stored as %s (%dx%d). Coordinates for photo tools refer to %dx%d pixels.", name, out.Width, out.Height, out.Width, out.Height)
	res, err := jsonImageResult(out, []image.Image{fitWithin(img, maxResultImageSide)}, hint)
	if err != nil {
		return nil, zero, err
	}
	return res, out, nil
}
