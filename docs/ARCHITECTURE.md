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

The root layout owns shared URL-aware navigation, theme provider, and one theme-aware Sonner Toaster. `next-themes` persists theme choice; the toggle waits for mounting to avoid hydration mismatch. shadcn/ui components are committed source, with shared Tailwind tokens/class merging. shadcn’s chart primitives wrap Recharts for progress presentation; Go owns source selection. The chart is dynamically loaded only for selected-exercise Sets history. Motion entrance effects leave SSR content visible and respect reduced motion. Existing dnd-kit sensors ignore interactive descendants, support mouse/touch/keyboard, and are shared by template occurrence and session-set sorting.

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
| `template_exercise_alternatives` | Ordered alternatives and independent targets beneath a default template slot; live exercise deletion protection |
| `session_exercise_choices` | Captured default/alternatives with identity, name, and targets; independent of mutable library/template records |
| `schema_migrations` | Applied embedded migration versions |

Composite foreign keys enforce common owner/session membership. Snapshot rows retain exercise identity/names without requiring a live library record. Cancelling an active session cascades its snapshot and sets. Soft-deleted set tombstones prevent delayed retries from resurrecting removed records.

Embedded numbered migrations run before serving requests, with transactional version recording and an advisory lock serializing starts. Applied versions are skipped; failures stop startup. Add new migrations, never rewrite applied SQL. Migrations 005/006 preserve existing active sessions while introducing retained completed workouts, 007 adds sets, 008 descriptions, 009 nullable recorded RIR, 010 backfills explicit set order from saved-time/UUID order, 011 adds constrained nonnull technique with Decent as the backfill/default, and 012 adds optional partial counts without backfilling invented values. Migration 013 adds template alternatives, captured session choices, and a selection revision. It backfills one default choice from every existing session occurrence without rewriting sets, UUIDs, order, tombstones, or targets. Normal restarts do not reset data. Volume persistence is not backup; define an export/restore procedure before relying on important history.

## Writes and concurrency

Exercise create/update trims names/descriptions, normalizes tags in vocabulary order, removes duplicate tags, and requires at least one tag in Go. Usage reads return distinct referencing templates including archived state. Delete locks the exercise `FOR UPDATE`; template writes lock selected exercises `FOR KEY SHARE`, with foreign keys as final protection against broken references.

Template create/update validates default/alternative targets and ownership, rejects duplicate identities within a slot, and replaces the complete ordered configuration in one transaction. Edits lock the parent; concurrent full configurations apply in commit order without draft merging. Duplicate locks the source and copies saved descriptions/occurrences/targets directly in SQL without numeric rounding. Archive/restore lock that parent and preserve references; archived edits/copies are rejected. Detail reads use read-only repeatable read to avoid mixed configurations.

Session start locks the owner `FOR NO KEY UPDATE`, then the source template `FOR SHARE` against editing/archiving, and copies configuration atomically. Existing active state yields a conflict containing its snapshot. Active/detail reads use repeatable read for selected configuration, captured choices, and sets together.

Selection PUT locks the same active session parent as set writes/order/delete/completion. It compares the supplied per-slot `selection_revision`, checks for zero surviving sets, and copies only a captured choice’s identity/name/targets into the selected occurrence, incrementing its revision. Set PUT/order require that revision for multi-choice slots; omission remains compatible only for unchanged single-choice slots. This prevents stale requests and A→B→A retries from attaching sets to another choice. Tombstones are never removed or reused by switching. DELETE remains UUID-scoped and safe for already-deleted IDs.

