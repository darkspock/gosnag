# GoSnag Roadmap

## Completed

| Epic | Description | Status |
|------|-------------|--------|
| Project Groups | Tab-based project organization (Production, Development, etc.) | Done |
| Issue Management | Ticket-based incident management with workflow statuses | Done |
| Unified Condition Engine | Composable AND/OR condition rules shared across alerts, priority, tags, and Jira | Done |
| N+1 Detection | Background worker that identifies repeated query patterns in stack traces | Done |
| Source Code Integration | GitHub and Bitbucket integration with suspect commits and release tracking | Done |
| Issue Tags | Manual and auto-rule tagging with AI-based classification | Done |
| Priority Scoring | Rule-based dynamic priority (0-100) with AI-powered rules | Done |
| AI Integration | Multi-provider AI for root cause analysis, merge suggestions, triage, and ticket descriptions | Done |

## In Progress

Nothing currently in progress.

## Pending

| Epic | Description | Doc | Priority |
|------|-------------|-----|----------|
| **DataCheck** | Proactive database integrity monitoring. Define SQL queries with cron schedules against external databases — GoSnag creates issues when assertions fail. Supports expect_empty, expect_rows, and row_count threshold modes. Integrates with existing alert pipeline. | [datacheck-epic.md](epics/datacheck-epic.md) | High |
| **Multi-Tenant Organizations** | Organization-level tenant boundary with per-org roles and project isolation. Enables SaaS model or team separation within a single GoSnag instance. | [multitenancy-epic.md](multitenancy-epic.md) | High |
| **Local Edge Relay Agent** | Lightweight Go binary running on app servers as a local Sentry-compatible relay. Sub-millisecond latency, local buffering, async forwarding. No filtering — intelligence stays server-side. | [gosnag-agent-epic.md](gosnag-agent-epic.md) | Medium |
