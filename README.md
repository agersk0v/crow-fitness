# Crow Fitness

A simple workout tracker for one person, with no authentication or accounts.

Create personal exercises and reusable workout templates, record one active workout, and browse completed sessions and sets. There is no login; use this on your own computer, not as a public service.

- [Product behavior](docs/PRODUCT.md): current pages, rules, and separately identified future work.
- [Architecture](docs/ARCHITECTURE.md): services, persistence, API contracts, and validation responsibilities.
- [Agent instructions](AGENTS.md): scope, data care, and PR review workflow.

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

The smoke check covers one exercise/template/session flow through Next.js, a validation response, active-session conflict, completion/cancellation, readiness outage/recovery, and saved-session persistence through database and whole-stack restarts. Detailed CRUD and history-query assertions belong to the Go tests. Page fetches verify routes render, not interactive form behavior. Each run owns a randomly named Compose project and removes only its disposable test volume afterward.

To run the same integration check locally, use Bash with Docker running:

```sh
bash scripts/smoke.sh
```

It uses port `13000`; set `SMOKE_PORT` to another available port if necessary. No `.env` file is required for the smoke check because it supplies its own test configuration.

## Use the app

Use the shared navbar for Exercises, Templates, and Workout history, and its light/dark toggle. Create tagged exercises first, then a template with planned working-set counts. Start workout opens the saved session; Home offers Continue session. Add/Save records sets in the active workout, Complete session puts them in history, and confirmed cancellation discards the workout and its sets. See [product behavior](docs/PRODUCT.md) for editing, reference, and filtering rules.

## Migrations and test databases

Go applies embedded numbered SQL migrations before serving traffic. Each migration and its version record commit together; an advisory lock serializes concurrent starts. Applied versions are skipped and normal restarts preserve data. Migration failures stop startup: inspect API logs. Add a new numbered migration rather than modifying applied SQL.

For PostgreSQL tests, set `TEST_DATABASE_URL` to a **separate disposable test database**, then run `go test -race ./...` from `backend/`. Never point it at your workout database: these tests apply migrations and mutate database records. Without this variable, PostgreSQL tests explicitly skip. The smoke script supplies its own isolated database and volume.

## Temporary dummy-data reset

For disposable local data only, set `ENABLE_DEV_RESET=true` in `.env` and run the Docker startup command again. Home then shows **Reset database and add dummy data**, with a confirmation before replacing the local owner's exercises, templates, sessions, and sets. Set the flag back to `false` and recreate containers to hide the button and disable the endpoint. Invalid flag values stop Go startup.

Reset is atomic and preserves tables, migration metadata, the owner record, and increasing IDs; it does not drop tables. It seeds 26 tagged exercises and four described templates (Upper 1, Lower 1, Upper 2, Lower 2), 48 completed workouts over 12 weeks with 1,032 completed sets, and one active Upper 1 workout with three saved sets. It replaces rather than accumulates data. If the response is lost, refresh and inspect the current data before confirming another reset. This temporary tool is disabled by default in both frontend and Go.

## UI development

Shared shadcn/ui source is in `frontend/components/ui/`, with configuration in `frontend/components.json`. From `frontend/`, add only needed components with:

```sh
npx shadcn@4.21.4 add <component-name>
```

Review generated source, dependencies, styles, and the lockfile, then run the frontend checks. Use shared theme tokens, visible keyboard focus, accessible labels, and the existing theme-aware Sonner Toaster. Motion for React entrance animations must stay brief, leave server-rendered content visible, and respect reduced-motion preferences.
