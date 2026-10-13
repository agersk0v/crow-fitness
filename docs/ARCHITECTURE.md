# Architecture

[PRODUCT.md](PRODUCT.md) owns approved requirements and unresolved product decisions. [README.md](../README.md) owns setup, API examples, and check commands. The implemented design below reflects the current repository; the session design is a proposal, not an implementation commitment.

## Implemented components

| Component | Responsibility | Location |
| --- | --- | --- |
| Next.js / TypeScript / Tailwind | Connection checks, exercise search/tags/edit/delete, template creation/list/detail, themes and accessible UI | `frontend/app/`, `frontend/components/`, `frontend/lib/` |
| Go `net/http` | Configuration, HTTP decoding, validation, ownership, persistence and graceful shutdown | `backend/cmd/api/`, `backend/internal/config/`, `health/`, `exercises/`, `templates/` |
| PostgreSQL / `pgx` pool | Durable records, foreign keys, constraints and transactions | SQL in the Go feature packages; `backend/internal/migrations/` |

The browser calls relative `/api` URLs. The server-only Next.js forwarding route uses runtime `API_INTERNAL_URL` and allows exercise/template collection GET/POST, template-detail GET, exercise PUT/DELETE and template-usage GET, and health/readiness GET. Responses are uncached. Only Go accesses PostgreSQL; database credentials stay server-side. Go sets request deadlines and server timeouts, and returns generic persistence errors while logging diagnostic details.

The local Docker stack publishes only the frontend on localhost. Go and PostgreSQL remain on the internal Compose network; PostgreSQL 18 data uses a named volume at `/var/lib/postgresql`. Dockerfiles and CI pin the supported runtime versions. Rebuilds, rather than source mounts, apply local code changes. With no login or authentication, anyone who can reach this app can operate on its data. The internal owner is not an access-control mechanism.

## Implemented data and migrations

IDs are generated bigint values serialized as decimal strings to avoid JavaScript integer precision loss. Go selects internal local owner 1 for every operation; clients cannot select an owner.

| Table | Main columns | Constraints |
| --- | --- | --- |
| `owners` | `id`, `label` | Seeded local owner |
| `exercises` | `id`, `owner_id`, `name`, `muscle_groups`, `created_at` | Owner foreign key, nonblank name, predefined tag constraint, unique `(owner_id, id)` |
| `templates` | `id`, `owner_id`, `name` | Owner foreign key, nonblank name, unique `(owner_id, id)` |
| `template_exercises` | `template_id`, `owner_id`, `position`, `exercise_id`, `set_count`, `rep_min`, `rep_max`, `target_rir` | Primary key `(template_id, position)`; composite foreign keys enforce common ownership; positive positions/counts; paired positive ordered rep bounds; nullable nonnegative finite RIR |

Template array order becomes one-based positions. Repeated exercise occurrences are allowed. Counts and rep bounds use bigint; RIR uses numeric without a fixed scale. Optional targets remain null, distinct from zero. Product rules for these targets are documented in PRODUCT.md.

Go validates and locks selected exercises before writing a template and all its occurrences in one transaction. Missing and foreign-owner exercises return the same field error. Failed writes roll back the entire template. Collection reads return ID/name summaries; detail and create responses return ordered configuration with exercise names. Template editing/deletion are not implemented.

Exercise create/update normalizes tags in vocabulary order and removes duplicates; PostgreSQL checks tag membership. Go returns the vocabulary with the exercise collection; Next.js filters the loaded list by name and one group. Update preserves the exercise ID. Usage reads return distinct referencing templates scoped to the owner. Delete locks the exercise `FOR UPDATE`, checks references, and deletes only when unused. This conflicts with template creation's `FOR KEY SHARE` lock: whichever operation commits first determines whether creation or deletion is rejected. Foreign keys remain the final reference protection. A conflict includes template links for the UI; missing/foreign records return the same 404. Native dialogs provide modal focus and Escape/Cancel behavior.

Embedded numbered SQL migrations run before Go serves traffic. A transaction and PostgreSQL advisory lock serialize concurrent starts; `schema_migrations` records applied versions. Reruns skip applied files and preserve records. Migration failure stops startup. Operational commands and data-preservation guidance are in README.md; add new migrations rather than changing applied SQL. A volume provides persistence, not backup; define an export/restore procedure before storing important workout history.

## Current validation coverage

Go tests cover configuration, credential encoding, health/readiness timeouts, HTTP decoding, business validation, ownership selection, and generic errors. Disposable PostgreSQL tests cover constraints, owner isolation, ordered target readback, migration reruns, and transaction rollback.

