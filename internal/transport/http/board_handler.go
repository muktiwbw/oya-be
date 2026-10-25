package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"oya-be/internal/board"
	"oya-be/internal/task"
)

func (a *API) createBoard(w http.ResponseWriter, r *http.Request) {
	claims, ok := UserFromContext(r.Context())
	if !ok {
		writeProblem(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}

	var req CreateBoardRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, a.bodyLimit)).Decode(&req); err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_json", "malformed JSON payload")
		return
	}

	created, err := a.boards.Create(r.Context(), claims.UserID, board.CreateInput{
		Title:       req.Title,
		Description: req.Description,
	})
	if err != nil {
		var valErr *board.ValidationError
		if errors.As(err, &valErr) {
			writeValidationErrors(w, []fieldError{{Field: valErr.Field, Reason: valErr.Reason}})
			return
		}
		writeProblem(w, http.StatusInternalServerError, "internal_error", "failed to create board")
		return
	}

	w.Header().Set("Location", "/api/v1/boards/"+created.PublicID)
	writeJSON(w, http.StatusCreated, BoardResponse{Data: newBoardDTO(created)})
}

func (a *API) listBoards(w http.ResponseWriter, r *http.Request) {
	claims, ok := UserFromContext(r.Context())
	if !ok {
		writeProblem(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}

	boards, err := a.boards.List(r.Context(), claims.UserID)
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "internal_error", "failed to list boards")
		return
	}

	dtos := make([]BoardDTO, 0, len(boards))
	for _, b := range boards {
		dtos = append(dtos, newBoardDTO(b))
	}
	writeJSON(w, http.StatusOK, ListBoardsResponse{Data: dtos})
}

func (a *API) getBoard(w http.ResponseWriter, r *http.Request) {
	claims, ok := UserFromContext(r.Context())
	if !ok {
		writeProblem(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}

	boardID := chi.URLParam(r, "board_public_id")
	b, err := a.boards.Get(r.Context(), claims.UserID, boardID)
	if err != nil {
		if errors.Is(err, board.ErrBoardNotFound) {
			writeProblem(w, http.StatusNotFound, "not_found", "board not found")
			return
		}
		writeProblem(w, http.StatusInternalServerError, "internal_error", "failed to get board")
		return
	}

	writeJSON(w, http.StatusOK, BoardResponse{Data: newBoardDTO(b)})
}

func (a *API) updateBoard(w http.ResponseWriter, r *http.Request) {
	claims, ok := UserFromContext(r.Context())
	if !ok {
		writeProblem(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}

	boardID := chi.URLParam(r, "board_public_id")
	var req UpdateBoardRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, a.bodyLimit)).Decode(&req); err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_json", "malformed JSON payload")
		return
	}

	updated, err := a.boards.Update(r.Context(), claims.UserID, boardID, board.UpdateInput{
		Title:       req.Title,
		Description: req.Description,
	})
	if err != nil {
		var valErr *board.ValidationError
		if errors.As(err, &valErr) {
			writeValidationErrors(w, []fieldError{{Field: valErr.Field, Reason: valErr.Reason}})
			return
		}
		if errors.Is(err, board.ErrBoardNotFound) {
			writeProblem(w, http.StatusNotFound, "not_found", "board not found")
			return
		}
		writeProblem(w, http.StatusInternalServerError, "internal_error", "failed to update board")
		return
	}

	writeJSON(w, http.StatusOK, BoardResponse{Data: newBoardDTO(updated)})
}

func (a *API) deleteBoard(w http.ResponseWriter, r *http.Request) {
	claims, ok := UserFromContext(r.Context())
	if !ok {
		writeProblem(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}

	boardID := chi.URLParam(r, "board_public_id")
	err := a.boards.Delete(r.Context(), claims.UserID, boardID)
	if err != nil {
		if errors.Is(err, board.ErrBoardNotFound) {
			writeProblem(w, http.StatusNotFound, "not_found", "board not found")
			return
		}
		writeProblem(w, http.StatusInternalServerError, "internal_error", "failed to delete board")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (a *API) listBoardTasks(w http.ResponseWriter, r *http.Request) {
	claims, ok := UserFromContext(r.Context())
	if !ok {
		writeProblem(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}

	boardID := chi.URLParam(r, "board_public_id")
	query := r.URL.Query()
	status := task.Status(query.Get("status"))
	limit := 20
	if rawLimit := query.Get("limit"); rawLimit != "" {
		parsed, err := strconv.Atoi(rawLimit)
		if err != nil {
			writeValidationErrors(w, []fieldError{{Field: "limit", Reason: "must be an integer"}})
			return
		}
		limit = parsed
	}
	cursor := query.Get("cursor")

	page, err := a.service.ListTasksByBoard(r.Context(), claims.UserID, boardID, status, limit, cursor)
	if err != nil {
		var valErr *task.ValidationError
		if errors.As(err, &valErr) {
			writeValidationErrors(w, []fieldError{{Field: valErr.Field, Reason: valErr.Reason}})
			return
		}
		if errors.Is(err, task.ErrInvalidCursor) {
			writeProblem(w, http.StatusBadRequest, "invalid_cursor", "cursor is invalid or does not match query parameters")
			return
		}
		if errors.Is(err, task.ErrBoardNotFound) {
			writeProblem(w, http.StatusNotFound, "not_found", "board not found")
			return
		}
		writeProblem(w, http.StatusInternalServerError, "internal_error", "failed to list tasks")
		return
	}

	dtos := make([]TaskDTO, 0, len(page.Tasks))
	for _, t := range page.Tasks {
		dtos = append(dtos, newTaskDTO(t))
	}

	writeJSON(w, http.StatusOK, ListTasksResponse{
		Data: dtos,
		Pagination: PaginationDTO{
			Limit:      page.Limit,
			Count:      len(page.Tasks),
			HasMore:    page.HasMore,
			NextCursor: page.NextCursor,
		},
	})
}
