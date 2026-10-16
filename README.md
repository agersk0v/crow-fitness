# Crow Fitness

A simple workout tracker for one person, with no authentication or accounts.

## Current status

The Next.js frontend, Go API, and PostgreSQL support creating, searching, tagging, renaming, and safely deleting personal exercises at `/exercises`. Names are trimmed and validated in Go. Saved exercises survive refresh and stack restart. Go selects one internal local owner; the app has no login or authentication. You can also create, edit, duplicate, archive, and restore persistent workout templates at `/templates`. Start a workout from an active template, continue the saved session from home, save weight/reps, or cancel it. Completing preserves its sets in Workout history at `/history`. Active exercises show previous completed performance without prefilling; exercise-specific completed-set history is available from the exercise library. Progress charts remain planned.

See [product requirements](docs/PRODUCT.md), [architecture](docs/ARCHITECTURE.md), and [agent instructions](AGENTS.md).

## Run locally with Docker

Install Git and Docker with Docker Compose v2. Docker Desktop includes Compose on macOS and Windows; on Linux use Docker Engine and the Compose plugin. Start Docker before running these commands. You do not need Node.js, Go, or PostgreSQL installed on your computer for this workflow.

Clone the repository and open a terminal in its root:

```sh
git clone https://github.com/agersk0v/crow-fitness.git
cd crow-fitness
cp .env.example .env
```

On Windows PowerShell, use `Copy-Item .env.example .env` instead of `cp`. Open `.env` in a text editor and replace the example password. This changes configuration, not application code. Letters, digits, underscores, and hyphens avoid special quoting rules. `.env` is ignored by Git.

| Setting | Purpose | Default/example |
| --- | --- | --- |
| `POSTGRES_PASSWORD` | Required local database password | Replace the example value |
| `APP_PORT` | Frontend port on your computer | `3000` |
| `ENABLE_DEV_RESET` | Temporary destructive reset/seed tool | `false` |

Start the stack:

```sh
docker compose up --build --detach --wait --wait-timeout 180
```

