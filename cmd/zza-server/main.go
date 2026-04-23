package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/webfraggle/zza-generate-images/internal/config"
	"github.com/webfraggle/zza-generate-images/internal/server"
	"github.com/webfraggle/zza-generate-images/internal/version"
	"github.com/webfraggle/zza-generate-images/web"
)

func main() {
	if err := rootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func rootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "zza-server",
		Short: "Zugzielanzeiger image generator (server)",
	}
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
	return &cobra.Command{
		Use:   "serve",
		Short: "Start the HTTP render server",
		Long: `Start the HTTP server. Configuration via environment variables:
  PORT                    (default: 8080)
  TEMPLATES_DIR           (default: ./templates)
  CACHE_DIR               (default: ./cache)
  CACHE_MAX_AGE_HOURS     (default: 24)
  CACHE_MAX_SIZE_MB       (default: 500)`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.Load()
			if err := config.ValidatePort(cfg.Port); err != nil {
				return err
			}

			srv, err := server.New(cfg, web.FS)
			if err != nil {
				return fmt.Errorf("serve: %w", err)
			}
			// EditorEnabled stays false — server build has no editor.

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			srv.StartCleanup(ctx, 15*time.Minute)

			httpSrv := &http.Server{
				Addr:         ":" + cfg.Port,
				Handler:      srv,
				ReadTimeout:  30 * time.Second,
				WriteTimeout: 60 * time.Second,
				IdleTimeout:  120 * time.Second,
			}

			quit := make(chan os.Signal, 1)
			signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

			go func() {
				log.Printf("zza-server listening on :%s (templates: %s, cache: %s)",
					cfg.Port, cfg.TemplatesDir, cfg.CacheDir)
				if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
					log.Fatalf("server error: %v", err)
				}
			}()

			<-quit
			log.Println("shutting down...")
			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer shutdownCancel()
			return httpSrv.Shutdown(shutdownCtx)
		},
	}
}
