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
| **Query Views / Reports** | Read-only query experience over controlled PostgreSQL views so users can build saved reports, inspect trends, and answer operational questions without leaving GoSnag. Inspired by tools like New Relic query consoles, but limited to whitelisted analytical views. | [query-views-requirements.md](query-views-requirements.md) | High |
| **Project Settings Schema Cleanup** | Finish the settings refactor by removing legacy project setting columns from `projects`, switching all reads/writes to the new domain tables, and closing the compatibility layer safely after rollout validation. | [project-settings-schema-cleanup.md](project-settings-schema-cleanup.md) | High |
| **Custom Ticket Workflows** | Per-project ticket workflows with customizable states, transitions, and sidebar counters so teams can model their own operational process instead of using a fixed status set. | TBD | High |
| **Ticket Provider Integrations** | Create and sync tickets with external providers such as GitHub and Bitbucket, so GoSnag tickets can be escalated or mirrored into engineering work trackers when needed. | TBD | High |
| **Multi-Tenant Organizations** | Organization-level tenant boundary with per-org roles and project isolation. Enables SaaS model or team separation within a single GoSnag instance. | [multitenancy-epic.md](multitenancy-epic.md) | High |
| **Local Edge Relay Agent** | Lightweight Go binary running on app servers as a local Sentry-compatible relay. Sub-millisecond latency, local buffering, async forwarding. No filtering — intelligence stays server-side. | [gosnag-agent-epic.md](gosnag-agent-epic.md) | Medium |
