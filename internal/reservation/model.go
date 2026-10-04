package reservation

import (
	"time"

	"github.com/google/uuid"
)

type Result struct {
	ID             uuid.UUID
	ShowID         uuid.UUID
	UserID         string
	Seats          []string
	AmountPaise    int64
	Status         string // held | confirmed
	IdempotencyKey string
	HeldUntil      *time.Time
}
