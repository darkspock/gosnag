# DB Query Analysis — Requirements Document

---

## 1. Overview

Add project-level database query analysis capabilities to GoSnag for events that contain `db.query` breadcrumbs.

The feature has two main goals:

- improve visibility of SQL activity already present in issue breadcrumbs
- allow on-demand analysis of repeated queries, likely N+1 patterns, and SQL execution plans

This feature is intended for request-scoped database issues such as:

- excessive query count
- repeated similar queries across a single request
- slow SQL
- missing indexes or poor plans on selected statements

---

## 2. Goals

- Show SQL breadcrumb timing and query metadata clearly in the issue detail UI
- Allow projects to configure a dedicated database analysis connection
- Provide an on-demand `Analyze` action for `db.query` activity
- Detect likely N+1 patterns from the queries visible in an issue
- Run `EXPLAIN` for supported queries using the configured project database connection
- Reuse existing N+1 concepts where possible, without depending only on batch detection

---

## 3. Non-Goals

- Automatic execution of `EXPLAIN ANALYZE` for every query
- Using the ingest DSN as a database analysis connection
- Full SQL linting or query optimization recommendations in the first iteration
- Automatic query rewriting
- Automatic execution of mutating SQL
- Replacing the existing batch N+1 detector

---

## 4. Problem Statement

### 4.1 Current Limitation

Issues may already contain `db.query` breadcrumbs with useful metadata such as:

- SQL text
- category
- level
- timing data like `duration_ms`

But the current UI renders breadcrumbs mostly as raw message/data text and does not expose SQL-oriented analysis.

### 4.2 Desired Outcome

When an issue contains SQL breadcrumbs, a user should be able to:

- see per-query timing clearly
- inspect grouped/repeated queries
- identify likely N+1 patterns
- run an on-demand `EXPLAIN` against a safe, configured analysis connection

---

## 5. Project Configuration

### 5.1 Analysis Connection

**REQ-DBA-001**: Each project MUST support a dedicated database analysis configuration, separate from the SDK ingest DSN.

**REQ-DBA-002**: The project-level configuration MUST support at least:

- `analysis_db_enabled` (boolean)
- `analysis_db_driver` (string)
- `analysis_db_dsn` (secret string)

**REQ-DBA-003**: The configuration SHOULD also support:

- `analysis_db_name`
- `analysis_db_schema`
- `analysis_db_readonly_expected`
- `analysis_db_notes`

**REQ-DBA-004**: The analysis connection MUST be treated as sensitive configuration and MUST NOT be returned to the frontend in clear text.

**REQ-DBA-005**: The project settings UI MUST provide a `Test connection` action.

### 5.2 Supported Drivers

**REQ-DBA-010**: The first iteration SHOULD support at least:

- MySQL / MariaDB
- PostgreSQL

**REQ-DBA-011**: The driver configuration MUST determine which `EXPLAIN` syntax is used.

---

## 6. Issue Detail UI

### 6.1 SQL Breadcrumb Visibility

**REQ-DBA-020**: In the issue detail page, `db.query` breadcrumbs MUST display SQL-specific metadata instead of only raw message text.

**REQ-DBA-021**: For each SQL breadcrumb, the UI SHOULD display:

- timestamp
- category
- SQL text
- level
- `duration_ms` when available

**REQ-DBA-022**: If `duration_ms` exists in breadcrumb data, it MUST be shown explicitly in the UI.

**REQ-DBA-023**: If `duration_ms` is missing, the UI SHOULD indicate that timing is unavailable rather than implying it is zero.

### 6.2 Analysis Action

**REQ-DBA-030**: The issue detail page MUST provide an `Analyze` action when the issue contains `db.query` breadcrumbs.

**REQ-DBA-031**: The `Analyze` action SHOULD be visible only when:

- the project has `analysis_db_enabled = true`
- the user has sufficient permissions

**REQ-DBA-032**: If the issue contains SQL breadcrumbs but no analysis connection is configured, the UI SHOULD show an explanatory empty state instead of a broken action.

### 6.3 Analysis Output

**REQ-DBA-040**: The UI SHOULD present analysis results in a SQL-focused panel with:

- summary
- repeated query groups
- likely N+1 signals
- `EXPLAIN` results when available
- warnings about missing timing data

---

## 7. Query Parsing and Extraction

### 7.1 Breadcrumb Source

**REQ-DBA-050**: The system MUST analyze `db.query` breadcrumbs from the issue’s latest relevant event.

**REQ-DBA-051**: The system SHOULD preserve both:

- raw SQL text
- structured metadata from breadcrumb `data`

### 7.2 Timing Data

**REQ-DBA-060**: The analysis system MUST check whether each SQL breadcrumb includes timing metadata such as `duration_ms`.

**REQ-DBA-061**: The analysis result MUST explicitly report whether per-query timing data is available for:

- all queries
- some queries
- no queries

**REQ-DBA-062**: Missing timing data MUST be considered a first-class diagnostic outcome.

---

## 8. N+1 Detection

### 8.1 On-Demand N+1 Analysis

**REQ-DBA-070**: The `Analyze` action MUST include issue-level N+1 detection based on the queries visible in the issue.

**REQ-DBA-071**: The system SHOULD detect repeated queries that differ only by literal values or parameter values.

**REQ-DBA-072**: The analysis SHOULD group queries by normalized shape.

**REQ-DBA-073**: The system SHOULD report:

- number of repetitions
- total estimated SQL time
- representative query shape
- likely table or entity involved

### 8.2 Heuristics

