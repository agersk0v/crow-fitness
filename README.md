# Crow Fitness

A simple workout tracker for one person, with no authentication or accounts.

## Current status

The Next.js frontend, Go API, and PostgreSQL support creating, searching, tagging, renaming, and safely deleting personal exercises at `/exercises`. Names are trimmed and validated in Go. Saved exercises survive refresh and stack restart. Go selects one internal local owner; the app has no login or authentication. You can also create, edit, duplicate, archive, and restore persistent workout templates at `/templates`. Start a workout from an active template, continue the saved session from home, or cancel/complete it without storing history. Logging and progress remain planned.

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

- Previous performance references and weight/reps logging.
- Workout history and sets grouped by exercise to track progress.

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

Open `/templates` from home or exercises. Give the template a name, add saved exercises, and drag each exercise card’s background to arrange them, or hold briefly before dragging on touch screens. Inputs, labels, dropdowns, and buttons keep their normal behavior. Keyboard users can focus the card, press Space, use arrow keys, and press Space to drop or Escape to cancel. Each occurrence requires a positive whole-number working-set count. Rep bounds are optional: leave both blank, use a positive minimum/maximum range, or equal values for a fixed target. Target RIR is optional and accepts nonnegative decimals, including zero. One rep/RIR target applies to all working sets of an occurrence; warm-up sets are excluded. The same exercise may appear more than once with different targets. Saving persists the entire ordered configuration atomically; select a saved template to reopen it. Active templates can be edited or duplicated. Archive preserves their configuration and hides them from the Active list; inspect or restore them in Archived. Archived templates are read-only and must be restored before editing or duplicating. Start workout opens a persisted active session. Permanent deletion and recording sets are not included.

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

Template editing offers Save/Cancel and retains drafts after failed saves. Duplication requests a name for the independent copy. Archive requires confirmation naming the template; Cancel/Escape leaves it unchanged. Archived exercise references are preserved until an active template is edited to remove them. Concurrent edits apply complete configurations in commit order; there is no draft merging. Active sessions preserve their own snapshots; completed workout history remains future work.

## Active workout sessions

Choose **Start workout** on an active template. The session saves a copy of the template name, ordered exercises/names, working-set counts, and optional rep/RIR targets. Later edits, archiving, or removal of an otherwise unused exercise do not change this copy. Only one session can be active; trying to start another offers **Continue session** without replacing it.

Home shows **Continue session**, including after refresh or a stack/database restart. **Continue later** returns home while retaining the session. **Cancel session** asks for confirmation; Keep session or Escape dismisses it. Confirmed cancellation and **Complete session** clear the session and return home. Neither saves set results or completed-session history. Request failures show an error and allow retry using the same action.

Migration `005_active_sessions.sql` runs automatically during API startup and preserves existing exercises/templates. No new environment settings or services are required.

- `POST /api/sessions`: `application/json` with `{"template_id":"1"}`; `201` with the copied session and `Location`. An existing session returns `409`, `error.code: "active_session_exists"`, and `error.session` containing the existing snapshot for continuation. Archived/empty templates return `409`; missing/foreign-owner templates return `404` when no session is active.
- `GET /api/sessions/active`: `200` with `{"session":null}` or `{"session":{...}}`.
- `GET /api/sessions/{id}`: `200` with the snapshot, or `404` if missing/ended/foreign-owned.
- `POST /api/sessions/{id}/cancel` and `/complete`: bodyless actions returning `204`; delete only that owner's matching session and its snapshot rows. Repeated requests and stale IDs are harmless, including when a newer workout exists.

Session resources contain string `id` and `template_id`, copied `name`, UTC `started_at`, and `exercises` using the same occurrence shape/target types as template details. Responses are uncached. Start accepts only a canonical positive string template ID; unknown fields/malformed JSON return `400`, bodies above 4 KiB return `413`, and unsupported media types return `415`. Persistence/forwarding failures use the existing generic `500`/`502` conventions. No set/history endpoints are available.

Focused PostgreSQL tests cover snapshots, null/zero/decimal targets, owner isolation, concurrent starts, atomic failed starts, migration reruns/reconnection, clearing rows, and stale end requests. The stack smoke also checks session forwarding and persistence through database/full-stack restarts.
