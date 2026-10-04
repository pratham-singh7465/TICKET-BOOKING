package handler

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"github.com/pratham-singh/ticket-booking/internal/auth"
	"github.com/pratham-singh/ticket-booking/internal/config"
	"github.com/pratham-singh/ticket-booking/internal/middleware"
)

func NewRouter(
	health *HealthHandler,
	me *MeHandler,
	shows *ShowHandler,
	reserve *ReserveHandler,
	tokenValidator auth.Validator,
	redisClient *redis.Client,
	logger *slog.Logger,
	cfg config.Config,
) http.Handler {
	r := chi.NewRouter()

	r.Use(chimiddleware.RealIP)
	r.Use(chimiddleware.RequestID)
	r.Use(middleware.RequestLogger(logger))
	r.Use(middleware.Recoverer(logger))
	r.Use(middleware.Metrics)
	r.Use(middleware.RedisRateLimit(redisClient, cfg.HTTP.RateLimitRPS, cfg.HTTP.RateLimitBurst))

	r.Get("/healthz", health.Liveness)
	r.Get("/readyz", health.Readiness)
	r.Handle("/metrics", promhttp.Handler())
	RegisterDocs(r)

	r.Route("/api/v1", func(r chi.Router) {
		r.With(middleware.AdminKey(cfg.AdminAPIKey)).Post("/shows", shows.Create)
		r.Get("/shows/{showID}", shows.Get)

		r.Group(func(r chi.Router) {
			r.Use(middleware.Authenticate(tokenValidator))
			r.Get("/me", me.Me)
			r.Post("/shows/{showID}/reserve", reserve.Reserve)
			r.Post("/reservations/{reservationID}/confirm", reserve.Confirm)
			r.Post("/reservations/{reservationID}/cancel", reserve.Cancel)
		})
	})

	return r
}
