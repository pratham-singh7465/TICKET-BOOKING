# Reservation flow (production)

1. **`POST /api/v1/shows/{id}/reserve`** — atomic **hold** in Postgres (`seats.status = held`, `held_until`, `reservations.status = pending`). Response `status: held`.
2. **`POST /api/v1/reservations/{id}/confirm`** — after payment succeeds; seats → `confirmed`, reservation → `booked`. Optional `PAYMENT_SIM_DELAY` simulates PSP latency while the hold remains in the DB.
3. **`POST /api/v1/reservations/{id}/cancel`** — owner releases a pending hold; seats return to `available`.
4. **Expiry** — on each reserve/confirm/cancel transaction, expired holds (`held_until < now()`) are released; pending reservations with no held seats are cancelled.

Double-sell prevention is entirely in Postgres: `FOR UPDATE` row locks and updates guarded on `status = available` / `held`.

Configure: `HOLD_TTL` (default `5m`), `PAYMENT_SIM_DELAY` (default `0`).

## Redis

- **`REDIS_URL`** — when set, enables:
  - **Global rate limit** (`ratelimit:global` token bucket Lua) shared by all API pods.
  - **Reserve idempotency cache** — successful `201` bodies keyed by `idem:reserve:{user}:{idempotency_key}`; retries return cached JSON with header `X-Idempotency-Replay: true` without hitting Postgres when fingerprint matches.
- Postgres remains authoritative (unique `idempotency_key`, seat locks). If Redis is down, rate limit falls back to in-process bucket; idempotency falls back to DB-only.
- Env: `IDEMPOTENCY_CACHE_TTL` (default `24h`).
