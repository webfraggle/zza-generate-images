package main

import (
	"fmt"
	"log"
	"os"

	"github.com/spf13/cobra"
	"github.com/webfraggle/zza-generate-images/internal/cli"
	"github.com/webfraggle/zza-generate-images/internal/config"
	"github.com/webfraggle/zza-generate-images/internal/desktop"
	"github.com/webfraggle/zza-generate-images/internal/editor"
	"github.com/webfraggle/zza-generate-images/internal/server"
	"github.com/webfraggle/zza-generate-images/internal/version"
	"github.com/webfraggle/zza-generate-images/web"
)

var templatesDirFlag string

func main() {
	if err := rootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func rootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "zza",
		Short: "Zugzielanzeiger desktop — editor + preview + render",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Default: open GUI.
			return runGUI(templatesDirFlag)
		},
	}
	root.PersistentFlags().StringVar(&templatesDirFlag, "templates-dir", "",
		"path to templates directory (defaults to sibling of executable)")
	root.AddCommand(cli.RenderCmd())
	root.AddCommand(serveCmd())
	root.AddCommand(versionCmd())
	return root
}

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version and exit",
		Run: func(cmd *cobra.Command, _ []string) {
			fmt.Fprintln(cmd.OutOrStdout(), version.Version)
		},
	}
}

func serveCmd() *cobra.Command {
	var port string
	c := &cobra.Command{
		Use:   "serve",
		Short: "Run the editor+preview server without opening a window",
		RunE: func(cmd *cobra.Command, args []string) error {
			handler, err := buildHandler(templatesDirFlag)
			if err != nil {
				return err
			}
			return desktop.RunServerOnly("127.0.0.1:"+port, handler)
		},
	}
	c.Flags().StringVar(&port, "port", "8080", "TCP port to listen on")
	return c
}

func runGUI(override string) error {
	handler, err := buildHandler(override)
	if err != nil {
		return err
	}
	return desktop.RunGUI("Zugzielanzeiger", handler)
}

// buildHandler wires the HTTP server with editor handlers attached.
func buildHandler(templatesOverride string) (*server.Server, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("locating executable: %w", err)
	}
	tdir, err := desktop.ResolveTemplatesDir(templatesOverride, exe)
	if err != nil {
		return nil, err
	}
	if err := desktop.EnsureTemplatesDir(tdir); err != nil {
		return nil, err
	}
	log.Printf("templates: %s", tdir)

	cfg := &config.Config{
		Port:             "0",
		TemplatesDir:     tdir,
		CacheDir:         cacheDirFor(),
		CacheMaxAgeHours: 24,
		CacheMaxSizeMB:   500,
		BaseURL:          "http://127.0.0.1",
	}
	srv, err := server.New(cfg, web.FS)
	if err != nil {
		return nil, err
	}
	srv.SetEditorEnabled(true)
	srv.RegisterEditor(editor.NewFSHandlers(tdir, srv.InvalidateTemplateCache))
	return srv, nil
}

// cacheDirFor returns a user-specific cache directory for desktop runs.
func cacheDirFor() string {
	if u, err := os.UserCacheDir(); err == nil {
		return u + string(os.PathSeparator) + "zza"
	}
	return "./cache"
}
