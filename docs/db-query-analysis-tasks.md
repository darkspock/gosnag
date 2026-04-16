# DB Query Analysis — Implementation Tasks

Reference: [db-query-analysis-epic.md](db-query-analysis-epic.md)  
Requirements: [db-query-analysis-requirements.md](db-query-analysis-requirements.md)

---

## Block 1: Project Configuration

### 1.1 Migration: analysis DB settings
- [ ] Add project-level fields for DB analysis configuration
- [ ] Create migration to add:
  - `analysis_db_enabled BOOLEAN NOT NULL DEFAULT false`
  - `analysis_db_driver TEXT NOT NULL DEFAULT ''`
  - `analysis_db_dsn TEXT NOT NULL DEFAULT ''`
  - optional metadata fields if we want them in the first pass:
    - `analysis_db_name`
    - `analysis_db_schema`
    - `analysis_db_notes`
- [ ] Create matching `down.sql`

### 1.2 Project model and API
- [ ] Extend backend project model / queries to read and update the new fields
- [ ] Ensure `analysis_db_dsn` is treated as secret and never returned in clear text
- [ ] Expose only safe metadata to the frontend:
  - `analysis_db_enabled`
  - `analysis_db_driver`
  - `analysis_db_configured`
  - optional non-sensitive metadata

### 1.3 Test connection endpoint
- [ ] Add `POST /api/v1/projects/{project_id}/db-analysis/test`
- [ ] Accept current saved config or current draft payload
- [ ] Attempt a lightweight connection check
- [ ] Return:
  - `ok`
  - sanitized error if failed
  - optional server/database metadata if safe
- [ ] Restrict to project admins

### 1.4 Frontend: Project Settings
- [ ] Add `Database Analysis` section in `ProjectSettings.tsx`
- [ ] Fields:
  - enable toggle
  - driver select
  - DSN/connection string input
  - optional metadata inputs
- [ ] Add `Test connection` button
- [ ] Show configured/not configured state without echoing the secret back

---

## Block 2: SQL Breadcrumb UX

### 2.1 Parse SQL breadcrumbs in the issue detail page
- [ ] Read current breadcrumb rendering in `web/src/pages/IssueDetail.tsx`
- [ ] Add specialized handling for `category === 'db.query'`
- [ ] Extract:
  - SQL text
  - `duration_ms`
  - breadcrumb time
  - level
  - any existing summary fields

### 2.2 Explicit timing display
- [ ] Add a visible `Duration` column or badge for SQL breadcrumbs
- [ ] If `duration_ms` exists, display it explicitly
- [ ] If `duration_ms` is missing, show `No timing data`
- [ ] Do not imply missing timing is `0ms`

### 2.3 SQL formatting and readability
- [ ] Improve SQL readability in the breadcrumb list:
  - preserve line breaks when useful
  - monospace rendering
  - horizontal scroll for long statements
- [ ] Keep raw SQL accessible for copy/debugging

### 2.4 SQL summary panel
- [ ] Add an issue detail subsection for SQL-only summary
- [ ] Show:
  - total SQL breadcrumbs
  - how many have timing
  - total time if available
  - number of normalized groups

---

## Block 3: Query Normalization and Grouping

### 3.1 Backend query extraction for on-demand analysis
- [ ] Create a package or module for issue-level SQL extraction
- [ ] Read latest relevant event data for the issue
- [ ] Extract `db.query` breadcrumbs from raw event payload
- [ ] Preserve both:
  - raw SQL
  - breadcrumb metadata

### 3.2 Normalization rules
- [ ] Implement SQL normalization heuristics for grouping:
  - collapse whitespace
  - normalize casing where helpful
  - replace obvious literal values with placeholders
  - normalize `IN (...)` and common scalar comparisons conservatively
- [ ] Keep the original SQL alongside the normalized form
- [ ] Avoid driver-specific overreach in the first iteration

### 3.3 Grouped query summary
- [ ] Group queries by normalized form
- [ ] Compute per-group:
  - repetition count
  - total duration if timing exists
  - average duration if timing exists
  - representative raw query
  - optional inferred table/entity

### 3.4 API for grouped summary
- [ ] Add endpoint or include in analysis response:
  - raw query count
  - grouped patterns
  - timing availability summary

---

## Block 4: On-Demand N+1 Analysis

### 4.1 Analyzer service
- [ ] Create issue-scoped SQL analysis service
- [ ] Input:
  - project ID
  - issue ID
- [ ] Output:
  - grouped queries
  - timing availability
  - heuristic N+1 findings
  - warnings

