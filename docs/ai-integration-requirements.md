# AI Integration — Requirements Document

Reference: [ai-integration-epic.md](ai-integration-epic.md)

---

## 1. Overview

Add AI capabilities to GoSnag for automated issue analysis, duplicate detection, deploy monitoring, and ticket assistance. Support multiple AI providers to allow teams to choose based on cost, latency, privacy, and existing infrastructure.

### 1.1 Goals

- Reduce mean time to triage by automating duplicate detection and priority suggestions
- Reduce time writing ticket descriptions by auto-generating context-rich first drafts
- Detect deploy-related regressions automatically and alert before manual detection
- Provide root cause analysis that correlates stack traces, commits, and deploy history
- Allow self-hosted AI (Ollama) for teams with data privacy requirements

### 1.2 Non-Goals

- Training custom models on project data
- AI-powered alert rule authoring
- Natural language search
- AI code fix suggestions or auto-generated PRs
- Multi-provider fallback chains

---

## 2. AI Provider Infrastructure

### 2.1 Provider Interface

**REQ-AI-001**: The system MUST define a Go interface `AIProvider` with methods:
- `Chat(ctx, request) → response, error`
- `Name() → string`

**REQ-AI-002**: The `ChatRequest` struct MUST support:
- System prompt (string)
- Message list (role + content)
- Max tokens (int)
- Temperature (float)
- JSON mode flag (bool)

**REQ-AI-003**: The `ChatResponse` struct MUST return:
- Content (string)
- Tokens used (int)

### 2.2 Provider Implementations

**REQ-AI-010**: The system MUST implement an OpenAI provider.
- Endpoint: `POST https://api.openai.com/v1/chat/completions`
- Auth: Bearer token from `AI_API_KEY`
- JSON mode: `response_format: { type: "json_object" }`
- Models: gpt-4o, gpt-4o-mini, gpt-4-turbo, o1-mini

**REQ-AI-011**: The system MUST implement a Groq provider.
- Endpoint: `POST https://api.groq.com/openai/v1/chat/completions`
- Auth: Bearer token from `AI_API_KEY`
- API is OpenAI-compatible; implementation MAY share code with OpenAI provider via a base URL parameter
- Models: llama-3.3-70b-versatile, llama-3.1-8b-instant, mixtral-8x7b-32768

**REQ-AI-012**: The system MUST implement an Amazon Bedrock provider.
- API: AWS SDK v2 `Converse` API
- Auth: AWS credential chain (env vars `AWS_ACCESS_KEY_ID`/`AWS_SECRET_ACCESS_KEY`, instance role, SSO profile)
- Region: from `AI_BEDROCK_REGION` (default: `eu-west-1`)
- Model ID: from `AI_BEDROCK_MODEL_ID`
- Models: anthropic.claude-3-haiku, anthropic.claude-3-sonnet, meta.llama3-70b-instruct, amazon.titan-text-express

**REQ-AI-013**: The system SHOULD implement an Anthropic Claude provider (direct API, not through Bedrock).
- Endpoint: `POST https://api.anthropic.com/v1/messages`
- Auth: `x-api-key` header
- JSON mode: via system prompt instruction

**REQ-AI-014**: The system SHOULD implement an Ollama provider.
- Endpoint: configurable via `AI_BASE_URL` (default: `http://localhost:11434`)
- API: `POST /api/chat`
- Auth: none (local)
- No token cost. Self-hosted.

**REQ-AI-015**: The system MAY implement a Google Gemini provider in a future iteration.

### 2.3 Configuration

**REQ-AI-020**: Global AI configuration via environment variables:

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `AI_PROVIDER` | Yes (if AI used) | `""` | Provider: `openai`, `groq`, `bedrock`, `claude`, `ollama` |
| `AI_API_KEY` | Provider-dependent | `""` | API key (not needed for Bedrock/Ollama) |
| `AI_MODEL` | No | Provider default | Model name/ID |
| `AI_BASE_URL` | No | `""` | Custom endpoint URL |
| `AI_BEDROCK_REGION` | Bedrock only | `eu-west-1` | AWS region |
| `AI_BEDROCK_MODEL_ID` | Bedrock only | `""` | Bedrock model ID |
| `AI_MAX_TOKENS_PER_DAY` | No | `100000` | Token budget per project per day |
| `AI_MAX_CALLS_PER_MINUTE` | No | `10` | Rate limit |

