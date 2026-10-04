package middleware

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

// AdminKey checks X-Admin-Key or Authorization: Bearer <key> against the configured admin secret.
func AdminKey(expected string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if expected == "" {
				writeAuthError(w, http.StatusInternalServerError, "admin API is not configured")
				return
			}
			key := r.Header.Get("X-Admin-Key")
			if key == "" {
				if plain, ok := parseBearer(r.Header.Get("Authorization")); ok {
					key = plain
				}
			}
			if key == "" || subtle.ConstantTimeCompare([]byte(key), []byte(expected)) != 1 {
				writeAuthError(w, http.StatusUnauthorized, "admin credentials required")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func parseBearer(authorization string) (string, bool) {
	const prefix = "Bearer "
	if !strings.HasPrefix(authorization, prefix) {
		return "", false
	}
	token := strings.TrimSpace(strings.TrimPrefix(authorization, prefix))
	return token, token != ""
}
