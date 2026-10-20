package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func request(h http.Handler, method, path, body, key string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	if key != "" {
		r.Header.Set("Idempotency-Key", key)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func TestRouteErrorsAndAllowedMethods(t *testing.T) {
	h := New(nil, nil, nil, nil, func(context.Context) error { return nil }, 65536, "dev", "unknown")
	id := "018f4b7e-3b75-7e1f-b5d5-2b2d7b0a7f12"
	for _, tc := range []struct {
		method, path string
		status       int
		allow        []string
	}{
		{http.MethodPost, "/healthz", 405, []string{"GET"}},
		{http.MethodPost, "/readyz", 405, []string{"GET"}},
		{http.MethodPost, "/version", 405, []string{"GET"}},
		{http.MethodDelete, "/api/v1/tasks", 405, []string{"POST"}},
		{http.MethodPost, "/api/v1/tasks/" + id, 405, []string{"DELETE", "GET", "PATCH"}},
		{http.MethodGet, "/api/v1/tasks/" + id + "/status", 405, []string{"PATCH"}},
		{http.MethodDelete, "/api/v1/boards", 405, []string{"GET", "POST"}},
		{http.MethodGet, "/api/v1/tasks/", 404, nil},
		{http.MethodGet, "/api/v1/tasks/" + id + "/extra", 404, nil},
		{http.MethodPost, "/missing", 404, nil},
	} {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			w := request(h, tc.method, tc.path, "", "")
			require.Equal(t, tc.status, w.Code, "%s", w.Body)
			assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
			assert.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"))
			_, err := uuid.Parse(w.Header().Get("X-Request-ID"))
			require.NoError(t, err)
			var allowed []string
			for _, header := range w.Header().Values("Allow") {
				for _, method := range strings.Split(header, ",") {
					allowed = append(allowed, strings.TrimSpace(method))
				}
			}
			assert.ElementsMatch(t, tc.allow, allowed)
			code := "not_found"
			if tc.status == 405 {
				code = "method_not_allowed"
			}
			assert.Contains(t, w.Body.String(), `"code":"`+code+`"`)
		})
	}
}

func TestRouterRecoveryUsesJSONError(t *testing.T) {
	h := New(nil, nil, nil, nil, func(context.Context) error { panic("private failure details") }, 65536, "dev", "unknown")
	w := request(h, http.MethodGet, "/readyz", "", "")
	require.Equal(t, http.StatusInternalServerError, w.Code)
	assert.JSONEq(t, `{"error":{"code":"internal_error","message":"internal server error"}}`, w.Body.String())
	assert.NotEmpty(t, w.Header().Get("X-Request-ID"))
	assert.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"))
}