A switch request contains `{exercise_position,exercise_id,selection_revision}`. A successful response is the active snapshot. A stale revision returns `409 selection_conflict`; surviving sets return `409 exercise_has_sets`. A lost result locks local conflicting controls and retries the same compare-and-swap. On revision conflict, fetch the saved snapshot and replace only that slot, preserving other components/drafts/blockers. A pending original request cannot apply after its revision has advanced. If a retry is rejected because another tab added sets, explicit occurrence-local Reload saved sets reconciles the snapshot before allowing writes. Other failures retain the old selection/inputs. Selected slot controls remount only after confirmed/reconciled success; keyed, abortable reference results prevent old-selection data from initializing the new inputs.

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
| PUT | `/sessions/{id}/selection` | Compare-and-swap a captured choice while the slot has no surviving sets |
| POST | `/sessions/{id}/complete`, `/cancel` | Bodyless lifecycle actions, `204` |
| PUT, DELETE | `/sessions/{id}/sets/{uuid}` | Save/replace or soft-delete a set in the active session |
| PUT | `/sessions/{id}/sets/order` | `{"exercise_position":1,"selection_revision":0,"set_ids":["..."]}` exact permutation |
| GET | `/sessions/{id}/previous`, `/bests` | Per-exercise references for an owner-scoped active session |
| GET | `/sets` | Completed nondeleted set collection, including captured session/exercise names and identities |
| GET | `/history/sessions`, `/history/sessions/{id}` | Completed summaries (including empty workouts) or read-only snapshot |
| GET | `/history/progress` | Requires one exercise ID; average recorded-load estimate across saved exercise sets per completed workout |
| GET | `/history/exercises`, `/history/templates` | Completed-snapshot filter identities with latest captured names |
| POST | `/dev/reset` | Explicitly enabled confirmed data replacement; otherwise `404` |

Create returns `201` and, for templates/sessions, Location; reads/updates return `200`; successful deletes/end actions return `204`. Errors use `{"error":{"code":"...","message":"...","fields":{...}}}` with fields where relevant: validation `400`, missing/foreign resource `404`, state/reference conflicts `409`, oversized body `413`, unsupported media type `415`, generic persistence `500`. Not all errors include fields. Template Delete is a UI label for archive; there is no permanent template DELETE endpoint.

Template writes send name, optional description, and ordered `exercises` with `exercise_id`, required `set_count`, nullable `rep_min`/`rep_max`, and nullable numeric `target_rir`. Default fields remain at each entry’s top level. Optional `alternatives` contains choice objects with the same exercise/target fields; array order is retained. Session occurrences return captured `choices` (default first) for multi-choice slots and `selection_revision`; their top-level fields always describe the selected choice. Array order becomes one-based position. Counts/reps are whole numbers; RIR is finite nonnegative numeric without a product-specific ceiling/rounding scale, subject to PostgreSQL numeric limits.

Example set PUT body:

```json
{"exercise_position":1,"selection_revision":0,"weight":"12.50","reps":8,"rir":1.5,"technique":"decent","partial_reps":2}
```

Weight is a nonnegative decimal string with at most two fractional digits; excess precision is rejected rather than rounded. RIR accepts a nonnegative JSON number or null, and omission clears it on replacement. Read payloads return weight as a string and RIR as a nullable decimal string. Technique must be an explicit lowercase `good`, `decent`, or `bad` on every PUT; missing, null, empty, unknown, or wrong-type values return 400. The database default backfills existing rows and supports internal legacy SQL inserts; it does not supply omitted HTTP values. Set JSON bodies are limited to 4 KiB. Optional `partial_reps` is a JSON integer; omitted/null/zero clears it on full PUT and reads return null or a positive count. Negative, fractional, wrong-type, or unsafe JavaScript integer counts are rejected with 400; storage constrains positive counts to the same safe-integer range. Full reps remain independent and required. Position cannot change for an existing UUID.

The legacy `/exercises/{id}/history` API is removed from both Go and Next, along with its dedicated query/models/store methods and tests. It returns 404, as does the removed `/exercises/[id]/history` page. Exercise cards use filtered Sets history; completed snapshots and reference readers remain available.

### Exercise library pagination

`GET /api/exercises` without pagination keeps its full-library `{items,muscle_groups}` response for template pickers. The Exercises page opts in with `paginated=true`, optional positive bigint `page` (default 1), trimmed literal case-insensitive name substring `q`, and optional predefined `muscle_group`, combined with AND. This mode rejects unknown/repeated parameters, invalid muscle groups, malformed/overflowing/nonpositive pages, and invalid pagination flags with the existing 400 validation envelope. A `page` request requires `paginated=true`. Next forwarding preserves the query string and validation response.

