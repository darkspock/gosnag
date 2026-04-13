# Epic: AI-Powered Priority Rules

## Overview
Extend the existing priority rules system with a new `ai_prompt` rule type. Each AI rule defines a custom prompt and an event threshold — the AI evaluates the issue exactly once, when the issue reaches that number of events.

## Problem
The current rule-based system handles deterministic conditions well (event count, level, velocity, pattern matching) but cannot capture nuanced assessments like "is this a data corruption issue?" or "does this error affect checkout flow?". AI evaluation fills this gap with natural language criteria.

## How It Works

### Rule Definition
An AI priority rule has:
- **Name**: descriptive label (e.g. "Critical business flow errors")
- **Prompt**: natural language instruction for the AI
- **Event threshold**: exact event count at which the AI evaluates
  - `1` = evaluate on the first event (first-impression triage)
  - `500` = evaluate when the issue hits 500 events (impact-based scoring)
- **Points**: maximum absolute points the AI can assign (e.g. 30 means AI returns between -30 and +30)

### Evaluation Flow
1. Event ingested → issue upserted → goroutine fires priority evaluation
2. Evaluator loads enabled rules for the project
3. For each `ai_prompt` rule:
   a. Check if `issue.event_count >= rule.threshold`
   b. If not enough events → skip
   c. Check `ai_priority_evaluations` for existing entry (issue_id + rule_id)
   d. If already evaluated → use stored points
   e. If already evaluated and status=error and retries<3 → retry AI call
   f. If already evaluated and status=error and retries>=3 → skip (max retries reached)
   g. If not evaluated → call AI, store result in log table, use returned points
   h. If AI call fails → store with status=error, increment retries
4. Final score clamped to 0–100

The AI evaluates **once per issue per rule**. A log table (`ai_priority_evaluations`) tracks which rules have been applied to which issues. Before calling the AI, the evaluator checks this table — if an entry exists, the rule is skipped. This handles concurrent event ingestion safely (no reliance on exact `==` match) and provides an audit log of AI decisions.

### AI Request
The AI receives:
- System prompt explaining the task and expected JSON format
- The user's custom prompt from the rule
- Issue context: title, level, platform, event count, culprit, latest stack trace

Response:
```json
{
  "points": 25,
  "reason": "This error occurs in the payment processing flow"
}
```

Points clamped to `[-rule.points, +rule.points]`.

### Token Budget
Both models (evaluation and thinking) share the same project daily token budget (`AI_MAX_TOKENS_PER_DAY`). AI priority evaluations use ~300-500 tokens each; the thinking assistant uses more per interaction but is only triggered manually by the admin.

## Database

### Existing table reuse (`priority_rules`)
- `rule_type` = `"ai_prompt"`
- `pattern` → stores the AI prompt text
- `threshold` → stores the event count trigger
- `points` → maximum absolute points the AI can assign

### New table: `ai_priority_evaluations`
Tracks executed AI rules per issue. Serves as both execution guard and audit log.
```sql
CREATE TABLE ai_priority_evaluations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    issue_id UUID NOT NULL REFERENCES issues(id) ON DELETE CASCADE,
    rule_id UUID NOT NULL REFERENCES priority_rules(id) ON DELETE CASCADE,
    status TEXT NOT NULL DEFAULT 'success',  -- success | error
    points INT NOT NULL DEFAULT 0,
    reason TEXT NOT NULL DEFAULT '',
    retries INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(issue_id, rule_id)
);
CREATE INDEX idx_ai_priority_eval_issue ON ai_priority_evaluations(issue_id);
```

## API

No new endpoints. Existing priority rules CRUD handles `ai_prompt` as another `rule_type`:
```
POST /api/v1/projects/{id}/priority-rules
{
  "name": "Business-critical errors",
  "rule_type": "ai_prompt",
  "pattern": "Assign high points if this error affects payment, checkout, or authentication flows",
  "threshold": 1,
  "points": 30,
  "enabled": true
}
```

## Frontend

### Settings Dialog
The `ai_prompt` rule type is only available in the dropdown when the project has AI enabled (`ai_enabled = true`). If AI is not enabled, the option does not appear.

When `rule_type == "ai_prompt"`:
- **Textarea** for the prompt (replaces pattern input)
- **Event threshold** number input with text: "AI evaluates when the issue reaches this many events"
- **Points** input with text: "Maximum points the AI can assign (±)"
- Hide operator field

### Rule Display
AI rules show:
- Rule name
- "AI: {truncated prompt}" as description
- "At {threshold} events" badge
- "±{points} pts"

### Recalculate All
The existing "Recalculate All" button deletes all `ai_priority_evaluations` entries for the project before re-evaluating, so AI rules run again with the current prompts. Only issues that meet `event_count >= threshold` will be re-evaluated.

## Rule Creation Assistant

