# Epic: GoSnag-First Incident Management

## Product Thesis

**GoSnag should resolve the majority of incidents end-to-end without leaving the platform. Jira and GitHub Issues remain as secondary systems for escalation or long-running planned work.**

Error monitoring tools today force a context switch: you see the error in one tool, then create a ticket in another to manage it. For incidents — things that are breaking right now — this overhead is unacceptable. Jira is designed for planned work (stories, epics, sprints). Using it for "production is throwing 500s, fix it now" adds ceremony where speed matters.

GoSnag already captures errors, groups them into issues, and lets teams create Jira/GitHub tickets. But creating an external ticket is currently the **only** way to track resolution. The gap is not "create ticket externally" (that already works via manual buttons on the issue detail page and auto-creation rules) — the gap is **triage, follow-through, and close without leaving GoSnag**.

## Terminology

- **Issue**: An error group detected by GoSnag. Accumulates events, has a monitoring status (open/resolved/ignored/snoozed). This is the **monitoring layer** — it exists automatically when errors arrive.
- **Ticket**: The unit of management created when someone decides to actively work on an issue. Has its own workflow (acknowledged → in progress → in review → done). This is the **management layer** — it exists only when a human decides to manage the issue.
- **Escalate**: Move a ticket out of GoSnag into Jira/GitHub when the problem needs long-running planned work.

An issue can exist without a ticket (monitoring only). When you click "Manage", a ticket is created and linked to the issue. The issue continues to accumulate events independently of the ticket's workflow state.

**The two-layer model:**

```
┌─────────────────────────────────────────────────────────┐
│  MONITORING LAYER (automatic)                           │
│  Issue: events, stack traces, breadcrumbs, tags, alerts │
│  Status: open / resolved / reopened / ignored / snoozed │
│  (unchanged from today)                                 │
└────────────────────┬────────────────────────────────────┘
                     │ "Manage" (creates ticket)
                     v
┌─────────────────────────────────────────────────────────┐
│  MANAGEMENT LAYER (intentional)                         │
│  Ticket: assignee, priority, due date, workflow status  │
│  Status: acknowledged → in_progress → in_review → done  │
│  Can escalate to Jira/GitHub                            │
└─────────────────────────────────────────────────────────┘
```

**Why separate entities?**
1. **Clean separation of concerns** — Monitoring (automatic, event-driven) vs. management (intentional, human-driven) are different concerns. Mixing them in one entity creates confusion about what "status" means.
2. **Issue status stays simple** — The existing `open/resolved/reopened/ignored/snoozed` status continues to work exactly as today. No migration, no breaking changes. Ingest behavior is unchanged.
3. **Ticket is opt-in** — Not every issue needs management. A flood of `info`-level issues doesn't need 500 tickets. You create tickets only for what matters.
4. **Multiple issues, one ticket** — A ticket could potentially link to multiple related issues (future: "these 3 errors are all caused by the same bug").
5. **Escalation is clean** — "Continue in Jira" changes the ticket's status to `escalated`, not the issue's. The issue keeps monitoring normally.

## What Already Exists

| Feature | Current State |
|---------|--------------|
| Issue statuses | `open`, `resolved`, `reopened`, `ignored`, `snoozed` — unchanged by this epic |
| Assignment | Single user on issue, nullable, no history |
| Comments | Flat (non-threaded), plain text, CRUD with ownership |
| Following | Per-user follow/unfollow, list sorted by followed first |
| Tags | Key:value, manual + auto-rules via condition engine |
| Priority | Auto-calculated 0–100 via rules, no manual override, no tiers |
| Alerts | Email (SMTP) + Slack on new/reopen only |
| External tickets | Manual create button (Jira/GitHub) + auto-creation rules |
| Activity log | None |
| Board view | None |
| Tickets | **Do not exist yet** |

---

## Block 1: Ticket Entity and Workflow

**Priority: Highest. Everything else depends on this.**

### 1.1 Ticket Data Model

```sql
CREATE TABLE tickets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    issue_id UUID NOT NULL REFERENCES issues(id) ON DELETE CASCADE,
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    status TEXT NOT NULL DEFAULT 'acknowledged'
        CHECK (status IN ('acknowledged', 'in_progress', 'in_review', 'done', 'escalated')),
    assigned_to UUID REFERENCES users(id) ON DELETE SET NULL,
    created_by UUID NOT NULL REFERENCES users(id),
    priority INT NOT NULL DEFAULT 50,       -- manual priority (P1=90, P2=70, P3=50, P4=25)
    due_date TIMESTAMPTZ,
    resolution_type TEXT,                    -- fixed, wontfix, duplicate, cannot_reproduce, by_design
    resolution_notes TEXT,
    fix_reference TEXT,                       -- commit SHA, PR URL, release version
    escalated_system TEXT,                   -- 'jira' or 'github'
    escalated_key TEXT,                      -- 'PROJ-123' or '#45'
    escalated_url TEXT,                      -- full URL to external ticket
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_tickets_issue ON tickets(issue_id);
CREATE INDEX idx_tickets_project_status ON tickets(project_id, status);
CREATE INDEX idx_tickets_assigned ON tickets(assigned_to) WHERE assigned_to IS NOT NULL;
```

