package show

import (
	"time"

	"github.com/google/uuid"
)

type SeatStatus string

const (
	SeatStatusAvailable SeatStatus = "available"
	SeatStatusHeld      SeatStatus = "held"
	SeatStatusConfirmed SeatStatus = "confirmed"
)

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

type SeatCounts struct {
	Available int
	Held      int
	Confirmed int
	Total     int
}

type ShowState struct {
	Show   Show
	Counts SeatCounts
}
