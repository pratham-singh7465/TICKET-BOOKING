#!/usr/bin/env bash
# Quick capability smoke test: one hot seat, many concurrent reserve attempts.
# Requires: curl, jq, running API + DB + Redis + migrations + dev users (002_auth).
#
# Usage:
#   BASE_URL=http://localhost:8080 CONCURRENCY=100 ./scripts/load-smoke.sh
# With resource caps:
#   COMPOSE_COMPATIBILITY=1 docker compose -f docker-compose.yml -f docker-compose.limits.yml up -d --build
#   BASE_URL=http://localhost:8080 CONCURRENCY=200 ./scripts/load-smoke.sh

set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:8080}"
ADMIN_KEY="${ADMIN_KEY:-dev-admin-change-me}"
CONCURRENCY="${CONCURRENCY:-50}"
SEATS_JSON='["HOT1","HOT2","HOT3","HOT4","HOT5"]'

if ! command -v jq >/dev/null 2>&1; then
  echo "jq is required" >&2
  exit 1
fi

echo "==> health"
curl -sf "$BASE_URL/healthz" | jq .
curl -sf "$BASE_URL/readyz" | jq .

echo "==> create show"
SHOW_JSON=$(curl -sf -X POST "$BASE_URL/api/v1/shows" \
  -H "Content-Type: application/json" \
  -H "X-Admin-Key: $ADMIN_KEY" \
  -d "{\"name\":\"load-smoke-$(date +%s)\",\"seats\":$SEATS_JSON,\"price_paise\":10000}")
SHOW_ID=$(echo "$SHOW_JSON" | jq -r .id)
echo "show_id=$SHOW_ID"

HOT_SEAT=$(echo "$SEATS_JSON" | jq -r '.[0]')
echo "==> hot seat storm: $CONCURRENCY workers -> seat $HOT_SEAT"

TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

worker() {
  local i=$1
  local user="user$(( (i % 5) + 1 ))"
  local token="token-${user}"
  local code
  code=$(curl -s -o "$TMP/out-$i.json" -w "%{http_code}" \
    -X POST "$BASE_URL/api/v1/shows/$SHOW_ID/reserve" \
    -H "Authorization: Bearer $token" \
    -H "Content-Type: application/json" \
    -H "Idempotency-Key: smoke-$i-$(date +%s%N)" \
    -d "{\"seats\":[\"$HOT_SEAT\"]}")
  echo "$code" >"$TMP/code-$i"
}

for i in $(seq 1 "$CONCURRENCY"); do
  worker "$i" &
done
wait

created=0
conflict=0
other=0
fivexx=0
for i in $(seq 1 "$CONCURRENCY"); do
  c=$(cat "$TMP/code-$i")
  case "$c" in
    201) created=$((created + 1)) ;;
    409) conflict=$((conflict + 1)) ;;
    5*) fivexx=$((fivexx + 1)); other=$((other + 1)) ;;
    *) other=$((other + 1)) ;;
  esac
done

echo ""
echo "=== reserve results (hot seat) ==="
echo "  201 created (hold): $created"
echo "  409 declined:     $conflict"
echo "  other:              $other"
echo "  5xx:                $fivexx"

echo "==> show state + reconciliation"
STATE=$(curl -sf "$BASE_URL/api/v1/shows/$SHOW_ID")
echo "$STATE" | jq '{counts, seats: [.seats[] | select(.seat_code=="'"$HOT_SEAT"'")]}'
AVAIL=$(echo "$STATE" | jq .counts.available)
HELD=$(echo "$STATE" | jq .counts.held)
CONF=$(echo "$STATE" | jq .counts.confirmed)
TOTAL=$(echo "$STATE" | jq .counts.total)
SUM=$((AVAIL + HELD + CONF))
if [[ "$SUM" -ne "$TOTAL" ]]; then
  echo "RECONCILE FAIL: available+held+confirmed=$SUM total=$TOTAL" >&2
  exit 1
fi
if [[ "$created" -gt 1 ]]; then
  echo "WARN: more than one 201 on same hot seat (expected 1)" >&2
  exit 1
fi
if [[ "$fivexx" -gt 0 ]]; then
  echo "WARN: saw 5xx during burst" >&2
  exit 1
fi
echo "OK: reconciliation holds; hot seat winners=$created"
