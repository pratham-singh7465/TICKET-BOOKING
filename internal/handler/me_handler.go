package handler

import (
	"net/http"

	"github.com/pratham-singh/ticket-booking/internal/auth"
)

type MeHandler struct{}

func NewMeHandler() *MeHandler {
	return &MeHandler{}
}

// Me proves token-derived identity; booking handlers must use auth.UserIDFromContext only.
func (h *MeHandler) Me(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"user_id": userID})
}
