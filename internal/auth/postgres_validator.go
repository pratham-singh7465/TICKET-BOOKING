package auth

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrInvalidToken = errors.New("invalid token")

type PostgresValidator struct {
	pool *pgxpool.Pool
}

func NewPostgresValidator(pool *pgxpool.Pool) *PostgresValidator {
	return &PostgresValidator{pool: pool}
}

func (v *PostgresValidator) ResolveUserID(ctx context.Context, plainToken string) (string, error) {
	hash := HashToken(plainToken)
	const q = `
		SELECT user_id
		FROM api_tokens
		WHERE token_hash = $1
		  AND revoked_at IS NULL
	`
	var userID string
	err := v.pool.QueryRow(ctx, q, hash).Scan(&userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrInvalidToken
		}
		return "", fmt.Errorf("lookup api token: %w", err)
	}
	return userID, nil
}
