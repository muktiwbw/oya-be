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
		{"create defaults", CreateTaskRequest{BoardID: "board-id", Title: "task"}, `{"board_id":"board-id","title":"task"}`},
		{"update without version", UpdateTaskStatusRequest{Status: "doing"}, `{"status":"doing","expected_version":null}`},
		{"create board", CreateBoardRequest{Title: "board"}, `{"title":"board"}`},
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
		{"task", TaskResponse{Data: TaskDTO{PublicID: "public-id", BoardID: "board-id", Title: "task", Description: "", Status: "todo", Version: 1, CreatedAt: "2026-10-16T00:00:00.000Z", UpdatedAt: "2026-10-16T00:00:00.000Z"}}, `{"data":{"public_id":"public-id","board_id":"board-id","title":"task","description":"","status":"todo","version":1,"created_at":"2026-10-16T00:00:00.000Z","updated_at":"2026-10-16T00:00:00.000Z"}}`},
		{"board", BoardResponse{Data: BoardDTO{PublicID: "board-id", Title: "Main Board", Description: "", CreatedAt: "2026-10-16T00:00:00.000Z", UpdatedAt: "2026-10-16T00:00:00.000Z"}}, `{"data":{"public_id":"board-id","title":"Main Board","description":"","created_at":"2026-10-16T00:00:00.000Z","updated_at":"2026-10-16T00:00:00.000Z"}}`},
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
	for _, tc := range []struct {
		name    string
		body    string
		dest    any
		wantErr bool
	}{
		{"valid create with description", `{"board_id":"b-1","title":"task","description":"# Markdown"}`, &CreateTaskRequest{}, false},
		{"invalid json", `{bad json`, &CreateTaskRequest{}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := json.Unmarshal([]byte(tc.body), tc.dest)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestStatusConstants(t *testing.T) {
	assert.Equal(t, 200, http.StatusOK)
	assert.Equal(t, 201, http.StatusCreated)
	assert.Equal(t, 400, http.StatusBadRequest)
	assert.Equal(t, 401, http.StatusUnauthorized)
	assert.Equal(t, 404, http.StatusNotFound)
	assert.Equal(t, 405, http.StatusMethodNotAllowed)
	assert.Equal(t, 409, http.StatusConflict)
	assert.Equal(t, 413, http.StatusRequestEntityTooLarge)
	assert.Equal(t, 415, http.StatusUnsupportedMediaType)
	assert.Equal(t, 500, http.StatusInternalServerError)
	assert.Equal(t, 503, http.StatusServiceUnavailable)
}