### 4.2 N+1 heuristics
- [ ] Reuse existing `internal/n1` concepts where reasonable
- [ ] Detect repeated query shapes that differ mainly by literals
- [ ] Flag likely N+1 when:
  - repeated `SELECT` forms dominate the request
  - repeated key lookup patterns appear
  - repetition count crosses a configurable or fixed threshold
- [ ] Mark findings as heuristic

### 4.3 Missing timing diagnostics
- [ ] Explicitly classify timing availability:
  - all queries timed
  - partial timing
  - no timing
- [ ] Add recommendation text when timing is absent

### 4.4 API endpoint
- [ ] Add `POST /api/v1/projects/{project_id}/issues/{issue_id}/db-analysis`
- [ ] Return:
  - grouped queries
  - N+1 findings
  - timing summary
  - warnings
  - optional `EXPLAIN` results
- [ ] Restrict execution to authorized users

### 4.5 Frontend: Analyze action
- [ ] Show `Analyze` only when the issue has `db.query` breadcrumbs
- [ ] If analysis DB is not configured, show explanatory empty state and link to project settings
- [ ] Render result panel with:
  - summary
  - repeated groups
  - likely N+1 findings
  - missing timing warning

---

## Block 5: Safe EXPLAIN

### 5.1 Driver abstraction
- [ ] Add backend support for driver-specific explain behavior
- [ ] First iteration:
  - MySQL / MariaDB
  - PostgreSQL
- [ ] Choose explain syntax based on `analysis_db_driver`

### 5.2 Safe SQL gate
- [ ] Reject:
  - multi-statement SQL
  - mutating SQL
  - DDL
  - anything that fails safe parsing
- [ ] First iteration should allow `SELECT` only

### 5.3 Query selection
- [ ] Allow running `EXPLAIN` on a representative query group, not every duplicate
- [ ] Return sanitized query text plus plan result

### 5.4 Frontend: EXPLAIN UX
- [ ] Add `Run EXPLAIN` action in the analysis panel
- [ ] Gate it behind configured analysis DB + permissions
- [ ] Show structured result when possible
- [ ] Highlight obvious red flags:
  - full scan
  - high estimated rows
  - missing index usage

---

## Block 6: Security and Safety

### 6.1 Secrets handling
- [ ] Ensure `analysis_db_dsn` is never returned in clear text
- [ ] Review logs to avoid leaking connection strings or query literals unnecessarily
- [ ] Sanitize backend errors returned to the UI

### 6.2 Permissions
- [ ] Restrict configuration and execution to project admins in first iteration
- [ ] Make read-only visibility of results a separate decision if needed later

### 6.3 Operational safety
- [ ] Enforce timeouts on DB analysis connections
- [ ] Limit result size returned from `EXPLAIN`
- [ ] Fail closed on parser ambiguity

---

## Block 7: Optional Persistence

### 7.1 Persisted analysis records
- [ ] Decide whether to persist analysis results in phase 2 or 3
- [ ] If yes, add table for:
  - issue ID
  - project ID
  - created at
  - timing summary
  - grouped findings
  - N+1 findings
  - explain payload
  - warnings

### 7.2 Cache and replay
- [ ] Reuse latest successful analysis when inputs have not changed
- [ ] Add invalidation strategy when new events arrive for the issue

---

## Block 8: Validation

### 8.1 Backend tests
- [ ] Unit tests for SQL extraction from breadcrumbs
- [ ] Unit tests for normalization and grouping
- [ ] Unit tests for N+1 heuristics
- [ ] Unit tests for safe SQL gating
- [ ] Tests for driver-specific `EXPLAIN`
- [ ] Tests for connection test endpoint

### 8.2 Frontend tests
- [ ] Component coverage for SQL breadcrumb rendering
- [ ] State coverage for:
  - timing present
  - timing missing
  - no SQL breadcrumbs
  - DB analysis not configured
  - analysis failure

### 8.3 Manual validation
- [ ] Validate with an issue containing many `db.query` breadcrumbs
- [ ] Validate with mixed timed / untimed SQL breadcrumbs
- [ ] Validate with a known N+1 request
- [ ] Validate with unsupported SQL to confirm rejection path

---

## Suggested Delivery Order

### Phase 1
- [ ] Block 1
- [ ] Block 2
- [ ] Block 3
- [ ] Block 4 without `EXPLAIN`

### Phase 2
- [ ] Block 5
- [ ] Block 6 hardening

### Phase 3
- [ ] Block 7
- [ ] UX polish and richer heuristics
