package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaults(t *testing.T) {
	c, err := Load(func(key string) (string, bool) {
		credentials := map[string]string{"DB_USER": "postgres", "DB_PASSWORD": "postgres"}
		value, ok := credentials[key]
		return value, ok
	})
	require.NoError(t, err)
	assert.Equal(t, Config{
		HTTPAddr:                  ":8080",
		DatabaseHost:              "localhost",
		DatabasePort:              5432,
		DatabaseName:              "oya",
		DatabaseUser:              "postgres",
		DatabasePassword:          "postgres",
		JWTSecret:                 "oya-dev-secret-change-in-production-min-32-chars",
		JWTExpiration:             15 * time.Minute,
		LogLevel:                  "info",
		ShutdownTimeout:           10 * time.Second,
		RequestBodyLimit:          65536,
		IdempotencyTTL:            24 * time.Hour,
		RefreshTTL:                720 * time.Hour,
		GoogleRedirectURL:         "http://localhost:5173/api/v1/auth/google/callback",
		GoogleFrontendRedirectURL: "http://localhost:5173/auth/callback",
		RefreshCookieSecure:       false,
	}, c)
}

func TestDatabaseCredentialsAreRequired(t *testing.T) {
	_, err := Load(func(string) (string, bool) { return "", false })
	require.ErrorContains(t, err, "DB_USER")
}

func TestOverrides(t *testing.T) {
	env := map[string]string{
		"OYA_HTTP_ADDR":                    "127.0.0.1:9000",
		"DB_HOST":                          "db.example.test",
		"DB_PORT":                          "5433",
		"DB_NAME":                          "custom_db",
		"DB_USER":                          "custom",
		"DB_PASSWORD":                      "secret",
		"OYA_JWT_SECRET":                   "super-secret-jwt-key-for-test",
		"OYA_JWT_EXPIRATION":               "12h",
		"OYA_LOG_LEVEL":                    "debug",
		"OYA_SHUTDOWN_TIMEOUT":             "5s",
		"OYA_REQUEST_BODY_LIMIT":           "32768",
		"OYA_IDEMPOTENCY_TTL":              "1h",
		"OYA_REFRESH_TTL":                  "2h",
		"OYA_GOOGLE_CLIENT_ID":             "client",
		"OYA_GOOGLE_CLIENT_SECRET":         "secret",
		"OYA_GOOGLE_REDIRECT_URL":          "http://localhost:8080/api/v1/auth/google/callback",
		"OYA_GOOGLE_FRONTEND_REDIRECT_URL": "http://localhost:5173/auth/callback",
		"OYA_REFRESH_COOKIE_SECURE":        "true",
	}
	c, err := Load(func(k string) (string, bool) { v, ok := env[k]; return v, ok })
	require.NoError(t, err)
	assert.Equal(t, Config{
		HTTPAddr:                  env["OYA_HTTP_ADDR"],
		DatabaseHost:              env["DB_HOST"],
		DatabasePort:              5433,
		DatabaseName:              env["DB_NAME"],
		DatabaseUser:              env["DB_USER"],
		DatabasePassword:          env["DB_PASSWORD"],
		JWTSecret:                 env["OYA_JWT_SECRET"],
		JWTExpiration:             12 * time.Hour,
		LogLevel:                  "debug",
		ShutdownTimeout:           5 * time.Second,
		RequestBodyLimit:          32768,
		IdempotencyTTL:            time.Hour,
		RefreshTTL:                2 * time.Hour,
		GoogleClientID:            "client",
		GoogleClientSecret:        "secret",
		GoogleRedirectURL:         "http://localhost:8080/api/v1/auth/google/callback",
		GoogleFrontendRedirectURL: "http://localhost:5173/auth/callback",
		RefreshCookieSecure:       true,
	}, c)
}

func TestInvalidConfiguration(t *testing.T) {
	for key, values := range map[string][]string{
		"OYA_HTTP_ADDR":                    {"", "localhost", "localhost:no", ":70000"},
		"DB_HOST":                          {"", "  "},
		"DB_PORT":                          {"abc", "0", "65536"},
		"DB_NAME":                          {"", "  "},
		"DB_USER":                          {"", "  "},
		"DB_PASSWORD":                      {"", "  "},
		"OYA_JWT_SECRET":                   {"", "too-short", "  "},
		"OYA_LOG_LEVEL":                    {"", "verbose"},
		"OYA_SHUTDOWN_TIMEOUT":             {"abc", "0s", "-1s"},
		"OYA_JWT_EXPIRATION":               {"abc", "0s", "-1h"},
		"OYA_IDEMPOTENCY_TTL":              {"abc", "0s", "-1h"},
		"OYA_REFRESH_COOKIE_SECURE":        {"not-a-boolean"},
		"OYA_GOOGLE_REDIRECT_URL":          {"relative-path"},
		"OYA_GOOGLE_FRONTEND_REDIRECT_URL": {"relative-path"},
		"OYA_REQUEST_BODY_LIMIT":           {"abc", "0", "-1", "65537"},
	} {
		for _, value := range values {
			t.Run(key+"="+value, func(t *testing.T) {
				_, err := Load(func(k string) (string, bool) {
					if k == key {
						return value, true
					}
					credentials := map[string]string{"DB_USER": "postgres", "DB_PASSWORD": "postgres"}
					credential, ok := credentials[k]
					return credential, ok
				})
				require.Error(t, err)
				assert.Contains(t, err.Error(), key)
			})
		}
	}
}
