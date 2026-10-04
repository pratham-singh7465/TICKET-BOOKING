package middleware

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/pratham-singh/ticket-booking/internal/auth"
)

func Authenticate(validator auth.Validator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			plain, ok := auth.ParseBearerToken(r.Header.Get("Authorization"))
			if !ok {
				writeAuthError(w, http.StatusUnauthorized, "missing or invalid Authorization header")
				return
			}

			userID, err := validator.ResolveUserID(r.Context(), plain)
			if err != nil {
				if errors.Is(err, auth.ErrInvalidToken) {
					writeAuthError(w, http.StatusUnauthorized, "invalid token")
					return
				}
				writeAuthError(w, http.StatusInternalServerError, "authentication failed")
				return
			}

			next.ServeHTTP(w, r.WithContext(auth.WithUserID(r.Context(), userID)))
		})
	}
}

func writeAuthError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
}