**Key design decisions:**
- One ticket per issue (enforced by application logic, not UNIQUE constraint — allows re-creating tickets if needed)
- `issue_id` links back to the monitoring layer
- `project_id` is denormalized for efficient board queries (avoids join through issues)
- `priority` is always manual on tickets (the auto-calculated priority stays on the issue)
- `created_by` tracks who initiated management
- Escalation fields live on the ticket, not the issue

### 1.2 Ticket State Machine

```
                ┌──────────────────┐
    create      │   ACKNOWLEDGED   │ ──── escalate ──→ ESCALATED
    ticket ───→ │   (triaging)     │                      │
                └───────┬──────────┘                      │
                        │ start                    pull back
                        v                                 │
                ┌──────────────────┐                      │
         ┌─────│   IN PROGRESS    │ ──── escalate ──→ ESCALATED
         │      │   (working)      │
     reject     └───────┬──────────┘
         │              │ submit
         │              v
         │      ┌──────────────────┐
         └──────│    IN REVIEW     │ ──── escalate ──→ ESCALATED
                │   (verifying)    │
                └───────┬──────────┘
                        │ approve
                        v
                ┌──────────────────┐
                │      DONE        │
                │   (resolved)     │
                └──────────────────┘
```

#### Transition Table

| From | To | Action |
|------|----|--------|
| `acknowledged` | `in_progress` | Start working |
| `acknowledged` | `done` | Quick resolve (trivial fix) |
| `acknowledged` | `escalated` | Continue in Jira/GitHub |
| `in_progress` | `in_review` | Submit for review |
| `in_progress` | `done` | Resolve directly |
| `in_progress` | `escalated` | Continue in Jira/GitHub |
| `in_review` | `in_progress` | Reject (needs more work) |
| `in_review` | `done` | Approve fix |
| `in_review` | `escalated` | Continue in Jira/GitHub |
| `escalated` | `in_progress` | Pull back to GoSnag |
| `escalated` | `done` | Resolved externally |
| `done` | `acknowledged` | Reopen ticket (e.g., fix didn't work) |

#### Ticket Lifecycle

1. **Create**: User clicks "Manage" on an issue → ticket created with status `acknowledged`, assigned to the user
2. **Work**: User moves through in_progress → in_review → done
3. **Done**: When ticket reaches `done`, the linked issue is automatically transitioned to `resolved` (with cooldown)
4. **Reopen**: If the issue gets new events and auto-reopens, the ticket (if `done`) can be re-activated to `acknowledged`
5. **Escalate**: At any active state, user can escalate to Jira/GitHub → ticket moves to `escalated`, external ticket is created/linked
6. **De-escalate**: Pull back from `escalated` to `in_progress` to resume management in GoSnag

### 1.3 Interaction Between Issue and Ticket

| Event | Issue Effect | Ticket Effect |
|-------|-------------|---------------|
| New error event arrives | event_count++, last_seen updated | No change (ticket tracks management, not events) |
| Issue auto-reopens (cooldown expired) | Status → `reopened` | If ticket exists and is `done`, ticket → `acknowledged` (optional, configurable) |
| Ticket reaches `done` | Issue → `resolved` (with cooldown) | — |
| Ticket escalated | No change to issue status | Ticket → `escalated`, external ticket created |
| User resolves issue directly (no ticket) | Issue → `resolved` | No ticket involved (simple mode behavior) |
| User ignores/snoozes issue | Issue → `ignored`/`snoozed` | If ticket exists, ticket is closed (status → `done`, resolution_type = issue status) |

**Issue status is NOT changed by ticket creation.** An issue stays `open` even when a ticket is `in_progress`. The issue status reflects the monitoring state (are errors still coming?), the ticket status reflects the management state (is someone working on it?).

The exception: when a ticket reaches `done`, the issue is resolved. This is the point where management and monitoring sync.

### 1.4 Per-Project Workflow Mode

Project setting `workflow_mode`:

- **`simple`** (default): No tickets. Current behavior unchanged. Issues have their existing status workflow.
- **`managed`**: Tickets enabled. "Manage" button appears on issues. Board view available.

### 1.5 API

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/projects/{id}/issues/{issue_id}/ticket` | Create ticket for issue |
| `GET` | `/projects/{id}/issues/{issue_id}/ticket` | Get ticket for issue (404 if none) |
| `PUT` | `/projects/{id}/tickets/{ticket_id}` | Update ticket (status, assignee, priority, etc.) |
| `GET` | `/projects/{id}/tickets/{ticket_id}/transitions` | Valid next statuses |
| `GET` | `/projects/{id}/tickets` | List tickets (for board view, filterable) |
| `GET` | `/projects/{id}/tickets/counts` | Count by status (for board column headers) |
| `POST` | `/projects/{id}/tickets/{ticket_id}/escalate` | Escalate to Jira/GitHub |

### 1.6 API and Surface Impact

| Surface | Impact |
|---------|--------|
| Issue list | Show ticket badge (status pill) if ticket exists |
| Issue detail | Show ticket panel or "Manage" button |
| Board view | New page, shows tickets as cards in columns |
| MCP server | New tools: `create_ticket`, `update_ticket`, `list_tickets` |
| Alerts | No change (alerts are on issues, not tickets) |
| Existing issue status API | Unchanged — issue status still works as before |

---

## Block 2: Activity Log

**Priority: High. Required before management UX and reporting can work.**

### 2.1 Activity as Source of Truth

The activity log is the **authoritative record** of every change to a ticket or issue. It is not a secondary projection — every transition must generate an activity entry.

### 2.2 Schema

```sql
CREATE TABLE issue_activities (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    issue_id UUID NOT NULL REFERENCES issues(id) ON DELETE CASCADE,
    ticket_id UUID REFERENCES tickets(id) ON DELETE CASCADE,  -- NULL for issue-only activities
    user_id UUID REFERENCES users(id) ON DELETE SET NULL,     -- NULL for system actions
    action TEXT NOT NULL,
    old_value TEXT,
    new_value TEXT,
    metadata JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_issue_activities_issue ON issue_activities(issue_id, created_at);
CREATE INDEX idx_issue_activities_ticket ON issue_activities(ticket_id, created_at) WHERE ticket_id IS NOT NULL;
```

### 2.3 Actions

**Issue-level actions** (ticket_id = NULL):
- `first_seen` — issue created by first event
- `auto_reopened` / `auto_unsnoozed` — system status changes
- `status_changed` — manual issue status changes (resolve, ignore, snooze)
- `assigned` / `unassigned` — issue-level assignment
- `commented` / `comment_edited` / `comment_deleted`
- `tagged` / `untagged`
- `priority_changed` — auto-calculated priority shift
- `merged`
- `jira_linked` / `github_linked` — external ticket created without managing
- `followed` / `unfollowed`

**Ticket-level actions** (ticket_id set):
- `ticket_created` — management started
- `ticket_status_changed` — workflow transition
- `ticket_assigned` / `ticket_unassigned`
- `ticket_priority_changed` — manual priority set/changed
- `ticket_due_date_set` / `ticket_due_date_cleared`
- `ticket_escalated` — moved to external tracker
- `ticket_de_escalated` — pulled back from external tracker
- `ticket_resolved` — ticket done, with resolution details
- `ticket_reopened` — ticket reactivated

### 2.4 UI: Unified Timeline

The issue detail page shows a unified timeline that merges:
- Issue activities (monitoring events)
- Ticket activities (management events) — if a ticket exists
- Comments

Sorted chronologically. Each entry shows: avatar, actor name, action description, relative time.

---

## Block 3: Management UX and Board

**Priority: Medium. Depends on Block 1 and Block 2.**

### 3.1 "Manage" Button

On the issue detail page:
- If no ticket exists: show "Manage" button
- Clicking "Manage" creates a ticket (status = `acknowledged`, assigned to current user)
- If a ticket exists: show the ticket management panel

### 3.2 Ticket Management Panel

Shown on the issue detail page when a ticket exists:

- **Status flow** — visual indicator of current position in the workflow, with action buttons for valid transitions
- **Assignee picker** — search/select from team members
- **Priority tier** — P1/P2/P3/P4 selector
- **Due date** — date picker
- **Resolution fields** — shown when moving to `done`: resolution type, fix reference, notes
- **"Continue in Jira/GitHub"** — escalation button (creates external ticket + transitions to `escalated`)
- **Escalated banner** — when status is `escalated`, show persistent link to external ticket + "Pull back to GoSnag" action

### 3.3 Issue List Integration

Each issue row shows:
- Existing: level, status, title, tags, events, users, trend
- **New**: ticket status badge (small pill: "In Progress", "In Review", etc.) if a ticket exists
- Clicking the ticket badge navigates to issue detail with ticket panel open

### 3.4 Kanban Board

New page at `/projects/{id}/board` (only available when `workflow_mode = 'managed'`):

**Columns:**
| Column | Ticket Status |
|--------|--------------|
| Triage | `acknowledged` |
| In Progress | `in_progress` |
| In Review | `in_review` |
| Done | `done` (recent, e.g., last 7 days) |
| Escalated | `escalated` (collapsed section, not draggable) |

**Cards show:** issue title (truncated), level badge, priority tier, assignee avatar, event count, due date indicator

**Drag and drop:** Move cards between columns = ticket status transition. Validates against state machine. Records activity.

**Filters:** level, search, tag, assignee, priority tier

**Toggle:** List view ↔ Board view via tabs in the project navigation.

### 3.5 Priority Tiers

| Tier | Score | Label |
|------|-------|-------|
| P1 | 90 | Critical |
| P2 | 70 | High |
| P3 | 50 | Medium (default) |
| P4 | 25 | Low |

Priority lives on the ticket. The issue retains its auto-calculated priority independently.

---

## Block 4: Collaboration

**Priority: Lower. Independent of Blocks 1–3.**

Each item is a separate deliverable.

### 4.1 Follower Notifications
Notify followers on: ticket created, status changed, commented, assigned.

### 4.2 @Mentions in Comments
Parse `@username`, notify mentioned users, render as styled spans.

### 4.3 Markdown Comments
GFM rendering, code blocks, sanitization. Image uploads as separate sub-deliverable.

---

## Block 5: Planning and Reporting

**Priority: Lowest. Requires solid activity log history.**

### 5.1 Resolution Tracking
Resolution type, fix reference, verification (events after resolve).

### 5.2 SLA Rules
Per-project: "P1 must be acknowledged within 15 min, resolved within 4h."

### 5.3 Reporting Dashboard
MTTA, MTTR, resolution rate, SLA compliance, throughput by status. All from activity log.

---

## Data Model Summary

### New Tables

| Table | Purpose |
|-------|---------|
| `tickets` | Management layer — workflow, assignee, priority, escalation |
| `issue_activities` | Source-of-truth activity timeline |

### Modified Tables

| Table | Changes |
|-------|---------|
| `projects` | Add `workflow_mode TEXT NOT NULL DEFAULT 'simple'` |

### Unchanged

| Table | Why |
|-------|-----|
| `issues` | Issue status, fields, and ingest behavior are completely unchanged |
| `events` | No changes |
| `issue_comments` | Stay as-is, referenced from activity log |

---

## Implementation Order

```
Block 1: Tickets ───────────────┐
  1.1 tickets table              │
  1.2 State machine              ├──> Block 2: Activity Log ──> Block 3: Management UX + Board
  1.3 CRUD API                   │      2.1 Schema                  3.1 "Manage" button
  1.4 Issue↔Ticket interaction   │      2.2 Record all changes      3.2 Ticket panel
  1.5 Escalation logic           │      2.3 Timeline UI             3.3 Issue list badges
  1.6 Workflow mode setting      │                                   3.4 Kanban board
                                 │
                                 └──> Block 4: Collaboration
                                 └──> Block 5: Reporting
```

## MVP

1. **Tickets table + state machine** (Block 1)
2. **Activity log** (Block 2)
3. **"Manage" button + ticket panel** (Block 3.1, 3.2)
4. **Issue list ticket badges** (Block 3.3)

**Deferred from MVP:** Board view, SLA, @mentions, markdown, reporting.

This gives: detect → manage → assign → work → review → done, with audit trail and escalation path, without touching the existing issue model.

---

## UX Principles

1. **Two layers, one screen** — Error data (monitoring) and ticket state (management) are both visible on the issue detail page. No tab-switching.
2. **Opt-in complexity** — Issues work exactly as before. Tickets appear only when someone decides to manage.
3. **Incident-first** — The workflow is optimized for fast triage and resolution, not backlog grooming.
4. **One code path** — Ticket transitions from the detail page, board drag-and-drop, and API all go through the same validation and produce the same activities.
5. **Activity as narrative** — The timeline tells the full story: "error detected → ticket created → assigned to Juan → in progress → fix submitted → reviewed → done."

## Out of Scope

- Sprint/iteration planning
- Story points / estimation
- Multiple tickets per issue (future consideration)
- Custom workflow states
- Custom fields on tickets
- Approval gates / required reviewers
