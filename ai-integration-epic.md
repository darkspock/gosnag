# Epic: AI-Powered Issue Intelligence

## Product Thesis

**GoSnag captures the data. AI makes sense of it.** Error monitoring tools generate massive amounts of data — stack traces, event patterns, deployment correlations, duplicate issues — that overwhelm teams. AI should handle the pattern recognition, correlation, and triage work so developers focus on fixing, not investigating.

This is not "AI for the sake of AI." Each capability solves a specific, measurable problem:

- **Auto-merge**: Duplicate issues waste time. Two issues about the same root cause should be one issue.
- **Deploy anomaly detection**: "Did the last deploy break something?" should be answered automatically, not by a human staring at dashboards.
- **Ticket description generation**: Writing a good ticket description from a stack trace is tedious. AI does it in seconds.
- **Root cause analysis**: "Why is this happening?" requires correlating multiple signals. AI can do the first pass.
- **Triage suggestions**: "Who should own this?" and "How critical is this?" can be inferred from code ownership, error patterns, and historical data.

## Architecture: Multi-Provider AI

### Provider Abstraction

GoSnag supports multiple AI providers. Teams choose based on cost, privacy, and capability:

```
┌─────────────────────────────────────────┐
│  AI Features                            │
│  (auto-merge, anomaly detection, etc.)  │
└──────────────┬──────────────────────────┘
               │
        ┌──────▼──────┐
        │  AI Provider │ ← interface
        │  Interface   │
        └──────┬──────┘
               │
    ┌──────┬───┼───┬──────┬──────┐
    │      │   │   │      │      │
┌───▼──┐┌──▼─┐┌▼───┐┌──▼──┐┌──▼──┐┌──▼──┐
│OpenAI││Groq││Bed-││Clau-││Gemi-││Olla-│
│      ││    ││rock││de   ││ni   ││ma   │
└──────┘└────┘└────┘└─────┘└─────┘└─────┘
```

**Provider strengths:**
- **OpenAI**: Best overall quality/cost. Broad model selection.
- **Groq**: Fastest inference (~200ms). Ideal for real-time features.
- **Bedrock**: AWS-native. Uses existing IAM, no extra API keys. Claude/Llama/Titan models.
- **Claude**: Best reasoning for complex root cause analysis.
- **Gemini**: Google ecosystem. Large context windows.
- **Ollama**: Self-hosted, zero cost, full privacy. Needs GPU.

### Provider Interface

```go
type AIProvider interface {
    // Chat sends a prompt and returns a response.
    Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error)
    
    // Name returns the provider identifier.
    Name() string
}

type ChatRequest struct {
    SystemPrompt string
    Messages     []Message
    MaxTokens    int
    Temperature  float64
    JSON         bool  // request structured JSON output
}

type Message struct {
    Role    string // "user", "assistant"
    Content string
}

type ChatResponse struct {
    Content    string
    TokensUsed int
}
```

### Configuration

Global AI provider config (one provider for the whole instance):

```
AI_PROVIDER=openai          # openai, groq, bedrock, claude, gemini, ollama
AI_API_KEY=sk-...           # API key for the provider
AI_MODEL=gpt-4o-mini        # model to use
AI_BASE_URL=                # custom endpoint (for Ollama, proxies, etc.)

# Bedrock-specific (uses AWS credentials from environment)
AI_BEDROCK_REGION=eu-west-1
AI_BEDROCK_MODEL_ID=anthropic.claude-3-haiku-20240307-v1:0
```

Per-project overrides (optional):
- Project settings can override the model or disable AI features
- Allows using a cheaper model for high-volume projects

### Cost Control

- **Token budget**: Max tokens per day per project (configurable, default: 100k)
- **Rate limiting**: Max AI calls per minute (prevent runaway loops)
- **Feature toggles**: Each AI feature can be enabled/disabled per project
- **Caching**: Identical prompts within a time window return cached results

---

## Block 1: AI Provider Infrastructure

**Priority: Highest. All features depend on this.**

### 1.1 Data Model

```sql
ALTER TABLE projects
    ADD COLUMN ai_enabled BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN ai_model TEXT NOT NULL DEFAULT '',           -- override global model
    ADD COLUMN ai_auto_merge BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN ai_anomaly_detection BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN ai_ticket_description BOOLEAN NOT NULL DEFAULT true,
    ADD COLUMN ai_root_cause BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN ai_triage BOOLEAN NOT NULL DEFAULT false;
```

### 1.2 Provider Implementations

