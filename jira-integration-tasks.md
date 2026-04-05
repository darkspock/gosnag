# Jira Integration — Create Tickets from GoSnag Issues

## Overview
Integrate GoSnag with Jira Cloud to create tickets from issues, both manually (button per issue) and automatically (rule-based conditions). Each GoSnag project can be linked to a different Jira project.

## Design

### Jira Connection (per GoSnag project in Settings)
- Jira base URL (e.g. https://company.atlassian.net)
- API email + API token (Jira Cloud API tokens)
- Jira project key (e.g. DEV, OPS)
- Issue type (e.g. Bug, Task)

### Manual Creation
- Button "Create Jira Ticket" on issue detail page
- Creates ticket with: title, link to GoSnag, level, event count, platform, latest stacktrace
- Stores Jira ticket key (e.g. DEV-123) on the GoSnag issue for reference
- Link to Jira ticket shown on issue detail

### Automatic Creation (Rules)
- Configurable rules per project in Settings
- Conditions (combinable with AND):
  - Level (fatal, error, warning, etc.)
  - Minimum event count
  - Minimum user count
  - Title pattern (contains/regex)
- Evaluated on event ingestion when conditions are met
- Only creates ticket once per issue (tracks jira_ticket_key on issue)
- Respects cooldown: don't create duplicate tickets for same issue

### Ticket Content
- Summary: `[GoSnag] {issue title}`
- Description: level, platform, event count, first/last seen, link to GoSnag issue, latest stacktrace

## Tasks

### Backend — Database
- [ ] Migration `000008_jira_integration` — add jira config fields to projects table + jira_ticket_key to issues table + jira_rules table
- [ ] SQL queries for jira config CRUD, rules CRUD, and issue jira_ticket_key update
- [ ] Run sqlc

### Backend — Jira Client
- [ ] `internal/jira/client.go` — Jira Cloud REST API client (create issue, basic auth with email+token)
- [ ] Build ticket description from GoSnag issue data

### Backend — Manual Creation
- [ ] Endpoint: POST `/api/v1/projects/{id}/issues/{issue_id}/jira` — creates Jira ticket, stores key
- [ ] Return Jira ticket key + URL

### Backend — Automatic Rules
- [ ] `internal/jira/rules.go` — evaluate rules against issue state
- [ ] Hook into ingest pipeline: after event processed, check rules, create ticket if conditions met
- [ ] Jira rules CRUD handler + routes

### Frontend — Settings
- [ ] Jira connection config section in ProjectSettings (URL, email, token, project key, issue type)
- [ ] Test connection button
- [ ] Jira rules management (add/edit/delete rules with condition builder)

### Frontend — Issue Detail
- [ ] "Create Jira Ticket" button (hidden if already linked)
- [ ] Show Jira ticket link (DEV-123) when linked
