package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/pratham-singh/ticket-booking/internal/show"
)

type ShowHandler struct {
	svc *show.Service
}

func NewShowHandler(svc *show.Service) *ShowHandler {
	return &ShowHandler{svc: svc}
}

type createShowRequest struct {
	Name         string   `json:"name"`
	Seats        []string `json:"seats"`
	PricePaise   int64    `json:"price_paise"`
	PerUserLimit int      `json:"per_user_limit"`
}

type seatResponse struct {
	SeatCode string `json:"seat_code"`
	Status   string `json:"status"`
}

type showResponse struct {
	ID           string         `json:"id"`
	Name         string         `json:"name"`
	PricePaise   int64          `json:"price_paise"`
	PerUserLimit int            `json:"per_user_limit"`
	Seats        []seatResponse `json:"seats"`
}

func (h *ShowHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createShowRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}

	created, err := h.svc.Create(r.Context(), show.CreateInput{
		Name:         req.Name,
		Seats:        req.Seats,
		PricePaise:   req.PricePaise,
		PerUserLimit: req.PerUserLimit,
	})
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusCreated, toShowResponse(created))
}

func toShowResponse(s show.Show) showResponse {
	seats := make([]seatResponse, len(s.Seats))
	for i, seat := range s.Seats {
		seats[i] = seatResponse{
			SeatCode: seat.Code,
			Status:   string(seat.Status),
		}
	}
	return showResponse{
		ID:           s.ID.String(),
		Name:         s.Name,
		PricePaise:   s.PricePaise,
		PerUserLimit: s.PerUserLimit,
		Seats:        seats,
	}
}

func mapShowError(w http.ResponseWriter, err error) {
	if errors.Is(err, show.ErrNotFound) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "show not found"})
		return
	}
	writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
}
