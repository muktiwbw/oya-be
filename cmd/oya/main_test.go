package main

import (
	"context"
	"net"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"oya-be/internal/platform/config"
)

func TestRunShutsDownOnCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()
	c := config.Config{DBPath: filepath.Join(t.TempDir(), "tasks.db"), ShutdownTimeout: time.Second, RequestBodyLimit: 65536, IdempotencyTTL: 24 * time.Hour}
	done := make(chan error, 1)
	go func() { done <- serve(ctx, c, listener) }()
	client := http.Client{Timeout: time.Second}
	deadline := time.Now().Add(5 * time.Second)
	for {
		res, err := client.Get("http://" + listener.Addr().String() + "/readyz")
		if err == nil {
			res.Body.Close()
			if res.StatusCode == 200 {
				break
			}
		}
		if time.Now().After(deadline) {
			require.FailNow(t, "server did not become ready")
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		require.FailNow(t, "shutdown stalled")
	}
}
