package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"oya-be/internal/user"
)

func (a *API) login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, a.bodyLimit)).Decode(&req); err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_json", "malformed JSON payload")
		return
	}

	if req.Username == "" || req.Password == "" {
		writeValidationErrors(w, []fieldError{
			{Field: "username", Reason: "username and password are required"},
		})
		return
	}

	token, u, err := a.users.Login(r.Context(), req.Username, req.Password)
	if err != nil {
		if errors.Is(err, user.ErrInvalidCredentials) {
			writeProblem(w, http.StatusUnauthorized, "invalid_credentials", "invalid username or password")
			return
		}
		writeProblem(w, http.StatusInternalServerError, "internal_error", "an unexpected error occurred")
		return
	}

	writeJSON(w, http.StatusOK, LoginResponse{
		Data: LoginResponseData{
			Token:     token,
			TokenType: "Bearer",
			User: UserDTO{
				PublicID:    u.PublicID,
				Email:       u.Email,
				Username:    u.Username,
				DisplayName: u.DisplayName,
			},
		},
	})
}
