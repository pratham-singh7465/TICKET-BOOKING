package reservation

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/pratham-singh/ticket-booking/internal/show"
)

type Service struct {
	repo             *Repository
	paymentSimDelay  time.Duration
}

func NewService(repo *Repository, paymentSimDelay time.Duration) *Service {
	return &Service{repo: repo, paymentSimDelay: paymentSimDelay}
}

type ReserveInput struct {
	ShowID         uuid.UUID
	UserID         string
	Seats          []string
	IdempotencyKey string
}

func (s *Service) Reserve(ctx context.Context, in ReserveInput) (Result, error) {
	key := strings.TrimSpace(in.IdempotencyKey)
	if key == "" {
		return Result{}, ErrIdempotencyKeyEmpty
	}
	if strings.TrimSpace(in.UserID) == "" {
		return Result{}, fmt.Errorf("user id is required")
	}

	seatCodes, err := show.NormalizeSeatCodes(in.Seats)
	if err != nil {
		return Result{}, fmt.Errorf("%w: %v", ErrInvalidSeats, err)
	}

	reqKey := RequestKey(in.ShowID, in.UserID, seatCodes)
	return s.repo.Reserve(ctx, in.ShowID, in.UserID, seatCodes, key, reqKey)
}

// Confirm simulates payment success after the hold was placed (optional PAYMENT_SIM_DELAY).
func (s *Service) Confirm(ctx context.Context, reservationID uuid.UUID, userID string) (Result, error) {
	if s.paymentSimDelay > 0 {
		// Payment gateway latency happens while seats stay held in Postgres (held_until).
		time.Sleep(s.paymentSimDelay)
	}
	return s.repo.Confirm(ctx, reservationID, userID)
}

func (s *Service) Cancel(ctx context.Context, reservationID uuid.UUID, userID string) error {
	return s.repo.Cancel(ctx, reservationID, userID)
}
