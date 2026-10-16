# Architecture

This document describes the implemented technical design. [PRODUCT](PRODUCT.md) owns user-visible rules; [README](../README.md) owns setup, configuration, operational data care, and check commands. Open issues/PRs describe future changes, not deployed contracts.

## Components and boundaries

| Component | Responsibility | Location |
| --- | --- | --- |
| Next.js, TypeScript, Tailwind | Pages, drafts, accessible controls, theme, server-side API forwarding | `frontend/app/`, `frontend/components/`, `frontend/lib/` |
| Go `net/http` | Decoding, validation, fixed owner selection, workout rules, transactions, shutdown | `backend/cmd/api/`, `backend/internal/` feature packages |
| PostgreSQL / pgx | Durable data, constraints, locks, exact numeric values | SQL in feature packages and `backend/internal/migrations/` |

The browser uses relative `/api` URLs. Next's server-only allowlisted forwarding route uses runtime `API_INTERNAL_URL`; responses are uncached and preserve upstream JSON/status/Location. An unreachable Go API returns generic `502 api_unavailable`. Go uses request deadlines/server timeouts and logs diagnostic errors while returning generic persistence failures. Only Go accesses PostgreSQL; credentials stay server-side.

Compose publishes only the frontend, bound to localhost. Go/PostgreSQL use the internal network; PostgreSQL 18 stores data in a named volume at `/var/lib/postgresql`. Dockerfiles and CI pin runtime versions. Rebuilds apply source changes; there are no development source mounts. One implicit owner (ID 1) is selected in Go; clients cannot supply identity. This is not authentication.

The root layout owns shared URL-aware navigation, theme provider, and one theme-aware Sonner Toaster. `next-themes` persists theme choice; the toggle waits for mounting to avoid hydration mismatch. shadcn/ui components are committed source, with shared Tailwind tokens/class merging. Motion entrance effects leave SSR content visible and respect reduced motion. Existing dnd-kit sensors ignore interactive descendants, support mouse/touch/keyboard, and are shared by template occurrence and session-set sorting.

## Persistence and migrations

Generated bigint IDs are JSON decimal strings to avoid JavaScript integer precision loss. Snapshot identity/order is independent of mutable library metadata. Timestamps are stored as instants and displayed in browser-local time. Numeric weight/RIR read values are exact decimal strings, not floating-point calculations.

| Table | Purpose and important constraints |
| --- | --- |
| `owners` | Seeded internal local owner |
| `exercises` | Owner, nonblank name, description, predefined muscle-group array; empty arrays remain valid for legacy storage |
| `templates` | Owner, nonblank name, description, archived flag |
| `template_exercises` | One-based position, exercise identity, positive working-set count, paired positive ordered rep bounds, nullable nonnegative numeric target RIR; repeated identities allowed |
| `workout_sessions` | Captured template identity/name, start and nullable completion time; partial unique index permits one unfinished session per owner |
| `session_exercises` | Captured exercise identities/names, positions, counts, and targets; no description snapshot |
| `session_sets` | Composite session/UUID identity, occurrence membership, exact weight, positive full reps, nullable positive partial-rep count, nullable numeric recorded RIR, required technique (`good`/`decent`/`bad`), saved timestamp, set order, deletion marker |
| `schema_migrations` | Applied embedded migration versions |

Composite foreign keys enforce common owner/session membership. Snapshot rows retain exercise identity/names without requiring a live library record. Cancelling an active session cascades its snapshot and sets. Soft-deleted set tombstones prevent delayed retries from resurrecting removed records.

Embedded numbered migrations run before serving requests, with transactional version recording and an advisory lock serializing starts. Applied versions are skipped; failures stop startup. Add new migrations, never rewrite applied SQL. Migrations 005/006 preserve existing active sessions while introducing retained completed workouts, 007 adds sets, 008 descriptions, 009 nullable recorded RIR, 010 backfills explicit set order from saved-time/UUID order, 011 adds constrained nonnull technique with Decent as the backfill/default, and 012 adds optional partial counts without backfilling invented values. Normal restarts do not reset data. Volume persistence is not backup; define an export/restore procedure before relying on important history.

## Writes and concurrency

Exercise create/update trims names/descriptions, normalizes tags in vocabulary order, removes duplicate tags, and requires at least one tag in Go. Usage reads return distinct referencing templates including archived state. Delete locks the exercise `FOR UPDATE`; template writes lock selected exercises `FOR KEY SHARE`, with foreign keys as final protection against broken references.

Template create/update validates all selected exercises and replaces the complete ordered configuration in one transaction. Edits lock the parent; concurrent full configurations apply in commit order without draft merging. Duplicate locks the source and copies saved descriptions/occurrences/targets directly in SQL without numeric rounding. Archive/restore lock that parent and preserve references; archived edits/copies are rejected. Detail reads use read-only repeatable read to avoid mixed configurations.

Session start locks the owner `FOR NO KEY UPDATE`, then the source template `FOR SHARE` against editing/archiving, and copies configuration atomically. Existing active state yields a conflict containing its snapshot. Active/detail reads use repeatable read for configuration and sets together.

