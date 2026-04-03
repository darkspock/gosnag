# GoSnag - Self-hosted Error Tracking

Sentry-compatible error tracking service. Go backend + React frontend, single binary with embedded SPA.

## Architecture

- **Backend**: Go with Chi router, sqlc for DB queries, PostgreSQL
- **Frontend**: React + TypeScript + Vite + Tailwind v4 (embedded via `go:embed`)
- **Auth**: Google Identity Services (client-side JS flow, no callback URL)
- **Deploy**: Docker Compose (single host) or any container orchestration

## Key Commands

```bash
make dev              # Run backend + frontend with hot reload
make build            # Build single binary (frontend + backend)
make docker           # Docker Compose up --build
make docker-up        # Docker Compose up --build -d (background)
make docker-down      # Docker Compose down
make sqlc             # Regenerate sqlc code from SQL queries
make migrate          # Run database migrations
make frontend         # Build frontend only
```

## Admin Management

```bash
# Local Docker:
make admin EMAIL=user@example.com

# Remote:
make remote-admin EMAIL=user@example.com HOST=<your-server-ip>
```

## Project Structure

- `cmd/gosnag/` — Entry point, router, background workers
- `internal/auth/` — Google Identity Services auth, session middleware
- `internal/ingest/` — Sentry SDK event ingestion (store + envelope)
- `internal/issue/` — Issue management, cooldown checker
- `internal/project/` — Project CRUD
- `internal/alert/` — Email/Slack alert system
- `internal/config/` — Environment config loading
- `internal/database/` — DB connection, migrations, sqlc generated code
- `web/` — React SPA (embedded at build time via `web/embed.go`)
- `migrations/` — SQL migration files

## Environment Variables

| Variable | Description |
|----------|-------------|
| `DATABASE_URL` | PostgreSQL connection string |
| `GOOGLE_CLIENT_ID` | Google OAuth client ID |
| `ALLOWED_DOMAIN` | Email domain restriction (optional) |
| `PORT` | Server port (default: 8080) |
