package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	AppName     string
	LogLevel    string
	AdminAPIKey string
	HTTP        HttpConfig
	Database    DatabaseConfig
	Reservation ReservationConfig
	Redis       RedisConfig
}

type RedisConfig struct {
	URL            string
	IdempotencyTTL time.Duration
}

type ReservationConfig struct {
	HoldTTL          time.Duration
	PaymentSimDelay  time.Duration
}

type HttpConfig struct {
	Addr              string
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	ShutdownTimeout   time.Duration
	RateLimitRPS      float64
	RateLimitBurst    int
}

type DatabaseConfig struct {
	URL             string
	MaxConns        int32
	MinConns        int32
	MaxConnLifetime time.Duration
	MaxConnIdleTime time.Duration
	HealthCheck     time.Duration
}

// loadDotEnv sets env vars from .env when not already set (local dev; Compose still uses real env).
func loadDotEnv() {
	data, err := os.ReadFile(".env")
	if err != nil {
		return
	}
	for line := range strings.SplitSeq(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		if key != "" && os.Getenv(key) == "" {
			_ = os.Setenv(key, val)
		}
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func durationEnv(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return fallback
	}
	return d
}

func intEnv(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func int32Env(key string, fallback int32) int32 {
	return int32(intEnv(key, int(fallback)))
}

func floatEnv(key string, fallback float64) float64 {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return fallback
	}
	return f
}

func Load() (Config, error) {
	loadDotEnv()

	cfg := Config{
		AppName:     envOr("APP_NAME", "ticket-booking"),
		LogLevel:    envOr("LOG_LEVEL", "info"),
		AdminAPIKey: envOr("ADMIN_API_KEY", ""),
		HTTP: HttpConfig{
			Addr:              envOr("HTTP_ADDR", ":8080"),
			ReadHeaderTimeout: durationEnv("HTTP_READ_HEADER_TIMEOUT", 5*time.Second),
			ReadTimeout:       durationEnv("HTTP_READ_TIMEOUT", 15*time.Second),
			WriteTimeout:      durationEnv("HTTP_WRITE_TIMEOUT", 15*time.Second),
			IdleTimeout:       durationEnv("HTTP_IDLE_TIMEOUT", 60*time.Second),
			ShutdownTimeout:   durationEnv("HTTP_SHUTDOWN_TIMEOUT", 20*time.Second),
			RateLimitRPS:      floatEnv("RATE_LIMIT_RPS", 200),
			RateLimitBurst:    intEnv("RATE_LIMIT_BURST", 400),
		},
		Database: DatabaseConfig{
			URL:             envOr("DB_URL", "postgres://app:app@localhost:5435/appdb?sslmode=disable"),
			MaxConns:        int32Env("DB_MAX_CONNS", 50),
			MinConns:        int32Env("DB_MIN_CONNS", 10),
			MaxConnLifetime: durationEnv("DB_MAX_CONN_LIFETIME", 30*time.Minute),
			MaxConnIdleTime: durationEnv("DB_MAX_CONN_IDLE_TIME", 5*time.Minute),
			HealthCheck:     durationEnv("DB_HEALTH_CHECK_PERIOD", time.Minute),
		},
		Reservation: ReservationConfig{
			HoldTTL:         durationEnv("HOLD_TTL", 5*time.Minute),
			PaymentSimDelay: durationEnv("PAYMENT_SIM_DELAY", 0),
		},
		Redis: RedisConfig{
			URL:            envOr("REDIS_URL", ""),
			IdempotencyTTL: durationEnv("IDEMPOTENCY_CACHE_TTL", 24*time.Hour),
		},
	}

	if cfg.Database.URL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}
	if cfg.HTTP.RateLimitRPS <= 0 {
		return Config{}, fmt.Errorf("RATE_LIMIT_RPS must be positive")
	}

	return cfg, nil
}
