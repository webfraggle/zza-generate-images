package main

import (
	"context"
	"encoding/json"
	"fmt"
	"image/png"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"crypto/rand"
	"encoding/hex"

	"github.com/spf13/cobra"
	"github.com/webfraggle/zza-generate-images/internal/config"
	"github.com/webfraggle/zza-generate-images/internal/db"
	"github.com/webfraggle/zza-generate-images/internal/editor"
	"github.com/webfraggle/zza-generate-images/internal/renderer"
	"github.com/webfraggle/zza-generate-images/internal/server"
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
		Use:   "zza",
		Short: "Zugzielanzeiger image generator",
	}
	root.AddCommand(renderCmd())
	root.AddCommand(serveCmd())
	return root
}

func renderCmd() *cobra.Command {
	var (
		templateName string
		inputFile    string
		outputFile   string
		templatesDir string
	)

	cmd := &cobra.Command{
		Use:   "render",
		Short: "Render a template with JSON input data to a PNG image",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Validate template name.
			if err := renderer.ValidateTemplateName(templateName); err != nil {
				return fmt.Errorf("render: %w", err)
			}

			// Read and parse JSON input.
			jsonBytes, err := os.ReadFile(inputFile)
			if err != nil {
				return fmt.Errorf("render: reading input file: %w", err)
			}
			var data map[string]interface{}
			if err := json.Unmarshal(jsonBytes, &data); err != nil {
				return fmt.Errorf("render: parsing JSON: %w", err)
			}

			// Load template.
			tmpl, err := renderer.LoadTemplate(templatesDir, templateName)
			if err != nil {
				return fmt.Errorf("render: loading template: %w", err)
			}

			// Render.
			r := renderer.New(templatesDir)
			img, err := r.Render(tmpl, data)
			if err != nil {
				return fmt.Errorf("render: rendering: %w", err)
			}

			// Write PNG output.
			outF, err := os.Create(outputFile)
			if err != nil {
				return fmt.Errorf("render: creating output file: %w", err)
			}

			if err := png.Encode(outF, img); err != nil {
				outF.Close()
				os.Remove(outputFile) // clean up partial file on encode error
				return fmt.Errorf("render: encoding PNG: %w", err)
			}

			if err := outF.Close(); err != nil {
				os.Remove(outputFile)
				return fmt.Errorf("render: closing output file: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Rendered %s to %s\n", templateName, outputFile)
			return nil
		},
	}

	cmd.Flags().StringVarP(&templateName, "template", "t", "", "Template name (required)")
	cmd.Flags().StringVarP(&inputFile, "input", "i", "", "Input JSON file (required)")
	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "Output PNG file (required)")
	cmd.Flags().StringVar(&templatesDir, "templates-dir", "./templates", "Templates directory")

	_ = cmd.MarkFlagRequired("template")
	_ = cmd.MarkFlagRequired("input")
	_ = cmd.MarkFlagRequired("output")

	return cmd
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
  CACHE_MAX_SIZE_MB       (default: 500)
  DB_PATH                 (default: ./zza.db)
  HMAC_SECRET             (default: auto-generated — set for persistent email hashes)
  EDIT_TOKEN_TTL_HOURS    (default: 24)
  BASE_URL                (default: http://localhost:8080)
  SMTP_HOST, SMTP_PORT, SMTP_USER, SMTP_PASS, SMTP_FROM`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.Load()
			if err := config.ValidatePort(cfg.Port); err != nil {
				return err
			}

			// Resolve HMAC secret — warn if not set persistently.
			hmacSecret := cfg.HMACSecret
			if hmacSecret == "" {
				b := make([]byte, 32)
				_, _ = rand.Read(b)
				hmacSecret = hex.EncodeToString(b)
				log.Println("WARNING: HMAC_SECRET not set — using ephemeral key. " +
					"Set HMAC_SECRET for persistent template ownership across restarts.")
			}

			// Open database.
			database, err := db.Open(cfg.DBPath)
			if err != nil {
				return fmt.Errorf("serve: opening database: %w", err)
			}
			defer database.Close()
			log.Printf("database: %s", cfg.DBPath)

			srv, err := server.New(cfg, web.FS)
			if err != nil {
				return fmt.Errorf("serve: %w", err)
			}

			// Register editor routes.
			srv.RegisterEditorRoutes(database, server.EditorConfig{
				HMACSecret: hmacSecret,
				TokenTTL:   time.Duration(cfg.EditTokenTTLHours) * time.Hour,
				Mail: editor.MailConfig{
					Host:    cfg.SMTPHost,
					Port:    cfg.SMTPPort,
					User:    cfg.SMTPUser,
					Pass:    cfg.SMTPPass,
					From:    cfg.SMTPFrom,
					BaseURL: cfg.BaseURL,
				},
			})

			// Start cache cleanup every 15 minutes.
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

			// Graceful shutdown on SIGINT / SIGTERM.
			quit := make(chan os.Signal, 1)
			signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

			go func() {
				log.Printf("zza server listening on :%s (templates: %s, cache: %s)",
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
