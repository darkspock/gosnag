# GoSnag

Self-hosted error tracking service compatible with [Sentry SDKs](https://docs.sentry.io/platforms/). Drop-in replacement that receives errors from any Sentry client and provides a clean dashboard to monitor, triage, and resolve issues.

## Features

- **Sentry SDK compatible** — Works with official Sentry SDKs for JavaScript, Python, Go, Ruby, Java, and more
- **Single binary** — Go backend with embedded React frontend and embedded SQL migrations
- **Google Sign-In** — Secure authentication via Google Identity Services
- **Real-time dashboard** — Browse projects, issues, and stack traces with a modern dark UI
- **Issue management** — Assign, resolve, snooze with cooldown timers, and track regressions
- **Alerts** — Email (SMTP) and Slack webhook notifications on new issues
- **Event retention** — Configurable automatic cleanup of old events
- **Rate limiting** — Per-IP sliding window rate limiter on ingest endpoints
- **Multi-user** — Role-based access control (admin / member), first user auto-promoted to admin

## Quick Start

### Docker Compose (recommended)

```bash
cp .env.example .env
# Edit .env with your GOOGLE_CLIENT_ID and DATABASE_URL

make docker-up
```

The app will be available at `http://localhost:8080`.

### From Source

```bash
# Prerequisites: Go 1.25+, Node 20+, PostgreSQL

make build
./gosnag
```

## Configuration

All configuration is via environment variables. See [`.env.example`](.env.example) for the full list.

| Variable | Required | Description |
|----------|----------|-------------|
| `DATABASE_URL` | Yes | PostgreSQL connection string |
| `GOOGLE_CLIENT_ID` | Yes | Google OAuth client ID (from Google Cloud Console) |
| `PORT` | No | Server port (default: 8080) |
| `LOG_LEVEL` | No | `debug`, `info`, `warn`, `error` (default: info) |
| `SESSION_SECRET` | No | Secret for session tokens |
| `DEFAULT_COOLDOWN_MINUTES` | No | Cooldown after resolving issues (default: 30) |
| `EVENT_RETENTION_DAYS` | No | Auto-delete events older than N days (default: 90, 0 = keep forever) |
| `INGEST_RATE_LIMIT_PER_MIN` | No | Max ingest requests per IP per minute (default: 0 = unlimited) |
| `SMTP_HOST`, `SMTP_PORT`, `SMTP_USER`, `SMTP_PASSWORD`, `SMTP_FROM` | No | Email alerts |
| `SLACK_WEBHOOK_URL` | No | Slack alerts |

## Connecting a Sentry SDK

Use your project's DSN (shown in Project Settings) with any Sentry SDK:

```javascript
// JavaScript example
Sentry.init({
  dsn: "https://<key>@your-gosnag-host/<project-id>",
});
```

```python
# Python example
sentry_sdk.init(dsn="https://<key>@your-gosnag-host/<project-id>")
```

## Admin Management

```bash
# Local Docker
make admin EMAIL=user@example.com

# Remote server
make remote-admin EMAIL=user@example.com HOST=your-server-ip
```

## Development

```bash
make dev        # Hot reload (backend + frontend)
make sqlc       # Regenerate DB queries after editing SQL
make migrate    # Run database migrations
```

## Tech Stack

- **Backend**: Go, Chi router, sqlc, PostgreSQL, golang-migrate
- **Frontend**: React, TypeScript, Vite, Tailwind CSS v4
- **Auth**: Google Identity Services (client-side flow)
- **Deploy**: Docker, Docker Compose

## License

[MIT](LICENSE)
