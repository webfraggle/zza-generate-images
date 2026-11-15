package mcp

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/webfraggle/zza-generate-images/docs"
	"github.com/webfraggle/zza-generate-images/internal/editor"
	"github.com/webfraggle/zza-generate-images/internal/renderer"
)

func init() { registerToolGroup("read", registerReadTools) }

type noIn struct{}

type templateIn struct {
	Template string `json:"template" jsonschema:"template name, e.g. sbb-105-v1"`
}

type fileIn struct {
	Template string `json:"template" jsonschema:"template name"`
	Name     string `json:"name" jsonschema:"file name inside the template, e.g. default.json"`
}

type templateSummary struct {
	Name        string `json:"name"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	Animated    bool   `json:"animated"`
	HasDefault  bool   `json:"has_default"`
	Error       string `json:"error,omitempty"`
}

type listTemplatesOut struct {
	Templates []templateSummary `json:"templates"`
}

type readTemplateOut struct {
	Template     string            `json:"template"`
	TemplateYAML string            `json:"template_yaml"`
	DefaultJSON  string            `json:"default_json"`
	Files        []editor.FileInfo `json:"files"`
	Versions     map[string]string `json:"versions"`
}

type readFileOut struct {
	Name    string `json:"name"`
	Content string `json:"content"`
	Version string `json:"version"`
}

func textResult(s string) *sdk.CallToolResult {
	return &sdk.CallToolResult{Content: []sdk.Content{&sdk.TextContent{Text: s}}}
}

func registerReadTools(s *sdk.Server, e *env) {
	sdk.AddTool(s, &sdk.Tool{
		Name:        "get_template_spec",
		Description: "Returns the YAML template specification (layers, expressions, filters, conditions, animation). Read it fully before writing template.yaml.",
	}, func(ctx context.Context, _ *sdk.CallToolRequest, _ noIn) (*sdk.CallToolResult, any, error) {
		return textResult(docs.TemplateSpec), nil, nil
	})

	sdk.AddTool(s, &sdk.Tool{
		Name:        "get_design_guide",
		Description: "Returns the design guide for display templates: 240x240 canvas, front/back halves with the platform number on the outside, flat colour look, font sizes, placeholders, workflow and test data. Read it before creating or changing a template.",
	}, func(ctx context.Context, _ *sdk.CallToolRequest, _ noIn) (*sdk.CallToolResult, any, error) {
		return textResult(docs.DesignGuide), nil, nil
	})

	sdk.AddTool(s, &sdk.Tool{
		Name:        "list_templates",
		Description: "Lists all templates with title, size, animation flag and load errors. Use it to find similar templates to learn from.",
	}, func(ctx context.Context, _ *sdk.CallToolRequest, _ noIn) (*sdk.CallToolResult, listTemplatesOut, error) {
		out, err := e.listTemplates()
		return nil, out, err
	})

	sdk.AddTool(s, &sdk.Tool{
		Name:        "read_template",
		Description: "Returns template.yaml, default.json (test data), the file list and the version of each text file. Pass a version as expected_version to write_file to avoid overwriting changes made in the editor.",
	}, func(ctx context.Context, _ *sdk.CallToolRequest, in templateIn) (*sdk.CallToolResult, readTemplateOut, error) {
		out, err := e.readTemplate(in.Template)
		return nil, out, err
	})

	sdk.AddTool(s, &sdk.Tool{
		Name:        "read_file",
		Description: "Returns one .yaml or .json file of a template with its version (e.g. an extra test data set).",
	}, func(ctx context.Context, _ *sdk.CallToolRequest, in fileIn) (*sdk.CallToolResult, readFileOut, error) {
		out, err := e.readFile(in.Template, in.Name)
		return nil, out, err
	})
}

// listTemplates reads the templates folder itself (not via gallery) so that
// broken templates show up with an error.
func (e *env) listTemplates() (listTemplatesOut, error) {
	entries, err := os.ReadDir(e.templatesDir)
	if err != nil {
		return listTemplatesOut{}, fmt.Errorf("cannot read templates folder")
	}
	out := listTemplatesOut{Templates: []templateSummary{}}
	for _, ent := range entries {
		name := ent.Name()
		if !ent.IsDir() || renderer.ValidateTemplateName(name) != nil {
			continue
		}
		dir := filepath.Join(e.templatesDir, name)
		if ok, _ := statRegular(filepath.Join(dir, "template.yaml")); !ok {
			continue
		}
		sum := templateSummary{Name: name}
		_, defErr := os.Stat(filepath.Join(dir, "default.json"))
		sum.HasDefault = defErr == nil
		tmpl, err := renderer.LoadTemplate(e.templatesDir, name)
		if err != nil {
			sum.Error = e.sanitizeErr(err)
		} else {
			sum.Title = tmpl.Meta.Name
			sum.Description = tmpl.Meta.Description
			sum.Width = tmpl.Meta.Canvas.Width
			sum.Height = tmpl.Meta.Canvas.Height
			sum.Animated = tmpl.IsAnimated()
		}
		out.Templates = append(out.Templates, sum)
	}
	sort.Slice(out.Templates, func(i, j int) bool { return out.Templates[i].Name < out.Templates[j].Name })
	return out, nil
}

func (e *env) readTemplate(name string) (readTemplateOut, error) {
	if _, err := e.templateDir(name); err != nil {
		return readTemplateOut{}, err
	}
	yamlData, err := e.readText(name, "template.yaml")
	if err != nil {
		return readTemplateOut{}, fileErr("template.yaml", err)
	}
	out := readTemplateOut{
		Template:     name,
		TemplateYAML: string(yamlData),
		Versions:     map[string]string{"template.yaml": editor.FileVersion(yamlData)},
	}
	switch def, err := e.readText(name, "default.json"); {
	case err == nil:
		out.DefaultJSON = string(def)
		out.Versions["default.json"] = editor.FileVersion(def)
	case errors.Is(err, editor.ErrFileNotFound):
	default:
		return readTemplateOut{}, fileErr("default.json", err)
	}
	files, err := editor.ListFiles(e.templatesDir, name)
	if err != nil {
		return readTemplateOut{}, fmt.Errorf("cannot list files of %q", name)
	}
	if files == nil {
		files = []editor.FileInfo{}
	}
	out.Files = files
	return out, nil
}

func (e *env) readFile(tmpl, name string) (readFileOut, error) {
	if _, err := e.templateDir(tmpl); err != nil {
		return readFileOut{}, err
	}
	data, err := e.readText(tmpl, name)
	if err != nil {
		return readFileOut{}, fileErr(name, err)
	}
	return readFileOut{Name: name, Content: string(data), Version: editor.FileVersion(data)}, nil
}

var (
	errFileTooLarge = errors.New("file too large")
	errNotRegular   = errors.New("not a regular file")
)

// readText reads a .yaml/.json file of a template after checking that it is a
// regular file (no symlink, directory, FIFO) of at most maxTextFileBytes.
func (e *env) readText(tmpl, name string) ([]byte, error) {
	if p, err := editor.SafeFilePath(e.templatesDir, tmpl, name); err == nil {
		fi, err := os.Lstat(p)
		switch {
		case err != nil:
			// not found etc. is reported by ReadTextFile
		case !fi.Mode().IsRegular():
			return nil, errNotRegular
		case fi.Size() > maxTextFileBytes:
			return nil, errFileTooLarge
		}
	}
	return editor.ReadTextFile(e.templatesDir, tmpl, name)
}

// sanitizeErr shortens a renderer error for tool output: the absolute
// templates path is removed and the text is cut to 300 characters.
func (e *env) sanitizeErr(err error) string {
	s := strings.ReplaceAll(err.Error(), e.templatesDir+string(filepath.Separator), "")
	s = strings.ReplaceAll(s, e.templatesDir, "")
	if r := []rune(s); len(r) > 300 {
		s = string(r[:300])
	}
	return s
}

// fileErr maps editor errors to short, content-free tool errors.
func fileErr(name string, err error) error {
	switch {
	case errors.Is(err, editor.ErrFileNotFound):
		return fmt.Errorf("file %q not found", name)
	case errors.Is(err, errFileTooLarge):
		return fmt.Errorf("file too large (max %d KiB)", maxTextFileBytes>>10)
	case errors.Is(err, errNotRegular):
		return fmt.Errorf("not a regular file")
	case errors.Is(err, editor.ErrForbidden):
		return fmt.Errorf("only .yaml and .json files can be read as text")
	case errors.Is(err, editor.ErrInvalidName):
		return fmt.Errorf("invalid file name %q (allowed: letters, digits, . _ -)", name)
	default:
		return fmt.Errorf("cannot read %q", name)
	}
}

// statRegular reports whether path is a regular file (not a symlink or dir).
func statRegular(path string) (bool, error) {
	fi, err := os.Lstat(path)
	if err != nil {
		return false, err
	}
	return fi.Mode().IsRegular(), nil
}
