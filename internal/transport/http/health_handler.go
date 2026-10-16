package httpapi

import (
	"net/http"

	"oya-be/internal/task"
)

func (a *API) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, HealthResponse{Status: "ok"})
}

func (a *API) readiness(w http.ResponseWriter, r *http.Request) {
	if err := a.ready(r.Context()); err != nil {
		writeFailure(w, r, task.ErrUnavailable)
		return
	}
	writeJSON(w, http.StatusOK, HealthResponse{Status: "ready"})
}

func (a *API) versionInfo(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, VersionResponse{Service: "oya", Version: a.version, Commit: a.commit})
}
