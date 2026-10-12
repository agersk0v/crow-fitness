#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."

# Every run owns a fresh Compose project and disposable volume.
export COMPOSE_PROJECT_NAME="crowfitness-smoke-${RANDOM}-${RANDOM}"
export POSTGRES_PASSWORD="smoke-only-test-password"
export APP_PORT="${SMOKE_PORT:-13000}"
base_url="http://127.0.0.1:${APP_PORT}"
response_file="$(mktemp)"

cleanup() {
  if [[ "$?" != 0 ]]; then
    docker compose logs --no-color || true
  fi
  docker compose down --volumes --remove-orphans >/dev/null 2>&1 || true
  rm -f "$response_file"
}
trap cleanup EXIT

assert_response() {
  local path="$1" expected_code="$2" expected_status="$3"
  local code
  code="$(curl --silent --show-error --max-time 8 --output "$response_file" --write-out '%{http_code}' "${base_url}${path}")"
  [[ "$code" == "$expected_code" ]] || { echo "Unexpected HTTP status for ${path}: ${code}"; return 1; }
  python3 - "$response_file" "$expected_status" <<'PY'
import json, sys
with open(sys.argv[1]) as file:
    body = json.load(file)
assert body['status'] == sys.argv[2], body
PY
}

wait_ready() {
  for attempt in {1..30}; do
    if curl --fail --silent --max-time 5 "${base_url}/api/ready" >/dev/null; then
      return 0
    fi
    sleep 1
  done
  echo "Database readiness did not recover"
  return 1
}

docker compose config --quiet
docker compose up --build --detach --wait --wait-timeout 180
curl --fail --silent --show-error --max-time 8 "$base_url" --output "$response_file"
grep -q 'Crow Fitness' "$response_file"
assert_response /api/health 200 ok
assert_response /api/ready 200 ok
docker compose exec -T postgres psql -U crowfitness -d crowfitness -v ON_ERROR_STOP=1 -c \
  "CREATE TABLE foundation_smoke (value text NOT NULL); INSERT INTO foundation_smoke VALUES ('persists');"
docker compose stop postgres
assert_response /api/health 200 ok
assert_response /api/ready 503 unavailable
docker compose start postgres
wait_ready
value="$(docker compose exec -T postgres psql -U crowfitness -d crowfitness -tAc 'SELECT value FROM foundation_smoke')"
[[ "$value" == "persists" ]] || { echo "Database data did not survive restart"; exit 1; }
assert_response /api/ready 200 ok
echo "Stack smoke check passed: frontend, API, readiness failure/recovery, and data persistence."
