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
	"github.com/webfraggle/zza-generate-images/internal/admin"
	"github.com/webfraggle/zza-generate-images/internal/cli"
	"github.com/webfraggle/zza-generate-images/internal/config"
	"github.com/webfraggle/zza-generate-images/internal/db"
	"github.com/webfraggle/zza-generate-images/internal/editor"
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
	root.AddCommand(cli.RenderCmd())
	root.AddCommand(serveCmd())
	root.AddCommand(totpSetupCmd())
	return root
}

func totpSetupCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "totp-setup",
		Short: "Generate a TOTP secret for admin authentication",
		Long: `Generates a new TOTP secret and prints it along with an otpauth:// URL.

Steps:
  1. Run this command and scan the otpauth:// URL with your authenticator app.
  2. Set the printed TOTP_SECRET value in your environment or .env file.
  3. Also set a long random ADMIN_TOKEN.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			secret, err := admin.GenerateSecret()
			if err != nil {
				return err
			}
			url := admin.OTPAuthURL(secret, "ZZA", "admin")
			fmt.Fprintf(cmd.OutOrStdout(), "TOTP_SECRET=%s\n\n", secret)
			fmt.Fprintf(cmd.OutOrStdout(), "otpauth URL (scan with authenticator):\n%s\n", url)
			return nil
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

			// Register admin routes.
			srv.RegisterAdminRoutes(database, server.AdminConfig{
				AdminToken:    cfg.AdminToken,
				TOTPSecret:    cfg.TOTPSecret,
				SecureCookies: cfg.SecureCookies,
			})

			// Register editor routes.
			srv.RegisterEditorRoutes(database, server.EditorConfig{
				TokenTTL: time.Duration(cfg.EditTokenTTLHours) * time.Hour,
				Mail: editor.MailConfig{
					Host:    cfg.SMTPHost,
					Port:    cfg.SMTPPort,
					User:    cfg.SMTPUser,
					Pass:    cfg.SMTPPass,
					From:    cfg.SMTPFrom,
					BaseURL: cfg.BaseURL,
				},
			})

			// Register create-new routes (same config as editor).
			srv.RegisterCreateRoutes(database, server.EditorConfig{
				TokenTTL: time.Duration(cfg.EditTokenTTLHours) * time.Hour,
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
