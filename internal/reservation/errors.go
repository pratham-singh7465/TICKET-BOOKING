package reservation

import "errors"

var (
	ErrShowNotFound            = errors.New("show not found")
	ErrSeatUnavailable         = errors.New("one or more seats are not available")
	ErrPerUserLimit            = errors.New("per-user seat limit exceeded")
	ErrIdempotencyConflict     = errors.New("idempotency key reused with different request")
	ErrInvalidSeats            = errors.New("invalid seat selection")
	ErrIdempotencyKeyEmpty     = errors.New("idempotency_key is required")
	ErrReservationNotFound     = errors.New("reservation not found")
	ErrNotOwner                = errors.New("not reservation owner")
	ErrInvalidReservationState = errors.New("reservation is not in a valid state for this action")
	ErrCannotCancelConfirmed   = errors.New("cannot cancel a confirmed reservation")
)
