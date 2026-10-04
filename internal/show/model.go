package show

import (
	"time"

	"github.com/google/uuid"
)

type SeatStatus string

const SeatStatusAvailable SeatStatus = "available"

type Show struct {
	ID             uuid.UUID
	Name           string
	PricePaise     int64
	PerUserLimit   int
	CreatedAt      time.Time
	Seats          []Seat
}

type Seat struct {
	Code   string
	Status SeatStatus
}