CI retains formatting, Go vet/race tests, and a Docker stack job. The Dockerfiles perform the production builds; Next.js checks TypeScript during its build. The smoke script checks a representative exercise/template flow through forwarding, a validation response, health/readiness outage recovery, and persistence through database and whole-stack restarts. HTTP page fetches are not interactive browser tests. Local commands and test-database setup are in README.md.

## Proposed sessions and logging

Nothing in this section is implemented. Resolve the remaining PRODUCT.md decisions before implementing the affected behavior. Keep one frontend, one Go API, and one database; introduce additional services only for an approved need.

### Proposed records and history retention

- `workout_sessions`: generated ID, owner, nullable template reference, copied template name, start/finish timestamps, active/completed status, and a unique client request UUID.
- `session_exercises`: session reference, stable exercise ID, copied exercise name/configuration, and occurrence position. Copy the approved template targets rather than rereading a template later.
- `performed_sets`: session-exercise reference, set number, weight, reps, and client request UUID. Enforce unique occurrence/set numbers and occurrence/request UUIDs.

Store timestamps in UTC and display them in the browser's local timezone. Require a finish timestamp only for completed sessions, no earlier than the start. If one active session is approved, enforce it per owner with a partial unique index. If kilograms with two decimal places is approved, `numeric(8,2)` and decimal-string JSON are proposed for weights; reject excess precision/range rather than silently rounding. Add indexes for real history joins and queries.

Session snapshots preserve history through template changes. A future template deletion should clear the session's template reference while retaining its copied configuration. Block deletion of referenced exercises. Session deletion and editing completed sessions require separate requirements.

### Proposed transactions

1. **Start:** accept a template ID and request UUID. Lock the owner's template, read its ordered configuration, and create the session and snapshot in one transaction. Future template edits/deletions must use the same lock. Reject missing/empty templates. A retry of the same request must reuse the existing session, compare the template ID, and never recopy a changed template.
2. **Reference performance:** if approved, select the most recent completed session containing recorded sets for each exercise, ordered by completion time then session ID. Include repeated occurrences in source occurrence/set order. Resolve the mapping into suggestions before implementing prefilling.
3. **Record:** lock the parent session, verify ownership, active status and occurrence membership, then insert valid weight/reps. A request UUID identifies a logical write; replay the same payload, reject changed payloads, and enforce uniqueness. Replay lookup precedes the active-status check so a lost response can be recovered after completion.
4. **Edit/remove:** acquire the same parent-session lock and require active status. Preserve other set numbers. If autosave is approved, serialize valid edits per set and retain failed drafts for retry.
5. **Complete:** wait for pending saves, then lock the session and persist completion once. Replays keep the existing finish time. The shared lock orders completion against writes. Resolve failed saves before finishing; never silently convert suggestions into performed sets.

Suggested values remain separate from performed records. Future template updates must validate and replace ordered configuration atomically. Generic writes must not bypass session-state or ownership rules. Offline draft persistence is outside the proposed MVP.

### Proposed API and validation

| Method | Endpoint | Purpose |
| --- | --- | --- |
| PUT, DELETE | `/api/templates/{id}` | Edit/delete a template |
| GET, POST | `/api/sessions` | List/start sessions |
| GET | `/api/sessions/{id}` | Snapshot, sets and reference performance |
| POST | `/api/sessions/{id}/sets` | Record a set |
| PATCH, DELETE | `/api/sets/{id}` | Edit/remove an active-session set |
| POST | `/api/sessions/{id}/complete` | Complete a session |
| GET | `/api/exercises/{id}/history` | Sets grouped by workout |

Use JSON and the existing collection/resource/error conventions. Proposed history pagination uses a default limit of 50, maximum 100, and an opaque cursor with stable session-time/ID, occurrence-position and set-number ordering. Progress scope and previous-performance selection remain conditional on PRODUCT.md approval.

Validate IDs/UUIDs, membership, content type, body size, unknown fields and numeric values. A 100-character name limit remains a proposal, not a current restriction. Use field errors for validation, 404 for missing resources, 409 for state/request conflicts, and generic errors for unexpected failures. Proposed successful responses are 200 for reads/updates/replays, 201 for new resources, and 204 for deletion. Retain the original request UUID when retrying uncertain starts/set writes; do not generate fresh UUIDs automatically.

Future tests should cover snapshots surviving template changes, request replay, reference selection, concurrent completion/writes, and concurrent starts if the one-active rule is approved. Interactive browser flows should cover session logging and visible save states when those features exist.

## Technical references

- [Next.js App Router](https://nextjs.org/docs/app)
- [Go net/http](https://pkg.go.dev/net/http)
- [pgx](https://pkg.go.dev/github.com/jackc/pgx/v5)
- [PostgreSQL partial indexes](https://www.postgresql.org/docs/current/indexes-partial.html)
