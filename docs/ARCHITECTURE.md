# Architecture

[PRODUCT.md](PRODUCT.md) owns approved requirements and unresolved product decisions. [README.md](../README.md) owns setup, API examples, and check commands. The implemented design below reflects the current repository; session/set persistence, completed-workout history, and previous-performance references are implemented; exercise progress remains proposed.

## Implemented components

| Component | Responsibility | Location |
| --- | --- | --- |
| Next.js / TypeScript / Tailwind | Connection checks, exercise search/tags/edit/delete, template creation/edit/duplicate/archive/restore, sessions and set entry/list, themes and accessible UI | `frontend/app/`, `frontend/components/`, `frontend/lib/` |
| Go `net/http` | Configuration, HTTP decoding, validation, ownership, persistence and graceful shutdown | `backend/cmd/api/`, `backend/internal/config/`, `health/`, `exercises/`, `templates/`, `sessions/` |
| PostgreSQL / `pgx` pool | Durable records, foreign keys, constraints and transactions | SQL in the Go feature packages; `backend/internal/migrations/` |

The browser calls relative `/api` URLs. The server-only Next.js forwarding route uses runtime `API_INTERNAL_URL` and allows exercise/template collection GET/POST, template-detail GET/PUT and duplicate/archive/restore POST, exercise PUT/DELETE and template-usage GET, session start POST, active/detail GET and cancel/complete POST, session-set PUT/DELETE, previous-performance GET, completed-session list/detail GET, and completed-sets GET, and health/readiness GET. Responses are uncached. Only Go accesses PostgreSQL; database credentials stay server-side. Go sets request deadlines and server timeouts, and returns generic persistence errors while logging diagnostic details.

The local Docker stack publishes only the frontend on localhost. Go and PostgreSQL remain on the internal Compose network; PostgreSQL 18 data uses a named volume at `/var/lib/postgresql`. Dockerfiles and CI pin the supported runtime versions. Rebuilds, rather than source mounts, apply local code changes. With no login or authentication, anyone who can reach this app can operate on its data. The internal owner is not an access-control mechanism.

## Implemented data and migrations

IDs are generated bigint values serialized as decimal strings to avoid JavaScript integer precision loss. Go selects internal local owner 1 for every operation; clients cannot select an owner.

| Table | Main columns | Constraints |
| --- | --- | --- |
| `owners` | `id`, `label` | Seeded local owner |
| `exercises` | `id`, `owner_id`, `name`, `muscle_groups`, `created_at` | Owner foreign key, nonblank name, predefined tag constraint, unique `(owner_id, id)` |
| `templates` | `id`, `owner_id`, `name`, `archived` | Owner foreign key, nonblank name, unique `(owner_id, id)` |
| `template_exercises` | `template_id`, `owner_id`, `position`, `exercise_id`, `set_count`, `rep_min`, `rep_max`, `target_rir` | Primary key `(template_id, position)`; composite foreign keys enforce common ownership; positive positions/counts; paired positive ordered rep bounds; nullable nonnegative finite RIR |

Template array order becomes one-based positions. Repeated exercise occurrences are allowed. Counts and rep bounds use bigint; RIR uses numeric without a fixed scale. Optional targets remain null, distinct from zero. Product rules for these targets are documented in PRODUCT.md.

Go validates and locks selected exercises before writing a template and all its occurrences in one transaction. Missing and foreign-owner exercises return the same field error. Failed writes roll back the entire template. Collection reads return ID/name summaries; detail and create responses return ordered configuration with exercise names. Edits lock the parent template, validate/lock selected exercises, and replace its name and ordered occurrences in one transaction. Duplication locks the source, rejects archived state, and copies occurrences directly in SQL into a new identity without decimal rounding. Archive/restore lock the same parent, preserve references, and safely repeat the same state transition. Lists select active or archived records explicitly; details use a read-only repeatable-read transaction to prevent mixed name/configuration snapshots. Concurrent edits apply whole configurations in commit order. Permanent template deletion is unavailable.

