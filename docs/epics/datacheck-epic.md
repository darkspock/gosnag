# Epic: DataCheck — Proactive Database Integrity Monitoring

## Vision

GoSnag is reactive today — it catches errors after they happen. DataCheck makes it **predictive** by letting teams define SQL queries that run on a schedule against their databases, detecting data corruption, orphaned records, and inconsistencies **before** they surface as runtime errors.

This is especially valuable for legacy applications where years of migrations, manual fixes, and edge-case bugs leave behind corrupt or inconsistent data that eventually causes production failures.

## Problem Statement

Legacy applications accumulate data integrity issues over time:
- Orphaned records from incomplete cascading deletes
- Broken foreign key relationships (soft deletes, missing constraints)
- Inconsistent state flags (e.g., orders marked "shipped" with no tracking number)
- Stale or expired data that should have been cleaned up
- Duplicate records from race conditions
- Schema drift between what the app expects and what exists

These problems are invisible until they cause a user-facing error. By then, the damage is done — users are impacted, support tickets are filed, and engineers scramble to fix both the symptom and the root cause.

## Solution

Add a **DataCheck** section to GoSnag where users can:

1. **Configure external database connections** per project (encrypted credentials, read-only enforced)
2. **Define check queries** — SQL queries with a configurable assertion on the result (expect empty, expect rows, or threshold-based)
3. **Set a cron schedule** — standard cron expressions for full scheduling flexibility (`*/5 * * * *`, `0 9 * * MON-FRI`, etc.)
4. **Generate issues automatically** — when a check's assertion fails, GoSnag creates/updates an issue (platform=`datacheck`) and calls `alertService.Notify()` — fully integrated with existing alert configs, no parallel notification system
5. **Track check history** — see when checks ran, what they found, and how results trend over time

## User Stories

### Database Connection Management

- **As an admin**, I want to add an external database connection to a project so that DataCheck can query it
- **As an admin**, I want to test a database connection before saving it so I know it works
- **As an admin**, I want connections to be encrypted at rest so credentials are secure
- **As an admin**, I want to enforce read-only connections so DataCheck can never modify user data
- **As an admin**, I want to support PostgreSQL and MySQL connections so I can monitor our main database engines

### Check Definition

- **As a user**, I want to create a named check with a SQL query so I can monitor a specific data integrity condition
- **As a user**, I want to choose an assertion mode for each check so it matches my use case:
  - **expect_empty** — alert when the query returns rows (e.g., "find restaurants with misconfigured Adyen" — any row = problem)
  - **expect_rows** — alert when the query returns zero rows (e.g., "reservations in the last 15 minutes" — no rows = something is wrong)
  - **row_count threshold** — alert when row count crosses a numeric threshold with a configurable operator (gt, gte, lt, lte, eq, neq)
- **As a user**, I want to set a cron schedule (e.g., `*/5 * * * *`, `0 8 * * MON-FRI`) so checks run at exactly the right times
- **As a user**, I want common cron presets available (every 1min, 5min, 15min, hourly, daily at 9am) so I don't have to write cron syntax from scratch
- **As a user**, I want to see a human-readable description of the cron expression (e.g., "Every 5 minutes", "Weekdays at 9:00 AM") so I can verify the schedule
- **As a user**, I want to set a severity level (info, warning, error, fatal) for each check so generated issues have the right priority
- **As a user**, I want to enable/disable individual checks without deleting them
- **As a user**, I want to add a description to each check explaining what it detects and why it matters
- **As a user**, I want to preview/test a check query before activating it so I can verify it works
- **As a user**, I want to see the last run result and next scheduled run for each check

### Issue Generation and Alerts

- **As a user**, I want DataCheck to create a GoSnag issue when a check fails so it flows through my existing triage workflow
- **As a user**, I want repeated failures of the same check to increment the existing issue's event count (not create duplicates) so I can see frequency without noise
- **As a user**, I want the generated issue to include the check name, row count, sample rows (first 10), and the query that was run so I have context to investigate
- **As a user**, I want the issue to auto-resolve when the check assertion passes again so resolved issues reflect actual state
- **As a user**, I want DataCheck issues to flow through the existing alert system so I can configure email/Slack notifications using alert configs with `platform_is: datacheck` conditions
- **As a user**, I want DataCheck issues to be visually distinguishable from regular error-ingested issues (platform badge or tag)

