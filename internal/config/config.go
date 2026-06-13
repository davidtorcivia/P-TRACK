package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	Security SecurityConfig
	SMTP     SMTPConfig
	Backup   BackupConfig
}

type ServerConfig struct {
	Port        string
	Environment string
}

type DatabaseConfig struct {
	Path string
}

type SecurityConfig struct {
	JWTSecret         string
	CSRFSecret        string
	SessionDuration   time.Duration
	RateLimitRequests int
	RateLimitWindow   time.Duration
	LoginRateLimit    int
	LoginRateWindow   time.Duration
	CSPEnabled        bool
	HSTSEnabled       bool
	// AllowedOrigins is the explicit list of origins permitted to make
	// credentialed cross-origin requests. Empty means same-origin only.
	AllowedOrigins []string
	// TrustedProxies is a list of CIDRs whose X-Forwarded-For/X-Real-IP
	// headers are trusted for client IP resolution. Empty means the
	// direct peer address is always used (headers are ignored).
	TrustedProxies []string
}

type SMTPConfig struct {
	Enabled  bool
	Host     string
	Port     int
	Username string
	Password string
	From     string
}

type BackupConfig struct {
	Enabled       bool
	Schedule      string
	RetentionDays int
}

// Load reads configuration from environment variables
func Load() (*Config, error) {
	sessionDuration, err := time.ParseDuration(getEnv("SESSION_DURATION", "336h"))
	if err != nil {
		sessionDuration = 336 * time.Hour
	}

	rateLimitWindow, err := time.ParseDuration(getEnv("RATE_LIMIT_WINDOW", "1m"))
	if err != nil {
		rateLimitWindow = 1 * time.Minute
	}

	loginRateWindow, err := time.ParseDuration(getEnv("LOGIN_RATE_WINDOW", "15m"))
	if err != nil {
		loginRateWindow = 15 * time.Minute
	}

	smtpPort, _ := strconv.Atoi(getEnv("SMTP_PORT", "587"))
	smtpEnabled, _ := strconv.ParseBool(getEnv("SMTP_ENABLED", "false"))
	backupEnabled, _ := strconv.ParseBool(getEnv("BACKUP_ENABLED", "true"))
	backupRetention, _ := strconv.Atoi(getEnv("BACKUP_RETENTION_DAYS", "30"))
	cspEnabled, _ := strconv.ParseBool(getEnv("CSP_ENABLED", "true"))
	hstsEnabled, _ := strconv.ParseBool(getEnv("HSTS_ENABLED", "true"))
	rateLimitReqs, _ := strconv.Atoi(getEnv("RATE_LIMIT_REQUESTS", "100"))
	loginRateLimit, _ := strconv.Atoi(getEnv("LOGIN_RATE_LIMIT", "5"))

	cfg := &Config{
		Server: ServerConfig{
			Port:        getEnv("PORT", "8080"),
			Environment: getEnv("ENVIRONMENT", "development"),
		},
		Database: DatabaseConfig{
			Path: getEnv("DATABASE_PATH", "./data/tracker.db"),
		},
		Security: SecurityConfig{
			JWTSecret:         getEnv("JWT_SECRET", ""),
			CSRFSecret:        getEnv("CSRF_SECRET", ""),
			SessionDuration:   sessionDuration,
			RateLimitRequests: rateLimitReqs,
			RateLimitWindow:   rateLimitWindow,
			LoginRateLimit:    loginRateLimit,
			LoginRateWindow:   loginRateWindow,
			CSPEnabled:        cspEnabled,
			HSTSEnabled:       hstsEnabled,
			AllowedOrigins:    splitAndTrim(getEnv("ALLOWED_ORIGINS", "")),
			TrustedProxies:    splitAndTrim(getEnv("TRUSTED_PROXIES", "")),
		},
		SMTP: SMTPConfig{
			Enabled:  smtpEnabled,
			Host:     getEnv("SMTP_HOST", ""),
			Port:     smtpPort,
			Username: getEnv("SMTP_USERNAME", ""),
			Password: getEnv("SMTP_PASSWORD", ""),
			From:     getEnv("SMTP_FROM", ""),
		},
		Backup: BackupConfig{
			Enabled:       backupEnabled,
			Schedule:      getEnv("BACKUP_SCHEDULE", "0 2 * * *"),
			RetentionDays: backupRetention,
		},
	}

	// Validate required fields
	if cfg.Security.JWTSecret == "" {
		return nil, ErrMissingJWTSecret
	}

	if cfg.Security.CSRFSecret == "" {
		return nil, ErrMissingCSRFSecret
	}

	// Reject weak secrets: HS256 / CSRF HMAC are brute-forceable offline if the
	// key has low entropy. Require at least 32 bytes.
	if len(cfg.Security.JWTSecret) < minSecretLength {
		return nil, ErrWeakJWTSecret
	}
	if len(cfg.Security.CSRFSecret) < minSecretLength {
		return nil, ErrWeakCSRFSecret
	}

	return cfg, nil
}

// minSecretLength is the minimum acceptable length (in bytes) for signing secrets.
const minSecretLength = 32

// splitAndTrim splits a comma-separated env value into a trimmed, non-empty slice.
func splitAndTrim(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

var (
	ErrMissingJWTSecret  = &ConfigError{"JWT_SECRET environment variable is required"}
	ErrMissingCSRFSecret = &ConfigError{"CSRF_SECRET environment variable is required"}
	ErrWeakJWTSecret     = &ConfigError{"JWT_SECRET must be at least 32 bytes; generate one with: openssl rand -base64 32"}
	ErrWeakCSRFSecret    = &ConfigError{"CSRF_SECRET must be at least 32 bytes; generate one with: openssl rand -base64 32"}
)

type ConfigError struct {
	Message string
}

func (e *ConfigError) Error() string {
	return e.Message
}
