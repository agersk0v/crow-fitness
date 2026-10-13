# Proposed architecture

## Status and scope

Crow Fitness is a simple workout tracker for one implicit user. The runnable foundation is implemented with Next.js, TypeScript, Tailwind CSS, a Go REST API, and PostgreSQL. Workout features and their data model remain proposals. Exercises are personal, with one internal local owner selected by Go. There is no authentication, login, or browser-provided user identity.

The current [README](../README.md) documents setup and checks; [PRODUCT.md](PRODUCT.md) defines requirements and open product decisions. The local foundation described below is implemented. Workout component responsibilities, schema, lifecycle, and REST endpoints are proposed designs; reconcile them with approved product decisions before implementing affected behavior.

## Components and responsibilities

| Component | Responsibility | Proposed approach |
| --- | --- | --- |
| Frontend | Connection-check page now; templates, workout entry, history, and progress planned | Next.js App Router, TypeScript, Tailwind CSS, and small client components |
| API | Validation, workout rules, snapshots, history queries | Go `net/http`, with handlers calling services and repositories |
| Database | Durable records, relationships, constraints and transactions | PostgreSQL accessed through `pgx` and a connection pool |

The browser uses relative `/api` URLs. Next.js forwards those requests to Go on the internal network; it does not duplicate workout business logic. Only Go accesses PostgreSQL. Keep mutable session and history responses uncached, and refresh relevant views after writes.

Use one frontend, one backend, and one database. Background workers, queues, microservices, an ORM, and a second API layer are unnecessary for the proposed MVP. Library versions and exact tooling will be selected and pinned when implementation starts.

## Proposed repository layout

| Path | Contents |
| --- | --- |
| `frontend/app/` | Routes for `/templates`, `/sessions`, `/sessions/[id]`, `/exercises`, and `/exercises/[id]` |
| `frontend/components/` | Reusable UI and workout entry forms |
| `frontend/lib/` | Typed API client and shared frontend types |
| `backend/cmd/api/` | Configuration and API server startup |
| `backend/internal/handlers/` | HTTP request decoding and response encoding |
| `backend/internal/service/` | Application rules and transaction orchestration |
| `backend/internal/repository/` | PostgreSQL queries using explicit SQL |
| `backend/internal/models/` | Domain and API data types |
| `backend/migrations/` | Ordered, versioned schema migrations |
| `docs/` | Product requirements and technical design |
| `docker-compose.yml` | Proposed local services and persistent database volume |

The foundation adds the frontend routes/components, Go entrypoint, configuration and health packages, Dockerfiles, Compose, and CI. Domain service/repository/models packages, migrations, and workout pages are future work.

## Proposed data model

Use generated `bigint` primary keys, `timestamptz` timestamps, and explicit foreign keys. Serialize IDs as decimal strings in JSON to avoid JavaScript integer precision loss. Store timestamps in UTC and display them in the browser's local timezone.

| Table | Main columns | Relationships and constraints |
| --- | --- | --- |
| `exercises` | `id`, `name` | Stable identity reused across templates and sessions; nonblank name |
| `workout_templates` | `id`, `name` | Nonblank name |
| `template_exercises` | `id`, `template_id`, `exercise_id`, `position`, `planned_sets` | References template and exercise; unique `(template_id, position)`; positive position and planned set count |
| `workout_sessions` | `id`, nullable `template_id`, `template_name`, `started_at`, nullable `finished_at`, `status`, `client_request_id` | Name snapshot; status `active` or `completed`; unique request UUID for safe start retries |
| `session_exercises` | `id`, `session_id`, `exercise_id`, `exercise_name`, `position`, `planned_sets` | References session and stable exercise; name/configuration snapshot; unique `(session_id, position)` |
| `performed_sets` | `id`, `session_exercise_id`, `set_number`, `weight_kg`, `reps`, `client_request_id` | References session exercise; unique `(session_exercise_id, set_number)` and `(session_exercise_id, client_request_id)` |

If kilograms are accepted as the MVP unit, propose `numeric(8,2)` for weights, including zero for bodyweight exercises. Accept at most two decimal places and enforce the storage range instead of silently rounding. Serialize weight as a decimal string such as `"60.00"`. Reps, set numbers, and planned set counts are positive integers. Ordering uses position/set number; gaps after deleting a set are allowed.

Require `finished_at` to be null for active sessions and present for completed sessions, with `finished_at >= started_at`. If the one-active-session rule is approved, enforce it with a partial unique index on `status` where `status = 'active'`, so concurrent requests cannot both start a workout.

Index foreign keys used in joins, especially `session_exercises.exercise_id` and `performed_sets.session_exercise_id`. Optimize further only when a real history query needs it.

### History retention

Deleting a template removes its template-exercise configuration but preserves sessions. Set the session's template foreign key to null on template deletion; retain the template name snapshot. Session exercises keep their copied position, planned sets, and exercise name even if a template or exercise is renamed later.

Exercise IDs remain stable across templates. Block deletion of referenced exercises; exercise deletion, session deletion, and editing completed sessions are outside this proposed MVP. All set mutations must pass the session-status check in the service. Do not expose generic repository writes that bypass that rule.

