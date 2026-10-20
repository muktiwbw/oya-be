// Package httpapi translates JSON HTTP requests into application use cases.
package httpapi

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"oya-be/internal/task"
)

func (a *API) getTask(w http.ResponseWriter, r *http.Request) {
	claims, ok := UserFromContext(r.Context())
	if !ok {
		writeProblem(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}
	t, err := a.service.GetTask(r.Context(), claims.UserID, chi.URLParam(r, "public_id"))
	if err != nil {
		writeFailure(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, TaskResponse{Data: newTaskDTO(t)})
}

func (a *API) createTask(w http.ResponseWriter, r *http.Request) {
	claims, ok := UserFromContext(r.Context())
	if !ok {
		writeProblem(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}
	body := CreateTaskRequest{Status: task.Todo}
	if !a.decodeJSON(w, r, &body) {
		return
	}
	if !body.Status.Valid() {
		writeFailure(w, r, &task.ValidationError{Field: "status", Reason: "must be todo, doing, or done"})
		return
	}
	key := r.Header.Get("Idempotency-Key")
	if values, ok := r.Header["Idempotency-Key"]; ok && (len(values) != 1 || strings.TrimSpace(key) == "") {
		writeFailure(w, r, &task.ValidationError{Field: "idempotency_key", Reason: "must contain one non-empty key"})
		return
	}
	out, err := a.service.CreateTask(r.Context(), claims.UserID, task.CreateInput{
		BoardID: body.BoardID, Title: body.Title, Description: body.Description, Status: body.Status,
	}, key)
	if err != nil {
		writeFailure(w, r, err)
		return
	}
	w.Header().Set("Location", "/api/v1/tasks/"+out.Task.PublicID)
	if out.Replayed {
		w.Header().Set("Idempotency-Replayed", "true")
	}
	writeJSON(w, http.StatusCreated, TaskResponse{Data: newTaskDTO(out.Task)})
}

func (a *API) updateTaskStatus(w http.ResponseWriter, r *http.Request) {
	claims, ok := UserFromContext(r.Context())
	if !ok {
		writeProblem(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}
	var body UpdateTaskStatusRequest
	if !a.decodeJSON(w, r, &body) {
		return
	}
	t, err := a.service.UpdateTaskStatus(r.Context(), claims.UserID, chi.URLParam(r, "public_id"), task.UpdateStatusInput{
		Status: body.Status, ExpectedVersion: body.ExpectedVersion,
	})
	if err != nil {
		writeFailure(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, TaskResponse{Data: newTaskDTO(t)})
}

func (a *API) updateTask(w http.ResponseWriter, r *http.Request) {
	claims, ok := UserFromContext(r.Context())
	if !ok {
		writeProblem(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}
	var body UpdateTaskRequest
	if !a.decodeJSON(w, r, &body) {
		return
	}
	t, err := a.service.UpdateTask(r.Context(), claims.UserID, chi.URLParam(r, "public_id"), task.UpdateTaskInput{
		Title: body.Title, Description: body.Description, ExpectedVersion: body.ExpectedVersion,
	})
	if err != nil {
		writeFailure(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, TaskResponse{Data: newTaskDTO(t)})
}

func (a *API) deleteTask(w http.ResponseWriter, r *http.Request) {
	claims, ok := UserFromContext(r.Context())
	if !ok {
		writeProblem(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}
	err := a.service.DeleteTask(r.Context(), claims.UserID, chi.URLParam(r, "public_id"))
	if err != nil {
		writeFailure(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
