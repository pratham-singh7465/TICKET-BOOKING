package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	ctx := context.Background()

	dsn := os.Getenv("DB_URL")
	if dsn == "" {
		dsn = "postgres://app:app@localhost:5432/appdb?sslmode=disable"
	}

	logger.Info("connecting", "dsn", dsn)

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		logger.Error("failed to create pool", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		logger.Error("failed to ping database", "error", err)
		os.Exit(1)
	}

	logger.Info("pong")
}
