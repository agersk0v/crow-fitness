#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."

# Every run owns a fresh Compose project and disposable volume.
export COMPOSE_PROJECT_NAME="crowfitness-smoke-${RANDOM}-${RANDOM}"
export POSTGRES_PASSWORD="smoke-only-test-password"
export APP_PORT="${SMOKE_PORT:-13000}"
base_url="http://127.0.0.1:${APP_PORT}"
response_file="$(mktemp)"
request_file="$(mktemp)"

cleanup() {
  if [[ "$?" != 0 ]]; then
    docker compose logs --no-color || true
  fi
  docker compose down --volumes --remove-orphans >/dev/null 2>&1 || true
  rm -f "$response_file" "$request_file"
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

assert_page() {
  curl --fail --silent --show-error --location --max-time 8 "${base_url}$1" --output "$response_file"
  grep -q "$2" "$response_file"
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
assert_page / 'Crow Fitness'
assert_page /exercises 'Exercise name'
assert_page /templates 'Create a template'
assert_response /api/health 200 ok
assert_response /api/ready 200 ok
exercise_id="$(curl --fail --silent --show-error "$base_url/api/exercises" -H 'Content-Type: application/json' --data '{"name":"  Smoke squat  ","muscle_groups":["quadriceps"]}' | python3 -c 'import json,sys; x=json.load(sys.stdin); assert x["name"]=="Smoke squat"; print(x["id"])')"
code="$(curl --silent --show-error --output "$response_file" --write-out '%{http_code}' "$base_url/api/exercises" -H 'Content-Type: application/json' --data '{"name":"   "}')"
[[ "$code" == 400 ]]
python3 - "$response_file" <<'CHECK'
import json,sys
with open(sys.argv[1]) as f: body=json.load(f)
assert body['error']['fields']['name']
CHECK
# One exercise/template/workout flow through Next.js. Detailed CRUD, target,
# ownership, and history-query rules are covered by the Go/PostgreSQL tests.
python3 - "$exercise_id" > "$request_file" <<'CHECK'
import json,sys
exercise=sys.argv[1]
print(json.dumps({'name':'  Smoke routine  ','exercises':[
 {'exercise_id':exercise,'set_count':3,'rep_min':8,'rep_max':12,'target_rir':1.5}]}))
CHECK
template_id="$(curl --fail --silent --show-error "$base_url/api/templates" -H 'Content-Type: application/json' --data-binary "@$request_file" | python3 -c 'import json,sys; x=json.load(sys.stdin); assert x["name"]=="Smoke routine"; print(x["id"])')"
assert_page "/templates/$template_id" 'Loading template'
# Session transport and restart persistence share this disposable stack.
session_id="$(curl --fail --silent --show-error "$base_url/api/sessions" -H 'Content-Type: application/json' --data "{\"template_id\":\"$template_id\"}" | python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])')"
curl --fail --silent --show-error "$base_url/api/sessions/$session_id/sets/11111111-1111-4111-8111-111111111111" -X PUT -H 'Content-Type: application/json' --data '{"technique":"decent","exercise_position":1,"weight":"12.50","reps":8,"rir":1.5}' --output "$response_file"
curl --fail --silent --show-error "$base_url/api/sets" --output "$response_file"
python3 - "$response_file" <<'CHECK'
import json,sys
with open(sys.argv[1]) as f: assert json.load(f)['items']==[]
CHECK
assert_session() {
  curl --fail --silent --show-error "$base_url/api/sessions/active" --output "$response_file"
  python3 - "$response_file" "$session_id" "$template_id" <<'CHECK'
import json,sys
with open(sys.argv[1]) as f: saved=json.load(f)['session']
assert saved['id']==sys.argv[2] and saved['template_id']==sys.argv[3] and saved['name']=='Smoke routine',saved
assert saved['exercises'][0]['name']=='Smoke squat' and saved['exercises'][0]['set_count']==3,saved
assert len(saved['sets'])==1,saved
assert saved['sets'][0]['weight']=='12.50' and saved['sets'][0]['reps']==8 and saved['sets'][0]['rir']=='1.5' and saved['sets'][0]['technique']=='decent',saved
CHECK
  curl --fail --silent --show-error "$base_url/api/sessions/$session_id" --output "$response_file"
}
assert_session
code="$(curl --silent --show-error --output "$response_file" --write-out '%{http_code}' "$base_url/api/sessions" -H 'Content-Type: application/json' --data "{\"template_id\":\"$template_id\"}")"
[[ "$code" == 409 ]]
python3 - "$response_file" "$session_id" <<'CHECK'
import json,sys
with open(sys.argv[1]) as f: body=json.load(f)
assert body['error']['code']=='active_session_exists' and body['error']['session']['id']==sys.argv[2],body
CHECK
assert_page "/sessions/$session_id" 'Loading session'
docker compose stop postgres
assert_response /api/health 200 ok
assert_response /api/ready 503 unavailable
docker compose start postgres
wait_ready
assert_session
docker compose down
docker compose up --detach --wait --wait-timeout 180
assert_session
code="$(curl --silent --show-error --output "$response_file" --write-out '%{http_code}' "$base_url/api/sessions/$session_id/complete" -X POST)"
[[ "$code" == 204 ]]
curl --fail --silent --show-error "$base_url/api/sets" --output "$response_file"
python3 - "$response_file" <<'CHECK'
import json,sys
with open(sys.argv[1]) as f: items=json.load(f)['items']
assert len(items)==1 and items[0]['exercise_name']=='Smoke squat' and items[0]['weight']=='12.50' and items[0]['reps']==8 and items[0]['rir']=='1.5',items
CHECK
session_id="$(curl --fail --silent --show-error "$base_url/api/sessions" -H 'Content-Type: application/json' --data "{\"template_id\":\"$template_id\"}" | python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])')"
# Check previous-performance forwarding against the just-completed workout.
curl --fail --silent --show-error "$base_url/api/sessions/$session_id/previous" --output "$response_file"
python3 - "$response_file" "$exercise_id" <<'CHECK'
import json,sys
with open(sys.argv[1]) as f: items=json.load(f)['items']
assert len(items)==1 and items[0]['exercise_id']==sys.argv[2],items
assert items[0]['occurrences'][0]['sets'][0]['weight']=='12.50',items
CHECK
curl --fail --silent --show-error "$base_url/api/sessions/$session_id/sets/11111111-1111-4111-8111-111111111111" -X PUT -H 'Content-Type: application/json' --data '{"technique":"decent","exercise_position":1,"weight":"0","reps":5}' --output "$response_file"
code="$(curl --silent --show-error --output "$response_file" --write-out '%{http_code}' "$base_url/api/sessions/$session_id/cancel" -X POST)"
[[ "$code" == 204 ]]
curl --fail --silent --show-error "$base_url/api/sessions/active" --output "$response_file"
python3 - "$response_file" <<'CHECK'
import json,sys
with open(sys.argv[1]) as f: assert json.load(f)['session'] is None
CHECK
curl --fail --silent --show-error "$base_url/api/sets" --output "$response_file"
python3 - "$response_file" <<'CHECK'
import json,sys
with open(sys.argv[1]) as f: items=json.load(f)['items']
assert len(items)==1 and items[0]['weight']=='12.50',items
CHECK
curl --fail --silent --show-error "$base_url/api/history/sessions" --output "$response_file"
python3 - "$response_file" <<'CHECK'
import json,sys
with open(sys.argv[1]) as f: items=json.load(f)['items']
assert len(items)==1 and items[0]['name']=='Smoke routine',items
CHECK
completed_id="$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["items"][0]["id"])' "$response_file")"
curl --fail --silent --show-error "$base_url/api/history/sessions/$completed_id" --output "$response_file"
python3 - "$response_file" <<'CHECK'
import json,sys
with open(sys.argv[1]) as f: saved=json.load(f)
assert saved['completed_at'] and saved['sets'][0]['weight']=='12.50' and saved['exercises'][0]['name']=='Smoke squat',saved
CHECK
# Route availability only; these fetches do not exercise interactive controls.
assert_page "/history/$completed_id" 'Loading completed session'
assert_page /sets 'Loading completed sets'
removed_history_status="$(curl --silent --show-error --max-time 8 --output "$response_file" --write-out '%{http_code}' "$base_url/exercises/$exercise_id/history")"
[[ "$removed_history_status" == "404" ]]
assert_response /api/ready 200 ok
echo "Stack smoke check passed: frontend, proxied workout lifecycle, validation, readiness recovery, and restart persistence."