**OpenAI** (`internal/ai/openai.go`):
- Models: gpt-4o, gpt-4o-mini, gpt-4-turbo, o1-mini
- API: `POST https://api.openai.com/v1/chat/completions`
- JSON mode: `response_format: { type: "json_object" }`

**Groq** (`internal/ai/groq.go`):
- Models: llama-3.3-70b-versatile, llama-3.1-8b-instant, mixtral-8x7b-32768
- API: OpenAI-compatible `POST https://api.groq.com/openai/v1/chat/completions`
- Ultra-fast inference, very low latency — ideal for real-time features (triage, auto-merge)
- JSON mode: same as OpenAI

**Amazon Bedrock** (`internal/ai/bedrock.go`):
- Models: anthropic.claude-3-haiku, anthropic.claude-3-sonnet, meta.llama3-70b-instruct, amazon.titan-text-express
- API: AWS SDK `InvokeModel` / `Converse` API
- Auth: uses AWS credential chain (env vars, instance role, SSO)
- For teams already on AWS — no extra API keys, uses existing IAM
- JSON mode: via system prompt or Converse API tool use

**Anthropic Claude** (`internal/ai/claude.go`):
- Models: claude-sonnet-4-20250514, claude-haiku
- API: `POST https://api.anthropic.com/v1/messages`
- JSON mode: via system prompt instruction

**Google Gemini** (`internal/ai/gemini.go`):
- Models: gemini-2.0-flash, gemini-1.5-pro
- API: `POST https://generativelanguage.googleapis.com/v1beta/models/{model}:generateContent`

**Ollama** (`internal/ai/ollama.go`):
- Models: llama3, mistral, codellama, etc.
- API: `POST http://localhost:11434/api/chat` (OpenAI-compatible mode also available)
- Self-hosted, no token cost, privacy-preserving

### 1.3 Settings UI

Project Settings → AI section:
- Toggle: "Enable AI features"
- Feature toggles: auto-merge, anomaly detection, ticket descriptions, root cause, triage
- Model override (optional)
- Token usage display (today / this week)

Global Admin Settings:
- Provider selection
- API key
- Model
- Base URL (for Ollama/proxies)
- Daily token budget

---

## Block 2: Smart Auto-Merge

**Problem**: Different error messages for the same root cause create duplicate issues. Example: `ConnectionError: Connection refused` and `ConnectionError: Connection reset` from the same service are likely the same underlying problem.

### 2.1 How It Works

When a new issue is created:

1. Fetch the top 10 open issues in the same project with their latest stack trace
2. Send to AI with the prompt:

```
You are analyzing error groups in an error tracking system.

New issue:
Title: {title}
Stack trace: {top 5 frames}
Level: {level}
Platform: {platform}

Existing open issues:
1. ID: {id}, Title: {title}, Stack: {top 3 frames}
2. ...

Are any of the existing issues likely the same root cause as the new issue?
Respond in JSON: { "merge_with": "issue_id" | null, "confidence": 0.0-1.0, "reason": "..." }

Only suggest merging if confidence > 0.8 and the root cause is clearly the same.
```

3. If AI returns a merge suggestion with confidence > 0.8:
   - **Auto-merge**: If project has `ai_auto_merge = true`, merge automatically
   - **Suggest**: Otherwise, show a suggestion on the issue detail page that the user can accept or dismiss

### 2.2 UI

On the issue detail page, a banner:

```
🤖 AI suggests this issue may be a duplicate of "ConnectionError in /api/users" (92% confidence)
   Reason: Both errors originate from the same database connection pool timeout.
   [Merge] [Dismiss]
```

### 2.3 Data Model

```sql
CREATE TABLE ai_merge_suggestions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    issue_id UUID NOT NULL REFERENCES issues(id) ON DELETE CASCADE,
    target_issue_id UUID NOT NULL REFERENCES issues(id) ON DELETE CASCADE,
    confidence REAL NOT NULL,
    reason TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'accepted', 'dismissed')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

---

## Block 3: Deploy Anomaly Detection

**Problem**: After a deploy, errors might spike, new error types appear, or existing issues reopen. Teams catch this manually by watching dashboards. AI should watch automatically and alert when something looks wrong.

### 3.1 How It Works

A background worker runs 15 minutes after each recorded deploy:

1. Get the deploy info (version, commit, timestamp)
2. Query: events in the 15 minutes after deploy vs. the 15 minutes before
3. Identify anomalies:
   - **New issue types**: Issues that didn't exist before the deploy
   - **Spike in events**: Issues with significantly more events post-deploy
   - **Reopened issues**: Issues that were resolved but came back
4. Send to AI for analysis:

```
A deploy just happened:
Version: {version}
Commit: {sha}
Environment: {env}
Deployed at: {timestamp}