**REQ-AI-021**: If `AI_PROVIDER` is empty or not set, all AI features MUST be disabled without error. The system operates normally without AI.

**REQ-AI-022**: If `AI_PROVIDER` is set but the API key is invalid, the system MUST log a warning at startup and disable AI features gracefully.

### 2.4 Per-Project Settings

**REQ-AI-030**: Each project MUST have the following AI-related settings:

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `ai_enabled` | boolean | false | Master toggle |
| `ai_model` | string | "" | Override global model |
| `ai_auto_merge` | boolean | false | Auto-merge duplicates without user confirmation |
| `ai_anomaly_detection` | boolean | false | Run post-deploy anomaly analysis |
| `ai_ticket_description` | boolean | true | Auto-generate ticket descriptions |
| `ai_root_cause` | boolean | false | Enable root cause analysis button |
| `ai_triage` | boolean | false | Show triage suggestions |

**REQ-AI-031**: All AI features MUST check both the global provider config AND the project's `ai_enabled` flag before executing. If either is disabled, the feature MUST NOT run.

**REQ-AI-032**: The project settings UI MUST show the AI section only when a global AI provider is configured.

### 2.5 Cost Control

**REQ-AI-040**: The system MUST track token usage per project per day.

**REQ-AI-041**: If a project exceeds its daily token budget (`AI_MAX_TOKENS_PER_DAY`), all AI features for that project MUST be paused until the next calendar day (UTC).

**REQ-AI-042**: The system MUST rate-limit AI calls to `AI_MAX_CALLS_PER_MINUTE` per project. Calls exceeding the limit MUST be dropped silently (no error to the user for background features) or return a user-facing message for on-demand features.

**REQ-AI-043**: The system SHOULD cache identical prompts within a 5-minute window and return the cached response.

### 2.6 Audit

**REQ-AI-050**: Each AI call MUST be logged with: project ID, feature name, model used, input token count, output token count, timestamp, latency (ms).

**REQ-AI-051**: The system MUST NOT log the full prompt or response content. Only metadata.

**REQ-AI-052**: Token usage MUST be visible in Project Settings → AI section (today's usage, this week's usage).

---

## 3. Smart Auto-Merge

### 3.1 Trigger

**REQ-MERGE-001**: When a new issue is created (first event with a new fingerprint), IF the project has `ai_enabled = true` AND `ai_auto_merge = true` OR the feature is enabled for suggestions, the system MUST evaluate the issue for potential duplicates.

**REQ-MERGE-002**: The evaluation MUST run asynchronously (goroutine) after event ingestion completes. It MUST NOT block the ingest response.

### 3.2 Input

**REQ-MERGE-010**: The system MUST fetch up to 10 open issues in the same project, ordered by last_seen DESC, including their latest event's stack trace (top 5 frames).

**REQ-MERGE-011**: The system MUST send the new issue's title, stack trace (top 5 frames), level, and platform, alongside the candidate issues, to the AI provider.

### 3.3 Output

**REQ-MERGE-020**: The AI response MUST be parsed as JSON with fields:
- `merge_with`: issue ID (string) or null
- `confidence`: float 0.0–1.0
- `reason`: string explanation

**REQ-MERGE-021**: If `confidence >= 0.8` AND `ai_auto_merge = true`, the system MUST automatically merge the new issue into the target issue using the existing merge functionality. An activity entry MUST be recorded with action `ai_auto_merged`.

**REQ-MERGE-022**: If `confidence >= 0.8` AND `ai_auto_merge = false`, the system MUST create a merge suggestion record (pending status) and NOT merge automatically.

**REQ-MERGE-023**: If `confidence < 0.8`, no action. The suggestion MUST NOT be stored.

### 3.4 Data Model

**REQ-MERGE-030**: New table `ai_merge_suggestions`:
- id (UUID, PK)
- issue_id (UUID, FK → issues, ON DELETE CASCADE)
- target_issue_id (UUID, FK → issues, ON DELETE CASCADE)
- confidence (REAL)
- reason (TEXT)
- status (TEXT: 'pending', 'accepted', 'dismissed')
- created_at (TIMESTAMPTZ)

### 3.5 UI

