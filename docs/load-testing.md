# Load testing on restricted resources

## 1. Start stack with CPU/RAM caps

Default stack:

```bash
docker compose up -d --build
```

Restricted overlay (example: 0.5 CPU / 256Mi API, 1 CPU / 512Mi Postgres):

```bash
export COMPOSE_COMPATIBILITY=1   # if deploy.limits are ignored, set this (Docker Compose)
docker compose -f docker-compose.yml -f docker-compose.limits.yml up -d --build
```

Edit `docker-compose.limits.yml` to match your laptop or cloud instance size.

**Local `go run` with caps (Linux/WSL):**

```bash
systemd-run --user -p MemoryMax=256M -p CPUQuota=50% -- \
  go run ./cmd/server
```

## 2. Smoke script (hot seat + reconciliation)

```bash
chmod +x scripts/load-smoke.sh
BASE_URL=http://localhost:8080 CONCURRENCY=100 ./scripts/load-smoke.sh
```

What it checks:

- Creates a show with 5 seats.
- Expects **exactly one** `201`, rest `409`, **zero 5xx**.
- `GET /shows/{id}` → `available + held + confirmed == total`.

## 3. Idempotency retry test (manual)

```bash
KEY=test-idem-1
curl -s -X POST "$BASE_URL/api/v1/shows/$SHOW_ID/reserve" \
  -H "Authorization: Bearer token-user1" \
  -H "Idempotency-Key: $KEY" \
  -H "Content-Type: application/json" \
  -d '{"seats":["HOT2"]}' -w "\n%{http_code}\n"
# repeat same command -> 201 replay (X-Idempotency-Replay: true if Redis hit first)
```

Correctness still holds if Redis is off (Postgres idempotency only); retries may be slower.