### History and Observability

- **As a user**, I want to see the execution history of each check (last N runs, pass/fail, row counts, duration) so I can spot trends
- **As a user**, I want to see a summary dashboard showing all checks, their status, and last run time
- **As a user**, I want to see if a check is failing due to a connection error vs. actual data problems so I can distinguish operational issues from data issues

### AI Integration (stretch)

- **As a user**, I want AI to suggest check queries based on my database schema so I don't have to write all checks from scratch
- **As a user**, I want AI to analyze check results and suggest root causes or fix queries

## Examples

### Corruption detection (expect_empty)

> "Find restaurants with misconfigured Adyen payment gateway"

| Field | Value |
|-------|-------|
| Query | `SELECT id, name FROM restaurants WHERE adyen_merchant_account IS NULL AND payment_provider = 'adyen'` |
| Assert mode | `expect_empty` |
| Schedule | `*/15 * * * *` (every 15 minutes) |
| Severity | `error` |

Any rows returned = a problem. Zero rows = all good. Configure an alert config with `platform_is: datacheck` to get Slack/email notifications.

### Liveness / activity check (expect_rows)

> "There should be reservations in the last 15 minutes during business hours"

| Field | Value |
|-------|-------|
| Query | `SELECT COUNT(*) as cnt FROM reservations WHERE created_at >= now() - interval '15 minutes' HAVING COUNT(*) > 0` |
| Assert mode | `expect_rows` |
| Schedule | `*/5 9-23 * * *` (every 5 minutes, 9am-11pm) |
| Severity | `warning` |

Zero rows returned = something is wrong (no reservations flowing). Rows returned = healthy. Issue auto-resolves when reservations resume.

### Threshold check (row_count)

> "Alert if more than 50 failed payment retries are pending"

| Field | Value |
|-------|-------|
| Query | `SELECT id FROM payment_retries WHERE status = 'failed' AND retry_count >= 3` |
| Assert mode | `row_count` |
| Operator | `gt` |
| Value | `50` |
| Schedule | `0 */2 * * *` (every 2 hours) |
| Severity | `fatal` |

Tolerate up to 50 stuck retries. Above that, it's a systemic problem.

## Data Model

### New Tables

```sql
-- External database connections (per project)
CREATE TABLE datacheck_connections (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id      UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name            TEXT NOT NULL,
    engine          TEXT NOT NULL CHECK (engine IN ('postgresql', 'mysql')),
    host            TEXT NOT NULL,
    port            INTEGER NOT NULL,
    database_name   TEXT NOT NULL,
    username        TEXT NOT NULL,
    password        TEXT NOT NULL,          -- encrypted at rest
    schema          TEXT NOT NULL DEFAULT 'public',     -- search_path for PostgreSQL, database schema for MySQL
    ssl_mode        TEXT NOT NULL DEFAULT 'require',
    max_open_conns  INTEGER NOT NULL DEFAULT 3,
    query_timeout   INTEGER NOT NULL DEFAULT 30,  -- seconds
    enabled         BOOLEAN NOT NULL DEFAULT true,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Check definitions
CREATE TABLE datacheck_checks (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id      UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    connection_id   UUID NOT NULL REFERENCES datacheck_connections(id) ON DELETE CASCADE,
    name            TEXT NOT NULL,
    description     TEXT NOT NULL DEFAULT '',
    sql_query       TEXT NOT NULL,
    cron_schedule   TEXT NOT NULL DEFAULT '0 * * * *',  -- cron expression (default: every hour)
    severity        TEXT NOT NULL DEFAULT 'warning' CHECK (severity IN ('info', 'warning', 'error', 'fatal')),
    -- Assertion mode: how to evaluate query results
    assert_mode     TEXT NOT NULL DEFAULT 'expect_empty' CHECK (assert_mode IN ('expect_empty', 'expect_rows', 'row_count')),
    assert_operator TEXT CHECK (assert_operator IN ('gt', 'gte', 'lt', 'lte', 'eq', 'neq')),  -- only for row_count mode
    assert_value    INTEGER,                            -- only for row_count mode
    enabled         BOOLEAN NOT NULL DEFAULT true,
    last_run_at     TIMESTAMPTZ,
    last_status     TEXT CHECK (last_status IN ('pass', 'fail', 'error', 'running')),
    last_row_count  INTEGER,
    next_run_at     TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Execution history
CREATE TABLE datacheck_runs (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    check_id        UUID NOT NULL REFERENCES datacheck_checks(id) ON DELETE CASCADE,
    status          TEXT NOT NULL CHECK (status IN ('pass', 'fail', 'error')),
    row_count       INTEGER NOT NULL DEFAULT 0,
    sample_rows     JSONB,                          -- first 10 rows as JSON
    error_message   TEXT,
    issue_id        UUID REFERENCES issues(id) ON DELETE SET NULL,
    duration_ms     INTEGER NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Indexes
CREATE INDEX idx_datacheck_connections_project ON datacheck_connections(project_id);
CREATE UNIQUE INDEX idx_datacheck_connections_name ON datacheck_connections(project_id, name);
CREATE INDEX idx_datacheck_checks_project ON datacheck_checks(project_id);
CREATE UNIQUE INDEX idx_datacheck_checks_name ON datacheck_checks(project_id, name);
CREATE INDEX idx_datacheck_checks_next_run ON datacheck_checks(next_run_at) WHERE enabled = true;
-- Note: next_run_at is computed from cron_schedule using a Go cron parser (e.g., github.com/robfig/cron/v3)
CREATE INDEX idx_datacheck_runs_check ON datacheck_runs(check_id, created_at DESC);
```