Exercise create/update requires at least one predefined muscle group, normalizes tags in vocabulary order and removes duplicates; PostgreSQL checks tag membership. Empty arrays remain valid in storage for legacy records; Go enforces the minimum on every create/update without backfilling existing data. Go returns the vocabulary with the exercise collection; Next.js filters the loaded list by name and one group. Update preserves the exercise ID. Usage reads return distinct referencing templates and their archived status scoped to the owner. Delete locks the exercise `FOR UPDATE`, checks references, and deletes only when unused. This conflicts with template creation's `FOR KEY SHARE` lock: whichever operation commits first determines whether creation or deletion is rejected. Foreign keys remain the final reference protection. A conflict includes template links for the UI; missing/foreign records return the same 404. Native dialogs provide modal focus and Escape/Cancel behavior.

Embedded numbered SQL migrations run before Go serves traffic. A transaction and PostgreSQL advisory lock serialize concurrent starts; `schema_migrations` records applied versions. Reruns skip applied files and preserve records. Migration failure stops startup. Operational commands and data-preservation guidance are in README.md; add new migrations rather than changing applied SQL. A volume provides persistence, not backup; define an export/restore procedure before storing important workout history.

## Implemented sessions and sets (#31 / #33)

Migration 005 introduced active snapshots. Migration 006 renames them to `workout_sessions` / `session_exercises`, adds nullable `completed_at`, and replaces owner uniqueness with a partial unique index for unfinished sessions. Existing active rows and their identities/configuration are preserved. Migration 007 adds `session_sets`: composite `(session_id, UUID)` identity, captured occurrence reference, exact decimal weight, positive reps, saved order timestamp, and a deletion marker. Foreign keys enforce snapshot membership and owner/session consistency. Cancelling cascades sets and occurrence rows.

Start locks the owner with `FOR NO KEY UPDATE`, compatible with template creation/duplication foreign-key checks, then locks the source template `FOR SHARE` against edits/archiving. It copies names/configuration atomically. An existing active session yields 409 and its snapshot for continuation. Active/detail reads use repeatable read and return saved sets from the same snapshot.

Set PUT locks the parent session, requires active state, validates occurrence ownership and decimal-string weight (up to two fractional digits), and inserts/replaces by the same UUID. Position is immutable for an existing set. Numeric has no fixed scale or product-specific ceiling; constraints reject excess scale instead of rounding. Deleted rows remain as tombstones so delayed PUT retries cannot recreate a removed set. Active concurrent edits apply in commit order; there is no draft merging. Frontend drafts keep their UUID until Save succeeds or server deletion confirms discard.

Completion locks the owner and updates the parent session once, ordering against set writes/deletion. It preserves the first completion time and snapshot/results and releases the active slot. Writes that race afterward are rejected. Cancel deletes only the supplied owner's active session; repeated/stale endings cannot affect a completed workout or newer active session. A completed-set query joins copied occurrence names and includes only nondeleted sets from completed workouts, with stable completion/session/occurrence/set ordering. The UI blocks completion for drafts/pending/failed writes, offers native cancel confirmation, and has a completed-sets tab in `/history`. Analytics and editing completed records remain outside scope.

## Implemented previous-performance references (#35)

`GET /api/sessions/{id}/previous` verifies the owner's active session and reads its membership and historical references in one read-only repeatable-read transaction. One SQL query selects the latest completed source per distinct current exercise identity that has nondeleted sets, then returns all its recorded matching source occurrences/sets. Completion timestamp/session ID break recency ties; source occurrence position, saved timestamp, and UUID order sets. Queries use captured names and decimal-string weights, not mutable library names. No schema change or writes are needed.

