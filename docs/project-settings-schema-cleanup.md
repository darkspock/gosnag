# Project Settings Schema Cleanup

## Goal

Finish the project settings refactor by removing legacy settings columns from `projects` and making the domain tables the only source of truth.

Scope:
- stop reading legacy settings from `projects`
- stop writing legacy settings to `projects`
- verify every feature reads from the new `project_*_settings` tables
- drop obsolete columns only after rollout validation

This is a rollout and cleanup task, not a product feature.

## Current State

Today the system is in a safe mixed mode:
- new domain tables already exist
- initial backfill has been done
- project settings flows can use the new tables
- legacy columns still exist in `projects`
- some compatibility paths still assume legacy data may be present

That is the correct intermediate state, but it is not the final schema.

## Target State

After cleanup:
- `projects` keeps only core identity and stable metadata
- settings live in their domain tables
- API responses are assembled from domain tables
- ingest, integrations, AI, issue settings, stacktrace settings, DB analysis, and route grouping all read from domain tables only
- legacy columns are removed from `projects`

## Rollout Plan

### Phase 1: Audit

Confirm every code path that still depends on legacy columns.

Checklist:
- project read handlers
- project update handlers
- cache and list endpoints
- ingest/runtime readers
- Jira/GitHub/repo integration readers
- AI settings readers
- DB analysis readers
- route grouping readers
- tests and fixtures

Deliverable:
- explicit inventory of remaining legacy reads/writes

### Phase 2: Migrate Reads

Make domain tables the only read path.

Rules:
- remove fallback-to-legacy logic where new rows are guaranteed
- create defaults in code or migration, not from legacy fields
- ensure empty/missing rows are normalized consistently

Validation:
- open project settings for old and new projects
- save each settings area independently
- run ingest for a project with active traffic
- test Jira/GitHub/repo/AI/DB analysis on a real project

### Phase 3: Migrate Writes

Stop writing legacy columns.

Rules:
- write only to `project_*_settings` tables
- ensure create-project flow seeds required settings rows
- ensure update-project flow never relies on legacy persistence

Validation:
- create a brand new project
- edit every settings section
- reload and confirm persistence

### Phase 4: Production Observation

Leave the schema in non-destructive mode for at least one deployment cycle.

Observe:
- settings page load failures
- missing values after refresh
- integration failures caused by empty settings
- cache/list regressions
- ingest grouping regressions

Recommended minimum:
- one full production deploy
- one settings edit cycle on real projects

### Phase 5: Contract

Drop legacy columns from `projects`.

Rules:
- do this only after reads and writes are fully migrated
- ship column drops in a dedicated migration
- keep the rollback path clear

Deliverable:
- migration removing obsolete settings columns
- updated generated SQL / models
- removal of compatibility code

## Risks

### Missing Settings After Refresh

Cause:
- a code path still reads from `projects`

Mitigation:
- audit all readers before destructive migration

### New Projects Missing Defaults

Cause:
- create flow assumes legacy defaults from `projects`

Mitigation:
- seed default rows for every required domain table

### Hidden Integration Regressions

Cause:
- tokens/config are still read from legacy fields in some services

Mitigation:
- smoke test Jira, GitHub, repo, AI, and DB analysis explicitly

### SQLC Churn

Cause:
- dropping columns changes generated project models and queries

Mitigation:
- land cleanup in a dedicated PR/commit
- regenerate and review all affected query code in one pass

## Pre-Drop Checklist

- all settings pages load correctly
- all settings save correctly
- project creation seeds new settings rows
- no remaining legacy reads in code search
- no remaining legacy writes in code search
- integration smoke tests pass
- ingest smoke tests pass
- tests pass

## Success Criteria

Cleanup is complete when:
- `projects` no longer stores migrated settings fields
- code no longer depends on legacy settings columns
- one production cycle has passed without settings regressions