Paginated responses retain items/vocabulary and add `page`, `page_size:4`, `total_items`, and `total_pages` (zero for no matches). Out-of-range pages clamp to the final available page or page 1 for no matches; the browser replaces its URL with the canonical page. A read-only repeatable-read transaction counts owner-scoped filtered rows then selects only the page using `ORDER BY exercises.id LIMIT 4 OFFSET ...`. Numeric ID supplies deterministic ordering independently of the text ID returned in JSON. `strpos(lower(name),lower(q))` treats SQL wildcard characters literally. The complete muscle-group vocabulary is independent of results. No schema migration or full-library client slicing is involved. Mutation success triggers a fresh page/count read without changing write contracts. Browser URL criteria and aborted/keyed requests isolate navigation results; local create inputs remain mounted.

## Historical reads, filters, and comparisons

Historical reads use captured names and owner-scoped identities. Completed snapshots are repeatable-read, with explicit completed-state selection; no mutable library join rewrites results. Lists order completion timestamp and numeric session ID descending, then occurrence and persisted set order.

`previous` selects the newest completed source with eligible sets per exercise; `bests` selects the workout containing the heaviest set, reps then Good > Decent > Bad technique breaking ties, then returns all exercise occurrences/sets from that workout. Both verify active membership and read in one read-only repeatable-read transaction. Their payloads contain exercise identity, source session/completion, and ordered occurrences; exercises without eligible history are omitted. They never write or prefill recorded sets.

History parses `q`, `exercise`, `template`, `session`, `from`, `to`, and `tz` into parameterized SQL criteria. Search is literal case-insensitive substring; session membership uses EXISTS to avoid repeated cards. Browser URLs additionally carry `tab` and retain both identity choices, while requests send exercise for Sets or template and optional exact completed-session identity for Sessions. The `session` selector is a positive bigint scoped to the owner and combined with the other criteria. Inclusive dates become start-inclusive/next-day-exclusive instants in the supplied IANA timezone (UTC if absent). Embedded Go timezone data supports DST in the minimal image. Malformed/repeated/unknown criteria, reversed dates, and invalid zones fail validation; valid unknown IDs yield no results.

Per-set `change: {weight, reps, technique, previous_technique}` uses the shared PostgreSQL comparison CTE to select the baseline before display filters. Match exercise identity, chronological prior workout, repeated occurrence ordinal, and surviving set ordinal. The prior workout need not contain saved sets, unlike reference selection. Exact signed decimal/integer strings avoid browser floating-point subtraction. Technique uses that same matched baseline: Go returns `improved`, `regressed`, or `unchanged` plus the previous rating, using Good > Decent > Bad. With no matched set there is no comparison object. Changes are not stored; RIR and partial counts are not compared. Partial counts are selected for display only and never enter comparison arithmetic, eligibility, or the personal-best ORDER BY.

Next uses URL-applied criteria separately from form drafts, with keyed/abortable results to prevent stale responses. Sessions loads snapshots per workout with independent retry; exercise accordions are local. Sets groups only matching rows. Sets workout links create a fresh `/history?tab=sessions&session=<id>` URL, clearing unrelated criteria. Apply/tab navigation retains that selector; Sets ignores it, and Clear workout filter removes only it. Direct completed detail remains supported, and `/sets` redirects to Sets history.

### Progress query and presentation

`GET /api/history/progress?exercise=<id>&q=...&from=...&to=...&tz=...` shares history parsing and the Sets SQL filter fragment. It reads only owner-scoped completed, nondeleted sets. No library/template join rewrites captured names or requires a live exercise record. Missing/invalid exercise IDs fail validation; valid unknown IDs return an empty collection.

