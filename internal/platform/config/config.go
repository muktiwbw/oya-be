// Package config loads validated runtime settings from environment variables.
package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	HTTPAddr                  string
	DatabaseHost              string
	DatabasePort              int
	DatabaseName              string
	DatabaseUser              string
	DatabasePassword          string
	JWTSecret                 string
	JWTExpiration             time.Duration
	LogLevel                  string
	ShutdownTimeout           time.Duration
	RequestBodyLimit          int64
	IdempotencyTTL            time.Duration
	RefreshTTL                time.Duration
	GoogleClientID            string
	GoogleClientSecret        string
	GoogleRedirectURL         string
	GoogleFrontendRedirectURL string
	RefreshCookieSecure       bool
}

// LoadFile reads optional dotenv settings without changing the process environment.
// Values supplied by lookup take precedence, including explicitly empty values.
func LoadFile(path string, lookup func(string) (string, bool)) (Config, error) {
	values, err := godotenv.Read(path)
	if err != nil && !os.IsNotExist(err) {
		return Config{}, fmt.Errorf("load environment file %s: %w", path, err)
	}
	return Load(func(key string) (string, bool) {
		if value, ok := lookup(key); ok {
			return value, true
		}
		value, ok := values[key]
		return value, ok
	})
}

// Load accepts an injected lookup function so tests never mutate process state.
func Load(lookup func(string) (string, bool)) (Config, error) {
	get := func(key, fallback string) string {
		if v, ok := lookup(key); ok {
			return v
		}
		return fallback
	}
	c := Config{
		HTTPAddr:                  get("OYA_HTTP_ADDR", ":8080"),
		DatabaseHost:              get("DB_HOST", "localhost"),
		DatabaseName:              get("DB_NAME", "oya"),
		DatabaseUser:              get("DB_USER", ""),
		DatabasePassword:          get("DB_PASSWORD", ""),
		JWTSecret:                 get("OYA_JWT_SECRET", "oya-dev-secret-change-in-production-min-32-chars"),
		LogLevel:                  get("OYA_LOG_LEVEL", "info"),
		GoogleClientID:            get("OYA_GOOGLE_CLIENT_ID", ""),
		GoogleClientSecret:        get("OYA_GOOGLE_CLIENT_SECRET", ""),
		GoogleRedirectURL:         get("OYA_GOOGLE_REDIRECT_URL", "http://localhost:5173/api/v1/auth/google/callback"),
		GoogleFrontendRedirectURL: get("OYA_GOOGLE_FRONTEND_REDIRECT_URL", "http://localhost:5173/auth/callback"),
	}
	_, port, err := net.SplitHostPort(c.HTTPAddr)
	if err != nil {
		return c, fmt.Errorf("OYA_HTTP_ADDR must include a host and port")
	}
	n, err := strconv.Atoi(port)
	if err != nil || n < 1 || n > 65535 {
		return c, fmt.Errorf("OYA_HTTP_ADDR port must be between 1 and 65535")
	}
	c.DatabasePort, err = strconv.Atoi(get("DB_PORT", "5432"))
	if err != nil || c.DatabasePort < 1 || c.DatabasePort > 65535 {
		return c, fmt.Errorf("DB_PORT must be between 1 and 65535")
	}
	if strings.TrimSpace(c.DatabaseHost) == "" {
		return c, fmt.Errorf("DB_HOST must not be empty")
	}
	if strings.TrimSpace(c.DatabaseName) == "" {
		return c, fmt.Errorf("DB_NAME must not be empty")
	}
	if strings.TrimSpace(c.DatabaseUser) == "" {
		return c, fmt.Errorf("DB_USER must not be empty")
	}
	if strings.TrimSpace(c.DatabasePassword) == "" {
		return c, fmt.Errorf("DB_PASSWORD must not be empty")
	}
	if len(strings.TrimSpace(c.JWTSecret)) < 16 {
		return c, fmt.Errorf("OYA_JWT_SECRET must be at least 16 characters long")
	}
	switch c.LogLevel {
	case "debug", "info", "warn", "error":
	default:
		return c, fmt.Errorf("OYA_LOG_LEVEL must be debug, info, warn, or error")
	}
	for _, setting := range []struct {
		key, fallback string
		dest          *time.Duration
	}{
		{"OYA_JWT_EXPIRATION", "15m", &c.JWTExpiration},
		{"OYA_SHUTDOWN_TIMEOUT", "10s", &c.ShutdownTimeout},
		{"OYA_IDEMPOTENCY_TTL", "24h", &c.IdempotencyTTL},
		{"OYA_REFRESH_TTL", "720h", &c.RefreshTTL},
	} {
		d, err := time.ParseDuration(get(setting.key, setting.fallback))
		if err != nil || d <= 0 {
			return c, fmt.Errorf("%s must be a positive duration", setting.key)
		}
		*setting.dest = d
	}
	if (c.GoogleClientID == "") != (c.GoogleClientSecret == "") {
		return c, fmt.Errorf("OYA_GOOGLE_CLIENT_ID and OYA_GOOGLE_CLIENT_SECRET must be set together")
	}
	for _, setting := range []struct {
		key, value string
	}{
		{"OYA_GOOGLE_REDIRECT_URL", c.GoogleRedirectURL},
		{"OYA_GOOGLE_FRONTEND_REDIRECT_URL", c.GoogleFrontendRedirectURL},
	} {
		parsed, parseErr := url.ParseRequestURI(setting.value)
		if parseErr != nil || parsed.Scheme == "" || parsed.Host == "" {
			return c, fmt.Errorf("%s must be an absolute URL", setting.key)
		}
	}
	c.RefreshCookieSecure, err = strconv.ParseBool(get("OYA_REFRESH_COOKIE_SECURE", "false"))
	if err != nil {
		return c, fmt.Errorf("OYA_REFRESH_COOKIE_SECURE must be true or false")
	}
	c.RequestBodyLimit, err = strconv.ParseInt(get("OYA_REQUEST_BODY_LIMIT", "65536"), 10, 64)
	if err != nil || c.RequestBodyLimit <= 0 || c.RequestBodyLimit > 65536 {
		return c, fmt.Errorf("OYA_REQUEST_BODY_LIMIT must be between 1 and 65536")
	}
	return c, nil
}
