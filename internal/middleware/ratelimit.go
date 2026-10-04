package middleware

import (
	"net/http"
	"sync"
	"time"
)

type tokenBucket struct {
	mu       sync.Mutex
	rate     float64
	burst    float64
	tokens   float64
	lastFill time.Time
}

func newTokenBucket(rps float64, burst int) *tokenBucket {
	if burst < 1 {
		burst = 1
	}
	return &tokenBucket{
		rate:     rps,
		burst:    float64(burst),
		tokens:   float64(burst),
		lastFill: time.Now(),
	}
}

func (b *tokenBucket) allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(b.lastFill).Seconds()
	b.lastFill = now
	b.tokens += elapsed * b.rate
	if b.tokens > b.burst {
		b.tokens = b.burst
	}
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

var (
	limiterMu sync.Mutex
	limiter   *tokenBucket
)

func RateLimit(rps float64, burst int) func(http.Handler) http.Handler {
	limiterMu.Lock()
	limiter = newTokenBucket(rps, burst)
	limiterMu.Unlock()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			limiterMu.Lock()
			lim := limiter
			limiterMu.Unlock()
			if lim != nil && !lim.allow() {
				http.Error(w, http.StatusText(http.StatusTooManyRequests), http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func ShutdownRateLimiter() {
	limiterMu.Lock()
	limiter = nil
	limiterMu.Unlock()
}