The UI makes one independent request for the session and shares results across repeated current occurrences. Its retry/error state leaves current set components and drafts mounted. References have their own read-only panel and never supply input values. Empty history has a distinct state. Tests cover source selection/exclusion, identity and owner scope, repeated occurrences, exact values, deterministic ties, reconnect, and unchanged current/history data.

## Implemented workout history (#37)

Go exposes `GET /api/history/sessions` and `/api/history/sessions/{id}`. The list includes all completed workouts for the local owner, including empty workouts, ordered by completion time then session ID descending. Detail reuses the read-only repeatable-read snapshot reader with an explicit completed-state filter. Active detail stays active-only. Detail returns captured configuration and nondeleted saved sets from the same snapshot and never joins mutable library names.

Next.js serves `/history` with Sessions/Sets tabs controlled by the URL's `tab` selector. Sets is first/default under #57; missing/unknown selectors fall back to it, while `tab=sessions` selects Sessions. Roving keyboard focus uses arrows/Home/End, and only the selected tab's data component is mounted. Cards show ordered exercise lists with independent accordions and compact targets (revised #57); `/history/[id]` remains available for direct links. Legacy `/sets` redirects to `/history?tab=sets`. Existing set data/entry and all mutation/state rules remain intact. No schema migration, extra dependency, or service is needed.

## Current validation coverage

Go tests cover configuration, credential encoding, health/readiness timeouts, HTTP decoding, business validation, ownership selection, and generic errors. Disposable PostgreSQL tests cover constraints, owner isolation, ordered target readback, migration reruns, and transaction rollback.

CI retains formatting, Go vet/race tests, and a Docker stack job. The Dockerfiles perform the production builds; Next.js checks TypeScript during its build. The smoke script checks a representative exercise/template/session flow through forwarding, a validation response, health/readiness outage recovery, and persistence through database and whole-stack restarts. HTTP page fetches are not interactive browser tests. Local commands and test-database setup are in README.md.

## Implemented exercise set history (#36)

`GET /api/exercises/{id}/history` reads the local owner's exercise identity and all completed nondeleted sets in a read-only repeatable-read transaction. The library name takes precedence; a completed snapshot supplies a fallback identity/name after library removal. Active-only snapshots do not expose a removed identity. Completed snapshots with no eligible sets remain readable as empty history.

The query scopes ownership and stable exercise ID, orders workouts by completion time and numeric session ID descending, then occurrences by source position and sets by saved time/UUID. Grouping preserves captured workout/occurrence names and decimal-string weights. No schema change or mutation is needed. The dedicated exercise-history UI and completed-workout links are removed under #57; library cards use Workout history filtered to the exercise in Sets. The backend read endpoint remains available without data changes. Identity and snapshot retention tests cover renames, template edits/archives, removal, reconnect, exclusions, and unchanged active data.

## Proposed future reference performance and history

This section proposes later expansion beyond the implemented set-entry/completion flow. The remaining proposals do not override #33. Resolve the remaining PRODUCT.md decisions before implementing the affected behavior. Keep one frontend, one Go API, and one database; introduce additional services only for an approved need.

### Proposed records and history retention

- `workout_sessions`: generated ID, owner, nullable template reference, copied template name, start/finish timestamps, active/completed status, and a unique client request UUID.
- `session_exercises`: session reference, stable exercise ID, copied exercise name/configuration, and occurrence position. Copy the approved template targets rather than rereading a template later.
- `performed_sets`: session-exercise reference, set number, weight, reps, and client request UUID. Enforce unique occurrence/set numbers and occurrence/request UUIDs.

Store timestamps in UTC and display them in the browser's local timezone. Require a finish timestamp only for completed sessions, no earlier than the start. When retaining completed sessions, preserve the approved one-active rule per owner with a partial unique index. If kilograms with two decimal places is approved, `numeric(8,2)` and decimal-string JSON are proposed for weights; reject excess precision/range rather than silently rounding. Add indexes for real history joins and queries.

Session snapshots preserve history through template changes. Template archiving retains references; future sessions must keep their copied configuration. Permanent deletion requires a separate approved requirement. Future recorded history needs its own retention/reference rules. Current active snapshots survive deletion of otherwise unused exercises. Editing/deleting completed sessions requires separate requirements.

### Proposed transactions

1. **Future start replay:** consider accepting a template ID and request UUID when introducing retained sessions. Lock the owner's template, read its ordered configuration, and create the session and snapshot in one transaction. Template edits and archive operations already use this parent lock. Reject missing/empty templates. A retry of the same request must reuse the existing session, compare the template ID, and never recopy a changed template.
2. **Future prefilling:** source selection and occurrence ordering are approved/implemented in #35. Resolve the mapping into suggested input rows before implementing prefilling.
3. **Record:** lock the parent session, verify ownership, active status and occurrence membership, then insert valid weight/reps. A request UUID identifies a logical write; replay the same payload, reject changed payloads, and enforce uniqueness. Replay lookup precedes the active-status check so a lost response can be recovered after completion.
4. **Edit/remove:** acquire the same parent-session lock and require active status. Preserve other set numbers. If autosave is approved, serialize valid edits per set and retain failed drafts for retry.
5. **Complete:** wait for pending saves, then lock the session and persist completion once. Replays keep the existing finish time. The shared lock orders completion against writes. Resolve failed saves before finishing; never silently convert suggestions into performed sets.

Suggested values remain separate from performed records. Template updates already validate and replace ordered configuration atomically. Generic writes must not bypass session-state or ownership rules. Offline draft persistence is outside the proposed MVP.

### Proposed API and validation

| Method | Endpoint | Purpose |
| --- | --- | --- |
| GET, POST | `/api/sessions` | List/start sessions |
| GET | `/api/sessions/{id}` | Snapshot, sets and reference performance |
| POST | `/api/sessions/{id}/sets` | Record a set |
| PATCH, DELETE | `/api/sets/{id}` | Edit/remove an active-session set |
| POST | `/api/sessions/{id}/complete` | Complete a session |

Use JSON and the existing collection/resource/error conventions. Proposed history pagination uses a default limit of 50, maximum 100, and an opaque cursor with stable session-time/ID, occurrence-position and set-number ordering. Future progress scope and prefilling remain conditional on PRODUCT.md approval; reference selection is approved in #35.

Validate IDs/UUIDs, membership, content type, body size, unknown fields and numeric values. A 100-character name limit remains a proposal, not a current restriction. Use field errors for validation, 404 for missing resources, 409 for state/request conflicts, and generic errors for unexpected failures. Proposed successful responses are 200 for reads/updates/replays, 201 for new resources, and 204 for deletion. Retain the original request UUID when retrying uncertain starts/set writes; do not generate fresh UUIDs automatically.

Future tests should cover snapshots surviving template changes, request replay, reference selection, concurrent completion/writes, and concurrent starts if the one-active rule is approved. Interactive browser flows should cover session logging and visible save states when those features exist.

## Technical references

- [Next.js App Router](https://nextjs.org/docs/app)
- [Go net/http](https://pkg.go.dev/net/http)
- [pgx](https://pkg.go.dev/github.com/jackc/pgx/v5)
- [PostgreSQL partial indexes](https://www.postgresql.org/docs/current/indexes-partial.html)

## Shared navigation (#41)

The root layout mounts one shared navbar inside the existing theme provider. A Suspense boundary permits URL-aware highlighting with `usePathname`/`useSearchParams` while static routes retain server-rendered links and the theme control. The fallback uses the same navbar without a selected destination. Main-page navbar copies are removed; contextual detail back links remain in page content. No backend, schema, dependency, or lifecycle changes are needed.

## Implemented history filtering (#43)

Go parses history query criteria and performs literal case-insensitive substring, identity, and completion-time filtering in the existing PostgreSQL queries. Session search and exercise membership use EXISTS to avoid duplicating cards for repeated occurrences. Date filters become inclusive start/exclusive next-day instants in the client's IANA timezone; standard-library embedded timezone data makes DST handling available in the existing minimal runtime image. API dates default to UTC when `tz` is absent. Validation rejects malformed IDs/dates/timezones, reversed ranges, duplicates, and unknown parameters.

`GET /api/history/exercises` lists completed-snapshot identities with the latest captured name, scoped to the local owner and independent of current filters. The UI uses URL query state for applied criteria, a separate form draft until Apply, and keyed/abortable result requests to prevent stale results. Tab changes retain criteria, and no-matches states offer clear filters. Existing services, collection shapes, ordering, and completed-detail routes remain intact; no schema or dependency changes are required.

## Temporary reset/seed tool (#46)

The explicit `ENABLE_DEV_RESET` configuration defaults to false and is propagated to Go and the Next.js runtime through Compose. Go registers the POST handler only when enabled; forwarding independently gates the route. The confirmation header is required and is not accepted from a normal cross-origin HTML form. Home is rendered dynamically to read the runtime setting and mounts one client wrapper for confirmation and active-card invalidation. Ordinary routes keep their existing forwarding timeout; reset permits nine seconds around an eight-second Go transaction deadline.

`internal/devdata` locks the application tables in EXCLUSIVE mode, replaces only local-owner workout records, and seeds all templates, captured workouts, and sets inside a single transaction. These locks also block row-locking readers, ordering against ordinary writes; MVCC reads can see the previously committed dataset. Failures, timeouts, or lock conflicts roll back the replacement. IDs/sequences are not reset, preventing old URLs from aliasing new records; foreign-owner records and migration metadata are preserved. Sessions are inserted with valid copied configuration, ordered saved timestamps, decimal weights, and deterministic UUIDs within each session. No schema migration, dependency, or new service is introduced.

Focused disposable-schema tests cover gates/confirmation, exact counts, ordinary session/history/previous-performance readback, replacement, late-failure rollback, and an in-flight write. Reset tests use a separate schema so their local-owner reset cannot touch other packages' fixtures. Browser checks cover disabled/enabled configuration, cancel/confirm, pending/failure feedback, refreshed active continuation, and the seeded templates/history.

## Exercise-card links and grouped Sets history (#45)

The exercise name's real link uses a positioned pseudo-element to cover its card, with Edit/Delete controls layered above it; dialogs remain sibling interactive elements in the browser top layer. No click handler wraps or redirects form/button events. The shared navbar removes its Sets destination and treats all history routes/tabs as Workout history.

The completed-set query adds captured `session_name` and stable `exercise_id` metadata without changing selection/order. The UI groups already filtered ordered rows by session ID and occurrence position, omitting empty groups and retaining repeated occurrences, captured names, and exact decimal weights. This avoids extra per-workout fetches or new backend rules. No schema/dependency changes are required.

## Template editor and list actions (#48)

List cards now show ordered exercises by default (#55), with Edit workout links and StartSession controls. Delete is available in the dedicated editor. The editor's native confirmation dialog calls the existing archive endpoint and returns to the list after success. Detail mounts TemplateForm immediately for active templates; archived detail stays read-only until restored. Copy uses the existing duplicate endpoint and preserves the mounted original draft.

TemplateForm reports normalized draft changes and pending saves to detail. A successful save updates the baseline and remounts the saved form. The title/header owns Save template via a native form ID association and renders the dirty-state label. Draft navigation/unload guards and page-level Cancel are removed; unsaved component state is discarded when leaving. No backend/schema/dependency changes are needed.

## Description persistence (#52)

Migration 008 adds `description text NOT NULL DEFAULT ''` to exercises/templates only. Go create/update validation trims outer whitespace and preserves internal newlines; string JSON fields use empty strings for absence (omitted/null input also clears on full PUT). Existing request body limits apply without a new per-description limit. Template list/detail/update/copy/archive/restore return the saved description; copying reads it under the existing template lock. Snapshot structs/queries stay unchanged. Seed maps provide descriptions for the existing 26 shared identities and four routines. React renders text with preserved whitespace/wrapping; spaced list containers and per-card borders preserve stretched links and independent actions.

## Inline workout history (#54)

The initial session-level accordion was superseded by the revised #57 design below. Snapshot reads remain abortable with per-workout retry. A shared snapshot-exercise component serves inline and direct-link views with appropriate heading levels. Sets keeps recorded content above supporting workout metadata.

## Template overview cards (#55, revised)

TemplateCard reads the existing template detail resource on mount and renders all occurrences as compact, read-only rows. Abortable per-card reads have independent loading/retry states and preserve the existing summary collection API. Active cards link to the dedicated editor and expose StartSession; archived cards link to read-only detail. No new API, schema, or dependencies are introduced.

The existing standalone TemplateDetail owns form drafts and the unsaved-state label. ArchiveTemplate moves the native archive confirmation into this editor, reports pending state to disable editing, and explains any unsaved draft loss. Save/copy/restore and snapshot persistence rules remain unchanged. Accordion state, inline editor callbacks, and aggregate list guards have been removed.

TemplateForm renders readonly occurrence summaries inside the existing SortableExercise wrappers. TemplateExerciseDialog is a sibling of the main form, avoiding nested forms and sortable event handlers around popup inputs. A modal-local draft applies by occurrence key or appends a new occurrence; cancelling never changes the main draft. Popup dirty state contributes to the unsaved-state label. Delete/reorder clear stale positional errors; Go still validates the complete configuration on Save, with errors shown on cards and in reopened target popups. No exercise-library writes are made by these controls.

StartSession has a prominent card-header variant with a larger primary button, while other usages keep their existing sizing. TemplateForm can expose its native form ID and omit its internal submit button so the editor header submits the same form without duplicating save logic. The unused draft-navigation hook is removed. Archive callbacks ignore completed responses after the editor has unmounted, preventing a late redirect after the user leaves.

Insertion popups capture an occurrence key as their anchor and splice the applied draft after that key; they do not locate anchors by exercise identity or display name. New insertions clear stale positional errors. Pointer cursor styles are scoped to template-page buttons and links, including descendant dialogs, without changing sortable card backgrounds or unrelated pages.

## Sets-first history (#57)

History page selection, keyboard tab order, URL serialization, and filter clearing use Sets as the default and explicitly serialize Sessions. The shared completed-snapshot renderer removes exercise-history links from both inline and direct detail views. The dedicated exercise-history client component is removed. Go queries, completed snapshots, grouping, and previous-performance references are unchanged.

The owner confirmed that `/exercises/[id]/history` is removed completely and returns 404, with no compatibility redirect. Exercise-specific browsing uses the existing Sets exercise filter.


### Revised history views (#57)

Sessions loads the existing read-only snapshot endpoint for each listed workout to show ordered exercise accordions and recorded-set counts. Exercise expansion is local and independent. The template filter uses `template` in the URL and parameterized owner-scoped SQL against the original `workout_sessions.template_id`; `GET /api/history/templates` supplies distinct completed-history identities with their latest captured names. No schema change is needed. Browser URLs retain both identity filters, while API queries send only template for Sessions or exercise for Sets. Shared search/local-date handling remains unchanged.

Completed history reads derive optional per-set `change: {weight, reps}` decimal/integer strings in PostgreSQL, using a shared read-only CTE for the Sets endpoint and completed snapshot sets. Source selection precedes display filters and uses owner-scoped exercise identity, chronological workout order, occurrence order, and surviving set order. No comparison is stored and no schema changes are needed. Active-session reads and previous-performance payloads remain unchanged. The shared recorded-set table renders exact signed strings without floating-point subtraction.
