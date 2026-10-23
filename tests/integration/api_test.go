//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
	tc "github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/network"
	"github.com/testcontainers/testcontainers-go/wait"
	"oya-be/internal/platform/auth"
)

func TestEndToEndLifecycle(t *testing.T) {
	ctx := context.Background()
	image := os.Getenv("OYA_TEST_IMAGE")
	if image == "" {
		image = "oya:integration"
	}

	nw, err := network.New(ctx)
	tc.CleanupNetwork(t, nw)
	require.NoError(t, err)

	initialMigrationPath, err := filepath.Abs(filepath.Join("..", "..", "migrations", "V1__initial_schema.sql"))
	require.NoError(t, err)
	googleAuthMigrationPath, err := filepath.Abs(filepath.Join("..", "..", "migrations", "V2__google_sso_sessions.sql"))
	require.NoError(t, err)

	pgContainer, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("oya"),
		postgres.WithUsername("postgres"),
		postgres.WithPassword("postgres"),
		postgres.WithInitScripts(initialMigrationPath, googleAuthMigrationPath),
		network.WithNetwork([]string{"postgres"}, nw),
		tc.WithWaitStrategy(wait.ForLog("database system is ready to accept connections").WithOccurrence(2).WithStartupTimeout(time.Minute)),
	)
	tc.CleanupContainer(t, pgContainer)
	require.NoError(t, err)

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	// Seed a whitelisted Google user directly; external Google callbacks are not part
	// of this containerized lifecycle test.
	conn, err := pgx.Connect(ctx, connStr)
	require.NoError(t, err)
	defer conn.Close(ctx)

	userUUID, err := uuid.NewV7()
	require.NoError(t, err)

	_, err = conn.Exec(ctx, `
		INSERT INTO users (public_id, email, google_subject, display_name)
		VALUES ($1, $2, $3, $4)
	`, userUUID, "test@example.com", "google-test-subject", "Test User")
	require.NoError(t, err)
	_, err = conn.Exec(ctx, `INSERT INTO whitelisted_users (email, whitelisted) VALUES ($1, true)`, "test@example.com")
	require.NoError(t, err)

	// Start application container
	appContainer, err := tc.GenericContainer(ctx, tc.GenericContainerRequest{
		ContainerRequest: tc.ContainerRequest{
			Image:        image,
			ExposedPorts: []string{"8080/tcp"},
			Networks:     []string{nw.Name},
			Env: map[string]string{
				"OYA_HTTP_ADDR":  ":8080",
				"DB_HOST":        "postgres",
				"DB_PORT":        "5432",
				"DB_NAME":        "oya",
				"DB_USER":        "postgres",
				"DB_PASSWORD":    "postgres",
				"OYA_JWT_SECRET": "test-secret-key-that-is-at-least-32-chars-long",
			},
			WaitingFor: wait.ForHTTP("/readyz").WithPort("8080/tcp").WithStartupTimeout(time.Minute),
		},
		Started: true,
	})
	tc.CleanupContainer(t, appContainer)
	require.NoError(t, err)

	host, err := appContainer.Host(ctx)
	require.NoError(t, err)
	port, err := appContainer.MappedPort(ctx, "8080/tcp")
	require.NoError(t, err)

	baseURL := fmt.Sprintf("http://%s:%s", host, port.Port())
	client := http.Client{Timeout: 10 * time.Second}

	tokenManager := auth.NewTokenManager("test-secret-key-that-is-at-least-32-chars-long", 15*time.Minute)
	var authToken string

	call := func(method, path, body, key string, want int) map[string]any {
		t.Helper()
		r, err := http.NewRequest(method, baseURL+path, strings.NewReader(body))
		require.NoError(t, err)
		r.Header.Set("Content-Type", "application/json")
		if key != "" {
			r.Header.Set("Idempotency-Key", key)
		}
		if authToken != "" {
			r.Header.Set("Authorization", "Bearer "+authToken)
		}
		res, err := client.Do(r)
		require.NoError(t, err)
		defer res.Body.Close()

		b, err := io.ReadAll(res.Body)
		require.NoError(t, err)

		require.Equal(t, want, res.StatusCode, "%s %s: got %d, want %d: %s", method, path, res.StatusCode, want, string(b))
		if len(b) == 0 {
			return nil
		}
		var decoded map[string]any
		err = json.Unmarshal(b, &decoded)
		require.NoError(t, err)
		return decoded
	}

	// 1. Public operational endpoints
	call("GET", "/healthz", "", "", 200)
	call("GET", "/readyz", "", "", 200)
	call("GET", "/version", "", "", 200)

	// 2. Unauthenticated calls to protected routes must be rejected with 401
	call("GET", "/api/v1/boards", "", "", 401)
	call("POST", "/api/v1/tasks", `{"title":"test"}`, "", 401)

	// 3. Password login has been removed and Google sign-in is disabled without credentials.
	call("POST", "/api/v1/auth/login", `{}`, "", 404)
	call("GET", "/api/v1/auth/google", "", "", 503)
	authToken, err = tokenManager.Generate(1, userUUID.String())
	require.NoError(t, err)

	// 4. Board CRUD
	boardCreated := call("POST", "/api/v1/boards", `{"title":"Product Roadmap","description":"Q4 initiatives"}`, "", 201)
	boardData := boardCreated["data"].(map[string]any)
	boardID := boardData["public_id"].(string)
	require.NotEmpty(t, boardID)

	// Get Board
	boardGet := call("GET", "/api/v1/boards/"+boardID, "", "", 200)
	require.Equal(t, "Product Roadmap", boardGet["data"].(map[string]any)["title"])

	// Update Board
	boardUpdated := call("PATCH", "/api/v1/boards/"+boardID, `{"title":"Product Roadmap 2026"}`, "", 200)
	require.Equal(t, "Product Roadmap 2026", boardUpdated["data"].(map[string]any)["title"])

	// List Boards
	boardList := call("GET", "/api/v1/boards", "", "", 200)
	require.Len(t, boardList["data"].([]any), 1)

	// 5. Task Creation & Immutability
	taskReqBody := fmt.Sprintf(`{"board_id":"%s","title":"Deliver FEAT-02","description":"PostgreSQL migration"}`, boardID)
	taskCreated := call("POST", "/api/v1/tasks", taskReqBody, "create-key-1", 201)
	taskData := taskCreated["data"].(map[string]any)
	taskID := taskData["public_id"].(string)
	require.NotEmpty(t, taskID)
	require.Equal(t, boardID, taskData["board_id"])

	// Replay with same idempotency key
	replayed := call("POST", "/api/v1/tasks", taskReqBody, "create-key-1", 201)
	require.Equal(t, taskID, replayed["data"].(map[string]any)["public_id"])

	// Replay with different body -> 409
	call("POST", "/api/v1/tasks", fmt.Sprintf(`{"board_id":"%s","title":"Different"}`, boardID), "create-key-1", 409)

	// Get Task
	taskGet := call("GET", "/api/v1/tasks/"+taskID, "", "", 200)
	require.Equal(t, "Deliver FEAT-02", taskGet["data"].(map[string]any)["title"])

	// Update Task
	taskUpdated := call("PATCH", "/api/v1/tasks/"+taskID, `{"title":"Deliver FEAT-02 Completed","expected_version":1}`, "", 200)
	require.Equal(t, "Deliver FEAT-02 Completed", taskUpdated["data"].(map[string]any)["title"])
	require.Equal(t, float64(2), taskUpdated["data"].(map[string]any)["version"])

	// Update Task Status
	call("PATCH", "/api/v1/tasks/"+taskID+"/status", `{"status":"doing","expected_version":2}`, "", 200)

	// List Tasks by Board
	boardTasks := call("GET", "/api/v1/boards/"+boardID+"/tasks", "", "", 200)
	tasksList := boardTasks["data"].([]any)
	require.Len(t, tasksList, 1)

	// Create second task on same board
	task2ReqBody := fmt.Sprintf(`{"board_id":"%s","title":"Task 2"}`, boardID)
	task2Created := call("POST", "/api/v1/tasks", task2ReqBody, "", 201)
	task2ID := task2Created["data"].(map[string]any)["public_id"].(string)

	// Soft-delete single task
	call("DELETE", "/api/v1/tasks/"+taskID, "", "", 204)
	call("GET", "/api/v1/tasks/"+taskID, "", "", 404)

	// Board task list now only has Task 2
	boardTasksAfterDelete := call("GET", "/api/v1/boards/"+boardID+"/tasks", "", "", 200)
	require.Len(t, boardTasksAfterDelete["data"].([]any), 1)

	// 6. Cascade Soft-Delete Board
	call("DELETE", "/api/v1/boards/"+boardID, "", "", 204)

	// Board is soft-deleted
	call("GET", "/api/v1/boards/"+boardID, "", "", 404)
	// Task 2 is cascaded soft-deleted
	call("GET", "/api/v1/tasks/"+task2ID, "", "", 404)
}
