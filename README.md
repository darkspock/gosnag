# GoSnag

Self-hosted error tracking service compatible with [Sentry SDKs](https://docs.sentry.io/platforms/). Drop-in replacement that receives errors from any Sentry client and provides a clean dashboard to monitor, triage, and resolve issues.

## Features

### Core

- **Sentry SDK compatible** — Works with official Sentry SDKs for JavaScript, Python, Go, Ruby, Java, and more
- **Single binary** — Go backend with embedded React frontend and embedded SQL migrations
- **Google Sign-In** — Secure authentication via Google Identity Services
- **Real-time dashboard** — Browse projects, issues, and stack traces with a modern dark UI
- **Issue management** — Assign, resolve, snooze with cooldown timers, track regressions, and merge duplicate issues
- **Event retention** — Configurable automatic cleanup of old events
- **Rate limiting** — Per-IP sliding window rate limiter on ingest endpoints
- **Multi-user** — Role-based access control (admin / member), first user auto-promoted to admin

### Alerting and Automation

- **Alerts** — Email (SMTP) and Slack webhook notifications with flexible condition-based filtering
- **Unified condition engine** — AND/OR composable rules shared across alerts, Jira, priority scoring, and auto-tagging. Supports conditions on level, platform, environment, release, title, event data, total events, velocity (1h/24h), and user count
- **Jira Cloud integration** — Automatic and manual Jira ticket creation with configurable rules per project
- **Priority scoring** — Rule-based dynamic priority scores (0-100) for issues based on velocity, event count, platform, and custom conditions
- **Auto-tagging** — Automatically apply key:value tags to issues when they match patterns or conditions. Tags are also manually assignable

### Organization

- **Project groups** — Organize projects into groups (e.g., Production, Development) with tab-based navigation
- **Favorites** — Star projects for quick access
- **Issue merge** — Merge duplicate issues into one, consolidating events and fingerprint aliases

### API and Integrations

- **Personal access tokens** — Per-user API tokens (`gsn_` prefix) with read or read/write permissions, optional expiry, and SHA-256 hashing
- **REST API** — Full management API for projects, issues, alerts, tags, and users
- **MCP server** — [Model Context Protocol](https://modelcontextprotocol.io/) server for AI assistant integration (Claude, etc.), exposing project, issue, alert, tag, and user management tools

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
| `SLACK_WEBHOOK_URL` | No | Default Slack webhook (can also be configured per alert) |
| `CORS_ALLOWED_ORIGINS` | No | Comma-separated list of allowed origins for the management API |

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

## API Access

### Personal Access Tokens

Generate tokens from **Settings > Access Tokens** in the web UI. Use them as Bearer tokens:

```bash
curl -H "Authorization: Bearer gsn_..." https://your-gosnag-host/api/v1/projects
```

Tokens inherit the creating user's role (admin or member) and can be scoped as `read` or `readwrite`.

### MCP Server (AI Integration)

GoSnag includes an MCP server for integration with AI assistants like Claude:

```json
{
  "mcpServers": {
    "gosnag": {
      "command": "node",
      "args": ["path/to/gosnag/mcp/dist/index.js"],
      "env": {
        "GOSNAG_URL": "https://your-gosnag-host",
        "GOSNAG_TOKEN": "gsn_your-personal-access-token"
      }
    }
  }
}
```

Available tools: `list_projects`, `get_project`, `create_project`, `update_project`, `delete_project`, `list_issues`, `get_issue`, `update_issue_status`, `get_issue_events`, `get_issue_counts`, `list_alerts`, `create_alert`, `list_issue_tags`, `add_issue_tag`, `list_users`.

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
make build      # Build single binary (frontend + backend)
make sqlc       # Regenerate DB queries after editing SQL
make migrate    # Run database migrations
make frontend   # Build frontend only
```

## Tech Stack

- **Backend**: Go, Chi router, sqlc, PostgreSQL, golang-migrate
- **Frontend**: React, TypeScript, Vite, Tailwind CSS v4
- **Auth**: Google Identity Services (client-side flow)
- **MCP**: TypeScript, `@modelcontextprotocol/sdk`
- **Deploy**: Docker, Docker Compose

## License

[MIT](LICENSE)
