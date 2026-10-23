package httpapi

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"oya-be/internal/task"
)

func TestHealthReadinessAndVersion(t *testing.T) {
	h := New(nil, nil, nil, nil, AuthSettings{}, func(context.Context) error { return nil }, 65536, "dev", "unknown")
	for _, path := range []string{"/healthz", "/readyz", "/version"} {
		w := request(h, "GET", path, "", "")
		assert.Equal(t, http.StatusOK, w.Code, "%s", w.Body)
	}
	h = New(nil, nil, nil, nil, AuthSettings{}, func(context.Context) error { return task.ErrUnavailable }, 65536, "dev", "unknown")
	w := request(h, "GET", "/healthz", "", "")
	assert.Equal(t, http.StatusOK, w.Code, "%s", w.Body)
	w = request(h, "GET", "/readyz", "", "")
	assert.Equal(t, http.StatusServiceUnavailable, w.Code, "%s", w.Body)
}
