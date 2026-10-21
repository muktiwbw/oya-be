package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"oya-be/internal/platform/auth"
)

func TestAuthMiddleware(t *testing.T) {
	tm := auth.NewTokenManager("test-secret-key-that-is-long-enough", time.Hour)
	h := New(nil, nil, nil, tm, func(context.Context) error { return nil }, 65536, "dev", "unknown")

	// Missing authorization header
	w := request(h, "GET", "/api/v1/boards", "", "")
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), `"code":"unauthorized"`)

	// Invalid header format
	r := httptest.NewRequest("GET", "/api/v1/boards", nil)
	r.Header.Set("Authorization", "Basic abc")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), `"code":"unauthorized"`)

	// Invalid token
	r = httptest.NewRequest("GET", "/api/v1/boards", nil)
	r.Header.Set("Authorization", "Bearer invalid-token-value")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), `"code":"unauthorized"`)

	// Expired token
	expiredTM := auth.NewTokenManager("test-secret-key-that-is-long-enough", -time.Hour)
	expiredToken, err := expiredTM.Generate(1, "u-1", "user")
	require.NoError(t, err)

	r = httptest.NewRequest("GET", "/api/v1/boards", nil)
	r.Header.Set("Authorization", "Bearer "+expiredToken)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), `"code":"unauthorized"`)
}

func TestLoginValidation(t *testing.T) {
	tm := auth.NewTokenManager("test-secret-key-that-is-long-enough", time.Hour)
	h := New(nil, nil, nil, tm, func(context.Context) error { return nil }, 65536, "dev", "unknown")

	w := request(h, "POST", "/api/v1/auth/login", `{"username":""}`, "")
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), `"code":"validation_error"`)

	w = request(h, "POST", "/api/v1/auth/login", `malformed json`, "")
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), `"code":"invalid_json"`)
}
