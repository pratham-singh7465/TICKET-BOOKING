package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/pratham-singh/ticket-booking/internal/config"
	"github.com/pratham-singh/ticket-booking/internal/handler"
	"github.com/pratham-singh/ticket-booking/internal/middleware"
	"github.com/pratham-singh/ticket-booking/internal/platform/database"
	"github.com/pratham-singh/ticket-booking/internal/platform/logging"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", slog.String("error", err.Error()))
		os.Exit(1)
	}
	logger := logging.New(cfg.LogLevel)
	slog.SetDefault(logger)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := database.NewPool(ctx, cfg.Database)
	if err != nil {
		logger.Error("database connection failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer pool.Close()

	healthHandler := handler.NewHealthHandler(pool, cfg.AppName)
	router := handler.NewRouter(healthHandler, logger, cfg)

	server := &http.Server{
		Addr:              cfg.HTTP.Addr,
		Handler:           router,
		ReadHeaderTimeout: cfg.HTTP.ReadHeaderTimeout,
		ReadTimeout:       cfg.HTTP.ReadTimeout,
		WriteTimeout:      cfg.HTTP.WriteTimeout,
		IdleTimeout:       cfg.HTTP.IdleTimeout,
		MaxHeaderBytes:    1 << 20,
	}
	go func() {
		logger.Info("http server listening",
			slog.String("addr", cfg.HTTP.Addr),
			slog.String("app", cfg.AppName),
		)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("http server failed", slog.String("error", err.Error()))
			os.Exit(1)
		}
	}()

	shutdownCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	<-shutdownCtx.Done()
	logger.Info("shutdown signal received")

	graceCtx, graceCancel := context.WithTimeout(context.Background(), cfg.HTTP.ShutdownTimeout)
	defer graceCancel()

	if err := server.Shutdown(graceCtx); err != nil {
		logger.Error("graceful shutdown failed", slog.String("error", err.Error()))
	}

	middleware.ShutdownRateLimiter()

	logger.Info("server stopped")
}
