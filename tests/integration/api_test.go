//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	tc "github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestContainerTaskLifecycle(t *testing.T) {
	ctx := context.Background()
	image := os.Getenv("OYA_TEST_IMAGE")
	if image == "" {
		image = "oya:integration"
	}
	volume := "oya-test-" + uuid.NewString()
	c, err := tc.GenericContainer(ctx, tc.GenericContainerRequest{ContainerRequest: tc.ContainerRequest{Image: image, ExposedPorts: []string{"8080/tcp"}, Mounts: tc.ContainerMounts{tc.VolumeMount(volume, "/data")}, WaitingFor: wait.ForHTTP("/readyz").WithPort("8080/tcp").WithStartupTimeout(time.Minute)}, Started: true})
	tc.CleanupContainer(t, c, tc.RemoveVolumes(volume))
	if err != nil {
		t.Fatal(err)
	}
	endpoint := func() string {
		t.Helper()
		host, err := c.Host(ctx)
		if err != nil {
			t.Fatal(err)
		}
		port, err := c.MappedPort(ctx, "8080/tcp")
		if err != nil {
			t.Fatal(err)
		}
		return fmt.Sprintf("http://%s:%s", host, port.Port())
	}
	base := endpoint()
	client := http.Client{Timeout: 10 * time.Second}
	call := func(method, path, body, key string, want int) map[string]any {
		t.Helper()
		r, err := http.NewRequest(method, base+path, strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		r.Header.Set("Content-Type", "application/json")
		if key != "" {
			r.Header.Set("Idempotency-Key", key)
		}
		res, err := client.Do(r)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		b, err := io.ReadAll(res.Body)
		if err != nil {
			t.Fatal(err)
		}
		if res.StatusCode != want {
			t.Fatalf("%s %s: got %d, want %d: %s", method, path, res.StatusCode, want, b)
		}
		var decoded map[string]any
		if err = json.Unmarshal(b, &decoded); err != nil {
			t.Fatal(err)
		}
		return decoded
	}
	call("GET", "/healthz", "", "", 200)
	call("GET", "/readyz", "", "", 200)
	call("GET", "/version", "", "", 200)
	call("DELETE", "/api/v1/tasks", "", "", 405)
	call("GET", "/api/v1/tasks/", "", "", 404)
	call("GET", "/missing", "", "", 404)
	created := call("POST", "/api/v1/tasks", `{"title":"container task"}`, "create-1", 201)["data"].(map[string]any)
	id := created["public_id"].(string)
	path := "/api/v1/tasks/" + id
	if _, ok := created["id"]; ok {
		t.Fatal("leaked internal ID")
	}
	call("GET", path, "", "", 200)
	call("PATCH", path+"/status", `{"status":"doing","expected_version":1}`, "", 200)
	call("PATCH", path+"/status", `{"status":"done","expected_version":1}`, "", 409)
	edited := call("PATCH", path, `{"title":"edited container task","description":"# Markdown","expected_version":2}`, "", 200)["data"].(map[string]any)
	if edited["title"] != "edited container task" || edited["description"] != "# Markdown" || edited["version"] != float64(3) {
		t.Fatal("task edit did not return changed task")
	}
	call("PATCH", path, `{"description":"","expected_version":3}`, "", 200)
	replayed := call("POST", "/api/v1/tasks", `{"title":"container task"}`, "create-1", 201)["data"].(map[string]any)
	if replayed["public_id"] != id || replayed["status"] != "todo" {
		t.Fatal("replay changed original response")
	}
	call("POST", "/api/v1/tasks", `{"title":"different"}`, "create-1", 409)
	call("POST", "/api/v1/tasks", `{"title":"container task"}`, "create-2", 201)
	page := call("GET", "/api/v1/tasks?limit=1", "", "", 200)["pagination"].(map[string]any)
	cursor, ok := page["next_cursor"].(string)
	if !ok || page["has_more"] != true {
		t.Fatal("missing next cursor")
	}
	call("GET", "/api/v1/tasks?limit=1&cursor="+cursor, "", "", 200)
	call("GET", "/api/v1/tasks?status=todo&cursor="+cursor, "", "", 400)
	call("POST", "/api/v1/tasks", `{"title":""}`, "correctable", 400)
	call("POST", "/api/v1/tasks", `{"title":"corrected"}`, "correctable", 201)
	call("GET", "/api/v1/tasks/bad", "", "", 400)
	for _, from := range []string{"todo", "doing", "done"} {
		for _, to := range []string{"todo", "doing", "done"} {
			row := call("POST", "/api/v1/tasks", `{"title":"transition","status":"`+from+`"}`, "", 201)["data"].(map[string]any)
			changed := call("PATCH", "/api/v1/tasks/"+row["public_id"].(string)+"/status", `{"status":"`+to+`","expected_version":1}`, "", 200)["data"].(map[string]any)
			if changed["status"] != to {
				t.Fatal("transition failed")
			}
			if from == to && changed["version"] != float64(1) {
				t.Fatal("no-op incremented version")
			}
		}
	}
	var wg sync.WaitGroup
	type concurrentResult struct {
		status int
		id     string
		err    error
	}
	results := make(chan concurrentResult, 4)
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req, err := http.NewRequest("POST", base+"/api/v1/tasks", strings.NewReader(`{"title":"concurrent retry"}`))
			if err != nil {
				results <- concurrentResult{err: err}
				return
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Idempotency-Key", "concurrent-key")
			res, err := client.Do(req)
			if err != nil {
				results <- concurrentResult{err: err}
				return
			}
			defer res.Body.Close()
			var decoded struct {
				Data struct {
					PublicID string `json:"public_id"`
				}
			}
			err = json.NewDecoder(res.Body).Decode(&decoded)
			results <- concurrentResult{res.StatusCode, decoded.Data.PublicID, err}
		}()
	}
	wg.Wait()
	close(results)
	concurrentID := ""
	for result := range results {
		if result.err != nil || result.status != 201 || result.id == "" {
			t.Fatalf("concurrent create failed: %+v", result)
		}
		if concurrentID == "" {
			concurrentID = result.id
		}
		if result.id != concurrentID {
			t.Fatal("duplicate concurrent task")
		}
	}
	version := 4
	for _, status := range []string{"done", "todo", "doing"} {
		call("PATCH", path+"/status", `{"status":"`+status+`","expected_version":`+fmt.Sprint(version)+`}`, "", 200)
		version++
	}
	if err = c.Stop(ctx, nil); err != nil {
		t.Fatal(err)
	}
	if err = c.Start(ctx); err != nil {
		t.Fatal(err)
	}
	base = endpoint()
	got := call("GET", path, "", "", 200)["data"].(map[string]any)
	if got["public_id"] != id || got["status"] != "doing" || got["title"] != "edited container task" || got["description"] != "" {
		t.Fatal("restart lost committed task")
	}
}
