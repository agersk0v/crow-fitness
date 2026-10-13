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
exercise_id="$(curl --fail --silent --show-error "$base_url/api/exercises" -H 'Content-Type: application/json' --data '{"name":"  Smoke squat  "}' | python3 -c 'import json,sys; x=json.load(sys.stdin); assert x["name"]=="Smoke squat"; print(x["id"])')"
assert_exercise() {
  curl --fail --silent --show-error "$base_url/api/exercises" --output "$response_file"
  python3 - "$response_file" "$exercise_id" <<'CHECK'
import json,sys
with open(sys.argv[1]) as f: items=json.load(f)['items']
assert any(x['id']==sys.argv[2] and x['name']=='Smoke squat' for x in items), items
CHECK
}
assert_exercise
curl --fail --silent --show-error "$base_url/exercises" --output "$response_file"
grep -q 'Exercise name' "$response_file"
code="$(curl --silent --show-error --output "$response_file" --write-out '%{http_code}' "$base_url/api/exercises" -H 'Content-Type: application/json' --data '{"name":"   "}')"
[[ "$code" == 400 ]]
python3 - "$response_file" <<'CHECK'
import json,sys
with open(sys.argv[1]) as f: body=json.load(f)
assert body['error']['fields']['name']
CHECK
docker compose stop postgres
assert_response /api/health 200 ok
assert_response /api/ready 503 unavailable
docker compose start postgres
wait_ready
assert_exercise
docker compose down
docker compose up --detach --wait --wait-timeout 180
assert_exercise
assert_response /api/ready 200 ok
echo "Stack smoke check passed: frontend, API, exercise validation, readiness recovery, and restart persistence."