Set PUT/DELETE/order writes lock the session row and require active state. PUT inserts/replaces by the same lowercase UUID, preserving occurrence membership and order on edits. New rows append and check the captured per-occurrence set cap under that lock; existing IDs permit edits/retries at the cap. Deleted UUIDs cannot be reused. Order requires an exact permutation of surviving sets in one occurrence, saved atomically. Readers order by `set_order`, then saved timestamp/UUID.

Completion locks the owner and updates the session once, preserving the first completion time and serializing with set changes. Backend completion permits partial/empty workouts; the frontend's missing-set warning uses a fresh active snapshot. Cancel only deletes the supplied owner's active session; stale endings cannot affect completed/newer sessions. Completed sets remain read-only.

The frontend keeps one UUID for an uncertain addition, independent Add/edit drafts, and the desired permutation for failed order retries. Pending/failed writes and open edits block completion. An explicit 400 validation response with `fields.set_count` for a new addition clears its uncertain-attempt state and exposes occurrence-local saved-set recovery. A successful active-snapshot GET replaces only that occurrence’s local saved sets and clears its rejected Add draft (including partials and resetting technique to Decent); failed GETs retain it. Other occurrence components remain mounted, preserving their drafts/blockers. Network/server failures still retry the same UUID. Reference requests do not remount inputs; a one-time initialization flag prevents late previous-weight data from overwriting manual values. Native dialogs handle confirmations/edits, Escape, and focus restoration. Template drafts are component-local and discarded on navigation, without an unload guard.

## API contracts

All paths below start with `/api`. Resource IDs are positive decimal strings. JSON writes validate content type, size, unknown fields, identity, membership, and business values. Exercises have a 16 KiB body limit; templates have 64 KiB. Descriptions are strings on reads; omitted/null/blank input normalizes to `""`, including clearing on full PUT. No separate description character limit is imposed.

| Methods | Path | Contract |
| --- | --- | --- |
| GET | `/health`, `/ready` | Liveness independent of DB; readiness checks DB with timeout, `503` when unavailable |
| GET, POST | `/exercises` | List items plus muscle-group vocabulary; create exercise |
| PUT, DELETE | `/exercises/{id}` | Replace name/tags/description; delete only if unreferenced |
| GET | `/exercises/{id}/templates` | Distinct active/archived referencing templates |
| GET, POST | `/templates` | Active list (or `?archived=true`); create full configuration |
| GET, PUT | `/templates/{id}` | Read or replace full ordered configuration |
| POST | `/templates/{id}/duplicate` | Named copy of saved configuration |
| POST | `/templates/{id}/archive`, `/restore` | Repeatable state actions; return resource |
| POST | `/sessions` | Start with `{"template_id":"1"}`; conflict supplies existing active session |
| GET | `/sessions/active` | `{"session":null}` or saved active snapshot |
| GET | `/sessions/{id}` | Active snapshot and surviving sets only |
| POST | `/sessions/{id}/complete`, `/cancel` | Bodyless lifecycle actions, `204` |
| PUT, DELETE | `/sessions/{id}/sets/{uuid}` | Save/replace or soft-delete a set in the active session |
| PUT | `/sessions/{id}/sets/order` | `{"exercise_position":1,"set_ids":["..."]}` exact permutation |
| GET | `/sessions/{id}/previous`, `/bests` | Per-exercise references for an owner-scoped active session |
| GET | `/sets` | Completed nondeleted set collection, including captured session/exercise names and identities |
| GET | `/history/sessions`, `/history/sessions/{id}` | Completed summaries (including empty workouts) or read-only snapshot |
| GET | `/history/exercises`, `/history/templates` | Completed-snapshot filter identities with latest captured names |
| POST | `/dev/reset` | Explicitly enabled confirmed data replacement; otherwise `404` |

Create returns `201` and, for templates/sessions, Location; reads/updates return `200`; successful deletes/end actions return `204`. Errors use `{"error":{"code":"...","message":"...","fields":{...}}}` with fields where relevant: validation `400`, missing/foreign resource `404`, state/reference conflicts `409`, oversized body `413`, unsupported media type `415`, generic persistence `500`. Not all errors include fields. Template Delete is a UI label for archive; there is no permanent template DELETE endpoint.

Template writes send name, optional description, and ordered `exercises` with `exercise_id`, required `set_count`, nullable `rep_min`/`rep_max`, and nullable numeric `target_rir`. Array order becomes one-based position. Counts/reps are whole numbers; RIR is finite nonnegative numeric without a product-specific ceiling/rounding scale, subject to PostgreSQL numeric limits.

Example set PUT body:

```json
{"exercise_position":1,"weight":"12.50","reps":8,"rir":1.5,"technique":"decent","partial_reps":2}
```