### Fingerprint Strategy

Each check generates issues with a deterministic fingerprint: `datacheck:{check_id}`. This ensures:
- Same check always maps to the same issue (upsert, not duplicate)
- Event count increments on repeated failures
- Issue auto-resolves when check passes (if previously failing)

## API Endpoints

### Connections

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/v1/projects/{id}/datacheck/connections` | List connections |
| `POST` | `/api/v1/projects/{id}/datacheck/connections` | Create connection |
| `PUT` | `/api/v1/projects/{id}/datacheck/connections/{conn_id}` | Update connection |
| `DELETE` | `/api/v1/projects/{id}/datacheck/connections/{conn_id}` | Delete connection (response includes count of checks that will be cascade-deleted) |
| `POST` | `/api/v1/projects/{id}/datacheck/connections/{conn_id}/test` | Test connection |

### Checks

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/v1/projects/{id}/datacheck/checks` | List checks |
| `POST` | `/api/v1/projects/{id}/datacheck/checks` | Create check |
| `GET` | `/api/v1/projects/{id}/datacheck/checks/{check_id}` | Get check detail |
| `PUT` | `/api/v1/projects/{id}/datacheck/checks/{check_id}` | Update check |
| `DELETE` | `/api/v1/projects/{id}/datacheck/checks/{check_id}` | Delete check |
| `POST` | `/api/v1/projects/{id}/datacheck/checks/{check_id}/run` | Run check manually |
| `POST` | `/api/v1/projects/{id}/datacheck/checks/{check_id}/preview` | Preview query (dry run, returns results without creating issue) |

### Runs

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/v1/projects/{id}/datacheck/checks/{check_id}/runs` | List run history |

### Dashboard

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/v1/projects/{id}/datacheck/summary` | Summary of all checks (status, last run, next run) |

## Architecture

### Backend Package: `internal/datacheck/`

```
internal/datacheck/
├── handler.go       # REST API handlers (connections, checks, runs)
├── checker.go       # Background worker — polls due checks and executes them
├── executor.go      # Connects to external DB, runs query, collects results
├── asserter.go      # Assertion evaluation (expect_empty, expect_rows, row_count)
├── connector.go     # Connection pool manager for external databases
├── cron.go          # Cron expression parsing and next-run computation (wraps robfig/cron)
└── encryption.go    # Credential encryption/decryption (reuse project token pattern)
```

**New dependency**: `github.com/robfig/cron/v3` — standard Go cron parser for schedule computation (parse expression, compute next run time). Not used as a scheduler — GoSnag's existing poll-based worker pattern handles execution.

