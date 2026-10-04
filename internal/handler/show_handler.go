package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
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

type seatCountsResponse struct {
	Available int `json:"available"`
	Held      int `json:"held"`
	Confirmed int `json:"confirmed"`
	Total     int `json:"total"`
}

type showStateResponse struct {
	ID           string             `json:"id"`
	Name         string             `json:"name"`
	PricePaise   int64              `json:"price_paise"`
	PerUserLimit int                `json:"per_user_limit"`
	Counts       seatCountsResponse `json:"counts"`
	Seats        []seatResponse     `json:"seats"`
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

func (h *ShowHandler) Get(w http.ResponseWriter, r *http.Request) {
	showID, err := uuid.Parse(chi.URLParam(r, "showID"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid show id"})
		return
	}

	state, err := h.svc.GetState(r.Context(), showID)
	if err != nil {
		mapShowError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toShowStateResponse(state))
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

func toShowStateResponse(state show.ShowState) showStateResponse {
	base := toShowResponse(state.Show)
	return showStateResponse{
		ID:           base.ID,
		Name:         base.Name,
		PricePaise:   base.PricePaise,
		PerUserLimit: base.PerUserLimit,
		Counts: seatCountsResponse{
			Available: state.Counts.Available,
			Held:      state.Counts.Held,
			Confirmed: state.Counts.Confirmed,
			Total:     state.Counts.Total,
		},
		Seats: base.Seats,
	}
}

func mapShowError(w http.ResponseWriter, err error) {
	if errors.Is(err, show.ErrNotFound) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "show not found"})
		return
	}
	writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
}
