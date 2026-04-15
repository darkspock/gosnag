# GoSnag

Self-hosted error tracking service compatible with [Sentry SDKs](https://docs.sentry.io/platforms/). Drop-in replacement that receives errors from any Sentry client and provides a clean dashboard to monitor, triage, and resolve issues.

## Features

### Core

- **Sentry SDK compatible** — Works with official Sentry SDKs for JavaScript, Python, Go, Ruby, Java, and more. Supports both legacy `/store/` and modern `/envelope/` ingestion formats
- **Single binary** — Go backend with embedded React frontend and embedded SQL migrations
- **Authentication** — Google Identity Services (OAuth) or local email-based login (`AUTH_MODE=local`)
- **Real-time dashboard** — Browse projects, issues, and stack traces with a modern dark UI
- **Issue lifecycle** — Full status workflow: open, resolved, reopened, ignored, snoozed. Cooldown timers prevent premature auto-reopen after resolution. Snooze by time or event count
- **Issue assignment** — Assign issues to team members, filter by assignee
- **Issue merge** — Merge duplicate issues into one, consolidating events and fingerprint aliases
- **Comments** — Threaded comments on issues with WYSIWYG rich-text editor, @mentions with autocomplete, user attribution, edit, and delete
- **Following** — Follow issues to receive notifications on new events
- **Resolved in release** — Tag issues as fixed in a specific version; auto-reopen if a new event arrives from a different release
- **Event retention** — Configurable automatic cleanup of old events
- **Rate limiting** — Per-IP sliding window rate limiter on ingest endpoints
- **Multi-user** — Role-based access control (admin / viewer), first user auto-promoted to admin
- **User management** — Invite users by email, manage roles and status (active, disabled, invited)
- **Activity log** — Full audit trail of status changes, assignments, merges, and ticket actions on each issue

### Alerting and Automation

- **Alerts** — Email (SMTP) and Slack webhook notifications with flexible condition-based filtering
- **Unified condition engine** — AND/OR composable rules shared across alerts, Jira, GitHub, priority scoring, and auto-tagging. Supports conditions on level, platform, environment, release, title, event data, total events, velocity (1h/24h), and user count
- **Jira Cloud integration** — Automatic and manual Jira ticket creation with configurable rules per project. Test connection, configure per-project credentials
- **GitHub Issues integration** — Automatic and manual GitHub issue creation with configurable rules, label assignment, and stack trace inclusion
- **Priority scoring** — Rule-based dynamic priority scores (0-100) for issues based on velocity, event count, platform, and custom conditions. Bulk recalculation when rules change
- **Auto-tagging** — Automatically apply key:value tags to issues when they match patterns or conditions. Tags are also manually assignable. Filter issues by tag
- **N+1 query detection** — Background worker that identifies repeated similar query patterns in error stack traces

### Ticket Management

- **Managed workflow** — Optional ticket-based incident management alongside issues. Configurable per project (simple mode or managed mode)
- **Ticket lifecycle** — Acknowledged → In Progress → In Review → Done, with Escalated and Won't Fix statuses. Status transitions enforced by rules
- **Manual tickets** — Create standalone tickets not linked to any specific issue
- **Ticket attachments** — Upload documents (PDF, CSV, JSON, ZIP, etc.) and images with S3 or local disk storage
- **Ticket-to-issue linking** — Link tickets to issues for traceability. Resolving a ticket can auto-resolve linked issues

### Source Code Integration

- **Repository connection** — Connect GitHub or Bitbucket repositories to projects with token-based authentication
- **Suspect commits** — Identify commits that touched files appearing in error stack traces
- **Release tracking** — Map releases to commits, generate release notes, view deploy history
- **Deploy webhook** — Receive deploy notifications and track deployments per project

### AI Integration

- **Multi-provider support** — OpenAI, Anthropic Claude, Groq, Amazon Bedrock, and Ollama (local). Provider-agnostic interface with configurable models
- **Root cause analysis** — AI-powered analysis of error causes with context synthesis
- **Merge suggestions** — AI-powered duplicate detection and merge recommendations, with background worker for continuous scanning
- **Ticket description generation** — Auto-generate detailed descriptions when creating Jira/GitHub tickets from issues
- **Priority rule suggestions** — AI recommends priority scoring rules based on project patterns
- **Alert suggestions** — AI recommends alert configurations based on project history
- **Tag suggestions** — AI recommends auto-tagging rules with classification analysis
- **Deploy anomaly detection** — Background worker that analyzes recent deployments for error spikes
- **Extended thinking** — Support for Bedrock/Claude thinking models for complex analysis
- **Per-project configuration** — Enable/disable individual AI features, set model overrides, configure daily token budgets and rate limits
- **Response caching** — Prompt-hash-based caching to avoid redundant API calls

### Organization

- **Project groups** — Organize projects into groups (e.g., Production, Development) with tab-based navigation
- **Favorites** — Star projects for quick access
- **Drag-and-drop reorder** — Reorder projects on the dashboard via drag-and-drop
- **Custom project icons** — Configurable icons and colors for each project

### Event Details

- **Stack traces** — Full frame-by-frame display with filename, function, line/column, and source context
- **Breadcrumbs** — Timeline of user actions leading up to the error
- **User context** — Affected user info (ID, email, IP)
- **Request context** — HTTP method, URL, headers, query params, POST data
- **Tags and extra data** — Custom key:value pairs and arbitrary context from the SDK
- **Release and environment** — Track which version and environment produced the error
- **Suspect commits** — View commits that may have introduced the error, linked to GitHub/Bitbucket

### Dashboard and Filtering

- **Project stats** — Open issues, total issues, 7-day trend chart, errors this week vs. last week
- **Issue filtering** — Filter by status, level, tag, assignee, release, date (today), and full-text search on title
- **Bulk operations** — Select multiple issues to delete or merge
- **Pagination** — Configurable page size with offset-based navigation
- **Keyboard shortcuts** — Navigate issues and search with keyboard

### API and Integrations

- **Personal access tokens** — Per-user API tokens (`gsn_` prefix) with read or read/write permissions, optional expiry, and SHA-256 hashing
- **Project-scoped tokens** — API tokens restricted to a single project
- **REST API** — Full management API for projects, issues, events, alerts, tags, comments, users, tickets, and tokens
- **MCP server** — [Model Context Protocol](https://modelcontextprotocol.io/) server for AI assistant integration (Claude, etc.), exposing project, issue, alert, tag, ticket, and user management tools

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
| `GOOGLE_CLIENT_ID` | Yes* | Google OAuth client ID (from Google Cloud Console). *Not required if `AUTH_MODE=local` |
| `PORT` | No | Server port (default: 8080) |
| `BASE_URL` | No | Public URL for DSN generation and OAuth redirects (default: `http://localhost:8080`) |
| `AUTH_MODE` | No | `google` (default) or `local` (email-based login, no OAuth required) |
| `LOG_LEVEL` | No | `debug`, `info`, `warn`, `error` (default: info) |
| `SESSION_SECRET` | No | Secret for session tokens |
| `DEFAULT_COOLDOWN_MINUTES` | No | Cooldown after resolving issues (default: 30) |
| `EVENT_RETENTION_DAYS` | No | Auto-delete events older than N days (default: 90, 0 = keep forever) |
| `INGEST_RATE_LIMIT_PER_MIN` | No | Max ingest requests per IP per minute (default: 0 = unlimited) |
| `SMTP_HOST`, `SMTP_PORT`, `SMTP_USER`, `SMTP_PASSWORD`, `SMTP_FROM` | No | Email alerts via SMTP |
| `SLACK_WEBHOOK_URL` | No | Default Slack webhook (can also be configured per alert) |
| `CORS_ALLOWED_ORIGINS` | No | Comma-separated list of allowed origins for the management API |
| `UPLOAD_S3_BUCKET` | No | S3 bucket for file uploads (if unset, uses local disk) |
| `UPLOAD_S3_REGION` | No | S3 bucket region (default: `AWS_REGION` or eu-west-1) |
| `UPLOAD_S3_PREFIX` | No | S3 key prefix for uploads (default: `uploads/`) |
| `UPLOAD_S3_CDN_URL` | No | CDN URL for serving uploaded files from S3 |
| `AI_PROVIDER` | No | AI provider: `openai`, `claude`, `groq`, `bedrock`, or `ollama` |
| `AI_API_KEY` | No | API key for the AI provider (not needed for Bedrock/Ollama) |
| `AI_MODEL` | No | Model ID to use (provider-specific) |
| `AI_BASE_URL` | No | Custom base URL for the AI provider (useful for Ollama) |
| `AI_BEDROCK_REGION` | No | AWS region for Bedrock (default: `AWS_REGION` or eu-west-1) |
| `AI_BEDROCK_MODEL_ID` | No | Bedrock model ID for standard requests |
| `AI_BEDROCK_THINKING_MODEL_ID` | No | Bedrock model ID for extended thinking requests |
| `AI_MAX_TOKENS_PER_DAY` | No | Daily token budget per project (default: 1,000,000) |
| `AI_MAX_CALLS_PER_MINUTE` | No | Max AI API calls per minute (default: 30) |

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

Available tools: `list_projects`, `get_project`, `create_project`, `update_project`, `delete_project`, `list_issues`, `get_issue`, `update_issue_status`, `get_issue_events`, `get_issue_counts`, `list_alerts`, `create_alert`, `list_issue_tags`, `add_issue_tag`, `list_users`, `create_ticket`, `get_ticket`, `update_ticket`, `list_tickets`, `get_ticket_counts`.

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

### Background Workers

GoSnag runs several background workers automatically:

- **Cooldown checker** (every 1 min) — Reopens snoozed/resolved issues when cooldown or snooze time expires
- **Session cleanup** (every 1 hour) — Deletes expired user sessions
- **Event retention** (every 6 hours) — Deletes events older than `EVENT_RETENTION_DAYS`
- **N+1 detector** (every 10 min) — Scans stack traces for repeated query patterns
- **Query pattern cleanup** (every 24 hours) — Cleans old N+1 detection data
- **AI merge checker** (every 5 min) — Finds and suggests duplicate issue merges (requires AI)
- **Deploy analyzer** (every 2 min) — Detects post-deploy error anomalies (requires AI)
- **AI cache cleanup** (every 1 min) — Removes stale cached AI responses

## Tech Stack

- **Backend**: Go, Chi router, sqlc, PostgreSQL, golang-migrate
- **Frontend**: React, TypeScript, Vite, Tailwind CSS v4, dnd-kit
- **Auth**: Google Identity Services (client-side flow) or local email login
- **MCP**: TypeScript, `@modelcontextprotocol/sdk`
- **Deploy**: Docker, Docker Compose

## License

[MIT](LICENSE)
