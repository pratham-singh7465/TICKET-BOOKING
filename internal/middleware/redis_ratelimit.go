package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisRateLimit applies a global token bucket shared across API instances.
// Falls back to in-memory limiter when client is nil.
func RedisRateLimit(client *redis.Client, rps float64, burst int) func(http.Handler) http.Handler {
	fallback := RateLimit(rps, burst)
	if client == nil {
		return fallback
	}
	if burst < 1 {
		burst = 1
	}

	// Token bucket state in one Redis hash: tokens + last_ts (ms).
	const script = `
local key = KEYS[1]
local rate = tonumber(ARGV[1])
local capacity = tonumber(ARGV[2])
local now = tonumber(ARGV[3])
local data = redis.call('HMGET', key, 'tokens', 'ts')
local tokens = tonumber(data[1])
local ts = tonumber(data[2])
if tokens == nil then
  tokens = capacity
  ts = now
end
local delta = math.max(0, now - ts) / 1000.0
tokens = math.min(capacity, tokens + delta * rate)
if tokens < 1 then
  redis.call('HMSET', key, 'tokens', tokens, 'ts', now)
  redis.call('EXPIRE', key, 120)
  return 0
end
tokens = tokens - 1
redis.call('HMSET', key, 'tokens', tokens, 'ts', now)
redis.call('EXPIRE', key, 120)
return 1
`

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			now := time.Now().UnixMilli()
			allowed, err := client.Eval(ctx, script, []string{"ratelimit:global"},
				strconv.FormatFloat(rps, 'f', -1, 64),
				strconv.Itoa(burst),
				strconv.FormatInt(now, 10),
			).Int()
			if err != nil {
				fallback(next).ServeHTTP(w, r)
				return
			}
			if allowed != 1 {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = w.Write([]byte(`{"error":"rate limit exceeded"}`))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
