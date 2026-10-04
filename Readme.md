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
| Seat hold / book / cancel APIs              | Planned |


## HTTP endpoints


| Method | Path       | Description                                                                             |
| ------ | ---------- | --------------------------------------------------------------------------------------- |
| `GET`  | `/healthz` | **Liveness** — process is up                                                            |
| `GET`  | `/readyz`  | **Readiness** — pings Postgres                                                          |
| `GET`  | `/metrics` | Prometheus scrape endpoint (`http_requests_total`, `http_request_duration_seconds`, …). |


Examples (API on port `8080`):

```bash
curl -s http://localhost:8080/healthz
curl -s http://localhost:8080/readyz
curl -s http://localhost:8080/metrics | head
```



## Project layout

```
cmd/server/              # Application entrypoint
internal/config/         # Env-based configuration
internal/handler/        # HTTP handlers and router
internal/middleware/     # Logging, metrics, rate limit, recovery
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
| `DB_URL`           | `postgres://app:app@localhost:5432/appdb?sslmode=disable` | Postgres (use host `db` inside Compose) |
| `LOG_LEVEL`        | `debug` / `info`                                          | slog level                              |
| `RATE_LIMIT_RPS`   | `200`                                                     | Sustained requests per second           |
| `RATE_LIMIT_BURST` | `500`                                                     | Burst size                              |




## Run locally

1. Start Postgres (applies `migrations/001_init.up.sql` on first empty volume):
  ```bash
   docker compose up -d db
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


Postgres (from the host, e.g. DBeaver):

- **URL:** `postgres://app:app@localhost:5432/appdb?sslmode=disable`
- **User / password:** `app` / `app`
- **SSL:** disabled

If port `5432` is already used by another Postgres on your machine, change the host mapping in `docker-compose.yml` (e.g. `"5433:5432"`) and use port `5433` in clients.

Inspect schema:

```bash
docker exec -it ticket-booking-db-1 psql -U app -d appdb -c '\dt'
```



## Database schema (initial)

Tables created on first DB init: `shows`, `seats`, `reservations`, `reservation_seats` (see `migrations/001_init.up.sql`).