Weight is a nonnegative decimal string with at most two fractional digits; excess precision is rejected rather than rounded. RIR accepts a nonnegative JSON number or null, and omission clears it on replacement. Read payloads return weight as a string and RIR as a nullable decimal string. Technique must be an explicit lowercase `good`, `decent`, or `bad` on every PUT; missing, null, empty, unknown, or wrong-type values return 400. The database default backfills existing rows and supports internal legacy SQL inserts; it does not supply omitted HTTP values. Set JSON bodies are limited to 4 KiB. Optional `partial_reps` is a JSON integer; omitted/null/zero clears it on full PUT and reads return null or a positive count. Negative, fractional, wrong-type, or unsafe JavaScript integer counts are rejected with 400; storage constrains positive counts to the same safe-integer range. Full reps remain independent and required. Position cannot change for an existing UUID.

The legacy `/exercises/{id}/history` API is removed from both Go and Next, along with its dedicated query/models/store methods and tests. It returns 404, as does the removed `/exercises/[id]/history` page. Exercise cards use filtered Sets history; completed snapshots and reference readers remain available.

## Historical reads, filters, and comparisons

Historical reads use captured names and owner-scoped identities. Completed snapshots are repeatable-read, with explicit completed-state selection; no mutable library join rewrites results. Lists order completion timestamp and numeric session ID descending, then occurrence and persisted set order.

`previous` selects the newest completed source with eligible sets per exercise; `bests` selects the workout containing the heaviest set, reps then Good > Decent > Bad technique breaking ties, then returns all exercise occurrences/sets from that workout. Both verify active membership and read in one read-only repeatable-read transaction. Their payloads contain exercise identity, source session/completion, and ordered occurrences; exercises without eligible history are omitted. They never write or prefill recorded sets.

History parses `q`, `exercise`, `template`, `session`, `from`, `to`, and `tz` into parameterized SQL criteria. Search is literal case-insensitive substring; session membership uses EXISTS to avoid repeated cards. Browser URLs additionally carry `tab` and retain both identity choices, while requests send exercise for Sets or template and optional exact completed-session identity for Sessions. The `session` selector is a positive bigint scoped to the owner and combined with the other criteria. Inclusive dates become start-inclusive/next-day-exclusive instants in the supplied IANA timezone (UTC if absent). Embedded Go timezone data supports DST in the minimal image. Malformed/repeated/unknown criteria, reversed dates, and invalid zones fail validation; valid unknown IDs yield no results.

Per-set `change: {weight, reps, technique, previous_technique}` uses the shared PostgreSQL comparison CTE to select the baseline before display filters. Match exercise identity, chronological prior workout, repeated occurrence ordinal, and surviving set ordinal. The prior workout need not contain saved sets, unlike reference selection. Exact signed decimal/integer strings avoid browser floating-point subtraction. Technique uses that same matched baseline: Go returns `improved`, `regressed`, or `unchanged` plus the previous rating, using Good > Decent > Bad. With no matched set there is no comparison object. Changes are not stored; RIR and partial counts are not compared. Partial counts are selected for display only and never enter comparison arithmetic, eligibility, or the personal-best ORDER BY.

Next uses URL-applied criteria separately from form drafts, with keyed/abortable results to prevent stale responses. Sessions loads snapshots per workout with independent retry; exercise accordions are local. Sets groups only matching rows. Sets workout links create a fresh `/history?tab=sessions&session=<id>` URL, clearing unrelated criteria. Apply/tab navigation retains that selector; Sets ignores it, and Clear workout filter removes only it. Direct completed detail remains supported, and `/sets` redirects to Sets history.

## Temporary reset and operational safety

`ENABLE_DEV_RESET` defaults false in Go and Next runtime configuration. When enabled, `/dev/reset` additionally requires `X-Confirm-Reset: replace-workout-data`. The isolated `devdata` package replaces only the selected owner's workout rows in one transaction, taking table locks to order concurrent writes. It preserves schema/migrations/owner/sequences; ordinary readers see committed data and failure rolls back. The response reports dataset counts and active-session identity. Synthetic descriptions/weights/reps exercise ordinary APIs; optional targets and recorded RIR remain unset. A deterministic technique cycle and paired week weight/rep values expose quality changes and equal-performance tie-breaking without changing dataset counts. Optional seeded partial counts vary independently and do not change recorded weights/full reps/technique. The feature can be removed through its registration/configuration/UI without changing normal workout rules. Enablement and lost-response handling are in README.

## Validation responsibilities

Go tests own detailed decoding/validation, ownership, transactions, constraints, migration upgrades, concurrent lifecycle/set writes, target/description round trips, history filtering/source selection, exact changes, and reset atomicity. PostgreSQL tests require a separate disposable database and explicitly skip without `TEST_DATABASE_URL`.

The smoke script builds production images and checks one browser-origin API flow through Next forwarding, frontend route availability, validation/conflict propagation, active saved-set restoration after database and full-stack restarts, completed reads/references, cancellation, and liveness/readiness outage recovery. It owns a fresh randomly named Compose project and volume, prints service logs on failure, and removes only its disposable resources. Page fetches do not verify interactive browser behavior.

CI retains frontend formatting, Go formatting/vet/race tests with PostgreSQL, and the dependent production-stack smoke job. Next's production build checks TypeScript. Commands belong in README; no additional test framework, service, or CI policy is required.
