package show

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Create(ctx context.Context, name string, pricePaise int64, perUserLimit int, seatCodes []string) (Show, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Show{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	const insertShow = `
		INSERT INTO shows (name, price_paise, per_user_limit)
		VALUES ($1, $2, $3)
		RETURNING id, name, price_paise, per_user_limit, created_at
	`
	var s Show
	err = tx.QueryRow(ctx, insertShow, name, pricePaise, perUserLimit).Scan(
		&s.ID, &s.Name, &s.PricePaise, &s.PerUserLimit, &s.CreatedAt,
	)
	if err != nil {
		return Show{}, fmt.Errorf("insert show: %w", err)
	}

	const insertSeats = `
		INSERT INTO seats (show_id, seat_code, status)
		SELECT $1, code, 'available'::seat_status
		FROM unnest($2::text[]) AS code
		ORDER BY code
	`
	if _, err = tx.Exec(ctx, insertSeats, s.ID, seatCodes); err != nil {
		return Show{}, fmt.Errorf("insert seats: %w", err)
	}

	const listSeats = `
		SELECT seat_code, status::text
		FROM seats
		WHERE show_id = $1
		ORDER BY seat_code
	`
	rows, err := tx.Query(ctx, listSeats, s.ID)
	if err != nil {
		return Show{}, fmt.Errorf("list seats: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var seat Seat
		var status string
		if err := rows.Scan(&seat.Code, &status); err != nil {
			return Show{}, fmt.Errorf("scan seat: %w", err)
		}
		seat.Status = SeatStatus(status)
		s.Seats = append(s.Seats, seat)
	}
	if err := rows.Err(); err != nil {
		return Show{}, fmt.Errorf("iterate seats: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return Show{}, fmt.Errorf("commit: %w", err)
	}
	return s, nil
}

func (r *Repository) GetByID(ctx context.Context, id uuid.UUID) (Show, error) {
	const qShow = `
		SELECT id, name, price_paise, per_user_limit, created_at
		FROM shows WHERE id = $1
	`
	var s Show
	err := r.pool.QueryRow(ctx, qShow, id).Scan(
		&s.ID, &s.Name, &s.PricePaise, &s.PerUserLimit, &s.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Show{}, ErrNotFound
		}
		return Show{}, fmt.Errorf("get show: %w", err)
	}

	const qSeats = `
		SELECT seat_code, status::text
		FROM seats WHERE show_id = $1
		ORDER BY seat_code
	`
	rows, err := r.pool.Query(ctx, qSeats, id)
	if err != nil {
		return Show{}, fmt.Errorf("list seats: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var seat Seat
		var status string
		if err := rows.Scan(&seat.Code, &status); err != nil {
			return Show{}, err
		}
		seat.Status = SeatStatus(status)
		s.Seats = append(s.Seats, seat)
	}
	return s, rows.Err()
}
