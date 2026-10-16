package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"oya-be/internal/persistence/sqlite"
	"oya-be/internal/platform/clock"
	"oya-be/internal/platform/ids"
	"oya-be/internal/task"
)

func testHandler(t *testing.T) http.Handler {
	t.Helper()
	store, err := sqlite.Open(context.Background(), filepath.Join(t.TempDir(), "tasks.db"))
	require.NoError(t, err)
	t.Cleanup(func() { store.Close() })
	return New(task.NewService(store, clock.System{}, ids.Generator{}, 24*time.Hour), store.Ready, 65536, "dev", "unknown")
}

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

func TestTaskHTTPFlow(t *testing.T) {
	h := testHandler(t)
	w := request(h, "POST", "/api/v1/tasks", `{"title":"task"}`, "key")
	require.Equal(t, http.StatusCreated, w.Code, "create: %s", w.Body)
	var created TaskResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))
	assert.Equal(t, int64(1), created.Data.Version)
	assert.Empty(t, created.Data.Description)
	require.NotEmpty(t, created.Data.PublicID)
	location := w.Header().Get("Location")
	assert.Equal(t, "/api/v1/tasks/"+created.Data.PublicID, location)
	w = request(h, "PATCH", location+"/status", `{"status":"doing","expected_version":1}`, "")
	require.Equal(t, http.StatusOK, w.Code, "%s", w.Body)
	w = request(h, "PATCH", location, `{"title":"  revised task  ","description":"# Notes","expected_version":2}`, "")
	require.Equal(t, http.StatusOK, w.Code, "%s", w.Body)
	var edited TaskResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &edited))
	assert.Equal(t, "revised task", edited.Data.Title)
	assert.Equal(t, "# Notes", edited.Data.Description)
	assert.Equal(t, int64(3), edited.Data.Version)
	w = request(h, "PATCH", location, `{"description":"","expected_version":3}`, "")
	require.Equal(t, http.StatusOK, w.Code, "%s", w.Body)
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &edited))
	assert.Empty(t, edited.Data.Description)
	assert.Equal(t, int64(4), edited.Data.Version)
	w = request(h, "PATCH", location+"/status", `{"status":"done","expected_version":1}`, "")
	require.Equal(t, http.StatusConflict, w.Code, "%s", w.Body)
	w = request(h, "POST", "/api/v1/tasks", `{"title":"task"}`, "key")
	require.Equal(t, http.StatusCreated, w.Code, "replay: %s", w.Body)
	assert.Equal(t, "true", w.Header().Get("Idempotency-Replayed"))
	assert.Contains(t, w.Body.String(), `"status":"todo"`)
	w = request(h, "POST", "/api/v1/tasks", `{"title":"different"}`, "key")
	require.Equal(t, http.StatusConflict, w.Code, "%s", w.Body)
	w = request(h, "GET", location, "", "")
	require.Equal(t, http.StatusOK, w.Code, "%s", w.Body)
	w = request(h, "GET", "/api/v1/tasks?limit=1", "", "")
	require.Equal(t, http.StatusOK, w.Code, "%s", w.Body)
	assert.Contains(t, w.Body.String(), `"next_cursor":null`)
}

func TestHTTPErrors(t *testing.T) {
	h := testHandler(t)
	for _, tc := range []struct {
		method, path, body string
		status             int
	}{
		{"POST", "/api/v1/tasks", `{}`, 400},
		{"POST", "/api/v1/tasks", `{"title":"task","description":null}`, 400},
		{"POST", "/api/v1/tasks", `{"title":"task","status":""}`, 400},
		{"POST", "/api/v1/tasks", `{"title":"task","extra":1}`, 400},
		{"POST", "/api/v1/tasks", `{"title":"task"} {}`, 400},
		{"GET", "/api/v1/tasks/bad", "", 400},
		{"GET", "/api/v1/tasks?cursor=bad", "", 400},
		{"GET", "/api/v1/tasks?limit=0", "", 400},
		{"DELETE", "/api/v1/tasks", "", 405},
		{"GET", "/unknown", "", 404},
		{"POST", "/api/v1/tasks", strings.Repeat(" ", 65537), 413},
	} {
		t.Run(tc.path+tc.body[:min(len(tc.body), 30)], func(t *testing.T) {
			w := request(h, tc.method, tc.path, tc.body, "")
			require.Equal(t, tc.status, w.Code, "%s", w.Body)
			var e ErrorResponse
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &e))
			assert.NotEmpty(t, e.Error.Code)
			assert.NotEmpty(t, e.Error.Message)
		})
	}
}
