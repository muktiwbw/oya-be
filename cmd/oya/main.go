package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"oya-be/internal/persistence/sqlite"
	"oya-be/internal/platform/clock"
	"oya-be/internal/platform/config"
	"oya-be/internal/platform/ids"
	"oya-be/internal/task"
	httpapi "oya-be/internal/transport/http"
)

var version = "dev"
var commit = "unknown"

func main() {
	if err := run(); err != nil {
		slog.Error("application failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
	c, err := config.LoadFile(".env", os.LookupEnv)
	if err != nil {
		return err
	}
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		_, port, err := net.SplitHostPort(c.HTTPAddr)
		if err != nil {
			return err
		}
		client := http.Client{Timeout: 2 * time.Second}
		res, err := client.Get("http://127.0.0.1:" + port + "/healthz")
		if err != nil {
			return err
		}
		defer res.Body.Close()
		if res.StatusCode != 200 {
			return fmt.Errorf("health check: %d", res.StatusCode)
		}
		return nil
	}
	var level slog.Level
	if err = level.UnmarshalText([]byte(c.LogLevel)); err != nil {
		return err
	}
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})))
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if len(os.Args) > 1 && os.Args[1] == "migrate" {
		if len(os.Args) != 2 {
			return fmt.Errorf("usage: oya migrate")
		}
		store, err := sqlite.Open(ctx, c.DBPath)
		if err != nil {
			return fmt.Errorf("migrate database: %w", err)
		}
		if err := store.Close(); err != nil {
			return fmt.Errorf("close migrated database: %w", err)
		}
		slog.Info("database migrations complete", "path", c.DBPath)
		return nil
	}
	listener, err := net.Listen("tcp", c.HTTPAddr)
	if err != nil {
		return err
	}
	defer listener.Close()
	return serve(ctx, c, listener)
}

func serve(ctx context.Context, c config.Config, listener net.Listener) error {
	store, err := sqlite.Open(ctx, c.DBPath)
	if err != nil {
		return err
	}
	defer store.Close()
	var stopping atomic.Bool
	ready := func(ctx context.Context) error {
		if stopping.Load() {
			return task.ErrUnavailable
		}
		return store.Ready(ctx)
	}
	service := task.NewService(store, clock.System{}, ids.Generator{}, c.IdempotencyTTL)
	server := &http.Server{Handler: httpapi.New(service, ready, c.RequestBodyLimit, version, commit), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second}
	shutdownDone := make(chan error, 1)
	go func() {
		select {
		case <-ctx.Done():
			stopping.Store(true)
			slog.Info("shutdown started")
			shutdownCtx, cancel := context.WithTimeout(context.Background(), c.ShutdownTimeout)
			defer cancel()
			err := server.Shutdown(shutdownCtx)
			if err != nil {
				server.Close()
			}
			shutdownDone <- err
		case <-shutdownDone:
		}
	}()
	slog.Info("server listening", "address", listener.Addr().String(), "version", version)
	err = server.Serve(listener)
	if !errors.Is(err, http.ErrServerClosed) {
		shutdownDone <- nil
		return err
	}
	err = <-shutdownDone
	slog.Info("shutdown complete")
	return err
}
