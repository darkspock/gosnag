# Validation: AI-Powered Priority Rules

**Type:** Epic
**Date:** 2026-04-13
**Status:** Approved with Clarifications

## Summary
Epic is well-scoped. All gaps were clarified during validation. The design reuses existing infrastructure (priority_rules table, AI service, token budget) and adds one new table for execution tracking/audit plus a second AI model for the rule creation assistant.

## Checklist Results

### Passed
- [x] Objective and value proposition are explicit
- [x] Primary entity (priority_rules with new type) identified
- [x] Evaluation flow is clearly documented
- [x] API reuses existing endpoints — one new endpoint for assistant
- [x] Token budget cost control via event threshold
- [x] Frontend changes scoped to settings dialog
- [x] Database schema reuses existing columns where possible
- [x] Multiple AI rules per project supported, no artificial limits

### Clarified During Validation
- [x] Concurrent event ingestion safety — uses `>=` threshold + log table lookup instead of exact `==` match. Log table (`ai_priority_evaluations`) acts as execution guard and audit trail
- [x] AI call failure handling — store with status=error, retry up to 3 times on subsequent events
- [x] Re-evaluation on prompt change — "Recalculate All" button clears AI evaluations and re-runs rules
- [x] AI enabled gate — `ai_prompt` rule type only available in frontend when project has `ai_enabled = true`
- [x] Multiple AI rules — each rule evaluated independently, points summed, no limit on number of rules
- [x] Rule assistant is conversational — frontend keeps message history, backend is stateless, full history sent each request
- [x] Issue context is opt-in — checkbox to include ~20 recent issues as context for the assistant
- [x] Thinking model shares the same daily token budget as the evaluation model

### Accepted Risks
- None

### Skipped
- None

## Entity Analysis

### `priority_rules` (existing, extended)
- New `rule_type` value: `"ai_prompt"`
- `pattern` field reused for AI prompt text
- `threshold` field reused for event count trigger
- `points` field = max absolute points AI can assign
- CRUD: fully covered by existing endpoints

### `ai_priority_evaluations` (new)
- Created on first AI evaluation of an issue+rule pair
- Updated on retry (retries counter, status, updated_at)
- Deleted on "Recalculate All" or when parent rule/issue is deleted (CASCADE)
- States: `success` | `error`
- No UI for browsing this table (audit log only)

## Collateral Impact
- **AI token budget**: shared across evaluation model and thinking model. Event threshold controls evaluation cost; assistant is manual-only.
- **Priority evaluator**: function signature changes (needs AI service parameter). Affects `router.go` call site.
- **Config**: two new optional env vars (`AI_THINKING_PROVIDER`, `AI_THINKING_MODEL`). Falls back to existing provider if not set.
- **Recalculate All**: existing endpoint extended to clear AI evaluation entries before re-running.

## Anti-Patterns Found
- None

## Recommendations
1. Consider showing AI evaluation results (reason) somewhere in the issue detail UI in a future iteration — the log table stores the reasoning.

## Decision
> **The user decides whether to proceed.** This validation is informational.