Post-deploy anomalies detected:
- 3 new issue types appeared
- Issue "ConnectionError in /api/users" spiked from 2 events/hour to 45 events/hour
- Issue "TimeoutError in /payments" was resolved but reopened

{List of new issues with stack traces}
{Commit diff summary if available}

Analyze these anomalies. Are they likely caused by the deploy?
Respond in JSON:
{
  "severity": "critical" | "warning" | "info" | "none",
  "summary": "one-line summary",
  "details": "explanation",
  "likely_caused_by_deploy": true/false,
  "recommended_action": "rollback" | "investigate" | "monitor" | "ignore"
}
```

5. Based on severity:
   - **Critical**: Send alert immediately (email + Slack)
   - **Warning**: Send alert
   - **Info**: Log, show in deploy history
   - **None**: No action

### 3.2 UI

On the project dashboard, a deploy health indicator:

```
Last deploy: v1.2.3 (15 min ago)
🔴 AI Alert: Deploy likely introduced 3 new errors. Spike in connection errors.
   Recommendation: Investigate ConnectionError in /api/users
   [View details] [Dismiss]
```

Deploy history page shows AI analysis for each deploy.

### 3.3 Data Model

```sql
CREATE TABLE deploy_analyses (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    deploy_id UUID NOT NULL REFERENCES deploys(id) ON DELETE CASCADE,
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    severity TEXT NOT NULL,         -- critical, warning, info, none
    summary TEXT NOT NULL,
    details TEXT NOT NULL,
    likely_deploy_caused BOOLEAN NOT NULL DEFAULT false,
    recommended_action TEXT NOT NULL,
    new_issues_count INT NOT NULL DEFAULT 0,
    spiked_issues_count INT NOT NULL DEFAULT 0,
    reopened_issues_count INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

---

## Block 4: AI Ticket Description

**Problem**: When a developer clicks "Manage" on an issue, the ticket starts with an empty description. Writing a good description requires reading the stack trace, understanding the context, checking event patterns, and looking at affected users. AI can generate a solid first draft.

### 4.1 How It Works

When a ticket is created (manually or from an issue):

1. Gather context:
   - Issue title, level, platform, culprit
   - Latest stack trace (top 10 frames)
   - Event count, user count, first/last seen
   - Breadcrumbs from latest event
   - Request context (URL, method, headers)
   - Tags
   - Suspect commits (if repo is configured)
2. Send to AI:

```
Generate a ticket description for this error. Include:
- A clear summary of what's happening
- The likely root cause based on the stack trace
- Impact assessment (how many users, how frequent)
- Suggested investigation steps
- Any relevant context from the request/breadcrumbs

Error details:
Title: {title}
Culprit: {culprit}
Level: {level}
Events: {count} across {user_count} users
First seen: {first_seen}

Stack trace:
{frames}

Request:
{method} {url}
{headers}

Breadcrumbs:
{last 10 breadcrumbs}

{Suspect commits if available}

Format as Markdown. Be concise but thorough.
```

3. Pre-fill the ticket description with the AI-generated content
4. User can edit before saving (AI is a starting point, not the final word)

### 4.2 UI

When clicking "Manage" on an issue:
- Ticket is created
- Description field shows "Generating description..." spinner
- AI-generated description appears in the WYSIWYG editor
- User can edit, then save

On the ticket detail page, a "Generate description" button if the description is empty.

---

## Block 5: AI Root Cause Analysis

**Problem**: Given a complex error with a deep stack trace, understanding the root cause requires expertise. AI can analyze the stack trace, correlate with recent changes, and suggest what's likely wrong.

### 5.1 How It Works

On-demand (user clicks "Analyze" on an issue or ticket):

1. Gather deep context:
   - Full stack trace of the latest event
   - Recent events timeline (are events increasing? stable? bursty?)
   - Similar issues in the project
   - Suspect commits
   - Deploy history (did this start after a specific deploy?)
   - Tags and environment
2. Send to AI for analysis
3. Display the analysis in a dedicated section

### 5.2 UI

A collapsible "AI Analysis" section on the issue detail page:

```
🤖 Root Cause Analysis (generated 5 min ago)

**Summary**: The ConnectionError is caused by the database connection pool 
being exhausted under load. The error started appearing after deploy v1.2.3 
which increased the default timeout from 5s to 30s, causing connections to 
be held longer.

**Evidence**:
- Error correlates with deploy v1.2.3 (15 min after deploy)
- Suspect commit abc1234 changed db_timeout from 5000 to 30000
- Spike pattern matches connection pool exhaustion (gradual increase, plateau)
- All affected requests are database-heavy endpoints

**Suggested fix**:
1. Revert the timeout change or reduce to 10s
2. Increase connection pool size from 10 to 25
3. Add connection pool monitoring

[Regenerate] [Copy to ticket description]
```

---

## Block 6: AI Triage Suggestions

**Problem**: New issues need to be assigned to the right person with the right priority. AI can suggest based on code ownership, historical assignment patterns, and error severity.

### 6.1 How It Works

When a new issue appears:

1. Analyze the stack trace to identify affected files/modules
2. Check suspect commits to see who last touched those files
3. Check historical patterns: who was assigned similar issues before?
4. Assess severity: event velocity, user impact, error level
5. Generate suggestion:

```json
{
  "suggested_assignee": "user_id",
  "assignee_reason": "Last modified auth.py (3 of 5 stack frames) 2 days ago",
  "suggested_priority": 90,
  "priority_reason": "High velocity (45 events/hour), affects 12 users, production-only"
}
```

### 6.2 UI

On the issue detail page, subtle suggestions:

```
Assignee: [dropdown]  💡 Suggest: Juan (last modified 3/5 stack files)
Priority: [dropdown]  💡 Suggest: P1 (45 events/hour, 12 users affected)
```

User can accept with one click or ignore.

---

## Implementation Order

```
Block 1: Provider Infrastructure ──┐
  1.1 AI provider interface         │
  1.2 OpenAI implementation         ├──> Block 4: Ticket Description (quickest win)
  1.3 Claude implementation         │
  1.4 Ollama implementation         ├──> Block 2: Auto-Merge
  1.5 Config + settings UI          │
                                    ├──> Block 5: Root Cause Analysis
                                    │
                                    ├──> Block 3: Deploy Anomaly Detection
                                    │
                                    └──> Block 6: Triage Suggestions
```

## MVP

1. **Provider infrastructure** with OpenAI + Groq + Bedrock (Block 1)
2. **AI ticket description** — auto-generate on ticket creation (Block 4)
3. **Auto-merge suggestions** — suggest duplicates, manual accept (Block 2)

These three give immediate visible value. Deploy anomaly detection and root cause analysis are more complex and can follow.

Adding Claude, Gemini, and Ollama as providers is straightforward once the interface exists — each is ~100 lines of Go.

---

## Cost Estimates

Estimated per provider, medium-sized project (~50 new issues/day):

| Provider | Model | Cost/month | Latency | Notes |
|----------|-------|-----------|---------|-------|
| **Groq** | llama-3.3-70b | ~$1.50 | ~200ms | Fastest. Best for real-time (triage, merge) |
| **OpenAI** | gpt-4o-mini | ~$2.70 | ~1-2s | Best quality/cost ratio |
| **Bedrock** | claude-3-haiku | ~$3.00 | ~1-3s | No extra keys if already on AWS |
| **Bedrock** | llama3-70b | ~$2.00 | ~2-4s | Good quality, AWS-native |
| **Claude** | claude-haiku | ~$5.00 | ~1-2s | Direct Anthropic API |
| **Ollama** | llama3 | $0 | ~3-5s | Self-hosted, private, needs GPU |

Recommendation: **Groq for latency-sensitive features** (auto-merge, triage) + **OpenAI or Bedrock for quality-sensitive features** (root cause, descriptions).

---

## Privacy and Security

- **No data leaves the instance with Ollama**: For teams with strict data policies, Ollama runs locally
- **API keys stored encrypted**: Same pattern as other credentials (never returned in API)
- **Prompt sanitization**: Strip PII from prompts if configured (email addresses, IP addresses)
- **Opt-in per project**: AI is disabled by default, enabled per project
- **Audit log**: Record when AI was used, what prompt was sent (without the response), token count

## Out of Scope

- Training custom models on project data
- AI-powered alerting rules (keep the existing condition engine)
- Natural language search ("show me all timeout errors from last week")
- AI code fix suggestions ("here's a PR that fixes this")
- Multi-provider fallback (use provider A, fall back to B if rate limited)
