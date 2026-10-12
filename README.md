# Crow Fitness

A simple workout tracker for one person, with no authentication or accounts.

## Current status

The runnable foundation is implemented: a Next.js frontend with TypeScript and Tailwind CSS, a Go API, PostgreSQL connectivity, and CI checks. The home page checks the API and database connection and links to an `/exercises` preview. The preview starts with a simple array of sample exercises and lets you add names to an in-memory list. Names are trimmed and blank names show an inline error. Refreshing restores the sample data; API integration and durable exercise storage are still pending in #11. Workout templates, logging, and exercise progress are planned and are not implemented yet.

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

CI runs those checks on PRs and commits to `main`, then builds and checks the full Docker stack. The smoke check tests frontend-to-API routing, database readiness failure/recovery, and persistence through a database restart. It uses a separate randomly named Compose project with disposable data and removes only that project's test volume afterward.

To run the same integration check locally, use Bash with Docker running:

```sh
bash scripts/smoke.sh
```

It uses port `13000`; set `SMOKE_PORT` to another available port if necessary. No `.env` file is required for the smoke check because it supplies its own test configuration.

## Planned features

- Reusable workout templates and sessions.
- Previous performance references and weight/reps logging.
- Workout history and sets grouped by exercise to track progress.

The product document lists decisions to confirm before implementing these flows. This setup is intended for your own computer; it has no authentication and is not configured for public hosting.
