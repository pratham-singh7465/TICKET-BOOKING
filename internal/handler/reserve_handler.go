package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/pratham-singh/ticket-booking/internal/auth"
	redisstore "github.com/pratham-singh/ticket-booking/internal/platform/redis"
	"github.com/pratham-singh/ticket-booking/internal/reservation"
	"github.com/pratham-singh/ticket-booking/internal/show"
)

const idempotencyScopeReserve = "reserve"

type ReserveHandler struct {
	svc  *reservation.Service
	idem *redisstore.IdempotencyStore
}

func NewReserveHandler(svc *reservation.Service, idem *redisstore.IdempotencyStore) *ReserveHandler {
	return &ReserveHandler{svc: svc, idem: idem}
}

type reserveRequest struct {
	Seats          []string `json:"seats"`
	IdempotencyKey string   `json:"idempotency_key"`
}

type reserveResponse struct {
	ReservationID  string     `json:"reservation_id"`
	ShowID         string     `json:"show_id"`
	UserID         string     `json:"user_id"`
	Seats          []string   `json:"seats"`
	AmountPaise    int64      `json:"amount_paise"`
	Status         string     `json:"status"`
	IdempotencyKey string     `json:"idempotency_key"`
	HeldUntil      *time.Time `json:"held_until,omitempty"`
}

func (h *ReserveHandler) Reserve(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	showID, err := uuid.Parse(chi.URLParam(r, "showID"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid show id"})
		return
	}

	var req reserveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}

	idempotencyKey := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if idempotencyKey == "" {
		idempotencyKey = strings.TrimSpace(req.IdempotencyKey)
	}

	seatCodes, err := show.NormalizeSeatCodes(req.Seats)
	if err != nil {
		writeReserveError(w, fmt.Errorf("%w: %v", reservation.ErrInvalidSeats, err))
		return
	}
	requestKey := reservation.RequestKey(showID, userID, seatCodes)

	if h.idem != nil && idempotencyKey != "" {
		status, body, cacheErr := h.idem.Get(r.Context(), idempotencyScopeReserve, userID, idempotencyKey, requestKey)
		if cacheErr != nil {
			if errors.Is(cacheErr, redisstore.ErrIdempotencyMismatch) {
				writeReserveError(w, reservation.ErrIdempotencyConflict)
				return
			}
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "idempotency cache error"})
			return
		}
		if status != 0 && len(body) > 0 {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-Idempotency-Replay", "true")
			w.WriteHeader(status)
			_, _ = w.Write(body)
			return
		}
	}

	result, err := h.svc.Reserve(r.Context(), reservation.ReserveInput{
		ShowID:         showID,
		UserID:         userID,
		Seats:          seatCodes,
		IdempotencyKey: idempotencyKey,
	})
	if err != nil {
		writeReserveError(w, err)
		return
	}

	resp := toReserveResponse(result)
	if h.idem != nil && idempotencyKey != "" {
		payload, _ := json.Marshal(resp)
		_ = h.idem.Set(r.Context(), idempotencyScopeReserve, userID, idempotencyKey, requestKey, http.StatusCreated, payload)
	}
	writeJSON(w, http.StatusCreated, resp)
}

func (h *ReserveHandler) Confirm(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	reservationID, err := uuid.Parse(chi.URLParam(r, "reservationID"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid reservation id"})
		return
	}

	result, err := h.svc.Confirm(r.Context(), reservationID, userID)
	if err != nil {
		writeReserveError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toReserveResponse(result))
}

func (h *ReserveHandler) Cancel(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	reservationID, err := uuid.Parse(chi.URLParam(r, "reservationID"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid reservation id"})
		return
	}

	if err := h.svc.Cancel(r.Context(), reservationID, userID); err != nil {
		writeReserveError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func toReserveResponse(result reservation.Result) reserveResponse {
	return reserveResponse{
		ReservationID:  result.ID.String(),
		ShowID:         result.ShowID.String(),
		UserID:         result.UserID,
		Seats:          result.Seats,
		AmountPaise:    result.AmountPaise,
		Status:         result.Status,
		IdempotencyKey: result.IdempotencyKey,
		HeldUntil:      result.HeldUntil,
	}
}

func writeReserveError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, reservation.ErrIdempotencyKeyEmpty),
		errors.Is(err, reservation.ErrInvalidSeats):
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
	case errors.Is(err, reservation.ErrShowNotFound),
		errors.Is(err, reservation.ErrReservationNotFound):
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
	case errors.Is(err, reservation.ErrNotOwner):
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
	case errors.Is(err, reservation.ErrSeatUnavailable):
		writeJSON(w, http.StatusConflict, map[string]string{
			"error":  "seat_unavailable",
			"detail": err.Error(),
		})
	case errors.Is(err, reservation.ErrPerUserLimit):
		writeJSON(w, http.StatusConflict, map[string]string{
			"error":  "per_user_limit",
			"detail": err.Error(),
		})
	case errors.Is(err, reservation.ErrIdempotencyConflict):
		writeJSON(w, http.StatusConflict, map[string]string{
			"error":  "idempotency_conflict",
			"detail": err.Error(),
		})
	case errors.Is(err, reservation.ErrInvalidReservationState),
		errors.Is(err, reservation.ErrCannotCancelConfirmed):
		writeJSON(w, http.StatusConflict, map[string]string{
			"error":  "invalid_state",
			"detail": err.Error(),
		})
	default:
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
	}
}
