package database

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
)

type ExecDB interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
}

func ExpireExpiredHolds(ctx context.Context, db ExecDB) error {
	const releaseSeats = `
		UPDATE seats
		SET status = 'available',
		    user_id = NULL,
		    reservation_id = NULL,
		    held_until = NULL,
		    version = version + 1
		WHERE status = 'held'
		  AND held_until IS NOT NULL
		  AND held_until < now()
	`
	if _, err := db.Exec(ctx, releaseSeats); err != nil {
		return fmt.Errorf("expire holds: %w", err)
	}
	const cancelRes = `
		UPDATE reservations r
		SET status = 'cancelled'
		WHERE r.status = 'pending'
		  AND NOT EXISTS (
		    SELECT 1 FROM seats s
		    WHERE s.reservation_id = r.id AND s.status = 'held'
		  )
	`
	if _, err := db.Exec(ctx, cancelRes); err != nil {
		return fmt.Errorf("cancel expired reservations: %w", err)
	}
	return nil
}