### Background Worker

The DataCheck checker runs as a single background goroutine (same pattern as existing workers):

```go
go datacheckChecker.Run(ctx, 30*time.Second)
```

Each tick:
1. Query `datacheck_checks` where `enabled = true AND next_run_at <= now() AND (last_status IS NULL OR last_status != 'running')` joined with `datacheck_connections` where `enabled = true`
2. For each due check:
   a. Set `last_status = 'running'`
   b. Open connection to external DB (pooled)
   b. Execute query with timeout
   c. Count rows, capture first 10 as sample
   d. **Evaluate assertion**:
      - `expect_empty`: FAIL if row_count > 0, PASS if row_count = 0
      - `expect_rows`: FAIL if row_count = 0, PASS if row_count > 0
      - `row_count`: FAIL if `row_count {operator} value` is true (e.g., `row_count > 100`)
   e. Record run in `datacheck_runs`
   f. **On FAIL** → upsert issue via `UpsertIssue` (platform=`datacheck`, fingerprint=`datacheck:{check_id}`), then call `alertService.Notify(projectID, issue, isNew)` — existing alert configs handle all notification routing
   g. **On PASS** — if issue exists and is open → auto-resolve
   h. Compute `next_run_at` from `cron_schedule` using cron parser, update `last_run_at`, `last_status`
3. Checks are processed sequentially within a tick (to bound concurrency on external DBs)

### Connection Pool Management

- Each `datacheck_connection` gets a lazily-initialized `*sql.DB` pool (max 3 conns)
- Pools are cached in memory, keyed by connection ID
- Pools are invalidated when connection config changes
- Stale pools (unused for 30 min) are closed
- All queries run in a read-only transaction: `SET TRANSACTION READ ONLY`

### Security

1. **Credentials encrypted at rest** — password field encrypted using the same pattern as Jira/GitHub tokens in projects
2. **Read-only enforcement** — every query wrapped in `BEGIN; SET TRANSACTION READ ONLY; <query>; ROLLBACK;`
3. **Query timeout** — configurable per connection (default 30s), enforced via `context.WithTimeout`
4. **No DDL/DML** — connection user should have SELECT-only grants (documented, not enforced at app level beyond read-only transaction)
5. **Result size cap** — max 10 sample rows stored, max 10,000 rows counted before short-circuit
6. **Connection count bounded** — max 3 connections per external DB, preventing resource exhaustion

### Issue Integration

DataCheck integrates with the existing alert pipeline — no parallel notification system:

1. **Issue creation**: `UpsertIssue` with fingerprint `datacheck:{check_id}`, platform `datacheck`, level from check severity
2. **Alert dispatch**: `alertService.Notify(projectID, issue, isNew)` — same call as ingest
3. **Routing**: Users configure alert configs with conditions like `platform_is: datacheck` to target DataCheck issues specifically, or broad rules catch them alongside regular errors
4. **Auto-resolve**: When a check passes after failing, the issue is resolved. If users have follower notifications configured, they get notified of the resolution too

**Issue fields:**
- Fingerprint: `datacheck:{check_id}`
- Platform: `datacheck`
- Level: from check severity (info/warning/error/fatal)
- Title: `[DataCheck] {check_name}`
- Event data (JSONB): query, row count, sample rows, connection name, duration, assertion mode

**Alert config examples:**
- `type: slack, conditions: {platform_is: datacheck, level: error}` → Slack for critical DataCheck failures
- `type: email, conditions: {platform_is: datacheck, title_contains: Adyen}` → email ops for Adyen checks
- `type: slack, conditions: {}` → catches everything (errors + DataCheck)

### Frontend

New section in Project Settings or dedicated tab in project view:

