# Ticket booking system(Go)

A **high-concurrency seat reservation** system: Postgres for atomicity, idempotent HTTP APIs, Docker, Prometheus metrics, and rate limiting under load.

## Current status


| Area                                        | Status  |
| ------------------------------------------- | ------- |
| HTTP server (chi router, graceful shutdown) | Done    |
| Health: liveness + readiness (DB ping)      | Done    |
| Structured request logging                  | Done    |
| Token-bucket rate limiting                  | Done    |
| Prometheus HTTP metrics                     | Done    |
| Postgres pool + schema migration (init SQL) | Done    |
| Bearer token auth (`user1`–`user5`, DB-backed) | Done |
| Admin `POST /api/v1/shows` (create show + seats) | Done |
| `POST /api/v1/shows/{id}/reserve` (atomic, idempotent) | Done |
| `GET /api/v1/shows/{id}` (seat status + counts) | Done |
| Cancel (owner)                              | Done |


## HTTP endpoints


| Method | Path       | Description                                                                             |
| ------ | ---------- | --------------------------------------------------------------------------------------- |
| `GET`  | `/healthz` | **Liveness** — process is up                                                            |
| `GET`  | `/readyz`  | **Readiness** — pings Postgres                                                          |
| `GET`  | `/metrics` | Prometheus scrape endpoint (`http_requests_total`, `http_request_duration_seconds`, …). |
| `GET`  | `/api/v1/shows/{showID}` | **Public** — per-seat status + counts (`available` / `held` / `confirmed`). |
| `POST` | `/api/v1/shows` | **Admin** — create show; header `X-Admin-Key` (see `ADMIN_API_KEY`). |
| `POST` | `/api/v1/shows/{showID}/reserve` | **User bearer** — hold seats (`status: held`); `Idempotency-Key` header or body. |
| `POST` | `/api/v1/reservations/{id}/confirm` | **Owner** — after payment → `confirmed`. |
| `POST` | `/api/v1/reservations/{id}/cancel` | **Owner** — release pending hold. |
| `GET`  | `/api/v1/me` | **Authenticated** — returns token-derived `user_id` (ignores any user field in body). |
| `GET`  | `/docs/` | **Swagger UI** — try APIs in the browser (spec at `/openapi.yaml`). |
| `GET`  | `/openapi.yaml` | OpenAPI 3 spec (source: `internal/apidocs/spec.yaml`). |

Examples (API on port `8080`):

```bash
curl -s http://localhost:8080/healthz
curl -s http://localhost:8080/readyz
curl -s http://localhost:8080/metrics | head
curl -s -H "Authorization: Bearer token-user1" http://localhost:8080/api/v1/me

curl -s -X POST http://localhost:8080/api/v1/shows \
  -H "Content-Type: application/json" \
  -H "X-Admin-Key: dev-admin-change-me" \
  -d '{"name":"friday-night","seats":["A1","A2","A3"],"price_paise":25000}'

# Reserve (use show id from create response)
curl -s -X POST "http://localhost:8080/api/v1/shows/<SHOW_ID>/reserve" \
  -H "Authorization: Bearer token-user1" \
  -H "Content-Type: application/json" \
  -H "Idempotency-Key: demo-reserve-1" \
  -d '{"seats":["A1"]}'

# After payment succeeds (use reservation_id from reserve response)
curl -s -X POST "http://localhost:8080/api/v1/reservations/<RESERVATION_ID>/confirm" \
  -H "Authorization: Bearer token-user1"
```

**Booking flow:** `reserve` → `held` (pay within `HOLD_TTL`) → `confirm` → `confirmed`; or `cancel` / wait for expiry. All-or-nothing holds; idempotent reserve (Postgres + **Redis** replay cache on retries); **Redis** global rate limit across API replicas. See [docs/reservation-concurrency.md](docs/reservation-concurrency.md).

Open **http://localhost:8080/docs/** for Swagger UI. Use **Authorize** with bearer value `token-user1`, then call `GET /api/v1/me`.

### Auth (dev users)

`migrations/002_auth.up.sql` creates `users` and `api_tokens`. With Docker Compose, migrations run **automatically** (see below). In DBeaver, refresh **public → Tables** to see `users` / `api_tokens`.

| User ID | Bearer token (dev only) |
|---------|-------------------------|
| `user1` | `token-user1` |
| `user2` | `token-user2` |
| `user3` | `token-user3` |
| `user4` | `token-user4` |
| `user5` | `token-user5` |

Send `Authorization: Bearer token-userN`. Booking handlers will take identity **only** from this token, never from JSON body fields. For Demo purpose API Token is used instead of Jwt for simpler implementation

## Project layout

