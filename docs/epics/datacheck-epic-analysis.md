# DataCheck Epic — Requirement Validation Analysis

**Document**: `docs/epics/datacheck-epic.md`
**Type**: Epic (full validation)
**Date**: 2026-04-15

---

## 1. Entities Identified

| Entity | Type | CRUD | Status |
|--------|------|------|--------|
| **DataCheck Connection** | Primary | Complete | Partial (enabled/disabled only) |
| **DataCheck Check** | Primary | Complete | Partial (enabled + last_status) |
| **DataCheck Run** | Primary | Create + Read only | Final states (pass/fail/error) |
| **Issue** (existing) | Affected | N/A — existing | Existing lifecycle applies |
| **Notification** (existing) | Affected | N/A — existing | Reused from alerts |

---

## 2. Missing Use Cases

### Must Define (blocks implementation)

| # | Gap | Impact | Question |
|---|-----|--------|----------|
| 1 | ~~**Notify-only has no recipient configuration**~~ | **RESOLVED** — Simplified: removed `on_fail_issue`/`on_fail_notify`. DataCheck always creates an issue and calls `alertService.Notify()`. Notification routing is fully handled by existing alert configs with `platform_is: datacheck` conditions. No parallel notification system. | N/A |
| 2 | **Run history retention policy missing** | `datacheck_runs` will grow unboundedly. A check running every minute produces 525,600 rows/year. With 20 checks, that's 10M+ rows/year. | Should there be a retention policy (e.g., `DATACHECK_RUN_RETENTION_DAYS`)? Or max runs per check? |
| 3 | **Concurrent execution protection undefined** | If a check is scheduled every 1 minute but takes 3 minutes to execute, the worker will find it "due" again while the previous execution is still running. | Should the worker skip checks currently in progress? Add a `running` state to `last_status`? Use a per-check lock? |

### Should Define (causes confusion or bugs if omitted)

| # | Gap | Impact | Question |
|---|-----|--------|----------|
| 4 | **Connection deletion cascade is silent** | `ON DELETE CASCADE` on `datacheck_checks` means deleting a connection destroys all its checks and their run history without warning. | Should the API refuse to delete a connection with active checks? Or show a confirmation with the count of checks that will be deleted? |
| 5 | **Permission model unclear** | User stories say "admin" for connections and "user" for checks. GoSnag has admin/viewer roles. Can viewers see checks? Can non-admin members create checks? | Define which roles can: view connections (hiding credentials), manage connections, view checks, manage checks, run checks manually, view run history. |
| 6 | **Cron expression validation** | No mention of validating the cron expression at creation/update time. An invalid expression would cause the worker to fail or skip the check silently. | Validate cron at API level when creating/updating a check. Return 400 with a clear error message for invalid expressions. |
| 7 | **Check behavior when connection is disabled** | The check has `enabled` and the connection has `enabled`, but the worker query only checks `datacheck_checks.enabled`. | Should the worker also skip checks whose connection is disabled? Or should disabling a connection automatically disable its checks? |
| 8 | **Issue lifecycle on check deletion** | When a check is deleted, its fingerprint `datacheck:{check_id}` becomes orphaned. The associated issue remains but can never be auto-resolved or have new events. | Should check deletion auto-resolve the linked issue? Add a comment? Leave it as-is? |
| 9 | **Schema/search_path for PostgreSQL connections** | GoSnag's own database uses `search_path=gosnag`. Many PostgreSQL databases use non-default schemas. The connection model has no schema field. | Add an optional `schema` field to `datacheck_connections` (sets `search_path` on connection). |

### Could Define (nice to have in epic, can defer to implementation)

| # | Gap | Impact | Question |
|---|-----|--------|----------|
| 10 | **Check name uniqueness** | No UNIQUE constraint on `(project_id, name)` for checks or connections. Users could create "Adyen check" twice. | Add a unique constraint on `(project_id, name)` for both tables? |
| 11 | **Clone/duplicate check** | No way to create a similar check from an existing one. Common workflow when monitoring the same pattern across different connections. | Add a "duplicate check" action to the API and UI? |
| 12 | **Bulk import/export of checks** | No way to seed a project with multiple checks at once. Setting up 20 checks one by one is tedious. | Defer to "Nice to Have" — already there as "Export check definitions (JSON)". Add import too. |

---

## 3. Missing State Information

### DataCheck Connection