## Session lifecycle and transactions

1. **Start a session:** accept a template ID and client-generated request UUID. In one transaction, lock the template row, read its ordered exercise configuration, insert an active session, and copy the configuration and names into session exercises. Template edits/deletions must acquire the same template lock. This prevents a start from copying a mixture of old and new configuration. Reject missing or empty templates. A failed copy rolls back the whole session.
2. **Show reference performance:** for each stable exercise ID, select the most recent completed session containing recorded sets for that exercise, ordered by completion time and then session ID. Return all matching performed sets in exercise-position/set-number order, including repeated occurrences of the exercise in that session. This lookup spans all templates and excludes the active session.
3. **Record a set:** lock the parent session row, verify it is active and the session exercise belongs to it, then persist valid weight and reps. A client request UUID identifies the logical create request. Retrying the same UUID and payload returns the existing record; reusing it with different data returns a conflict. Uniqueness constraints protect concurrent retries. Replay lookup precedes the active-status check, so a lost response can still be recovered after completion.
4. **Edit or remove a set:** acquire the same parent-session lock and check active status. Updates replace supplied fields; deletion leaves other set numbers unchanged. Suggested values are never inserted by these operations unless the user records the set.
5. **Complete the workout:** the frontend waits for pending saves, then the API locks the session, changes its status and finish timestamp in one transaction, and returns the session. Completing an already completed session returns its existing result. The shared session lock orders completion against set writes; later writes are rejected.

Template creation/update replaces the ordered configuration atomically after validating all exercise IDs. Validate before writing, then retain database constraints as the final integrity check. Never update a session by rereading a template.

### Suggested values and persistence

Empty planned rows and any prefilled weight/reps are frontend suggestions, separate from `performed_sets`. Display previous performance independently. A visible Record/Done action creates the performed set; a prefilled row does not count toward history or totals by itself. This is a proposal pending the product decision on logging behavior.

If autosave is approved, automatically persist edits to already recorded sets only when both fields are valid. Serialize saves per set, keep unsaved input visible, and show saving/saved/error states. Wait for outstanding saves before completing a session. On reload, restore persisted performed sets from Go; unsaved drafts are not guaranteed to survive. Offline persistence is outside the MVP.

## Proposed REST API

All endpoints use JSON under `/api`. Collection GET responses use `{ "items": [...] }`; resource responses return the resource object directly. History uses a bounded `limit` (default 50, maximum 100) and an opaque cursor for stable pagination, ordered by session start time, session ID, exercise position, and set number. Invalid cursors return a validation error. Add pagination to other lists if needed.

| Method | Endpoint | Purpose |
| --- | --- | --- |
| GET, POST | `/api/exercises` | List/create exercises |
| GET | `/api/exercises/{id}/history` | Recorded sets grouped by session, with completion status |
| GET, POST | `/api/templates` | List/create templates |
| GET, PUT, DELETE | `/api/templates/{id}` | Read, atomically replace, or delete a template |
| GET, POST | `/api/sessions` | List sessions or start from a template |
| GET | `/api/sessions/{id}` | Session snapshots, performed sets, and separate previous-performance references |
| POST | `/api/sessions/{id}/sets` | Record a set for a session exercise |
| PATCH, DELETE | `/api/sets/{id}` | Edit/remove a set in an active session |
| POST | `/api/sessions/{id}/complete` | Complete the session |

History includes persisted sets from active and completed sessions with a status label; previous-performance suggestions use completed sessions only. Proposed successful responses: `200` for reads/updates/completion and create replays, `201` for new resources, and `204` for deletion. Return a `Location` header on creation.

### Request and response examples

Create a template with `POST /api/templates` (array order defines positions):

```json
{
  "name": "Push Day",
  "exercises": [{ "exercise_id": "1", "planned_sets": 3 }]
}
```

Response (`201`):

```json
{
  "id": "2",
  "name": "Push Day",
  "exercises": [{ "exercise_id": "1", "position": 1, "planned_sets": 3 }]
}
```

Start a workout with `POST /api/sessions`:

```json
{
  "template_id": "2",
  "client_request_id": "01a15e02-398d-4a39-9297-b4cb1a604ae4"
}
```

Response (`201`):

```json
{ "id": "7", "template_name": "Push Day", "status": "active" }
```

The frontend opens `/sessions/7` and fetches the detail resource. Each session exercise has its own ID, stable exercise ID, name/configuration snapshot, `sets` array, and nullable `previous_performance` object. The reference object identifies the completed source session and its sets; an exercise without prior performance has null. Suggestions do not appear in `sets`.

Record a set with `POST /api/sessions/7/sets`:

```json
{
  "session_exercise_id": "12",
  "set_number": 1,
  "weight_kg": "60.00",
  "reps": 10,
  "client_request_id": "9983cb79-bc33-4a06-a647-1de8bb3be5cd"
}
```

Response (`201`):

