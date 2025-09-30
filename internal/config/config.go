package config

import (
	"os"
	"strconv"
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
	JWTSecret          string
	CSRFSecret         string
	SessionDuration    time.Duration
	RateLimitRequests  int
	RateLimitWindow    time.Duration
	LoginRateLimit     int
	LoginRateWindow    time.Duration
	CSPEnabled         bool
	HSTSEnabled        bool
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
	Enabled        bool
	Schedule       string
	RetentionDays  int
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
			JWTSecret:          getEnv("JWT_SECRET", ""),
			CSRFSecret:         getEnv("CSRF_SECRET", ""),
			SessionDuration:    sessionDuration,
			RateLimitRequests:  rateLimitReqs,
			RateLimitWindow:    rateLimitWindow,
			LoginRateLimit:     loginRateLimit,
			LoginRateWindow:    loginRateWindow,
			CSPEnabled:         cspEnabled,
			HSTSEnabled:        hstsEnabled,
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
			Enabled:        backupEnabled,
			Schedule:       getEnv("BACKUP_SCHEDULE", "0 2 * * *"),
			RetentionDays:  backupRetention,
		},
	}

	// Validate required fields
	if cfg.Security.JWTSecret == "" {
		return nil, ErrMissingJWTSecret
	}

	if cfg.Security.CSRFSecret == "" {
		return nil, ErrMissingCSRFSecret
	}

	return cfg, nil
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
)

type ConfigError struct {
	Message string
}

func (e *ConfigError) Error() string {
	return e.Message
}