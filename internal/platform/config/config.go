// Package config loads validated runtime settings from environment variables.
package config

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	HTTPAddr         string
	DatabaseURL      string
	JWTSecret        string
	JWTExpiration    time.Duration
	LogLevel         string
	ShutdownTimeout  time.Duration
	RequestBodyLimit int64
	IdempotencyTTL   time.Duration
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
		HTTPAddr:    get("OYA_HTTP_ADDR", ":8080"),
		DatabaseURL: get("OYA_DATABASE_URL", "postgres://postgres:postgres@localhost:5432/oya?sslmode=disable"),
		JWTSecret:   get("OYA_JWT_SECRET", "oya-dev-secret-change-in-production-min-32-chars"),
		LogLevel:    get("OYA_LOG_LEVEL", "info"),
	}
	_, port, err := net.SplitHostPort(c.HTTPAddr)
	if err != nil {
		return c, fmt.Errorf("OYA_HTTP_ADDR must include a host and port")
	}
	n, err := strconv.Atoi(port)
	if err != nil || n < 1 || n > 65535 {
		return c, fmt.Errorf("OYA_HTTP_ADDR port must be between 1 and 65535")
	}
	if strings.TrimSpace(c.DatabaseURL) == "" {
		return c, fmt.Errorf("OYA_DATABASE_URL must not be empty")
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
		{"OYA_JWT_EXPIRATION", "24h", &c.JWTExpiration},
		{"OYA_SHUTDOWN_TIMEOUT", "10s", &c.ShutdownTimeout},
		{"OYA_IDEMPOTENCY_TTL", "24h", &c.IdempotencyTTL},
	} {
		d, err := time.ParseDuration(get(setting.key, setting.fallback))
		if err != nil || d <= 0 {
			return c, fmt.Errorf("%s must be a positive duration", setting.key)
		}
		*setting.dest = d
	}
	c.RequestBodyLimit, err = strconv.ParseInt(get("OYA_REQUEST_BODY_LIMIT", "65536"), 10, 64)
	if err != nil || c.RequestBodyLimit <= 0 || c.RequestBodyLimit > 65536 {
		return c, fmt.Errorf("OYA_REQUEST_BODY_LIMIT must be between 1 and 65536")
	}
	return c, nil
}
