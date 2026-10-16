package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaults(t *testing.T) {
	c, err := Load(func(string) (string, bool) { return "", false })
	require.NoError(t, err)
	assert.Equal(t, Config{HTTPAddr: ":8080", DBPath: "./data/oya.db", LogLevel: "info", ShutdownTimeout: 10 * time.Second, RequestBodyLimit: 65536, IdempotencyTTL: 24 * time.Hour}, c)
}

func TestOverrides(t *testing.T) {
	env := map[string]string{"OYA_HTTP_ADDR": "127.0.0.1:9000", "OYA_DB_PATH": "/tmp/tasks.db", "OYA_LOG_LEVEL": "debug", "OYA_SHUTDOWN_TIMEOUT": "5s", "OYA_REQUEST_BODY_LIMIT": "32768", "OYA_IDEMPOTENCY_TTL": "1h"}
	c, err := Load(func(k string) (string, bool) { v, ok := env[k]; return v, ok })
	require.NoError(t, err)
	assert.Equal(t, Config{HTTPAddr: env["OYA_HTTP_ADDR"], DBPath: env["OYA_DB_PATH"], LogLevel: "debug", ShutdownTimeout: 5 * time.Second, RequestBodyLimit: 32768, IdempotencyTTL: time.Hour}, c)
}

func TestInvalidConfiguration(t *testing.T) {
	for key, values := range map[string][]string{
		"OYA_HTTP_ADDR":          {"", "localhost", "localhost:no", ":70000"},
		"OYA_DB_PATH":            {"", "  "},
		"OYA_LOG_LEVEL":          {"", "verbose"},
		"OYA_SHUTDOWN_TIMEOUT":   {"abc", "0s", "-1s"},
		"OYA_IDEMPOTENCY_TTL":    {"abc", "0s", "-1h"},
		"OYA_REQUEST_BODY_LIMIT": {"abc", "0", "-1", "65537"},
	} {
		for _, value := range values {
			t.Run(key+"="+value, func(t *testing.T) {
				_, err := Load(func(k string) (string, bool) { return value, k == key })
				require.Error(t, err)
				assert.Contains(t, err.Error(), key)
			})
		}
	}
}