```
cmd/server/              # Application entrypoint
internal/config/         # Env-based configuration
internal/handler/        # HTTP handlers and router
internal/auth/           # Token hashing, DB lookup, request context user_id
internal/middleware/     # Logging, metrics, rate limit, auth, recovery
internal/platform/       # Database pool, logging
migrations/              # SQL schema (shows, seats, reservations, …)
deploy/                  # Prometheus config
docker/                  # API image build
```



## Prerequisites

- Go 1.27+
- Docker & Docker Compose



## Configuration

Copy the example env file and adjust if needed:

```bash
cp .env.example .env
```

Important variables:


| Variable           | Default (example)                                         | Purpose                                 |
| ------------------ | --------------------------------------------------------- | --------------------------------------- |
| `HTTP_ADDR`        | `:8080`                                                   | Listen address                          |
| `DB_URL`           | `postgres://app:app@localhost:5435/appdb?sslmode=disable` | Postgres on host (Compose maps **5435→5432**; use host `db` inside Compose) |
| `LOG_LEVEL`        | `debug` / `info`                                          | slog level                              |
| `RATE_LIMIT_RPS`   | `200`                                                     | Sustained requests per second           |
| `RATE_LIMIT_BURST` | `500`                                                     | Burst size                              |
| `ADMIN_API_KEY`    | (required for create show)                                | Admin `X-Admin-Key` / Bearer for `POST /api/v1/shows` |
| `REDIS_URL`        | `redis://localhost:6379/0`                                | Distributed rate limit + reserve idempotency cache |
| `IDEMPOTENCY_CACHE_TTL` | `24h`                                              | Redis TTL for cached `201` reserve responses |




## Load testing (restricted resources)

See [docs/load-testing.md](docs/load-testing.md). Quick smoke test:

```bash
docker compose up -d --build
# optional caps: COMPOSE_COMPATIBILITY=1 docker compose -f docker-compose.yml -f docker-compose.limits.yml up -d --build
chmod +x scripts/load-smoke.sh
CONCURRENCY=100 BASE_URL=http://localhost:8080 ./scripts/load-smoke.sh
```

## Database migrations (Docker)

`docker compose up` runs a one-shot **`migrate`** service after Postgres is healthy. It applies any new `migrations/*_*.up.sql` files and records them in `schema_migrations` (safe on every up; already-applied versions are skipped).

```bash
docker compose up -d          # db → migrate → api (and prometheus)
docker compose up -d db migrate   # only DB + migrations (for local `go run`)
```

Manual helper (same logic): `./scripts/apply-migrations.sh`

## Run locally

1. Start Postgres **and migrations**:
  ```bash
   docker compose up -d db migrate
  ```
2. Run the API (loads `.env` if you export it, or set `DB_URL` explicitly):
  ```bash
   export $(grep -v '^#' .env | xargs)   # optional
   go run ./cmd/server
  ```
3. Verify health:
  ```bash
   curl -s http://localhost:8080/healthz
   curl -s http://localhost:8080/readyz
  ```



## Run with Docker Compose

Full stack: Postgres, API, Prometheus.

```bash
docker compose up --build -d
```


| Service       | URL                                            |
| ------------- | ---------------------------------------------- |
| API           | [http://localhost:8080](http://localhost:8080) |
| Prometheus UI | [http://localhost:9090](http://localhost:9090) |


Postgres (DBeaver / GUI):

1. Start DB: `docker compose up -d db` — host port **5435** → container `5432`.
2. **DBeaver inside WSL** or **Go on WSL:** host `127.0.0.1`, port `5435`.
3. **DBeaver on Windows, Docker in WSL** — `127.0.0.1:5435` often returns *connection refused* because Windows `localhost` is not WSL. Use the **WSL IP** as host:
   - PowerShell: `wsl hostname -I` → first IP (e.g. `172.19.253.242`)
   - DBeaver **Host:** that IP, **Port:** `5435`, **Database:** `appdb`, **User/Password:** `app` / `app`
   - JDBC: `jdbc:postgresql://172.19.253.242:5435/appdb?sslmode=disable` (replace with your IP)
   - With **Docker Desktop** and WSL integration enabled, `127.0.0.1:5435` from Windows may work; if not, use the WSL IP.

- **Go / psql from WSL:** `postgres://app:app@127.0.0.1:5435/appdb?sslmode=disable`
- **SSL:** disabled


Inspect schema:

```bash
docker exec -it ticket-booking-db-1 psql -U app -d appdb -c '\dt'
```



## Database schema (initial)

Tables from migrations: `shows`, `seats`, `reservations`, `reservation_seats` (`001_init`), then `users`, `api_tokens` (`002_auth`).