**REQ-MERGE-040**: If an issue has a pending merge suggestion, the issue detail page MUST show a banner with:
- The target issue title
- The confidence percentage
- The AI-generated reason
- "Merge" button (executes merge, sets status = 'accepted')
- "Dismiss" button (sets status = 'dismissed')

**REQ-MERGE-041**: Dismissed suggestions MUST NOT reappear.

### 3.6 API

**REQ-MERGE-050**: `GET /projects/{id}/issues/{id}/merge-suggestion` — returns the pending suggestion or null.
**REQ-MERGE-051**: `POST /projects/{id}/issues/{id}/merge-suggestion/accept` — executes merge.
**REQ-MERGE-052**: `POST /projects/{id}/issues/{id}/merge-suggestion/dismiss` — dismisses.

---

## 4. Deploy Anomaly Detection

### 4.1 Trigger

**REQ-DEPLOY-001**: When a deploy is recorded (`POST /projects/{id}/deploys`), IF the project has `ai_enabled = true` AND `ai_anomaly_detection = true`, the system MUST schedule an analysis to run 15 minutes after the deploy timestamp.

**REQ-DEPLOY-002**: The analysis MUST run as a background worker, not blocking any request.

### 4.2 Data Collection

**REQ-DEPLOY-010**: The system MUST query:
- Events in the 15 minutes after deploy vs. the 15 minutes before deploy
- New issues (fingerprints that didn't exist before the deploy)
- Issues with event velocity increase > 3x post-deploy
- Issues that transitioned from resolved/ignored to reopened within the post-deploy window

**REQ-DEPLOY-011**: If no anomalies are detected (no new issues, no spikes, no reopens), the system MUST store an analysis with severity = 'none' and NOT call the AI provider.

### 4.3 AI Analysis

**REQ-DEPLOY-020**: If anomalies are detected, the system MUST send them to the AI provider with:
- Deploy info (version, commit, environment, timestamp)
- List of new issues (title + top 3 stack frames)
- List of spiked issues (title + pre/post event rate)
- List of reopened issues
- Commit diff summary (if source code integration is configured)

**REQ-DEPLOY-021**: The AI response MUST be parsed as JSON:
- `severity`: "critical", "warning", "info", "none"
- `summary`: one-line summary
- `details`: multi-line explanation
- `likely_caused_by_deploy`: boolean
- `recommended_action`: "rollback", "investigate", "monitor", "ignore"

### 4.4 Actions

**REQ-DEPLOY-030**: On severity "critical": send alert via all configured channels (email + Slack).
**REQ-DEPLOY-031**: On severity "warning": send alert via all configured channels.
**REQ-DEPLOY-032**: On severity "info" or "none": no alert. Store the analysis.

### 4.5 Data Model

**REQ-DEPLOY-040**: New table `deploy_analyses`:
- id (UUID, PK)
- deploy_id (UUID, FK → deploys, ON DELETE CASCADE)
- project_id (UUID, FK → projects, ON DELETE CASCADE)
- severity (TEXT)
- summary (TEXT)
- details (TEXT)
- likely_deploy_caused (BOOLEAN)
- recommended_action (TEXT)
- new_issues_count (INT)
- spiked_issues_count (INT)
- reopened_issues_count (INT)
- created_at (TIMESTAMPTZ)

### 4.6 UI

**REQ-DEPLOY-050**: The project page MUST show a deploy health banner when the latest deploy has a critical or warning analysis.

**REQ-DEPLOY-051**: A deploys page (`/projects/{id}/deploys`) MUST list recent deploys with their AI analysis summary and severity badge.

---

## 5. AI Ticket Description

### 5.1 Trigger

**REQ-DESC-001**: When a ticket is created from an issue (via "Manage" button), IF the project has `ai_enabled = true` AND `ai_ticket_description = true`, the system MUST automatically generate a description.

**REQ-DESC-002**: The generation MUST happen asynchronously. The ticket is created immediately with an empty description. The frontend polls or uses a callback to fill the description when ready.

**REQ-DESC-003**: For manually created tickets (no linked issue), the system MUST NOT auto-generate a description.

### 5.2 Input

**REQ-DESC-010**: The system MUST gather:
- Issue title, level, platform, culprit
- Latest event's stack trace (top 10 frames)
- Event count and user count
- First seen and last seen timestamps
- Breadcrumbs from latest event (last 10)
- Request context (method, URL, headers)
- Tags
- Suspect commits (if source code integration is configured)

### 5.3 Output

**REQ-DESC-020**: The AI response MUST be formatted as HTML (to be inserted into the WYSIWYG editor).

**REQ-DESC-021**: The description MUST include:
- A summary of the error
- Likely root cause based on the stack trace
- Impact assessment (event count, user count, frequency)
- Suggested investigation steps

**REQ-DESC-022**: The generated description MUST be saved to the ticket's `description` field via the existing update API.

### 5.4 UI

**REQ-DESC-030**: After ticket creation from an issue, the ticket detail page MUST show a loading indicator while the description is being generated.

**REQ-DESC-031**: Once generated, the description MUST appear in the WYSIWYG editor, editable by the user.

**REQ-DESC-032**: If the ticket has an empty description AND is linked to an issue, a "Generate description" button MUST be visible.

### 5.5 API

**REQ-DESC-040**: `POST /projects/{id}/tickets/{id}/generate-description` — triggers AI description generation. Returns `{ status: "generating" }`.
**REQ-DESC-041**: The generation result is saved directly to the ticket. The frontend refreshes the ticket to see the updated description.

---

## 6. AI Root Cause Analysis

### 6.1 Trigger

**REQ-RCA-001**: Root cause analysis is on-demand only. The user clicks "Analyze" on an issue detail or ticket detail page.

**REQ-RCA-002**: The system MUST check `ai_enabled = true` AND `ai_root_cause = true` before executing.

### 6.2 Input

**REQ-RCA-010**: The system MUST gather:
- Full stack trace of the latest event
- Event timeline (last 24h: are events increasing, stable, bursty?)
- Top 5 similar issues in the project (by stack trace similarity)
- Suspect commits (if source code integration is configured)
- Recent deploys (last 3)
- Tags, environment, release

### 6.3 Output

**REQ-RCA-020**: The AI response MUST include:
- Summary (1–2 sentences)
- Evidence list (what data supports the conclusion)
- Suggested fix (actionable steps)

**REQ-RCA-021**: The analysis MUST be stored and displayed. It MUST NOT be regenerated on every page load.

### 6.4 UI

**REQ-RCA-030**: The issue detail page MUST show a collapsible "AI Analysis" section with the analysis content rendered as Markdown.

**REQ-RCA-031**: A "Regenerate" button MUST allow the user to request a fresh analysis.

**REQ-RCA-032**: A "Copy to ticket" button MUST copy the analysis content to the linked ticket's description (appended, not replaced).

### 6.5 API

**REQ-RCA-040**: `POST /projects/{id}/issues/{id}/analyze` — triggers analysis. Returns the analysis.
**REQ-RCA-041**: `GET /projects/{id}/issues/{id}/analysis` — returns the latest stored analysis or null.

---

## 7. AI Triage Suggestions

### 7.1 Trigger

**REQ-TRIAGE-001**: When a new issue is created, IF the project has `ai_enabled = true` AND `ai_triage = true`, the system MUST generate triage suggestions.

**REQ-TRIAGE-002**: The suggestions MUST be generated asynchronously after event ingestion.

### 7.2 Input

**REQ-TRIAGE-010**: The system MUST gather:
- Stack trace files/modules
- Suspect commits (who last touched those files)
- Historical assignment patterns (who was assigned similar issues)
- Event velocity, user count, error level

### 7.3 Output

**REQ-TRIAGE-020**: The AI response MUST include:
- `suggested_assignee`: user ID or null
- `assignee_reason`: one-line explanation
- `suggested_priority`: integer (90/70/50/25)
- `priority_reason`: one-line explanation

**REQ-TRIAGE-021**: Suggestions MUST be stored per issue.

### 7.4 UI

**REQ-TRIAGE-030**: The issue detail page MUST show suggestions inline next to the assignee and priority dropdowns. Example: a lightbulb icon with tooltip showing the suggestion and reason.

**REQ-TRIAGE-031**: Clicking the suggestion MUST apply it (assign the user or set the priority). One-click accept.

### 7.5 API

**REQ-TRIAGE-040**: `GET /projects/{id}/issues/{id}/triage-suggestion` — returns the suggestion or null.

---

## 8. Settings UI

### 8.1 Project Settings — AI Section

**REQ-UI-001**: The Project Settings page MUST show an "AI" section when a global AI provider is configured.

**REQ-UI-002**: The section MUST contain:
- Master toggle: "Enable AI features" (controls `ai_enabled`)
- Feature toggles for each capability (auto-merge, anomaly detection, ticket descriptions, root cause, triage)
- Model override input (optional, text field)
- Token usage display: "Today: X tokens / Y budget" with progress bar

**REQ-UI-003**: Feature toggles MUST be disabled (grayed out) when the master toggle is off.

### 8.2 Admin Settings — AI Provider

**REQ-UI-010**: The Admin Settings page MUST have an "AI Provider" section with:
- Provider dropdown (OpenAI, Groq, Bedrock, Claude, Ollama)
- API key input (password field, shows "configured" when set)
- Model input
- Base URL input (for Ollama/proxies)
- Bedrock-specific: region, model ID (shown only when provider = bedrock)
- Daily token budget input
- Rate limit input
- "Test Connection" button: sends a simple prompt and verifies the provider responds

---

## 9. Error Handling

**REQ-ERR-001**: If the AI provider returns an error (rate limit, timeout, invalid response), the system MUST:
- Log the error with provider name, HTTP status, and latency
- NOT crash, hang, or affect non-AI functionality
- Show a user-friendly message for on-demand features ("AI analysis temporarily unavailable")
- Silently skip for background features (auto-merge, triage, anomaly detection)

**REQ-ERR-002**: AI responses that fail JSON parsing MUST be logged and discarded. The feature MUST fall back to no-AI behavior.

**REQ-ERR-003**: All AI calls MUST have a timeout of 30 seconds. Exceeded timeouts MUST be treated as errors.

---

## 10. Privacy

**REQ-PRIV-001**: AI features MUST be opt-in per project (`ai_enabled = false` by default).

**REQ-PRIV-002**: With Ollama provider, no data leaves the GoSnag instance. The system MUST document this clearly in the settings UI.

**REQ-PRIV-003**: The system SHOULD offer a PII stripping option that removes email addresses and IP addresses from prompts before sending to external providers.

**REQ-PRIV-004**: API keys MUST never be returned in API responses. Use the `_set` boolean pattern.

---

## 11. Testing

**REQ-TEST-001**: Each provider implementation MUST have unit tests that verify request construction and response parsing using a mock HTTP server.

**REQ-TEST-002**: The auto-merge feature MUST have integration tests verifying: merge on high confidence, suggestion on medium confidence, no action on low confidence.

**REQ-TEST-003**: The deploy anomaly detection MUST have tests for: no anomalies = no AI call, anomalies detected = AI called, critical severity = alert sent.

---

## 12. Implementation Phases

### Phase 1 (MVP)
- AI provider interface + OpenAI + Groq + Bedrock implementations
- Global config (env vars) + per-project settings (DB + UI)
- Token tracking and rate limiting
- AI ticket description generation
- Auto-merge suggestions (manual accept only, no auto-merge yet)

### Phase 2
- Auto-merge (automatic execution when `ai_auto_merge = true`)
- Deploy anomaly detection + alerts
- Root cause analysis (on-demand)

### Phase 3
- Triage suggestions
- Claude + Ollama providers
- PII stripping
- Token usage dashboard

---

## 13. Acceptance Criteria

| Feature | Acceptance Criteria |
|---------|-------------------|
| Provider infra | Can switch between OpenAI, Groq, and Bedrock by changing env var; all features work with each |
| Ticket description | Clicking "Manage" on an issue generates a description within 10s that includes summary, root cause, and impact |
| Auto-merge suggestion | New issue that is a clear duplicate shows a merge suggestion banner within 30s |
| Auto-merge execution | With `ai_auto_merge = true`, the duplicate is merged automatically with activity log entry |
| Deploy anomaly | 15 min after a deploy that introduces errors, an alert is sent with severity and recommendation |
| Root cause analysis | Clicking "Analyze" generates a structured analysis within 15s |
| Triage suggestion | New issue shows assignee and priority suggestions within 30s |
| Cost control | Exceeding daily token budget pauses AI features; resumes next day |
| Privacy | Ollama provider keeps all data local; no external API calls |