```
Project > DataCheck
├── Connections tab          # Manage DB connections
│   ├── Add connection form (engine, host, port, database, user, password, SSL)
│   └── Test connection button
├── Checks tab               # Manage checks
│   ├── Check list (name, assertion badge, status badge, last run, next run, row count)
│   ├── Add/edit check form:
│   │   ├── Name, description
│   │   ├── Connection selector
│   │   ├── SQL query editor (syntax-highlighted, with preview button)
│   │   ├── Assertion mode selector:
│   │   │   ├── "Expect empty" — alert when query returns rows
│   │   │   ├── "Expect rows" — alert when query returns no rows
│   │   │   └── "Row count" — alert when count {operator} {value}
│   │   ├── Cron schedule:
│   │   │   ├── Preset buttons (every 1m, 5m, 15m, 1h, daily 9am)
│   │   │   ├── Raw cron expression input
│   │   │   └── Human-readable description (auto-generated, e.g. "Every 5 minutes")
│   │   └── Severity selector (info, warning, error, fatal)
│   ├── Run now button
│   └── Preview button (shows query results without creating issue)
└── History tab              # Per-check run log
    ├── Run list (timestamp, status, row count, duration, linked issue)
    └── Row count trend mini-chart
```

## Supported Database Engines

### v1 (MVP)

- **PostgreSQL** — using `lib/pq` or `pgx` (already in project dependencies)
- **MySQL** — using `go-sql-driver/mysql`

### Future

- SQL Server
- SQLite
- MongoDB (requires different query model)

## Configuration

New environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `DATACHECK_ENABLED` | `true` | Enable/disable the DataCheck worker globally |
| `DATACHECK_POLL_INTERVAL` | `30` | Seconds between worker polls for due checks |
| `DATACHECK_ENCRYPTION_KEY` | (required if DataCheck used) | 32-byte hex key for encrypting connection credentials |
| `DATACHECK_RUN_RETENTION_DAYS` | `90` | Auto-delete run history older than N days (0 = keep forever) |

## Acceptance Criteria

### Must Have (MVP)

- [ ] CRUD for database connections (PostgreSQL only for MVP)
- [ ] Connection test endpoint
- [ ] Credential encryption at rest
- [ ] CRUD for checks with SQL query, cron schedule, severity, assertion mode
- [ ] Three assertion modes: expect_empty, expect_rows, row_count (with operator and value)
- [ ] Cron-based scheduling with `next_run_at` computed from cron expression
- [ ] Common cron presets in the UI (every 1min, 5min, 15min, hourly, daily)
- [ ] Human-readable cron description displayed next to the expression
- [ ] Background worker executes due checks on schedule
- [ ] Read-only transaction enforcement
- [ ] Query timeout enforcement
- [ ] Issue creation/update on check failure (fingerprint-based dedup, platform=`datacheck`)
- [ ] Alert dispatch via existing `alertService.Notify()` — no parallel notification system
- [ ] Issue auto-resolve on check recovery
- [ ] Run history with status, row count, sample rows, duration
- [ ] Frontend: connection management UI
- [ ] Frontend: check management UI with query editor and cron builder
- [ ] Frontend: run now / preview functionality
- [ ] Frontend: run history view

### Should Have

- [ ] MySQL support
- [ ] Check summary dashboard with status overview
- [ ] Row count trend chart per check
- [ ] DataCheck issues visually tagged in issue list (platform badge or tag)
- [ ] MCP server tools for DataCheck (list checks, run check, get results)
- [ ] Bulk enable/disable checks

### Nice to Have

- [ ] AI-suggested check queries based on schema introspection
- [ ] AI analysis of check results
- [ ] Export check definitions (JSON) for sharing/backup
- [ ] Check templates library (common integrity checks)
- [ ] Notification when a check is failing due to connection error (distinct from data issues)

## Risks and Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| External DB credentials compromised | High | Encrypt at rest, read-only enforcement, audit log |
| Runaway query exhausts external DB | High | Query timeout, max connections (3), row count cap (10k) |
| Too many checks overwhelm GoSnag worker | Medium | Sequential execution, configurable poll interval, per-check frequency limits |
| Connection pooling leaks | Medium | Pool cleanup on config change, stale pool eviction |
| Users write destructive SQL | High | Read-only transaction wrapping, documentation recommending SELECT-only DB users |
| Check generates too many events | Medium | Fingerprint dedup (one issue per check), auto-resolve on recovery |

## Out of Scope

- Monitoring non-SQL databases (MongoDB, Redis, etc.)
- Arbitrary script execution (only SQL queries)
- Cross-database joins or federated queries
- Automated remediation (running fix queries)
- Real-time streaming checks (only scheduled polling)
