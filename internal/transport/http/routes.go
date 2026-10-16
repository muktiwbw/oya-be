package httpapi

import (
	"context"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"oya-be/internal/task"
)

type API struct {
	service         *task.Service
	ready           func(context.Context) error
	bodyLimit       int64
	version, commit string
}

func New(s *task.Service, ready func(context.Context) error, limit int64, version, commit string) http.Handler {
	a := &API{service: s, ready: ready, bodyLimit: limit, version: version, commit: commit}
	router := chi.NewRouter()
	router.Use(responseHeaders, recoverJSON)
	router.NotFound(func(w http.ResponseWriter, r *http.Request) {
		writeProblem(w, http.StatusNotFound, "not_found", "route not found")
	})
	router.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		// Match against the registered routes so Allow remains correct when
		// routes change, without maintaining a second route table.
		path := r.URL.Path
		if r.URL.RawPath != "" {
			path = r.URL.RawPath
		}
		var allowed []string
		for _, method := range []string{
			http.MethodConnect, http.MethodDelete, http.MethodGet, http.MethodHead,
			http.MethodOptions, http.MethodPatch, http.MethodPost, http.MethodPut, http.MethodTrace,
		} {
			if router.Match(chi.NewRouteContext(), method, path) {
				allowed = append(allowed, method)
			}
		}
		if len(allowed) == 0 {
			writeProblem(w, http.StatusNotFound, "not_found", "route not found")
			return
		}
		w.Header().Set("Allow", strings.Join(allowed, ", "))
		writeProblem(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
	})
	router.Get("/healthz", a.health)
	router.Get("/readyz", a.readiness)
	router.Get("/version", a.versionInfo)
	router.Route("/api/v1", func(r chi.Router) {
		r.Post("/tasks", a.createTask)
		r.Get("/tasks", a.listTasks)
		r.Get("/tasks/{public_id}", a.getTask)
		r.Patch("/tasks/{public_id}", a.updateTask)
		r.Patch("/tasks/{public_id}/status", a.updateTaskStatus)
	})
	return router
}
