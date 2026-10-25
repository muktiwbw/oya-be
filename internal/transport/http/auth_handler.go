package httpapi

import (
	"errors"
	"net/http"

	"oya-be/internal/user"
)

const refreshCookieName = "oya_refresh"

func userDTO(u user.User) UserDTO {
	return UserDTO{PublicID: u.PublicID, Email: u.Email, DisplayName: u.DisplayName}
}

func (a *API) startGoogle(w http.ResponseWriter, r *http.Request) {
	if a.users == nil {
		writeProblem(w, http.StatusServiceUnavailable, "service_unavailable", "google sign-in is not configured")
		return
	}
	url, err := a.users.StartGoogle(r.Context())
	if err != nil {
		if errors.Is(err, user.ErrGoogleDisabled) {
			writeProblem(w, http.StatusServiceUnavailable, "service_unavailable", "google sign-in is not configured")
		} else {
			writeProblem(w, http.StatusInternalServerError, "internal_error", "unable to start google sign-in")
		}
		return
	}
	http.Redirect(w, r, url, http.StatusFound)
}

func (a *API) googleCallback(w http.ResponseWriter, r *http.Request) {
	access, refresh, _, err := a.users.CompleteGoogle(r.Context(), r.URL.Query().Get("state"), r.URL.Query().Get("code"))
	if err != nil {
		http.Redirect(w, r, a.auth.FrontendRedirectURL+"?error=access_denied", http.StatusFound)
		return
	}
	a.setRefreshCookie(w, refresh)
	_ = access
	http.Redirect(w, r, a.auth.FrontendRedirectURL, http.StatusFound)
}

func (a *API) refresh(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(refreshCookieName)
	if err != nil {
		writeProblem(w, http.StatusUnauthorized, "unauthorized", "invalid session")
		return
	}
	access, refresh, u, err := a.users.Refresh(r.Context(), cookie.Value)
	if err != nil {
		a.clearRefreshCookie(w)
		writeProblem(w, http.StatusUnauthorized, "unauthorized", "invalid session")
		return
	}
	a.setRefreshCookie(w, refresh)
	writeJSON(w, http.StatusOK, AuthResponse{Data: AuthResponseData{Token: access, TokenType: "Bearer", User: userDTO(u)}})
}

func (a *API) logout(w http.ResponseWriter, r *http.Request) {
	claims, ok := UserFromContext(r.Context())
	if !ok {
		writeProblem(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}
	cookie, err := r.Cookie(refreshCookieName)
	if err != nil {
		writeProblem(w, http.StatusUnauthorized, "unauthorized", "invalid session")
		return
	}
	if err := a.users.Logout(r.Context(), claims.UserID, cookie.Value); err != nil {
		writeProblem(w, http.StatusUnauthorized, "unauthorized", "invalid session")
		return
	}
	a.clearRefreshCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) me(w http.ResponseWriter, r *http.Request) {
	claims, ok := UserFromContext(r.Context())
	if !ok {
		writeProblem(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}
	u, err := a.users.GetUser(r.Context(), claims.UserID)
	if err != nil {
		writeProblem(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}
	writeJSON(w, http.StatusOK, UserResponse{Data: userDTO(u)})
}

func (a *API) setRefreshCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{Name: refreshCookieName, Value: token, Path: "/", HttpOnly: true, Secure: a.auth.RefreshCookieSecure, SameSite: http.SameSiteLaxMode, MaxAge: 60 * 60 * 24 * 30})
}
func (a *API) clearRefreshCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: refreshCookieName, Value: "", Path: "/", HttpOnly: true, Secure: a.auth.RefreshCookieSecure, SameSite: http.SameSiteLaxMode, MaxAge: -1})
}
