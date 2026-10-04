package config

import (
	"os"
	"time"
)

type Config struct {
	AppName  string
	LogLevel string
	HTTP     HttpConfig
	Database DatabaseConfig
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
	MaxConnLifeTime time.Duration
	MaxConnIdleTime time.Duration
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
