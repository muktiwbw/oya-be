package httpapi

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequestDTOContracts(t *testing.T) {
	for _, tc := range []struct {
		name string
		body any
		want string
	}{
		{"create defaults", CreateTaskRequest{Title: "task"}, `{"title":"task"}`},
		{"update without version", UpdateTaskStatusRequest{Status: "doing"}, `{"status":"doing","expected_version":null}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body, err := json.Marshal(tc.body)
			require.NoError(t, err)
			assert.JSONEq(t, tc.want, string(body))
		})
	}
}

func TestResponseDTOContracts(t *testing.T) {
	for _, tc := range []struct {
		name string
		body any
		want string
	}{
		{"task", TaskResponse{Data: TaskDTO{PublicID: "public-id", Title: "task", Description: "", Status: "todo", Version: 1, CreatedAt: "2026-10-16T00:00:00.000Z", UpdatedAt: "2026-10-16T00:00:00.000Z"}}, `{"data":{"public_id":"public-id","title":"task","description":"","status":"todo","version":1,"created_at":"2026-10-16T00:00:00.000Z","updated_at":"2026-10-16T00:00:00.000Z"}}`},
		{"empty list", ListTasksResponse{Data: []TaskDTO{}, Pagination: PaginationDTO{Limit: 50}}, `{"data":[],"pagination":{"limit":50,"count":0,"has_more":false,"next_cursor":null}}`},
		{"error without details", ErrorResponse{Error: ErrorDTO{Code: "not_found", Message: "task not found"}}, `{"error":{"code":"not_found","message":"task not found"}}`},
		{"validation details", ErrorResponse{Error: ErrorDTO{Code: "validation_error", Message: "invalid title", Details: []ErrorDetailDTO{{Field: "title", Reason: "required"}}}}, `{"error":{"code":"validation_error","message":"invalid title","details":[{"field":"title","reason":"required"}]}}`},
		{"health", HealthResponse{Status: "ok"}, `{"status":"ok"}`},
		{"version", VersionResponse{Service: "oya", Version: "dev", Commit: "unknown"}, `{"service":"oya","version":"dev","commit":"unknown"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body, err := json.Marshal(tc.body)
			require.NoError(t, err)
			assert.JSONEq(t, tc.want, string(body))
		})
	}
}

func TestRequestDTOOptionalFields(t *testing.T) {
	h := testHandler(t)
	w := request(h, http.MethodPost, "/api/v1/tasks", `{"title":"task"}`, "")
	require.Equal(t, http.StatusCreated, w.Code)
	var created TaskResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))
	assert.Equal(t, "", created.Data.Description)
	assert.EqualValues(t, "todo", created.Data.Status)
	for _, body := range []string{
		`{"title":"task","description":null}`,
		`{"title":"task","status":null}`,
		`{"title":"task","status":""}`,
	} {
		w := request(h, http.MethodPost, "/api/v1/tasks", body, "")
		assert.Equal(t, http.StatusBadRequest, w.Code, "%s", w.Body)
	}
	for _, body := range []string{`{"status":"doing"}`, `{"status":"doing","expected_version":null}`, `{"status":"doing","expected_version":0}`} {
		w := request(h, http.MethodPatch, "/api/v1/tasks/"+created.Data.PublicID+"/status", body, "")
		assert.Equal(t, http.StatusBadRequest, w.Code, "%s", w.Body)
	}
	for _, body := range []string{
		`{"expected_version":1}`,
		`{"title":null,"expected_version":1}`,
		`{"title":"task"}`,
		`{"title":"task","status":"doing","expected_version":1}`,
	} {
		w := request(h, http.MethodPatch, "/api/v1/tasks/"+created.Data.PublicID, body, "")
		assert.Equal(t, http.StatusBadRequest, w.Code, "%s", w.Body)
	}
}
