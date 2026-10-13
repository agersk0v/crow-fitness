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
assert body['error']['templates']==[{'id':sys.argv[2],'name':'Smoke routine','archived':False}]
CHECK
# Exercise the management routes once; detailed rule cases stay in Go tests.
curl --fail --silent --show-error "$base_url/api/templates/$template_id" -X PUT -H 'Content-Type: application/json' --data-binary "@$request_file" --output "$response_file"
copy_id="$(curl --fail --silent --show-error "$base_url/api/templates/$template_id/duplicate" -H 'Content-Type: application/json' --data '{"name":"Smoke copy"}' | python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])')"
curl --fail --silent --show-error "$base_url/api/templates/$copy_id/archive" -X POST --output "$response_file"
curl --fail --silent --show-error "$base_url/api/templates/$copy_id/restore" -X POST --output "$response_file"
curl --fail --silent --show-error "$base_url/api/templates/$copy_id/archive" -X POST --output "$response_file"
assert_archived_copy() {
  curl --fail --silent --show-error "$base_url/api/templates?archived=true" --output "$response_file"
  python3 - "$response_file" "$copy_id" <<'CHECK'
import json,sys
with open(sys.argv[1]) as f: items=json.load(f)['items']
assert any(x['id']==sys.argv[2] and x['name']=='Smoke copy' and x['archived'] for x in items),items
CHECK
  curl --fail --silent --show-error "$base_url/api/templates/$copy_id" --output "$response_file"
  python3 - "$response_file" "$copy_id" "$template_id" <<'CHECK'
import json,sys
with open(sys.argv[1]) as f: saved=json.load(f)
assert saved['id']==sys.argv[2] and saved['id']!=sys.argv[3] and saved['archived']
assert saved['exercises'][0]['name']=='Smoke squat'
CHECK
}
assert_archived_copy
# Session transport and restart persistence share this disposable stack.
session_id="$(curl --fail --silent --show-error "$base_url/api/sessions" -H 'Content-Type: application/json' --data "{\"template_id\":\"$template_id\"}" | python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])')"
curl --fail --silent --show-error "$base_url/api/sessions/$session_id/sets/11111111-1111-4111-8111-111111111111" -X PUT -H 'Content-Type: application/json' --data '{"exercise_position":1,"weight":"12.50","reps":8}' --output "$response_file"
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
assert saved['sets']==[{'id':'11111111-1111-4111-8111-111111111111','exercise_position':1,'weight':'12.50','reps':8}],saved
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
curl --fail --silent --show-error "$base_url/sessions/$session_id" --output "$response_file"
grep -q 'Loading session' "$response_file"
docker compose stop postgres
assert_response /api/health 200 ok
assert_response /api/ready 503 unavailable
docker compose start postgres
wait_ready
assert_exercise
assert_template
assert_archived_copy
assert_session
docker compose down
docker compose up --detach --wait --wait-timeout 180
assert_exercise
assert_template
assert_archived_copy
assert_session
code="$(curl --silent --show-error --output "$response_file" --write-out '%{http_code}' "$base_url/api/sessions/$session_id/complete" -X POST)"
[[ "$code" == 204 ]]
curl --fail --silent --show-error "$base_url/api/sets" --output "$response_file"
python3 - "$response_file" <<'CHECK'
import json,sys
with open(sys.argv[1]) as f: items=json.load(f)['items']
assert len(items)==1 and items[0]['exercise_name']=='Smoke squat' and items[0]['weight']=='12.50' and items[0]['reps']==8,items
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
curl --fail --silent --show-error "$base_url/api/sessions/$session_id/sets/11111111-1111-4111-8111-111111111111" -X PUT -H 'Content-Type: application/json' --data '{"exercise_position":1,"weight":"0","reps":5}' --output "$response_file"
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
curl --fail --silent --show-error "$base_url/api/exercises/$exercise_id/history" --output "$response_file"
python3 - "$response_file" "$exercise_id" <<'CHECK'
import json,sys
with open(sys.argv[1]) as f: history=json.load(f)
assert history['id']==sys.argv[2] and len(history['workouts'])==1,history
assert history['workouts'][0]['name']=='Smoke routine' and history['workouts'][0]['occurrences'][0]['sets'][0]['weight']=='12.50',history
CHECK
curl --fail --silent --show-error "$base_url/exercises/$exercise_id/history" --output "$response_file"
grep -q 'Loading exercise history' "$response_file"
curl --fail --silent --show-error "$base_url/history/$completed_id" --output "$response_file"
grep -q 'Loading completed session' "$response_file"
curl --fail --silent --show-error --location "$base_url/sets" --output "$response_file"
grep -q 'Loading completed sets' "$response_file"
curl --fail --silent --show-error "$base_url/api/sets?q=SMOKE&exercise=$exercise_id" --output "$response_file"
python3 - "$response_file" <<'CHECK'
import json,sys
with open(sys.argv[1]) as f: items=json.load(f)['items']
assert len(items)==1 and items[0]['weight']=='12.50',items
CHECK
curl --fail --silent --show-error "$base_url/api/history/sessions?q=SMOKE&exercise=$exercise_id" --output "$response_file"
python3 - "$response_file" <<'CHECK'
import json,sys
with open(sys.argv[1]) as f: items=json.load(f)['items']
assert len(items)==1 and items[0]['name']=='Smoke routine',items
CHECK
curl --fail --silent --show-error "$base_url/api/history/exercises" --output "$response_file"
python3 - "$response_file" "$exercise_id" <<'CHECK'
import json,sys
with open(sys.argv[1]) as f: items=json.load(f)['items']
assert any(item['id']==sys.argv[2] for item in items),items
CHECK
code="$(curl --silent --show-error --output "$response_file" --write-out '%{http_code}' "$base_url/api/sets?from=2030-02-30")"
[[ "$code" == 400 ]]
python3 - "$response_file" <<'CHECK'
import json,sys
with open(sys.argv[1]) as f: body=json.load(f)
assert body['error']['code']=='validation_error',body
CHECK
assert_response /api/ready 200 ok
echo "Stack smoke check passed: frontend, API, exercise/template/session routing, validation, readiness recovery, and restart persistence."