```json
{
  "id": "20",
  "session_exercise_id": "12",
  "set_number": 1,
  "weight_kg": "60.00",
  "reps": 10
}
```

Reuse the original UUID when retrying a start/set create after an uncertain network result. For a session-start replay, compare the template ID; never recopy a changed template. Retain the request identifier with the resource. Do not retry creates with fresh UUIDs automatically. Handle uniqueness conflicts inside the transaction without leaving partial data.

### Validation and errors

Trim names and require nonempty values with a proposed maximum of 100 characters. Reject malformed IDs/UUIDs, unknown JSON fields, unsupported content types, out-of-range numbers, nonexistent exercises, and invalid session-exercise membership. Limit JSON request bodies and return field-level messages that the frontend can show next to inputs.

Use a consistent error envelope:

```json
{
  "error": {
    "code": "validation_error",
    "message": "Check the set values.",
    "fields": { "reps": "Must be a positive integer." }
  }
}
```

Use `400` for malformed/invalid input, `404` for missing resources, `409` for state or uniqueness conflicts (including completed-session edits and an existing active workout), `413` for oversized bodies, `415` for unsupported content types, and `500` for unexpected failures. Log diagnostic detail server-side without returning SQL errors or credentials. Configure HTTP timeouts, request-context cancellation, and graceful shutdown.

## Proposed local operation

Use Docker Compose for the frontend, Go API, and PostgreSQL. Publish the frontend port to localhost; forward `/api` to the internal Go service. PostgreSQL and the API need no host-published ports for ordinary use. Use a named database volume and environment-based configuration; keep database credentials server-side and out of Git.

Run migrations explicitly before serving traffic and check database readiness rather than relying only on container startup order. A database volume provides persistence, not a backup; define a simple export/restore procedure before storing important workout history. With no authentication, the intended deployment is local or on a private network; anyone who can reach the app can change its data.

The local foundation uses npm, Node.js 24.19.0, Go 1.27.2, and PostgreSQL 18.6. The user selected a local Docker run with rebuilds after changes, without hot reload. The Next.js server forwards `/api/exercises` GET/POST, `/api/health`, and `/api/ready` to Go using a runtime server-only `API_INTERNAL_URL`; additional routes will be added with workout features. The Go server uses `net/http` and a `pgx` pool, a database-independent liveness endpoint, and a two-second database readiness check. Database connection details are constructed from server-only environment variables. The frontend is published on localhost; Go and PostgreSQL stay on the internal Compose network. PostgreSQL data uses a named volume mounted at `/var/lib/postgresql` for the PostgreSQL 18 image layout.

Dockerfiles, Compose configuration, health checks, and setup/check commands now exist and are documented in the README. The API applies embedded versioned exercise migrations transactionally before serving traffic, with an advisory lock to serialize starts. The migration seeds internal owner 1 and exercise rows reference it. Other workout schema choices remain proposals. CI performs frontend checks/build, Go checks/tests/build, and an isolated stack smoke check covering readiness failure/recovery and database restart persistence. It requests only read access to repository contents.

## Validation planned for implementation

- Service tests for validation, request replay, state transitions, and previous-performance selection across templates.
- PostgreSQL integration tests for atomic snapshots, template edits/deletion retaining history, decimal values, duplicate-create retries, and concurrent completion/set writes. Test concurrent session starts if the one-active rule is accepted.
- HTTP tests for the documented request/response shapes, field errors, and status codes.
- A browser flow that creates exercises/template, starts a session, records a set, refreshes, completes the workout, and verifies history and next-session references. Verify suggested rows are not recorded and failed saves remain visible.
- Frontend type checking/build and Go formatting/tests once the project tooling exists. Use a separate test database and never reset the user's workout database for tests.

The existing Go tests cover configuration validation, credential encoding, health responses, and readiness timeout behavior. The Docker smoke check covers the implemented stack. The workout-oriented tests above remain planned until those features exist.

## Product decisions still needed

| Decision | Proposed default for review |
| --- | --- |
| Initial weight/reps | Prefill from previous completed performance as unrecorded suggestions; leave unmatched rows blank |
| Planned rows | Positive planned set count per template exercise; extra sets can be recorded |
| Active workouts | One at a time, enforced in PostgreSQL |
| Units | Kilograms, at most two decimal places |
| Save behavior | Explicitly record new sets; autosave valid edits to recorded sets |
| Previous performance | Most recent completed session containing recorded sets for the exercise, regardless of template |
| Completion | Permit completion with fewer than the planned sets, including an empty session; no fabricated sets |

Confirm these in the product document before coding affected flows. Editing completed workouts, cancellation/resume rules beyond an active session, and exercise renaming/deletion can be specified separately if needed. No proposed default in this document overrides a later approved product requirement.

## Technical references

- [Next.js App Router documentation](https://nextjs.org/docs/app)
- [Go net/http documentation](https://pkg.go.dev/net/http)
- [pgx documentation](https://pkg.go.dev/github.com/jackc/pgx/v5)
- [PostgreSQL partial indexes](https://www.postgresql.org/docs/current/indexes-partial.html)
