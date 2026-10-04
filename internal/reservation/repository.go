package reservation

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pratham-singh/ticket-booking/internal/platform/database"
)

type Repository struct {
	pool    *pgxpool.Pool
	holdTTL time.Duration
}

func NewRepository(pool *pgxpool.Pool, holdTTL time.Duration) *Repository {
	if holdTTL <= 0 {
		holdTTL = 5 * time.Minute
	}
	return &Repository{pool: pool, holdTTL: holdTTL}
}

// Reserve atomically places a time-boxed hold (payment window). Seats are not sold until Confirm.
func (r *Repository) Reserve(
	ctx context.Context,
	showID uuid.UUID,
	userID string,
	seatCodes []string,
	idempotencyKey string,
	requestKey string,
) (Result, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Result{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := database.ExpireExpiredHolds(ctx, tx); err != nil {
		return Result{}, err
	}

	existing, err := loadByIdempotencyKey(ctx, tx, idempotencyKey)
	if err != nil {
		return Result{}, err
	}
	if existing != nil {
		if existing.requestKey != requestKey {
			return Result{}, ErrIdempotencyConflict
		}
		if err := tx.Commit(ctx); err != nil {
			return Result{}, err
		}
		return existing.result, nil
	}

	var pricePaise int64
	var perUserLimit int
	const showQ = `SELECT price_paise, per_user_limit FROM shows WHERE id = $1`
	err = tx.QueryRow(ctx, showQ, showID).Scan(&pricePaise, &perUserLimit)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Result{}, ErrShowNotFound
		}
		return Result{}, fmt.Errorf("load show: %w", err)
	}

	const lockSeats = `
		SELECT seat_code, status::text
		FROM seats
		WHERE show_id = $1 AND seat_code = ANY($2::text[])
		ORDER BY seat_code
		FOR UPDATE
	`
	rows, err := tx.Query(ctx, lockSeats, showID, seatCodes)
	if err != nil {
		return Result{}, fmt.Errorf("lock seats: %w", err)
	}
	defer rows.Close()

	found := make(map[string]string, len(seatCodes))
	for rows.Next() {
		var code, status string
		if err := rows.Scan(&code, &status); err != nil {
			return Result{}, err
		}
		found[code] = status
	}
	if err := rows.Err(); err != nil {
		return Result{}, err
	}
	if len(found) != len(seatCodes) {
		return Result{}, ErrInvalidSeats
	}
	for _, code := range seatCodes {
		if found[code] != "available" {
			return Result{}, ErrSeatUnavailable
		}
	}

	const countUserSeats = `
		SELECT COUNT(*)
		FROM seats
		WHERE show_id = $1
		  AND user_id = $2
		  AND status IN ('held', 'confirmed')
	`
	var active int
	if err := tx.QueryRow(ctx, countUserSeats, showID, userID).Scan(&active); err != nil {
		return Result{}, fmt.Errorf("count user seats: %w", err)
	}
	if active+len(seatCodes) > perUserLimit {
		return Result{}, ErrPerUserLimit
	}

	amount := pricePaise * int64(len(seatCodes))
	reservationID := uuid.New()
	heldUntil := time.Now().Add(r.holdTTL)

	const insertRes = `
		INSERT INTO reservations (id, show_id, user_id, amount_paise, status, idempotency_key, request_key)
		VALUES ($1, $2, $3, $4, 'pending', $5, $6)
	`
	if _, err = tx.Exec(ctx, insertRes, reservationID, showID, userID, amount, idempotencyKey, requestKey); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			again, err := loadByIdempotencyKey(ctx, tx, idempotencyKey)
			if err != nil {
				return Result{}, err
			}
			if again == nil {
				return Result{}, fmt.Errorf("insert reservation: %w", err)
			}
			if again.requestKey != requestKey {
				return Result{}, ErrIdempotencyConflict
			}
			if err := tx.Commit(ctx); err != nil {
				return Result{}, err
			}
			return again.result, nil
		}
		return Result{}, fmt.Errorf("insert reservation: %w", err)
	}

	const updateSeats = `
		UPDATE seats
		SET status = 'held',
		    user_id = $3,
		    reservation_id = $4,
		    held_until = $5,
		    version = version + 1
		WHERE show_id = $1
		  AND seat_code = ANY($2::text[])
		  AND status = 'available'
	`
	tag, err := tx.Exec(ctx, updateSeats, showID, seatCodes, userID, reservationID, heldUntil)
	if err != nil {
		return Result{}, fmt.Errorf("update seats: %w", err)
	}
	if tag.RowsAffected() != int64(len(seatCodes)) {
		return Result{}, ErrSeatUnavailable
	}

	const insertResSeats = `
		INSERT INTO reservation_seats (reservation_id, seat_code)
		SELECT $1, code FROM unnest($2::text[]) AS code
	`
	if _, err = tx.Exec(ctx, insertResSeats, reservationID, seatCodes); err != nil {
		return Result{}, fmt.Errorf("insert reservation_seats: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return Result{}, fmt.Errorf("commit: %w", err)
	}

	return Result{
		ID:             reservationID,
		ShowID:         showID,
		UserID:         userID,
		Seats:          seatCodes,
		AmountPaise:    amount,
		Status:         "held",
		IdempotencyKey: idempotencyKey,
		HeldUntil:      &heldUntil,
	}, nil
}

