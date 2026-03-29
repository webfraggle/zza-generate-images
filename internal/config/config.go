package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds all runtime configuration loaded from environment variables.
type Config struct {
	Port             string
	TemplatesDir     string
	CacheDir         string
	CacheMaxAgeHours int
	CacheMaxSizeMB   int64

	// Database
	DBPath string

	// Editor / Auth
	EditTokenTTLHours int    // EDIT_TOKEN_TTL_HOURS
	BaseURL           string // BASE_URL — used in email links (e.g. "https://zza.example.com")

	// SMTP
	SMTPHost string
	SMTPPort string
	SMTPUser string
	SMTPPass string
	SMTPFrom string

	// Admin
	AdminToken    string // ADMIN_TOKEN
	TOTPSecret    string // TOTP_SECRET (Base32)
	SecureCookies bool   // SECURE_COOKIES — set to true in production (HTTPS)
}

// Load reads configuration from environment variables and applies defaults.
//
//	PORT                    default "8080"
//	TEMPLATES_DIR           default "./templates"
//	CACHE_DIR               default "./cache"
//	CACHE_MAX_AGE_HOURS     default 24
//	CACHE_MAX_SIZE_MB       default 500
//	DB_PATH                 default "./zza.db"
//	EDIT_TOKEN_TTL_HOURS    default 24
//	BASE_URL                default "http://localhost:8080"
//	SMTP_HOST               default ""
//	SMTP_PORT               default "587"
//	SMTP_USER               default ""
//	SMTP_PASS               default ""
//	SMTP_FROM               default ""
func Load() *Config {
	return &Config{
		Port:              envStr("PORT", "8080"),
		TemplatesDir:      envStr("TEMPLATES_DIR", "./templates"),
		CacheDir:          envStr("CACHE_DIR", "./cache"),
		CacheMaxAgeHours:  envInt("CACHE_MAX_AGE_HOURS", 24),
		CacheMaxSizeMB:    int64(envInt("CACHE_MAX_SIZE_MB", 500)),
		DBPath:            envStr("DB_PATH", "./zza.db"),
		EditTokenTTLHours: envInt("EDIT_TOKEN_TTL_HOURS", 24),
		BaseURL:           envStr("BASE_URL", "http://localhost:8080"),
		SMTPHost:          envStr("SMTP_HOST", ""),
		SMTPPort:          envStr("SMTP_PORT", "587"),
		SMTPUser:          envStr("SMTP_USER", ""),
		SMTPPass:          envStr("SMTP_PASS", ""),
		SMTPFrom:          envStr("SMTP_FROM", ""),
		AdminToken:        envStr("ADMIN_TOKEN", ""),
		TOTPSecret:        envStr("TOTP_SECRET", ""),
		SecureCookies:     envStr("SECURE_COOKIES", "") == "true",
	}
}

// ValidatePort checks that the port string is a valid TCP port number (1–65535).
func ValidatePort(port string) error {
	n, err := strconv.Atoi(port)
	if err != nil || n < 1 || n > 65535 {
		return fmt.Errorf("config: invalid PORT value %q (must be 1–65535)", port)
	}
	return nil
}

func envStr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return def
}
