#!/bin/sh
# Runs on every `docker compose up`: applies pending migrations/*.up.sql in order.
set -eu

export PGPASSWORD="${PGPASSWORD:-app}"

until psql -h "${PGHOST:-db}" -U "${PGUSER:-app}" -d "${PGDATABASE:-appdb}" -c 'SELECT 1' >/dev/null 2>&1; do
  echo "waiting for postgres..."
  sleep 1
done

psql -h "${PGHOST:-db}" -U "${PGUSER:-app}" -d "${PGDATABASE:-appdb}" -v ON_ERROR_STOP=1 <<'SQL'
CREATE TABLE IF NOT EXISTS schema_migrations (
    version TEXT PRIMARY KEY,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Databases created before schema_migrations (initdb.d only) are marked applied if tables exist.
INSERT INTO schema_migrations (version)
SELECT '001_init'
WHERE EXISTS (
    SELECT 1 FROM information_schema.tables
    WHERE table_schema = 'public' AND table_name = 'shows'
)
ON CONFLICT DO NOTHING;

INSERT INTO schema_migrations (version)
SELECT '002_auth'
WHERE EXISTS (
    SELECT 1 FROM information_schema.tables
    WHERE table_schema = 'public' AND table_name = 'users'
)
ON CONFLICT DO NOTHING;
SQL

MIGRATIONS_DIR="${MIGRATIONS_DIR:-/migrations}"

for f in "${MIGRATIONS_DIR}"/*_*.up.sql; do
  [ -f "$f" ] || continue
  ver=$(basename "$f" .up.sql)
  applied=$(psql -h "${PGHOST:-db}" -U "${PGUSER:-app}" -d "${PGDATABASE:-appdb}" -tAc \
    "SELECT 1 FROM schema_migrations WHERE version = '${ver}'" | tr -d '[:space:]')
  if [ "$applied" = "1" ]; then
    echo "skip ${ver}"
    continue
  fi
  echo "apply ${ver}"
  psql -h "${PGHOST:-db}" -U "${PGUSER:-app}" -d "${PGDATABASE:-appdb}" -v ON_ERROR_STOP=1 -f "$f"
  psql -h "${PGHOST:-db}" -U "${PGUSER:-app}" -d "${PGDATABASE:-appdb}" -v ON_ERROR_STOP=1 \
    -c "INSERT INTO schema_migrations (version) VALUES ('${ver}')"
done

echo "migrations complete"