// Confirm finalizes payment: pending reservation + held seats → booked / confirmed.
func (r *Repository) Confirm(ctx context.Context, reservationID uuid.UUID, userID string) (Result, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Result{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := database.ExpireExpiredHolds(ctx, tx); err != nil {
		return Result{}, err
	}

	res, err := lockReservation(ctx, tx, reservationID, userID)
	if err != nil {
		return Result{}, err
	}
	if res.dbStatus == "booked" {
		if err := tx.Commit(ctx); err != nil {
			return Result{}, err
		}
		return res.result, nil
	}
	if res.dbStatus != "pending" {
		return Result{}, ErrInvalidReservationState
	}

	const confirmRes = `
		UPDATE reservations
		SET status = 'booked'
		WHERE id = $1 AND user_id = $2 AND status = 'pending'
	`
	tag, err := tx.Exec(ctx, confirmRes, reservationID, userID)
	if err != nil {
		return Result{}, fmt.Errorf("confirm reservation: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return Result{}, ErrInvalidReservationState
	}

	const confirmSeats = `
		UPDATE seats
		SET status = 'confirmed',
		    held_until = NULL,
		    version = version + 1
		WHERE reservation_id = $1
		  AND user_id = $2
		  AND status = 'held'
	`
	tag, err = tx.Exec(ctx, confirmSeats, reservationID, userID)
	if err != nil {
		return Result{}, fmt.Errorf("confirm seats: %w", err)
	}
	if tag.RowsAffected() != int64(len(res.result.Seats)) {
		return Result{}, ErrSeatUnavailable
	}

	res.result.Status = "confirmed"
	res.result.HeldUntil = nil
	if err := tx.Commit(ctx); err != nil {
		return Result{}, err
	}
	return res.result, nil
}

// Cancel releases a pending hold for the owning user only.
func (r *Repository) Cancel(ctx context.Context, reservationID uuid.UUID, userID string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := database.ExpireExpiredHolds(ctx, tx); err != nil {
		return err
	}

	res, err := lockReservation(ctx, tx, reservationID, userID)
	if err != nil {
		return err
	}
	if res.dbStatus == "cancelled" {
		return tx.Commit(ctx)
	}
	if res.dbStatus == "booked" {
		return ErrCannotCancelConfirmed
	}
	if res.dbStatus != "pending" {
		return ErrInvalidReservationState
	}

	const cancelRes = `
		UPDATE reservations
		SET status = 'cancelled'
		WHERE id = $1 AND user_id = $2 AND status = 'pending'
	`
	if _, err := tx.Exec(ctx, cancelRes, reservationID, userID); err != nil {
		return err
	}

	const releaseSeats = `
		UPDATE seats
		SET status = 'available',
		    user_id = NULL,
		    reservation_id = NULL,
		    held_until = NULL,
		    version = version + 1
		WHERE reservation_id = $1
		  AND user_id = $2
		  AND status = 'held'
	`
	tag, err := tx.Exec(ctx, releaseSeats, reservationID, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != int64(len(res.result.Seats)) {
		return ErrInvalidReservationState
	}
	return tx.Commit(ctx)
}

type lockedReservation struct {
	result   Result
	dbStatus string
}

func lockReservation(ctx context.Context, tx pgx.Tx, reservationID uuid.UUID, userID string) (lockedReservation, error) {
	const q2 = `
		SELECT r.user_id, r.status::text, r.show_id, r.amount_paise, r.idempotency_key,
		       COALESCE(array_agg(rs.seat_code ORDER BY rs.seat_code) FILTER (WHERE rs.seat_code IS NOT NULL), '{}'),
		       MAX(s.held_until)
		FROM reservations r
		LEFT JOIN reservation_seats rs ON rs.reservation_id = r.id
		LEFT JOIN seats s ON s.reservation_id = r.id AND s.seat_code = rs.seat_code
		WHERE r.id = $1
		GROUP BY r.id, r.user_id, r.status, r.show_id, r.amount_paise, r.idempotency_key
		FOR UPDATE OF r
	`
	var owner string
	var dbStatus string
	var showID uuid.UUID
	var amount int64
	var idemKey string
	var seats []string
	var heldUntil *time.Time

	err := tx.QueryRow(ctx, q2, reservationID).Scan(
		&owner, &dbStatus, &showID, &amount, &idemKey, &seats, &heldUntil,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return lockedReservation{}, ErrReservationNotFound
	}
	if err != nil {
		return lockedReservation{}, fmt.Errorf("lock reservation: %w", err)
	}
	if owner != userID {
		return lockedReservation{}, ErrNotOwner
	}

	apiStatus := mapDBStatusToAPI(dbStatus)
	return lockedReservation{
		dbStatus: dbStatus,
		result: Result{
			ID:             reservationID,
			ShowID:         showID,
			UserID:         userID,
			Seats:          seats,
			AmountPaise:    amount,
			Status:         apiStatus,
			IdempotencyKey: idemKey,
			HeldUntil:      heldUntil,
		},
	}, nil
}

func mapDBStatusToAPI(dbStatus string) string {
	switch dbStatus {
	case "booked":
		return "confirmed"
	case "pending":
		return "held"
	default:
		return dbStatus
	}
}

type storedReservation struct {
	result     Result
	requestKey string
}

func loadByIdempotencyKey(ctx context.Context, tx pgx.Tx, idempotencyKey string) (*storedReservation, error) {
	const q = `
		SELECT r.id, r.show_id, r.user_id, r.amount_paise, r.request_key, r.status::text,
		       COALESCE(array_agg(rs.seat_code ORDER BY rs.seat_code) FILTER (WHERE rs.seat_code IS NOT NULL), '{}'),
		       MAX(s.held_until)
		FROM reservations r
		LEFT JOIN reservation_seats rs ON rs.reservation_id = r.id
		LEFT JOIN seats s ON s.reservation_id = r.id AND s.seat_code = rs.seat_code
		WHERE r.idempotency_key = $1 AND r.status IN ('pending', 'booked')
		GROUP BY r.id, r.show_id, r.user_id, r.amount_paise, r.request_key, r.status
	`
	var seatCodes []string
	var res Result
	var reqKey, dbStatus string
	var heldUntil *time.Time
	err := tx.QueryRow(ctx, q, idempotencyKey).Scan(
		&res.ID, &res.ShowID, &res.UserID, &res.AmountPaise, &reqKey, &dbStatus, &seatCodes, &heldUntil,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("load idempotent reservation: %w", err)
	}
	res.Seats = seatCodes
	res.Status = mapDBStatusToAPI(dbStatus)
	res.IdempotencyKey = idempotencyKey
	res.HeldUntil = heldUntil
	return &storedReservation{result: res, requestKey: reqKey}, nil
}
