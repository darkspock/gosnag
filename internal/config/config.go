package config

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Port        int
	DatabaseURL string
	LogLevel    slog.Level

	// Auth mode: "google" (default) or "local" (no OAuth, email-only login)
	AuthMode string

	// Google OAuth (client ID only — GIS flow verifies tokens server-side)
	GoogleClientID string

	// Alerts - Email
	SMTPHost     string
	SMTPPort     int
	SMTPUser     string
	SMTPPassword string
	SMTPFrom     string

	// Alerts - Slack
	SlackWebhookURL string

	// Session
	SessionSecret string

	// Base URL for the app (used in emails, OAuth redirects)
	BaseURL string

	// Additional allowed browser origins for auth/management API CORS.
	CORSAllowedOrigins []string

	// Default cooldown in minutes when resolving issues
	DefaultCooldownMinutes int

	// Event retention in days (0 = keep forever)
	EventRetentionDays int

	// Rate limit: max requests per minute per IP for ingest endpoints (0 = unlimited)
	IngestRateLimitPerMin int

	// Uploads - S3 (if UPLOAD_S3_BUCKET is set, use S3; otherwise local disk)
	UploadS3Bucket    string
	UploadS3Region    string
	UploadS3Prefix    string
	UploadS3CDNURL    string
}

func Load() (*Config, error) {
	port := getEnvInt("PORT", 8080)
	dbURL := getEnv("DATABASE_URL", "")
	if dbURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}

	authMode := getEnv("AUTH_MODE", "google")
	if authMode != "local" && authMode != "google" {
		authMode = "google"
	}

	return &Config{
		Port:        port,
		DatabaseURL: dbURL,
		LogLevel:    parseLogLevel(getEnv("LOG_LEVEL", "info")),

		AuthMode:       authMode,
		GoogleClientID: getEnv("GOOGLE_CLIENT_ID", ""),

		SMTPHost:     getEnv("SMTP_HOST", ""),
		SMTPPort:     getEnvInt("SMTP_PORT", 587),
		SMTPUser:     getEnv("SMTP_USER", ""),
		SMTPPassword: getEnv("SMTP_PASSWORD", ""),
		SMTPFrom:     getEnv("SMTP_FROM", "gosnag@localhost"),

		SlackWebhookURL: getEnv("SLACK_WEBHOOK_URL", ""),

		SessionSecret: getEnv("SESSION_SECRET", "change-me-in-production"),

		BaseURL: getEnv("BASE_URL", "http://localhost:8080"),

		CORSAllowedOrigins: getEnvCSV("CORS_ALLOWED_ORIGINS"),

		DefaultCooldownMinutes: getEnvInt("DEFAULT_COOLDOWN_MINUTES", 30),

		EventRetentionDays:    getEnvInt("EVENT_RETENTION_DAYS", 90),
		IngestRateLimitPerMin: getEnvInt("INGEST_RATE_LIMIT_PER_MIN", 0),

		UploadS3Bucket: getEnv("UPLOAD_S3_BUCKET", ""),
		UploadS3Region: getEnv("UPLOAD_S3_REGION", getEnv("AWS_REGION", "eu-west-1")),
		UploadS3Prefix: getEnv("UPLOAD_S3_PREFIX", "uploads/"),
		UploadS3CDNURL: getEnv("UPLOAD_S3_CDN_URL", ""),
	}, nil
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	val := os.Getenv(key)
	if val == "" {
		return fallback
	}
	n, err := strconv.Atoi(val)
	if err != nil {
		return fallback
	}
	return n
}

func getEnvCSV(key string) []string {
	val := os.Getenv(key)
	if val == "" {
		return nil
	}

	parts := strings.Split(val, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func parseLogLevel(s string) slog.Level {
	switch s {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