**REQ-DBA-080**: The first iteration MAY use heuristics such as:

- repeated `SELECT` statements with the same structure
- repeated `WHERE id = ?` or equivalent key lookup shape
- repeated queries within the same request span or timestamp cluster

**REQ-DBA-081**: N+1 analysis MUST be labeled as heuristic unless backed by stronger instrumentation.

### 8.3 Integration with Existing N+1 System

**REQ-DBA-090**: The design SHOULD reuse existing N+1 extraction/detection logic where feasible.

**REQ-DBA-091**: On-demand issue analysis and background N+1 detection MAY coexist.

**REQ-DBA-092**: The on-demand analyzer MUST NOT depend on the batch detector having already run.

---

## 9. EXPLAIN Plan Analysis

### 9.1 Supported Statements

**REQ-DBA-100**: `EXPLAIN` MUST only be offered for statements considered safe for analysis in the first iteration.

**REQ-DBA-101**: The first iteration SHOULD support `SELECT` statements only.

**REQ-DBA-102**: Mutating statements such as `INSERT`, `UPDATE`, `DELETE`, DDL, and multi-statement payloads MUST NOT be executed as part of analysis.

### 9.2 Execution Mode

**REQ-DBA-110**: The first iteration SHOULD use plain `EXPLAIN`, not `EXPLAIN ANALYZE`, by default.

**REQ-DBA-111**: If `EXPLAIN ANALYZE` is added later, it MUST require stronger safeguards and explicit confirmation.

### 9.3 Query Selection

**REQ-DBA-120**: The analyzer SHOULD allow selecting a representative query for `EXPLAIN`.

**REQ-DBA-121**: If many similar queries are present, the system SHOULD prefer the normalized representative query rather than running `EXPLAIN` for every duplicate.

### 9.4 Result Display

**REQ-DBA-130**: The UI SHOULD display `EXPLAIN` output in a readable structured format when possible.

**REQ-DBA-131**: The analysis SHOULD highlight obvious risk signals such as:

- full table scan
- missing index usage
- high estimated rows
- repeated nested lookups

---

## 10. Security and Safety

### 10.1 Connection Security

**REQ-DBA-140**: The analysis DB connection string MUST be stored securely and treated as secret project configuration.

**REQ-DBA-141**: The frontend MUST only receive a boolean “configured” signal and any non-sensitive metadata needed for UX.

### 10.2 Access Control

**REQ-DBA-150**: Only authorized users SHOULD be allowed to run query analysis and `EXPLAIN`.

**REQ-DBA-151**: The first iteration SHOULD restrict analysis execution to project admins.

### 10.3 Safe SQL Handling

**REQ-DBA-160**: The analyzer MUST reject:

- multi-statement SQL
- obviously mutating SQL
- statements that fail safe parsing

**REQ-DBA-161**: The analyzer SHOULD sanitize or redact sensitive literal values before persisting analysis artifacts or logs.

---

## 11. API Requirements

**REQ-DBA-170**: The backend MUST provide endpoints to:

- configure project analysis DB settings
- test the analysis DB connection
- trigger issue SQL analysis
- retrieve analysis results

**REQ-DBA-171**: The SQL analysis endpoint SHOULD be issue-scoped, for example:

- `POST /projects/{project_id}/issues/{issue_id}/db-analysis`

**REQ-DBA-172**: The API SHOULD return:

- availability of timing data
- grouped query patterns
- likely N+1 findings
- `EXPLAIN` results if executed
- warnings and errors

---

## 12. Data Model

### 12.1 Project Settings

**REQ-DBA-180**: Project persistence MUST support the database analysis configuration fields.

### 12.2 Analysis Results

**REQ-DBA-181**: The system MAY persist on-demand SQL analysis results for auditability and caching.

**REQ-DBA-182**: If persisted, a SQL analysis record SHOULD include:

- issue ID
- project ID
- analysis timestamp
- timing availability summary
- grouped query findings
- likely N+1 findings
- `EXPLAIN` payload
- warnings

---

## 13. UX Requirements

### 13.1 Missing Timing

**REQ-DBA-190**: If `duration_ms` is missing for all SQL breadcrumbs, the analysis MUST explicitly say so.

**REQ-DBA-191**: The missing timing case SHOULD recommend improving instrumentation rather than pretending analysis is complete.

### 13.2 Empty Analysis States

**REQ-DBA-192**: If there are no `db.query` breadcrumbs, the UI MUST not show SQL analysis actions.

**REQ-DBA-193**: If there are SQL breadcrumbs but the project analysis DB is not configured, the UI SHOULD point the user to Project Settings.

---

## 14. Rollout Plan

### 14.1 Phase 1

- show `duration_ms` in SQL breadcrumbs when present
- add project-level DB analysis connection settings
- add `Test connection`
- add issue-level `Analyze` action
- implement heuristic N+1 grouping from issue breadcrumbs

### 14.2 Phase 2

- add safe `EXPLAIN` for supported `SELECT` statements
- add structured result UI
- add optional result persistence

### 14.3 Phase 3

- richer normalization of duplicate queries
- optional AI-assisted explanation of SQL findings
- support for more drivers and more advanced execution-plan heuristics

---

## 15. Key Product Decisions

- The analysis connection is project-specific and separate from ingest
- `duration_ms` visibility is required before deeper SQL analysis is trustworthy
- N+1 detection should work both in batch and on-demand
- `EXPLAIN` must be safe, explicit, and restricted
- SQL analysis should augment issue triage, not turn GoSnag into a general DB console
