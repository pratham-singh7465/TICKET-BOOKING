#!/usr/bin/env bash
# Local / CI helper — same migrations as the compose `migrate` service.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
export PGHOST="${PGHOST:-127.0.0.1}"
export PGPORT="${PGPORT:-5435}"
export PGUSER="${PGUSER:-app}"
export PGPASSWORD="${PGPASSWORD:-app}"
export PGDATABASE="${PGDATABASE:-appdb}"

if docker compose -f "$ROOT/docker-compose.yml" ps -q db 2>/dev/null | grep -q .; then
  echo "Running migrations via docker compose migrate service..."
  docker compose -f "$ROOT/docker-compose.yml" run --rm migrate
  exit 0
fi

if command -v psql >/dev/null 2>&1; then
  export PGHOST PGPORT PGUSER PGPASSWORD PGDATABASE
  export MIGRATIONS_DIR="$ROOT/migrations"
  sh "$ROOT/scripts/docker-migrate.sh"
  exit 0
fi

echo "Start Postgres (docker compose up -d db) or install psql, then re-run." >&2
exit 1
