// Package mcp implements the desktop build's MCP server (stdio). It lets AI
// clients read, create and edit display templates and look at rendered
// previews. DESKTOP-ONLY: the server build must not import this package.
package mcp

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/webfraggle/zza-generate-images/internal/renderer"
	"github.com/webfraggle/zza-generate-images/internal/version"
)

// Config configures the MCP server.
type Config struct {
	// TemplatesDir is the folder holding the template directories.
	TemplatesDir string
	// FontsDir is the shared fonts folder. Empty means "fonts" next to TemplatesDir.
	FontsDir string
}

// env is the state shared by all tools.
type env struct {
	templatesDir string // absolute
	fontsDir     string // absolute
	// writeMu serialises all writing tool calls of this process.
	writeMu sync.Mutex
}

type toolGroup struct {
	name     string
	register func(s *sdk.Server, e *env)
}

var toolGroups []toolGroup

// registerToolGroup registers a group of tools. Each tools_*.go file calls it
// from its own init() so parallel work never touches a shared list.
func registerToolGroup(name string, register func(*sdk.Server, *env)) {
	toolGroups = append(toolGroups, toolGroup{name: name, register: register})
}

func newEnv(cfg Config) (*env, error) {
	if cfg.TemplatesDir == "" {
		return nil, fmt.Errorf("mcp: templates dir must not be empty")
	}
	tdir, err := filepath.Abs(cfg.TemplatesDir)
	if err != nil {
		return nil, fmt.Errorf("mcp: resolving templates dir: %w", err)
	}
	fdir := cfg.FontsDir
	if fdir == "" {
		fdir = filepath.Join(filepath.Dir(tdir), "fonts")
	}
	fdir, err = filepath.Abs(fdir)
	if err != nil {
		return nil, fmt.Errorf("mcp: resolving fonts dir: %w", err)
	}
	return &env{templatesDir: tdir, fontsDir: fdir}, nil
}

// NewServer builds the MCP server with all registered tool groups.
func NewServer(cfg Config) (*sdk.Server, error) {
	e, err := newEnv(cfg)
	if err != nil {
		return nil, err
	}
	s := sdk.NewServer(
		&sdk.Implementation{Name: "zza", Title: "Zugzielanzeiger", Version: version.Version},
		&sdk.ServerOptions{
			Instructions: serverInstructions,
			Capabilities: &sdk.ServerCapabilities{},
		},
	)
	groups := append([]toolGroup(nil), toolGroups...)
	sort.Slice(groups, func(i, j int) bool { return groups[i].name < groups[j].name })
	for _, g := range groups {
		g.register(s, e)
	}
	return s, nil
}

// Run serves MCP on stdin/stdout until ctx ends or stdin closes. Logs go to
// stderr; stdout belongs to the protocol.
func Run(ctx context.Context, cfg Config) error {
	e, err := newEnv(cfg)
	if err != nil {
		return err
	}
	s, err := NewServer(cfg)
	if err != nil {
		return err
	}
	log.Printf("zza mcp: templates=%s fonts=%s", e.templatesDir, e.fontsDir)
	return s.Run(ctx, &sdk.StdioTransport{})
}

// templateDir validates the template name and returns the absolute folder of
// an existing template (one with a template.yaml).
func (e *env) templateDir(name string) (string, error) {
	if err := renderer.ValidateTemplateName(name); err != nil {
		return "", fmt.Errorf("invalid template name: %s", strings.TrimPrefix(err.Error(), "renderer: "))
	}
	dir := filepath.Join(e.templatesDir, name)
	if fi, err := statRegular(filepath.Join(dir, "template.yaml")); err != nil || !fi {
		return "", fmt.Errorf("template %q not found", name)
	}
	return dir, nil
}