### Overview
An AI-powered assistant helps admins create priority rules — both basic and AI prompt rules. It uses a **thinking/reasoning model** (separate from the evaluation model) to analyze the project's error patterns and suggest appropriate rules.

### Two AI Models
The system uses two distinct models:
1. **Evaluation model** (existing): fast, cheap model (e.g. Haiku) for runtime rule evaluation. Configured via `AI_PROVIDER` / `AI_MODEL`.
2. **Thinking model** (new): reasoning-capable model (e.g. Claude Sonnet/Opus via Bedrock) for the rule creation assistant. Configured via `AI_THINKING_PROVIDER` / `AI_THINKING_MODEL`. Falls back to the evaluation model if not configured.

### How It Works
1. Admin clicks "AI Assistant" button in the priority rules settings
2. A checkbox "Include recent issues as context" lets the admin choose whether to send issue samples to the AI
3. The assistant receives: project name, existing rules, and optionally ~20 recent issues (title, level, platform, event count)
4. The admin describes what they want in natural language (e.g. "I want to prioritize database errors and payment failures")
5. The thinking model analyzes the request and suggests one or more rules (basic or AI prompt) as structured JSON
6. The admin reviews, edits if needed, and confirms — rules are created via the existing CRUD API

### Assistant API
Conversational endpoint — the frontend sends the full message history each time (same pattern as a chat):
```
POST /api/v1/projects/{id}/priority-rules/suggest
{
  "include_issues": true,
  "messages": [
    { "role": "user", "content": "I want to prioritize errors that affect user payments" },
    { "role": "assistant", "content": "{...previous suggestions...}" },
    { "role": "user", "content": "Add something for database connection errors too" }
  ]
}

Response:
{
  "message": "Here are the updated suggestions...",
  "suggestions": [
    {
      "name": "Payment flow errors",
      "rule_type": "title_contains",
      "pattern": "payment|checkout|stripe|charge",
      "points": 30,
      "explanation": "Matches errors mentioning payment-related keywords"
    },
    {
      "name": "AI: Business-critical impact",
      "rule_type": "ai_prompt",
      "pattern": "Evaluate if this error could block users from completing purchases...",
      "threshold": 1,
      "points": 25,
      "explanation": "AI evaluates first event for business impact on payment flow"
    },
    {
      "name": "Database connection errors",
      "rule_type": "title_contains",
      "pattern": "connection refused|ECONNRESET|database.*timeout",
      "points": 25,
      "explanation": "Matches database connectivity issues"
    }
  ]
}
```
The conversation state lives in the frontend. Each request sends the full history so the backend remains stateless.

### Backend Configuration
```
AI_THINKING_PROVIDER=bedrock        # optional, falls back to AI_PROVIDER
AI_THINKING_MODEL=eu.anthropic.claude-sonnet-4-20250514-v1:0  # optional, falls back to AI_MODEL
```

A second provider instance is created in `NewService` when thinking config is present. The service exposes a `ThinkingChat()` method that routes to the thinking model.

### Frontend
- "AI Assistant" button in priority rules settings (only visible when AI enabled)
- Opens a chat-like dialog: message history + text input + list of suggested rules
- The admin can iterate: refine suggestions, ask for more rules, adjust points
- Each suggestion has: name, type badge, explanation, and "Add" / "Edit" / "Dismiss" buttons
- Adding a suggestion creates the rule via the existing API
- Conversation state is kept in frontend React state (no backend persistence)

## Implementation Tasks

### Phase 1: Database
- [ ] Migration 000039: create `ai_priority_evaluations` table
- [ ] SQL queries: get/upsert evaluations
- [ ] Regenerate sqlc

### Phase 2: AI Prompt Rule Evaluation
- [ ] Add `ai_prompt` case to `evaluator.go` — check `event_count >= threshold`, check log, call AI, clamp points
- [ ] Pass AI service to `Evaluate` function from router
- [ ] Build prompt with issue context (title, level, platform, event count, stack trace)
- [ ] Error handling with retry logic (max 3)

### Phase 3: Thinking Model
- [ ] Add `AI_THINKING_PROVIDER` / `AI_THINKING_MODEL` config
- [ ] Create second provider instance in AI service
- [ ] Add `ThinkingChat()` method to service
- [ ] New endpoint: `POST /projects/{id}/priority-rules/suggest`
- [ ] Build system prompt for rule suggestion with project context

### Phase 4: Frontend
- [ ] Add `ai_prompt` to RULE_TYPES with `needsPrompt: true`
- [ ] Textarea for prompt in rule form dialog
- [ ] Event threshold input
- [ ] Display AI rules appropriately in the list
- [ ] AI Assistant dialog with chat input and suggestion cards
- [ ] "Add" button on each suggestion to create rule via API

### Phase 5: Deploy
- [ ] Build and verify compilation
- [ ] Commit and deploy
- [ ] Configure `AI_THINKING_MODEL` env var in EB