The initial build downloads dependencies and takes longer than later starts. Open [http://localhost:3000](http://localhost:3000), or your configured `APP_PORT`. Click **Check again** if needed; both connection indicators should show **Connected**. The frontend is bound to localhost, and the API/database have no host-published ports.

After pulling code changes, run the same startup command to rebuild. This is a production-style local run, with no hot reload.

## Verify and inspect

```sh
docker compose ps
curl --fail http://localhost:3000/api/health
curl --fail http://localhost:3000/api/ready
docker compose logs --tail=100
```

Use `curl.exe` in Windows PowerShell, or open those two URLs in your browser. Adjust the port if you changed `APP_PORT`.

| Endpoint | Healthy response | Failure behavior |
| --- | --- | --- |
| `/api/health` | `200`, `{"status":"ok"}` | Tests the Go server independently of PostgreSQL |
| `/api/ready` | `200`, `{"status":"ok","database":"connected"}` | `503` when PostgreSQL is unavailable |

If Go itself cannot be reached, the frontend forwarding route returns `502`. The Go API remains live during a database outage and readiness recovers when PostgreSQL becomes available again.

## Stop and preserve data

```sh
docker compose down
```

This stops/removes the containers but keeps the named PostgreSQL volume. Start again with the startup command. Do not add `--volumes` or `-v` unless you intentionally want to delete the local database. A volume provides persistence, not a backup.

PostgreSQL applies its initialization password only to a new volume. Editing `POSTGRES_PASSWORD` after the database exists does not update the stored password; keep the original value until you intentionally change the database user's password. Do not delete a populated volume just to fix a connection problem.

## Troubleshooting

- **Docker cannot connect:** start Docker Desktop/the Docker daemon, then retry.
- **Port already in use:** change `APP_PORT` in `.env`, then run the startup command again.
- **Database connection unavailable:** check `docker compose logs --tail=100 postgres api` and that the password matches the existing database.
- **Frontend/API changes not visible:** rebuild with the startup command; local source files are not mounted into the running containers.
- **Missing configuration:** copy `.env.example` to `.env` and set `POSTGRES_PASSWORD`.

## Development checks

Docker is the recommended local workflow. For checks outside Docker, use Node.js 24 and Go 1.27.2, matching the Docker images and CI.

From `frontend/`:

```sh
npm ci
npm run format:check
npm run typecheck
npm run build
```

From `backend/`:

```sh
gofmt -l .
go vet ./...
go test -race ./...
go build -o bin/api ./cmd/api
```

`gofmt -l .` should print no files. `npm run format` and `gofmt -w .` apply formatting when needed.

CI runs frontend formatting and Go formatting, vet, and race tests (including disposable PostgreSQL tests) on PRs and commits to `main`. Its stack job builds both production Docker images and runs the smoke check; the Next.js build also checks TypeScript. Standalone build/typecheck commands above remain available locally.

The smoke check covers representative exercise/template/session lifecycle requests through Next.js, one validation error, readiness outage/recovery, and persistence through database and whole-stack restarts. Page fetches verify routes render, not interactive form behavior. Each run owns a randomly named Compose project and removes only its disposable test volume afterward.

To run the same integration check locally, use Bash with Docker running:

```sh
bash scripts/smoke.sh
```

It uses port `13000`; set `SMOKE_PORT` to another available port if necessary. No `.env` file is required for the smoke check because it supplies its own test configuration.

## UI components and animations

The frontend uses shadcn/ui source components in `frontend/components/ui/`, with shared class merging in `frontend/lib/utils.ts`. The configuration in `frontend/components.json` uses the existing `@/` alias, App Router, TypeScript, and Tailwind CSS v4. The exercise form uses Button and a destructive Alert for validation errors, preserving the entered name for retry. Successful additions show a Sonner toast containing the trimmed name. A single theme-aware Toaster is mounted inside the root theme provider; use `toast.success(...)` from `sonner` for success feedback rather than adding a second live announcement. The form saves to PostgreSQL through Go and shows loading, validation, and retry states. Theme tokens in `app/globals.css` preserve the dark/lime appearance; the header theme toggle switches between dark and light mode. Dark is the default; `next-themes` saves the choice in browser local storage across navigation and refresh. The provider applies the saved theme before the page paints, while the toggle waits for mounting to avoid hydration mismatches.

From `frontend/`, add only the components needed for a feature:

```sh
npx shadcn@4.21.4 add <component-name>
```

Review generated source, dependencies, and stylesheet changes, then run the frontend checks. Import components from `@/components/ui/<component-name>`. shadcn/ui copies editable source into the repository; it is not a single runtime component library. Use the shared tokens and retain accessible labels and visible keyboard focus.

Motion for React provides brief 180 ms entrance animations for the home/exercises content and newly added exercise rows. Existing rows do not replay their entrance on form updates. The shared `useEntranceAnimation` hook uses opacity and a 6 px translation after mounting, leaving server-rendered content visible. Reduced-motion preferences skip the animation; cleanup restores the visible state if the preference changes during an animation. Import `motion` and `useReducedMotion` from `motion/react` inside a component with `"use client"`. Keep animations brief and use opacity/transforms. Respect reduced-motion preferences by omitting movement or rendering immediately; animations must not block actions or hide content. Commit package and lockfile changes whenever dependencies change.

## Planned features

- Optional previous-performance prefilling (requires separate decisions).
- Exercise progress charts.

The product document lists decisions to confirm before implementing these flows. This setup is intended for your own computer; it has no authentication and is not configured for public hosting.

## Exercises API and migrations

The API applies embedded SQL migrations from `backend/internal/migrations/` before serving requests. The documented Docker startup command runs them automatically. Each migration and its version record commit in one transaction; an advisory lock serializes concurrent starts. Applied migrations are skipped. Restart does not reset or delete data. Add new numbered migrations rather than editing applied SQL. Migration failures stop API startup; inspect API logs.

`GET /api/exercises` returns `200` with `items` containing exercise IDs, names, and `muscle_groups` arrays, plus the predefined `muscle_groups` vocabulary. `POST /api/exercises` accepts `application/json` and `{"name":"  Squat  ","muscle_groups":["quadriceps"]}`, returning `201` with `{"id":"1","name":"Squat","muscle_groups":["quadriceps"]}`. Missing, null, or empty tags are rejected on create/update with `error.fields.muscle_groups`. Unknown tags return `400` with `error.fields.muscle_groups`; duplicate tags are normalized. Names must be nonblank after trimming; requests are limited to 16 KiB. Duplicate names are allowed. Blank names return `400` with `error.fields.name`; malformed JSON/unknown fields return `400`, oversized requests `413`, and unsupported media types `415`. Persistence failures return generic `500` errors; unreachable Go returns `502` from Next.js. Responses are uncached.

Exercise records reference seeded local owner 1. Go selects it; clients cannot choose an owner. This implements personal ownership without authentication. Anyone who can reach the local app can operate on this owner's data.

For PostgreSQL tests, set `TEST_DATABASE_URL` to a **separate disposable test database**, then run `go test -race ./...` from `backend/`. Never use your workout database. Without the variable, PostgreSQL tests are explicitly skipped. See Development checks above for the isolated stack smoke workflow.

On `/exercises`, case-insensitive name substring search combines with one muscle-group filter (or All). An empty library and no matches have distinct messages. Creating or editing an exercise requires at least one tag and may include multiple groups: chest, back, shoulders, biceps, triceps, forearms, core, glutes, quadriceps, hamstrings, and calves. This vocabulary is returned by Go, rather than maintained separately in the browser.

**Edit** opens the saved name/tags with Save/Cancel and inline errors. Failed saves retain the draft. Renaming keeps the exercise ID and updates the name displayed in templates. **Delete** opens a confirmation naming the exercise and checks usage; referencing templates are linked and block deletion. Escape/Cancel closes the dialog, with no mutation.

- `PUT /api/exercises/{id}` replaces the name and tags, returning `200` with the updated exercise. At least one muscle group is required; clearing every tag is rejected.
- `GET /api/exercises/{id}/templates` returns `200` with `{"items":[{"id":"1","name":"Push day","archived":false}]}`; repeated occurrences count once per template. Archived references still block deletion and are labeled in the UI.
- `DELETE /api/exercises/{id}` returns `204` for an unreferenced exercise, or `409` with `error.code: "exercise_in_use"` and `error.templates` containing the referencing template IDs/names. No references are removed automatically. Missing/foreign-owner exercises return `404` for these operations.

Migration `003_exercise_tags.sql` adds empty tags to existing exercises without resetting data. These legacy exercises remain available and must receive at least one tag when next edited. PostgreSQL constraints validate tags and preserve template references. Go locks deletion against concurrent template creation, so stale usage information cannot permit deleting a referenced exercise.

## Workout templates

Open `/templates` from home or exercises. Give the template a name, add saved exercises, and drag each exercise card’s background to arrange them, or hold briefly before dragging on touch screens. Inputs, labels, dropdowns, and buttons keep their normal behavior. Keyboard users can focus the card, press Space, use arrow keys, and press Space to drop or Escape to cancel. Each occurrence requires a positive whole-number working-set count. Rep bounds are optional: leave both blank, use a positive minimum/maximum range, or equal values for a fixed target. Target RIR is optional and accepts nonnegative decimals, including zero. One rep/RIR target applies to all working sets of an occurrence; warm-up sets are excluded. The same exercise may appear more than once with different targets. Saving persists the entire ordered configuration atomically; select a saved template to reopen it. Active templates can be edited or duplicated. Archive preserves their configuration and hides them from the Active list; inspect or restore them in Archived. Archived templates are read-only and must be restored before editing or duplicating. Start workout opens a persisted active session. Permanent template deletion is not included; session set entry is described below.

Migrations `002_templates.sql` and `004_template_archive.sql` use the same startup process described above. Existing templates remain active. Template writes reject missing or foreign-owner exercises.

- `GET /api/templates`: active templates by default, `200` with `{"items":[{"id":"1","name":"Push day","archived":false}]}`. Use `?archived=true` for Archived; invalid selector values return `400`.
- `POST /api/templates`: `application/json`, with a trimmed nonblank name and at least one exercise. Returns `201` with the full resource and a `Location` header.
- `GET /api/templates/{id}`: `200` with the full resource, including `archived`, or `404` if absent or owned by another owner. Archived templates remain inspectable.
- `PUT /api/templates/{id}`: replace the name and ordered exercise configuration atomically, using the same JSON body and rules as create; `200` with the saved resource. Removing an occurrence does not delete its library exercise.
- `POST /api/templates/{id}/duplicate`: `application/json` with `{"name":"Push day B"}`; `201` with a new independent resource and `Location`. All saved occurrences/targets are copied in PostgreSQL without numeric rounding. Names need not be globally unique.
- `POST /api/templates/{id}/archive` and `/restore`: bodyless actions returning `200` with the resource. Repeating either action is safe. Archived edits/duplication return `409` with `error.code: "template_archived"`; permanent template DELETE is unavailable.

Example create body:

```json
{
  "name": "Push day",
  "exercises": [
    { "exercise_id": "1", "set_count": 3, "rep_min": 8, "rep_max": 12, "target_rir": 1.5 },
    { "exercise_id": "2", "set_count": 2, "rep_min": null, "rep_max": null, "target_rir": 0 }
  ]
}
```

Array order determines `position` in the response. Each returned occurrence also includes its exercise name. Absent optional values are returned as `null`; zero RIR remains zero. Counts/reps must be whole numbers; RIR is a JSON number stored as a decimal without a fixed rounding scale or product-specific ceiling. PostgreSQL's numeric storage limits still apply. Requests are limited to 64 KiB and unknown fields are rejected. Invalid values return `400` with `error.message` and, for business validation, `error.fields` keyed by `name`, `exercises`, or e.g. `exercises.0.set_count`, `exercises.0.rep_range`, `exercises.0.target_rir`, and `exercises.0.exercise_id`. Responses are uncached; transport/media/size/persistence errors use the same status conventions as exercises.

PostgreSQL tests cover template ordering, optional/fixed targets, ownership, constraints, rollback, and migration reruns. The smoke check covers transport and restart persistence.

Template editing offers Save/Cancel and retains drafts after failed saves. Duplication requests a name for the independent copy. Archive requires confirmation naming the template; Cancel/Escape leaves it unchanged. Archived exercise references are preserved until an active template is edited to remove them. Concurrent edits apply complete configurations in commit order; there is no draft merging. Active sessions preserve their own snapshots; completed sets are retained in the Sets tab of Workout history.

## Active workout sessions

Choose **Start workout** on an active template. The session saves a copy of the template name, ordered exercises/names, working-set counts, and optional rep/RIR targets. Later edits, archiving, or removal of an otherwise unused exercise do not change this copy. Only one session can be active; trying to start another offers **Continue session** without replacing it.

Home shows **Continue session**, including after refresh or a stack/database restart. **Continue later** returns home while retaining the session. **Cancel session** asks for confirmation; Keep session or Escape dismisses it. Confirmed cancellation discards the active session and all its sets. **Complete session** retains the workout snapshot and its saved sets, frees the active slot, and returns home. Complete with fewer or no sets if needed; template targets are suggestions, not recorded results. Request failures show an error and allow retry using the same action.

Migrations `005_active_sessions.sql`, `006_completed_sessions.sql`, and `007_session_sets.sql` run automatically during API startup. Existing exercises/templates and active snapshots are preserved. No new environment settings or services are required.

- `POST /api/sessions`: `application/json` with `{"template_id":"1"}`; `201` with the copied session and `Location`. An existing session returns `409`, `error.code: "active_session_exists"`, and `error.session` containing the existing snapshot for continuation. Archived/empty templates return `409`; missing/foreign-owner templates return `404` when no session is active.
- `GET /api/sessions/active`: `200` with `{"session":null}` or `{"session":{...}}`.
- `GET /api/sessions/{id}`: `200` with the snapshot, or `404` if missing/completed/cancelled/foreign-owned.
- `POST /api/sessions/{id}/cancel`: bodyless, `204`; delete only the matching active session and cascade its sets/snapshot. It never deletes completed workouts or a newer active session.
- `POST /api/sessions/{id}/complete`: bodyless, `204`; retain the snapshot/sets and record completion once. Retries preserve the first completion time, and missing/stale IDs do not affect another workout.
- `PUT /api/sessions/{id}/sets/{setID}`: `application/json` with `{"exercise_position":1,"weight":"12.50","reps":8}`; `200` with the saved set. The lowercase UUID identifies one logical set; reuse it for edits/retries and use a fresh UUID for a new set. Weight is a decimal **string** in kilograms, nonnegative with up to two decimals, without a product-specific ceiling; PostgreSQL numeric storage limits apply. Reps are positive whole-number JSON values within bigint range (the browser requires safely representable integers). The position identifies a captured occurrence, not just an exercise ID. A saved set cannot move between occurrences.
- `DELETE /api/sessions/{id}/sets/{setID}`: `204`; remove an active-session set from visible results. Retrying deletion is safe. A late PUT retry cannot recreate a deleted set (`409`, `set_deleted`); adding it again requires a new UUID.
- `GET /api/sets`: `200` with `items` containing only completed-workout sets: string `id`, `session_id`, `weight`, integer `exercise_position` and `reps`, copied `exercise_name`, and UTC `completed_at`. Order is newest completion time/session first, then occurrence and saved-set order, with UUID tie-breaking. No filters, analytics, or detailed workout-history UI.

Session resources contain string `id` and `template_id`, copied `name`, UTC `started_at`, nullable `completed_at`, `sets` with ID/occurrence/weight/reps, and `exercises` using the same occurrence shape/target types as template details. Responses are uncached. Start accepts only a canonical positive string template ID; unknown fields/malformed JSON return `400`, bodies above 4 KiB return `413`, and unsupported media types return `415`. Persistence/forwarding failures use the existing generic `500`/`502` conventions. Set validation failures include `error.fields` for weight/reps/occurrence. Completed-session set writes return `409` with `session_completed`; foreign/missing sessions return `404`. Completion and writes use the same parent lock, so a write either commits before completion or is rejected afterward. Concurrent active edits apply in commit order.

Focused PostgreSQL tests cover snapshots, null/zero/decimal targets, owner isolation, concurrent starts, atomic failed starts, migration reruns/reconnection, clearing rows, and stale end requests. The stack smoke also checks session forwarding and persistence through database/full-stack restarts.

On a session page, **Add set** opens blank weight/reps fields; **Save set** persists them to the active workout. Saved rows offer Edit/Save/Cancel and Delete. They appear in **Workout history → Sets** only after completion. The Sets tab groups recorded sets by workout and exercise occurrence, with exact kilograms/reps and browser-local completion dates.

Completion is disabled while a set draft, write, or unresolved error exists. Save or explicitly discard a new draft, cancel an unsubmitted edit, or retry a failed edit/delete before completing. A new set whose save response was lost can be retried with the same UUID or discarded with a server deletion. A failed edit must be resolved by retrying Save before cancelling the edit, because the server may already have saved it. Cancel session still discards the entire active workout after confirmation. Refresh restores successful saves; unsaved drafts have no offline persistence guarantee.

## Previous performance in an active session

Each exercise shows its latest completed workout with saved sets, across any template. Exercise identity connects history through renames. Empty workouts, deleted sets, and active/cancelled sessions are excluded. Repeated source occurrences retain their captured names and are labeled separately in source occurrence/set order; repeated current occurrences may share the same reference. Exact weight/reps and a local-time completion date are displayed read-only. No eligible history shows **No previous performance yet**.

Reference requests run independently from current-set entry. **Retry previous performance** reloads only the references, preserving current drafts and controls. Refresh/Continue session reload references and saved current sets. Previous results never prefill, create, or complete any current set.

- `GET /api/sessions/{id}/previous`: uncached `200` with `{"items":[...]}` for the owner's active session. A reference contains string `exercise_id`, string source `session_id`, UTC `completed_at`, and `occurrences` with source `position`, captured `name`, and `sets` using the existing saved-set shape. Exercises without history are omitted; no history returns `{"items":[]}`. Missing/completed/cancelled/foreign sessions return `404`; invalid IDs return `400`. Persistence/forwarding failures use the existing generic `500`/`502` conventions. Completion time/session ID and occurrence/saved timestamp/UUID provide stable tie-breaking.

## Workout history

Open **Workout history** from home, exercises, templates, or an active session. `/history` starts with **Sessions**: template-style cards for completed workouts, newest first, using the captured name and a local-time completion date. Selecting a card expands a read-only snapshot inline, with ordered exercise occurrences, template targets, and completed sets in separate sections. Only one workout stays expanded at a time; it can also be collapsed. `/history/[id]` remains available for direct links. Repeated workouts are separate cards, and repeated exercises remain separate occurrences. Empty completed workouts still have a card and detail; they add no set rows.

**Sets** groups completed sets by workout and exercise occurrence, emphasizing exercise names and recorded weight/reps above the smaller workout name and local date. Tabs use `/history?tab=sessions` or `?tab=sets` so refresh/back restores selection. Arrow keys, Home, and End move and activate tab focus; Tab enters the selected panel. `/sets` redirects to the Sets tab, preserving existing links. Active/cancelled workouts are excluded from both views. Loading failures offer retry, and missing/foreign/noncompleted detail IDs show not found. No editing, deletion, duplication, or template-creation actions are exposed on completed detail.

- `GET /api/history/sessions`: uncached `200`, `{"items":[{"id":"1","name":"Push day","completed_at":"..."}]}`. Completion time and numeric session ID sort descending, including workouts with no sets.
- `GET /api/history/sessions/{id}`: uncached `200` with the saved session resource, including nonnull `completed_at`, copied `exercises`, and nondeleted `sets`. Missing, foreign, active, or cancelled sessions return `404`; invalid IDs return `400`. Existing generic persistence/forwarding errors apply. The active `/api/sessions/{id}` endpoint remains active-only.

No schema/environment changes are required. Completed-history reads preserve copied names, targets, exact weights/reps, and source order after template/library changes. Focused tests cover filtering, empty/repeated workouts, ownership, deterministic ordering, preserved snapshots, and reconnect; browser checks cover tabs, keyboard focus, read-only details, retry, and legacy navigation.

## Exercise set history (#36)

Choose **View history** in the exercise library or **View exercise history** in a completed workout. `/exercises/[id]/history` groups completed, nondeleted sets across all templates by workout, newest first. Each group links to its completed workout and shows the captured workout name, browser-local completion date, and separate exercise occurrences in source order, with exact kilograms/reps. Active, cancelled, and empty results do not appear. The view is read-only, with loading, empty, not-found, and retry states.

History uses exercise identity, so renaming does not split results. The page title uses the current library name, or the latest completed snapshot name after removal. Removing an otherwise unused library exercise preserves its history URL and links from completed workouts. Template edits and archives never rewrite captured results.

- `GET /api/exercises/{id}/history`: uncached `200` with string `id`, `name`, and `workouts`. Each workout contains string `session_id`, captured `name`, UTC `completed_at`, and ordered `occurrences` with `position`, captured `name`, and existing saved-set resources. Workouts sort by completion time then numeric session ID descending; sets sort by saved time then UUID within each occurrence. An owned library exercise or retained completed snapshot with no eligible sets returns `workouts: []`. Missing/foreign identities return `404`, invalid IDs `400`; generic persistence/forwarding errors use `500`/`502`.

No migration, dependencies, or environment changes are required. Focused tests cover identity/ownership, exclusions, ordering, repeated occurrences, exact values, retention, reconnect, and unchanged active data.

## Shared navigation (#41)

Every page uses the same navbar: Home, Exercises, Templates, and Workout history. Sets remains available as a history tab. The current section is highlighted; exercise/template/completed-workout details select their parent section, and active workouts select Templates. Both history tabs select Workout history in the navbar. Detail pages retain contextual back links. Navigation wraps on small screens and supports keyboard focus. The shared light/dark toggle keeps the existing saved theme across navigation and refresh.

## Search and filter workout history (#43)

Both history tabs offer Search names, Exercise, From date, and To date controls. Apply search and filters (or Enter) puts the criteria in the URL; typing alone does not change results. Clear filters retains the selected tab. Switching tabs retains applied criteria, and refresh/back/forward restore the controls and results. Name search is a trimmed, case-insensitive literal substring of captured workout/exercise names; Sessions may match any captured occurrence, while Sets match each set's occurrence or workout name. Filters combine with AND. Sessions can match an exercise even if it has no recorded sets.

URLs use `tab`, `q`, `exercise`, `from`, and `to`, omitting blank criteria and the default Sessions tab. For example `/history?tab=sets&q=press&exercise=12&from=2026-10-01&to=2026-10-13`. Dates are inclusive browser-local completion dates, including daylight-saving changes; either endpoint is optional. Historical exercise options include identities from empty completed snapshots and removed library exercises, labeled with their latest captured name and ID. Renames do not rewrite search names or identity.

`GET /api/history/sessions` and `GET /api/sets` accept `q`, `exercise`, `from`, `to`, and API-only `tz` (IANA timezone, supplied by the browser for dates; defaults to UTC for direct API calls). Malformed/repeated/unknown filter parameters, invalid IDs/dates/timezones, and reversed date ranges return uncached `400` validation errors. Unknown page `tab` values still fall back to Sessions; other malformed page criteria show a validation message and block results until corrected/cleared. Unknown valid exercise IDs simply produce no matches. `GET /api/history/exercises` returns uncached `{"items":[{"id":"12","name":"Press"}]}` from the local owner's completed snapshots only.

Results remain completed-only and newest-first, with deleted sets excluded. No matching results has a clear-filters action; load failures can be retried for the current criteria. No schema, dependencies, or environment changes are needed.

## Temporary dummy-data reset (#46)

Home's three content shortcuts have been removed; use the shared navbar to browse exercises, templates, and history. The reset tool is disabled by default. To use it on a disposable local workout database, set `ENABLE_DEV_RESET=true` in `.env` and run the usual Docker startup command. Compose supplies the setting to both Go and Next.js. Home reads it at runtime, so production images work with the selected setting. Set it back to `false` and recreate the containers to remove the button and disable the endpoint. Invalid values stop Go startup.

**Reset database and add dummy data** opens a confirmation describing the data replacement. Cancel/Escape leaves data intact; confirmation disables repeated submissions while working. The operation replaces the internal local owner's exercises, templates, active/completed sessions, and sets in one transaction, preserving the schema, migration records, owner record, and increasing IDs. A failed transaction rolls back. It never drops tables or resets sequences. Concurrent writes are ordered by temporary table locks; ordinary reads see committed data.

The dataset contains 26 tagged exercise identities, Upper 1 / Lower 1 / Upper 2 / Lower 2 with the supplied order and working-set counts, 48 completed workouts over the preceding 12 weeks, 1,032 completed sets, and an active Upper 1 session with three saved sets. Shared movements reuse IDs; rep-range and RIR targets remain unset. Completion dates are in the past; weights/reps vary deterministically, including zero and decimal kilograms. These are synthetic testing values. The active sets are excluded from completed history. Success refreshes Home's active-session card; history, filters, and previous performance use the ordinary APIs.

- `POST /api/dev/reset`: available only when `ENABLE_DEV_RESET=true` in Go and the forwarding frontend. Requires `X-Confirm-Reset: replace-workout-data`; no request body is needed. Returns uncached `200` with `exercises`, `templates`, `completed_sessions`, `completed_sets`, `active_session_id`, and `active_sets`. Missing confirmation returns `400`; persistence failures return generic `500`, forwarding failures `502`. When disabled, it returns `404`.

If a response is lost, refresh to inspect the current data before confirming another replacement. Repeating a successful reset replaces the dataset rather than accumulating it. The tool is temporary and isolated in `backend/internal/devdata/` and the Home component; remove its registration/configuration/UI when testing no longer needs it. Tests always use a separate disposable database/schema.

## Exercise cards and grouped Sets history (#45)

Click an exercise's name or card background to open `/history?tab=sets&exercise=<id>`. The card is an ordinary link with keyboard focus/new-tab support; Edit and Delete retain their independent dialogs, validation, and template-usage blocking. The separate library View history button is removed. Dedicated `/exercises/[id]/history` pages and completed-detail links remain available.

Sets history now groups matching completed sets by workout, then source exercise occurrence, matching the dedicated exercise-history presentation. Each group shows its captured workout name, browser-local date, and completed-detail link; occurrences keep captured names and exact weight/reps in source order. Repeated workout names and exercise occurrences stay separate by IDs/positions. Existing search/exercise/date criteria are applied before grouping, so empty groups and excluded sets do not appear. Loading, retry, and empty/no-match behavior remain intact.

`GET /api/sets` keeps its existing collection/query behavior and adds string `session_name` (captured workout name) and `exercise_id` to every item. Both tabs highlight Workout history; the separate Sets navbar link is removed, while the Sets tab and legacy `/sets` redirect remain. No schema, dependencies, or environment changes are needed.

## Direct template editing and card actions (#48)

Template cards expand inline editors under #55; `/templates/[id]` remains available for direct links. Start workout and Delete are independent list-card actions. Delete calls the existing archive endpoint: its confirmation explicitly says the template is only archived, can be restored from Archived, and existing active/completed workouts remain intact. Archived cards expand read-only details with Restore template; restoration returns the template to Active. Direct archived-detail URLs remain available. No permanent deletion is added.

The editor offers Save template, Cancel, and Copy workout. Saving stays in editing mode and resets the saved draft baseline; failed saves retain field values and ordering. Cancel and ordinary navigation ask before discarding unsaved changes. Browser refresh/exit uses the native unsaved-changes prompt, and browser Back is guarded while a draft is changed or a save/copy is pending. Modified/new-tab links keep the current draft mounted.

Copy workout uses the last saved configuration, with an explicit notice when edits are unsaved. Its name dialog retains input on validation/failure. A successful copy leaves the original draft intact and offers an Edit copied workout link; opening that link follows the same discard-confirmation rule. The underlying duplicate/archive/restore/session APIs and workout snapshot rules are unchanged. No schema, dependency, framework, or CI changes are needed.

## Descriptions and individual cards (#52)

Exercises and active/archived templates have optional plain-text descriptions and individually bordered, spaced cards. Create/edit forms support multiline descriptions; blank descriptions leave no empty card area. Copy workout copies the saved template description, while unsaved description edits participate in the existing discard/copy rules. Dummy exercises and all four dummy routines include example descriptions.

The exercise and template create/update/read/list resources include `description` as a string. Omitted, null, empty, and whitespace-only input normalize to `""`; surrounding whitespace is trimmed, internal line breaks remain, and rich text/HTML is rendered literally. PUT replaces the full resource, so omission also clears a description. There is no separate description character limit; existing total JSON body limits remain 16 KiB for exercises and 64 KiB for templates. Wrong JSON types are rejected without truncation. Descriptions are not included in search matching.

Migration 008 adds nonnull text columns with empty defaults, preserving existing records and migration history. Descriptions belong only to exercises/templates; active and completed snapshots are neither expanded nor rewritten. Card navigation and independent controls retain their destinations/behavior. No dependency or environment changes are required.

## Inline template editing (#55)

Click a template header to expand its editor on the list page. Start workout and Delete sit beside the template name and stay independently available on active cards. Accordion headers show a pointer cursor on hover. Copy workout appears only inside an expanded editor and copies the last saved configuration, with a notice about unsaved changes. Saving updates the card name/description in place; a copy is added to Active without replacing the original draft.

Collapsing retains drafts and removes editor controls from keyboard navigation until reopened. Cancel confirms discarding edits and resets the inline editor to its last saved configuration. Leaving the list, switching Active/Archived, or starting a workout protects unsaved edits across all mounted editors, including collapsed ones. Delete still archives, explicitly warns about discarding that template’s draft, and preserves active/completed history. Archived accordions are read-only until Restore moves the template back to Active. Direct template URLs retain their existing editor and navigation behavior.
