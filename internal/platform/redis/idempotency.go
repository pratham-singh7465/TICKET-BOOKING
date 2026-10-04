package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// IdempotencyStore caches successful mutation responses so retries skip Postgres.
type IdempotencyStore struct {
	client *redis.Client
	ttl    time.Duration
	prefix string
}

func NewIdempotencyStore(client *redis.Client, ttl time.Duration) *IdempotencyStore {
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	return &IdempotencyStore{
		client: client,
		ttl:    ttl,
		prefix: "idem",
	}
}

type cachedResponse struct {
	RequestKey string `json:"request_key"`
	Status     int    `json:"status"`
	Body       []byte `json:"body"`
}

var ErrIdempotencyMismatch = errors.New("idempotency key reused with different request")

func (s *IdempotencyStore) key(scope, userID, idempotencyKey string) string {
	return fmt.Sprintf("%s:%s:%s:%s", s.prefix, scope, userID, idempotencyKey)
}

// Get returns a cached HTTP response when the fingerprint matches.
func (s *IdempotencyStore) Get(ctx context.Context, scope, userID, idempotencyKey, requestKey string) (status int, body []byte, err error) {
	if s == nil || s.client == nil {
		return 0, nil, nil
	}
	raw, err := s.client.Get(ctx, s.key(scope, userID, idempotencyKey)).Bytes()
	if errors.Is(err, redis.Nil) {
		return 0, nil, nil
	}
	if err != nil {
		return 0, nil, fmt.Errorf("redis idempotency get: %w", err)
	}
	var cached cachedResponse
	if err := json.Unmarshal(raw, &cached); err != nil {
		return 0, nil, fmt.Errorf("decode idempotency cache: %w", err)
	}
	if cached.RequestKey != requestKey {
		return 0, nil, ErrIdempotencyMismatch
	}
	return cached.Status, cached.Body, nil
}

func (s *IdempotencyStore) Set(ctx context.Context, scope, userID, idempotencyKey, requestKey string, status int, body []byte) error {
	if s == nil || s.client == nil {
		return nil
	}
	payload, err := json.Marshal(cachedResponse{
		RequestKey: requestKey,
		Status:     status,
		Body:       body,
	})
	if err != nil {
		return err
	}
	if err := s.client.Set(ctx, s.key(scope, userID, idempotencyKey), payload, s.ttl).Err(); err != nil {
		return fmt.Errorf("redis idempotency set: %w", err)
	}
	return nil
}
