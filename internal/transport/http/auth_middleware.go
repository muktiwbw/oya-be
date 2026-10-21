package httpapi

import (
	"context"
	"net/http"
	"strings"

	"oya-be/internal/platform/auth"
)

type userContextKey struct{}

func ContextWithUser(ctx context.Context, claims *auth.Claims) context.Context {
	return context.WithValue(ctx, userContextKey{}, claims)
}

func UserFromContext(ctx context.Context) (*auth.Claims, bool) {
	claims, ok := ctx.Value(userContextKey{}).(*auth.Claims)
	return claims, ok
}

func (a *API) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			writeProblem(w, http.StatusUnauthorized, "unauthorized", "missing authorization header")
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
			writeProblem(w, http.StatusUnauthorized, "unauthorized", "invalid authorization header format")
			return
		}

		claims, err := a.tokens.Validate(parts[1])
		if err != nil {
			writeProblem(w, http.StatusUnauthorized, "unauthorized", "invalid or expired token")
			return
		}

		ctx := ContextWithUser(r.Context(), claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