| Check | Status |
|-------|--------|
| Initial status | `enabled = true` — defined |
| All statuses | Only enabled/disabled. **Missing: connection health.** No way to know if a connection is currently reachable without running the test endpoint. |
| Delete strategy | Hard delete with CASCADE — defined but **risky** (see gap #4) |

**Recommendation**: Consider adding a `last_tested_at` and `last_test_status` (ok/failed) to the connection table, updated whenever any check uses the connection. This gives visibility into connection health on the connections list without requiring manual tests.

### DataCheck Check

| Check | Status |
|-------|--------|
| Initial status | `enabled = true`, `last_status = NULL` — defined |
| All statuses | enabled (bool) + last_status (pass/fail/error/NULL) — **missing: `running` state** |
| Transitions | Not documented. Implied: NULL → pass/fail/error on first run |
| Side effects | Issue creation/notification on fail, auto-resolve on pass — defined |
| Time-based transitions | Cron-based execution — defined |

**Recommendation**: Add a `running` state to `last_status` to prevent concurrent execution (gap #3). Worker sets `last_status = 'running'` before execution, then updates to pass/fail/error after.

### DataCheck Run

Immutable audit records — no state machine needed. Statuses (pass/fail/error) are final.

---

## 4. Collateral Impact on Existing GoSnag Features

### Impact Table

| Existing Feature | Impact Type | Description | Severity |
|------------------|-------------|-------------|----------|
| **Issue list** | Behavioral Change | DataCheck issues appear alongside error-ingested issues. Users may be confused seeing `[DataCheck] Adyen config` next to `TypeError: undefined`. | Medium |
| **Project stats** | Behavioral Change | "Open issues", "errors this week", 7-day trend chart will include DataCheck issues. A project with 0 real errors but 3 failing checks would show "3 open issues". | Medium |
| **Alert evaluation** | Behavioral Change | Existing broad alert rules (e.g., "all errors") will fire for DataCheck issues too. Users may get unexpected alerts. | Medium |
| **Priority scoring** | Behavioral Change | Velocity-based rules behave differently for scheduled checks vs. organic error spikes. A check running every minute has constant "velocity" by design. | Low |
| **Issue merge** | Risk | DataCheck issues could be merged with regular issues (or with each other). Merging two DataCheck issues would break fingerprint routing. | Medium |
| **Jira integration** | Behavioral Change | DataCheck issues could auto-create Jira tickets if Jira rules match `platform_is: datacheck` or broad rules. | Low |
| **MCP server** | No Impact | `list_issues` will include DataCheck issues — this is expected behavior. | None |
| **Event retention** | Data Impact | DataCheck events (stored as regular events via UpsertIssue) are subject to `EVENT_RETENTION_DAYS`. But `datacheck_runs` table has its own history with no cleanup. Two retention paths for the same data. | Medium |
| **N+1 detector** | No Impact | DataCheck events won't have stack traces. N+1 detector won't process them. | None |

### Recommendations

1. **Issue list**: Add `platform` filter to issue list so users can show/hide DataCheck issues. Consider a filter preset: "DataCheck only" / "Errors only" / "All".
2. **Project stats**: Decide whether DataCheck issues count in project dashboard stats or are tracked separately.
3. **Alert evaluation**: Document that existing alerts will fire for DataCheck issues. Consider adding `platform_is` / `platform_is_not` to the alert condition builder.
4. **Issue merge**: Prevent merging DataCheck issues (they have special fingerprint semantics). Or at minimum warn.

---

## 5. Slicing Assessment

### Size Assessment

| Indicator | Value | Assessment |
|-----------|-------|------------|
| New entities | 3 | OK |
| Affected existing entities | 2 | OK |
| Use cases | ~20+ | Large |
| State complexity | Low-Medium | OK |
| New integrations | External DB connections | Significant |
| MVP acceptance criteria | 20 items | Large for a single slice |

### Current Slicing

The epic uses MoSCoW (Must/Should/Nice) which is good, but the MVP is still large (20 items). Consider splitting MVP further:

**Suggested delivery phases:**

| Phase | Scope | Delivers |
|-------|-------|----------|
| **1: Core engine** | Connections CRUD + test, Checks CRUD with expect_empty only, Background worker, Issue creation, Basic frontend | A working DataCheck that detects data corruption |
| **2: Full assertions + scheduling** | expect_rows, row_count modes, Cron presets UI, Human-readable cron | Full flexibility in check definition |
| **3: Notifications + history** | on_fail_notify, Run history UI, Trend charts | Observability and lightweight alerting |
| **4: Polish** | Auto-resolve, Summary dashboard, Platform filter in issue list | Production readiness |

Each phase is independently deployable and delivers user value.

---

## 6. Out of Scope Dependencies

| Out-of-Scope Item | Dependency on Current Scope | Risk |
|--------------------|----------------------------|------|
| MongoDB support | The `engine` CHECK constraint (`'postgresql', 'mysql'`) will need a migration to extend. The executor pattern (SQL-based) doesn't translate to MongoDB. | **Low** — migration is trivial. But the architecture assumes SQL; MongoDB would need a different executor. The current design doesn't block it but doesn't accommodate it either. |
| Automated remediation | Run history stores `sample_rows` (read-only context). No "fix query" or "remediation action" field. | **Low** — can be added later as a separate field without breaking current design. |
| Check templates library | No `is_template` flag or shared/system-level check table. | **Low** — can be added as a separate table or seeded via import. |

---

## 7. Open Questions

Listed in priority order (most critical first):

All open questions have been resolved:

| # | Question | Resolution |
|---|----------|------------|
| 1 | Notification routing | **RESOLVED** — No parallel system. DataCheck creates issues and calls `alertService.Notify()`. Users configure alert configs with `platform_is: datacheck` conditions. |
| 2 | Run history retention | **RESOLVED** — Configurable via `DATACHECK_RUN_RETENTION_DAYS` env var, default 90 days. |
| 3 | Concurrent execution | **RESOLVED** — `running` state added to `last_status`. Worker skips checks already running. |
| 4 | Role permissions | **RESOLVED** — Members can manage both connections and checks. Viewers read-only. |
| 5 | Project dashboard stats | **RESOLVED** — DataCheck issues count in stats (open issues, trends, etc.). |
| 6 | Connection deletion | **RESOLVED** — Allow with warning. API response includes count of checks that will be cascade-deleted. Frontend shows confirmation dialog. |
| 7 | Issue on check deletion | **RESOLVED** — Issue is left as-is (not auto-resolved). Orphaned but harmless. |
| 8 | Issue merge | **RESOLVED** — DataCheck issues (`platform = 'datacheck'`) cannot be merged. Prevents broken fingerprint routing. |
| 9 | PostgreSQL schema | **RESOLVED** — `schema` field added to connections (default `public`). Sets `search_path` on connect. |
| 10 | Name uniqueness | **RESOLVED** — Unique constraint on `(project_id, name)` for both connections and checks. |

---

## 8. Business Alignment Checklist

| Check | Status |
|-------|--------|
| Primary company objective identified | Not specified |
| Contribution to objective explained | Partially — "predictive monitoring for legacy apps" is the value prop, but no link to a specific business metric |
| KPIs defined with baseline and target | Not specified |
| Justification with specific numbers | Not specified |
| Evidence source documented | Not specified |
| Revenue impact quantified | Not specified |

**Note**: This may be intentional — GoSnag is an internal/open-source tool where business metrics don't apply in the traditional sense. If this is targeting external users or a paid tier, business alignment should be defined.

---

## 9. Full Checklist

### Content Completeness
- [x] Summary explains What, Why, Who
- [x] Business context/problem is clear
- [x] Functional requirements have acceptance criteria
- [x] Data requirements (input/output) specified
- [x] Out of scope defined

### Use Case Coverage
- [x] CRUD operations checked for each entity
- [x] Lifecycle transitions identified
- [ ] **Inverse operations considered** — missing: disable connection cascade, check deletion cleanup
- [ ] **Error recovery paths defined** — missing: invalid cron, connection failure during check
- [ ] **Undo/cancel flows addressed** — missing: what if user accidentally deletes a connection?

### Entity Status & Transitions
- [ ] **All entities have statuses defined** — missing: connection health status, check `running` state
- [x] Initial status specified
- [ ] **All valid transitions documented** — implicit only
- [x] Transition triggers defined (cron schedule, assertion result)
- [ ] **Side effects documented** — partially (issue creation yes, but connection disable cascade no)
- [x] Delete strategy defined (hard delete with CASCADE)

### Collateral Impact
- [ ] **Affected existing entities identified** — not in the epic (identified in this analysis)
- [ ] **Shared data dependencies mapped** — issue table is shared, not documented
- [ ] **Impact on existing business rules analyzed** — alert rules, priority scoring not analyzed
- [ ] **External integrations checked** — Jira auto-creation impact not analyzed
- [ ] **Breaking changes flagged** — no breaking changes, but behavioral changes not flagged

### Slicing
- [x] Requirement size assessed (MoSCoW)
- [x] Each slice delivers value independently
- [ ] **Dependencies between slices documented** — not explicit
- [x] MVP clearly identified
- [ ] **No critical functionality deferred** — run retention is missing from all tiers

### Testing & Definition of Done
- [ ] **Test types specified** — not defined
- [ ] **Critical test scenarios identified** — not defined
- [ ] **Acceptance criteria are testable** — yes, mostly
- [ ] **Quality gates specified** — not defined
- [ ] **Deployment/release criteria defined** — not defined

---

## 10. Recommendations

1. **Define notification routing** before implementation — this is a blocker. Simplest option: add `notify_emails` (text array) and `notify_slack_webhook` (text) fields to `datacheck_checks`, falling back to project-level alert defaults if empty.

2. **Add run retention** to MVP scope — it's a data growth risk. Mirror the existing `EVENT_RETENTION_DAYS` pattern with a `DATACHECK_RUN_RETENTION_DAYS` config.

3. **Add `running` state** to prevent concurrent execution. Simple and avoids a class of hard-to-debug issues.

4. **Add `schema` field** to connections — PostgreSQL users will need this immediately (your own DB uses `search_path=gosnag`).

5. **Document collateral impact** in the epic — especially the issue list mixing and alert evaluation behavior. This avoids surprises during implementation.

6. **Consider phased delivery** within MVP — the current MVP has 20 items. Splitting into 2-3 phases would make each deployable unit more manageable and testable.
