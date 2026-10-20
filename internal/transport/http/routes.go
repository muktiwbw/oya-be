package httpapi

import (
	"context"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"oya-be/internal/board"
	"oya-be/internal/platform/auth"
	"oya-be/internal/task"
	"oya-be/internal/user"
)

type API struct {
	service         *task.Service
	users           *user.Service
	boards          *board.Service
	tokens          *auth.TokenManager
	ready           func(context.Context) error
	bodyLimit       int64
	version, commit string
}

func New(
	taskService *task.Service,
	userService *user.Service,
	boardService *board.Service,
	tokenManager *auth.TokenManager,
	ready func(context.Context) error,
	limit int64,
	version, commit string,
) http.Handler {
	a := &API{
		service:   taskService,
		users:     userService,
		boards:    boardService,
		tokens:    tokenManager,
		ready:     ready,
		bodyLimit: limit,
		version:   version,
		commit:    commit,
	}
	router := chi.NewRouter()
	router.Use(responseHeaders, recoverJSON)
	router.NotFound(func(w http.ResponseWriter, r *http.Request) {
		writeProblem(w, http.StatusNotFound, "not_found", "route not found")
	})
	router.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
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
		// Public auth endpoints
		r.Post("/auth/login", a.login)

		// Protected endpoints
		r.Group(func(pr chi.Router) {
			pr.Use(a.authenticate)

			// Board endpoints
			pr.Post("/boards", a.createBoard)
			pr.Get("/boards", a.listBoards)
			pr.Get("/boards/{board_public_id}", a.getBoard)
			pr.Patch("/boards/{board_public_id}", a.updateBoard)
			pr.Delete("/boards/{board_public_id}", a.deleteBoard)
			pr.Get("/boards/{board_public_id}/tasks", a.listBoardTasks)

			// Task endpoints
			pr.Post("/tasks", a.createTask)
			pr.Get("/tasks/{public_id}", a.getTask)
			pr.Patch("/tasks/{public_id}", a.updateTask)
			pr.Patch("/tasks/{public_id}/status", a.updateTaskStatus)
			pr.Delete("/tasks/{public_id}", a.deleteTask)
		})
	})

	return router
}
