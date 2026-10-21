package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"oya-be/internal/task"
)

type fieldError struct {
	Field  string
	Reason string
}

func writeValidationErrors(w http.ResponseWriter, errs []fieldError) {
	details := make([]ErrorDetailDTO, len(errs))
	for i, e := range errs {
		details[i] = ErrorDetailDTO{Field: e.Field, Reason: e.Reason}
	}
	writeJSON(w, http.StatusBadRequest, ErrorResponse{
		Error: ErrorDTO{
			Code:    "validation_error",
			Message: "request validation failed",
			Details: details,
		},
	})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeProblem(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, ErrorResponse{Error: ErrorDTO{Code: code, Message: message}})
}

func writeFailure(w http.ResponseWriter, r *http.Request, err error) {
	var ve *task.ValidationError
	switch {
	case errors.As(err, &ve):
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: ErrorDTO{
			Code: "validation_error", Message: ve.Error(),
			Details: []ErrorDetailDTO{{Field: ve.Field, Reason: ve.Reason}},
		}})
	case errors.Is(err, task.ErrInvalidCursor):
		writeProblem(w, http.StatusBadRequest, "invalid_cursor", "cursor is malformed or does not match the status filter")
	case errors.Is(err, task.ErrNotFound):
		writeProblem(w, http.StatusNotFound, "not_found", "task not found")
	case errors.Is(err, task.ErrBoardNotFound):
		writeProblem(w, http.StatusNotFound, "not_found", "board not found")
	case errors.Is(err, task.ErrVersionConflict):
		writeProblem(w, http.StatusConflict, "version_conflict", "task version does not match expected_version")
	case errors.Is(err, task.ErrIdempotencyKeyReused):
		writeProblem(w, http.StatusConflict, "idempotency_key_reused", "key was already used with different input")
	case errors.Is(err, task.ErrUnavailable):
		writeProblem(w, http.StatusServiceUnavailable, "service_unavailable", "service unavailable")
	default:
		slog.Error("request failed", "request_id", w.Header().Get("X-Request-ID"), "error", err)
		writeProblem(w, http.StatusInternalServerError, "internal_error", "internal server error")
	}
}

func (a *API) decodeJSON(w http.ResponseWriter, r *http.Request, dest any) bool {
	media := strings.TrimSpace(strings.Split(r.Header.Get("Content-Type"), ";")[0])
	if !strings.EqualFold(media, "application/json") {
		writeProblem(w, http.StatusUnsupportedMediaType, "unsupported_media_type", "Content-Type must be application/json")
		return false
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, a.bodyLimit))
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		writeProblem(w, http.StatusRequestEntityTooLarge, "request_too_large", "request body exceeds limit")
		return false
	}
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_request", "cannot read request body")
		return false
	}
	body = bytes.TrimSpace(body)
	if len(body) == 0 || body[0] != '{' {
		writeProblem(w, http.StatusBadRequest, "invalid_request", "body must contain one JSON object")
		return false
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if decoder.Decode(dest) != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_request", "invalid JSON or unsupported field")
		return false
	}
	if err = decoder.Decode(&struct{}{}); err != io.EOF {
		writeProblem(w, http.StatusBadRequest, "invalid_request", "body must contain one JSON object")
		return false
	}
	// All supplied fields in the MVP request contracts are non-nullable.
	// Inspect presence separately because encoding/json accepts null for strings
	// and treats omitted and null pointer fields alike.
	var fields map[string]json.RawMessage
	if err = json.Unmarshal(body, &fields); err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_request", "invalid JSON")
		return false
	}
	for field, value := range fields {
		if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			writeFailure(w, r, &task.ValidationError{Field: field, Reason: "must not be null"})
			return false
		}
	}
	return true
}

func parseListTasksRequest(r *http.Request) (ListTasksRequest, error) {
	query := r.URL.Query()
	in := ListTasksRequest{Status: task.Status(query.Get("status")), Limit: 50, Cursor: query.Get("cursor")}
	if in.Status != "" && !in.Status.Valid() {
		return in, &task.ValidationError{Field: "status", Reason: "must be todo, doing, or done"}
	}
	if rawLimit := query.Get("limit"); rawLimit != "" {
		limit, err := strconv.Atoi(rawLimit)
		if err != nil || limit < 1 || limit > 100 {
			return in, &task.ValidationError{Field: "limit", Reason: "must be between 1 and 100"}
		}
		in.Limit = limit
	}
	return in, nil
}