The response is `{metric:"average_estimated_strength",items:[...]}`. The query returns every matching saved set in completion-time/numeric-workout-ID, occurrence, persisted set order, saved-time, and UUID order. Go groups them by workout and calculates each exact rational estimate `weight × (1 + reps/30)` with `math/big.Rat`. Average unrounded estimates across all matching occurrences, rounding the result to two decimal places only for display. Zero-weight sets count in the denominator; any positive full-rep count qualifies. Technique and partials do not affect the calculation. No exercise classification or bodyweight/assistance correction is applied.

Each point includes `session_id`, `session_name`, `completed_at`, `exercise_name`, decimal-string `average_estimate`, `set_count`, and ordered `sets`. Source sets contain `exercise_position`, surviving `set_number`, `set_id`, exact decimal-string `weight`, integer-string `reps`, nullable decimal-string `rir`, technique, partial count, and display-rounded per-set `estimate`. The query avoids unrelated historical set-change baselines and does not write workout data.

The frontend converts average estimates to chart coordinates only; exact source values remain in touch/keyboard tooltips and the existing recorded-set list. Distinct session-ID categories preserve same-day/same-instant workouts, with local date labels. shadcn ChartContainer/ChartTooltipContent and Recharts LineChart provide presentation and accessibility. The latest average and contributing count also have a readable text summary; there is no duplicate expandable chart-data list. Animations are disabled. Keyed components and aborted requests isolate URL-filter changes; chart loading/retry/unavailable states remain independent of recorded sets.

## Temporary reset and operational safety

`ENABLE_DEV_RESET` defaults false in Go and Next runtime configuration. When enabled, `/dev/reset` additionally requires `X-Confirm-Reset: replace-workout-data`. The isolated `devdata` package replaces only the selected owner's workout rows in one transaction, taking table locks to order concurrent writes. It preserves schema/migrations/owner/sequences; ordinary readers see committed data and failure rolls back. The response reports dataset counts and active-session identity. Synthetic descriptions/weights/reps exercise ordinary APIs; optional targets and recorded RIR remain unset. A deterministic technique cycle and paired week weight/rep values expose quality changes and equal-performance tie-breaking without changing dataset counts. A shared seed variation map configures eleven alternatives across ten slots in all four routines, using existing exercise identities. Historical snapshots rotate actual choices with their normal sample weights and the slot’s original set count, preserving 48 workouts/1,032 sets and the active three-set default example. Optional seeded partial counts vary independently and do not change recorded weights/full reps/technique. The feature can be removed through its registration/configuration/UI without changing normal workout rules. Enablement and lost-response handling are in README.

## Validation responsibilities

Go tests own detailed decoding/validation, ownership, transactions, constraints, migration upgrades, concurrent lifecycle/set writes, target/description round trips, history filtering/source selection, exact changes, and reset atomicity. PostgreSQL tests require a separate disposable database and explicitly skip without `TEST_DATABASE_URL`.

The smoke script builds production images and checks one browser-origin API flow through Next forwarding, frontend route availability, validation/conflict propagation, active saved-set restoration after database and full-stack restarts, completed reads/references, cancellation, and liveness/readiness outage recovery. It owns a fresh randomly named Compose project and volume, prints service logs on failure, and removes only its disposable resources. Page fetches do not verify interactive browser behavior.

CI retains frontend formatting, Go formatting/vet/race tests with PostgreSQL, and the dependent production-stack smoke job. Next's production build checks TypeScript. After those checks pass on a push to main, a matrix job builds the existing API/frontend Dockerfiles for Linux AMD64 and ARM64 and publishes to GHCR using a job-scoped package-write token. Repository-root build contexts, separate component caches, source/revision labels, and full-commit-SHA plus latest tags are used. PRs never publish. After both components publish, a dependent release job adds matching main-<CI run number>-<full SHA> tags without rebuilding. It promotes frontend first and API last; external image automation can use the API tag as the signal that a matching pair is available. Numeric CI run order supports chronological selection even when an older run is retried. Publication itself does not deploy containers; the separate crow-kube repository owns Flux deployment. Setup and package access instructions belong in README.
