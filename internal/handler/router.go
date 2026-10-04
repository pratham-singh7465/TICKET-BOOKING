package handler

import (
	"log/slog"
	"net/http"

	"github.com/pratham-singh/ticket-booking/internal/config"
	"github.com/pratham-singh/ticket-booking/internal/middleware"
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func NewRouter(
	health *HealthHandler,
	logger *slog.Logger,
	cfg config.Config,
) http.Handler {
	r := chi.NewRouter()

	r.Use(chimiddleware.RealIP)
	r.Use(chimiddleware.RequestID)
	r.Use(middleware.RequestLogger(logger))
	r.Use(middleware.Recoverer(logger))
	r.Use(middleware.Metrics)
	r.Use(middleware.RateLimit(cfg.HTTP.RateLimitRPS, cfg.HTTP.RateLimitBurst))

	r.Get("/healthz", health.Liveness)
	r.Get("/readyz", health.Readiness)
	r.Handle("/metrics", promhttp.Handler())

	return r
}
