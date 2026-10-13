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
exercise_id="$(curl --fail --silent --show-error "$base_url/api/exercises" -H 'Content-Type: application/json' --data '{"name":"  Smoke squat  ","muscle_groups":["quadriceps"]}' | python3 -c 'import json,sys; x=json.load(sys.stdin); assert x["name"]=="Smoke squat"; print(x["id"])')"
assert_exercise() {
  curl --fail --silent --show-error "$base_url/api/exercises" --output "$response_file"
  python3 - "$response_file" "$exercise_id" <<'CHECK'
import json,sys
with open(sys.argv[1]) as f: items=json.load(f)['items']
assert any(x['id']==sys.argv[2] and x['name']=='Smoke squat' and x['muscle_groups']==['quadriceps'] for x in items), items
CHECK
}
curl --fail --silent --show-error "$base_url/api/exercises/$exercise_id" -X PUT -H 'Content-Type: application/json' --data '{"name":" Smoke squat ","muscle_groups":["quadriceps"]}' --output "$response_file"
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
# Verify a representative template through Next.js forwarding. Detailed target,
# ordering, ownership, and rollback cases stay in the PostgreSQL tests.
python3 - "$exercise_id" > "$request_file" <<'CHECK'
import json,sys
exercise=sys.argv[1]
print(json.dumps({'name':'  Smoke routine  ','exercises':[
 {'exercise_id':exercise,'set_count':3,'rep_min':8,'rep_max':12,'target_rir':1.5}]}))
CHECK
template_id="$(curl --fail --silent --show-error "$base_url/api/templates" -H 'Content-Type: application/json' --data-binary "@$request_file" | python3 -c 'import json,sys; x=json.load(sys.stdin); assert x["name"]=="Smoke routine"; print(x["id"])')"
assert_template() {
  curl --fail --silent --show-error "$base_url/api/templates" --output "$response_file"
  python3 - "$response_file" "$template_id" <<'CHECK'
import json,sys
with open(sys.argv[1]) as f: items=json.load(f)['items']
assert any(x['id']==sys.argv[2] and x['name']=='Smoke routine' for x in items),items
CHECK
  curl --fail --silent --show-error "$base_url/api/templates/$template_id" --output "$response_file"
  python3 - "$response_file" "$request_file" <<'CHECK'
import json,sys
with open(sys.argv[1]) as f: saved=json.load(f)
with open(sys.argv[2]) as f: request=json.load(f)
assert saved['name']=='Smoke routine'
assert len(saved['exercises'])==1
for i,(actual,expected) in enumerate(zip(saved['exercises'],request['exercises'])):
    assert actual['position']==i+1 and actual['name']=='Smoke squat',actual
    for key,value in expected.items(): assert actual[key]==value,(key,actual)
CHECK
}
assert_template
curl --fail --silent --show-error "$base_url/templates" --output "$response_file"
grep -q 'Create a template' "$response_file"
curl --fail --silent --show-error "$base_url/templates/$template_id" --output "$response_file"
grep -q 'Loading template' "$response_file"
code="$(curl --silent --show-error --output "$response_file" --write-out '%{http_code}' "$base_url/api/exercises/$exercise_id" -X DELETE)"
[[ "$code" == 409 ]]
python3 - "$response_file" "$template_id" <<'CHECK'
import json,sys
with open(sys.argv[1]) as f: body=json.load(f)
assert body['error']['code']=='exercise_in_use'
assert body['error']['templates']==[{'id':sys.argv[2],'name':'Smoke routine'}]
CHECK
docker compose stop postgres
assert_response /api/health 200 ok
assert_response /api/ready 503 unavailable
docker compose start postgres
wait_ready
assert_exercise
assert_template
docker compose down
docker compose up --detach --wait --wait-timeout 180
assert_exercise
assert_template
assert_response /api/ready 200 ok
echo "Stack smoke check passed: frontend, API, exercise/template routing, validation, readiness recovery, and restart persistence."
