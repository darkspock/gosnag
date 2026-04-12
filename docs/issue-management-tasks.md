# Issue Management — Implementation Tasks

Reference: [issue-management-epic.md](issue-management-epic.md)

---

## Block 1: State Machine and Workflow

### 1.1 Migration: Extend status CHECK and add workflow_mode
- [ ] Create migration `000028_workflow_statuses.up.sql`:
  - `ALTER TABLE issues DROP CONSTRAINT issues_status_check`
  - `ALTER TABLE issues ADD CONSTRAINT issues_status_check CHECK (status IN ('open', 'resolved', 'reopened', 'ignored', 'snoozed', 'acknowledged', 'in_progress', 'in_review', 'escalated'))`
  - `ALTER TABLE projects ADD COLUMN workflow_mode TEXT NOT NULL DEFAULT 'simple'`
- [ ] Create matching `down.sql`

### 1.2 State machine: Transition validation in Go
- [ ] Create `internal/workflow/transitions.go`:
  - Define `ValidTransitions map[string][]string` — the complete transition table from the epic
  - `func IsValidTransition(from, to string) bool`
  - `func ValidNextStatuses(from string) []string`
  - `func FilterByWorkflowMode(statuses []string, mode string) []string` — removes `acknowledged`, `in_progress`, `in_review`, `escalated` in simple mode
- [ ] Write unit tests for the transition table: every valid transition passes, every invalid is rejected, simple mode filtering works

### 1.3 Update issue status handler to enforce transitions
- [ ] Read `internal/issue/handler.go` UpdateStatus function
- [ ] Load project to get `workflow_mode`
- [ ] Validate requested transition against `workflow.IsValidTransition(currentStatus, newStatus)`
- [ ] Filter allowed statuses by workflow mode
- [ ] Return `400 Bad Request` with message listing valid transitions if invalid
- [ ] Keep existing cooldown/snooze/resolved_in_release logic for `resolved` status

### 1.4 Update ingest behavior for new statuses
- [ ] Read `internal/ingest/handler.go` processEvent function (the reopen logic around line 191)
- [ ] Add `acknowledged`, `in_progress`, `in_review`, `escalated` to the status check:
  - These statuses do NOT trigger reopen logic — just increment event_count and update last_seen
  - Only `resolved` checks cooldown/release, only `snoozed` checks snooze expiry
- [ ] Ensure `ignored` issues increment silently (no alerts triggered)
- [ ] Write tests: new event on each status produces correct behavior

### 1.5 New API endpoint: valid transitions
- [ ] Add `GET /api/v1/projects/{project_id}/issues/{issue_id}/transitions` that returns the list of valid next statuses for the current issue, respecting workflow_mode
- [ ] Frontend will use this to render only valid action buttons

### 1.6 Update status filter and counts
- [ ] Update `GetIssueCountsByStatus` query — new statuses will appear automatically since it's `GROUP BY status`
- [ ] Update `ListIssuesByProject` — the `status` filter param must accept new values (already works since it's `WHERE status = $2`)
- [ ] Verify the level filter sidebar in the frontend handles unknown statuses gracefully

### 1.7 Update frontend status badges and colors
- [ ] Add new status entries to `STATUS_COLORS` in `IssueList.tsx`:
  - `acknowledged`: `'warning'` (amber)
  - `in_progress`: `'secondary'` (blue tint)
  - `in_review`: `'outline'` (neutral)
  - `escalated`: `'secondary'` (distinct color, e.g., purple tint)
- [ ] Add same to `IssueDetail.tsx` status display
- [ ] Update status action buttons in `IssueDetail.tsx` to use the transitions endpoint instead of hardcoded buttons

### 1.8 Update frontend filter sidebar for new statuses
- [ ] Read `IssueList.tsx` filter section (errorsFilters, warningsFilters, infoFilters)
- [ ] Add filter options for: `acknowledged`, `in_progress`, `in_review`, `escalated`
- [ ] Group logically: Triage (open, reopened), Active (acknowledged, in_progress, in_review), Closed (resolved, ignored, snoozed), External (escalated)
- [ ] Show counts for each new status
- [ ] Only show extended statuses when project `workflow_mode === 'extended'`

### 1.9 Update MCP server
- [ ] Read `mcp/src/index.ts` — find `update_issue_status` tool
- [ ] Accept new status values
- [ ] Validate transitions (call API, which enforces the state machine)
- [ ] Update tool description to list all valid statuses

### 1.10 Project settings: workflow mode toggle
- [ ] Add `workflow_mode` to `ProjectSettings.tsx` general section
- [ ] Radio or toggle: Simple / Extended
- [ ] Save via existing `updateProject` API
- [ ] Show explanation: "Simple: open/resolved/ignored. Extended: adds acknowledged, in progress, in review, escalated."

---

## Block 2: Activity Log

### 2.1 Migration: issue_activities table
- [ ] Create migration `000029_issue_activities.up.sql`:
  ```sql
  CREATE TABLE issue_activities (
      id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
      issue_id UUID NOT NULL REFERENCES issues(id) ON DELETE CASCADE,
      user_id UUID REFERENCES users(id) ON DELETE SET NULL,
      action TEXT NOT NULL,
      old_value TEXT,
      new_value TEXT,
      metadata JSONB,
      created_at TIMESTAMPTZ NOT NULL DEFAULT now()
  );
  CREATE INDEX idx_issue_activities_issue ON issue_activities(issue_id, created_at);
  ```
- [ ] Create matching `down.sql`

### 2.2 Activity recording service
- [ ] Create `internal/activity/service.go`:
  - `func Record(ctx, queries, issueID, userID *uuid.UUID, action, oldValue, newValue string, metadata json.RawMessage)`
  - `userID` is nil for system actions
- [ ] Create SQL queries in `internal/database/queries/activities.sql`:
  - `InsertActivity` — insert a new activity entry
  - `ListActivitiesByIssue` — paginated, ordered by `created_at DESC`

### 2.3 Wire activity recording into existing handlers
- [ ] **Status changes** (`internal/issue/handler.go` UpdateStatus): record `status_changed` with old/new status
- [ ] **Assignment** (`internal/issue/handler.go` Assign): record `assigned` or `unassigned`
- [ ] **Comments** (`internal/comment/handler.go`): record `commented` on create, `comment_edited` on update, `comment_deleted` on delete
- [ ] **Tags** (`internal/tags/handler.go`): record `tagged` on add, `untagged` on remove
- [ ] **Auto-tags** (`internal/tags/auto.go`): record `auto_tagged` with user_id=NULL
- [ ] **Follow/unfollow** (`internal/issue/handler.go`): record `followed`/`unfollowed`
- [ ] **Merge** (`internal/issue/handler.go` Merge): record `merged` on both source and target issues
- [ ] **Jira link** (`internal/jira/handler.go` CreateTicket + auto.go): record `jira_linked`
- [ ] **GitHub link** (`internal/github/handler.go` CreateIssueHandler + auto.go): record `github_linked`

### 2.4 Wire activity recording into ingest (system actions)
- [ ] **Auto-reopen** (`internal/ingest/handler.go` around line 198): record `auto_reopened` with user_id=NULL
- [ ] **Auto-unsnooze** (`internal/issue/cooldown.go`): record `auto_unsnoozed`
- [ ] **Priority change** (`internal/priority/evaluator.go`): record `priority_changed` when value actually changes
- [ ] **First seen** (`internal/ingest/handler.go`): record `first_seen` when `isNew == true`

### 2.5 Activity list API endpoint
- [ ] Add `GET /api/v1/projects/{project_id}/issues/{issue_id}/activities?limit=50&offset=0`
- [ ] Returns activities joined with user name/email/avatar for display
- [ ] Route: add to `router.go` under the issue routes

### 2.6 Frontend: Unified timeline
- [ ] Add `listActivities` to `api.ts` with `Activity` interface
- [ ] In `IssueDetail.tsx`: fetch activities alongside events and comments
- [ ] Create `ActivityTimeline` component that merges:
  - Activities (status changes, assignments, tags, links, system events)
  - Comments (rendered inline with edit/delete)
  - Sorted by `created_at` ascending (oldest first, like a chat)
- [ ] Each entry: avatar (user or system icon), actor name, action description, relative time
- [ ] Replace the current separate comments section with the unified timeline
- [ ] Comment input stays at the bottom of the timeline

---

## Block 3: Management UX and Board

### 3.1 Management panel on issue detail
- [ ] Redesign the action area in `IssueDetail.tsx`:
  - **Status selector**: dropdown showing valid transitions (from `/transitions` endpoint)
  - **Assignee picker**: searchable dropdown of team members
  - **Priority tier**: P1/P2/P3/P4 selector (only in extended mode)
  - **Due date**: date picker (optional)
- [ ] Resolution dialog: when selecting `resolved`, show a dialog with:
  - Resolution type (`fixed`, `wontfix`, `duplicate`, `cannot_reproduce`, `by_design`)
  - Fix reference (free text: commit SHA, PR URL, release)
  - Resolution notes (free text)
- [ ] All actions go through the same API endpoints that record activities

### 3.2 "Continue in Jira/GitHub" escalation flow
- [ ] Add migration: `resolution_type TEXT`, `resolution_notes TEXT`, `fix_reference TEXT` to issues table (can be in same migration as 3.1 fields or separate)
- [ ] "Continue in Jira/GitHub" button in management panel:
  - Only visible when integration is configured
  - If no external ticket exists: opens confirmation dialog with preview → creates ticket → transitions to `escalated`
  - If external ticket already exists (from auto-rules): just transitions to `escalated`
  - If both Jira AND GitHub configured: show choice dialog
- [ ] Escalated issue banner: persistent bar at top of issue detail showing external ticket link
- [ ] Activity log records `escalated` with metadata `{system, key, url}`
- [ ] "Pull back to GoSnag" action on escalated issues → transitions to `in_progress`

### 3.3 Priority override
- [ ] Add migration: `manual_priority INT` to issues table (nullable)
- [ ] Update `issueJSON` to include `manual_priority` and computed `effective_priority`
- [ ] `effective_priority = COALESCE(manual_priority, priority)` — can be done in SQL or Go
- [ ] API: `PUT /issues/{id}/priority` with `{manual_priority: 90}` or `{manual_priority: null}` to clear
- [ ] Activity log: `priority_overridden` / `priority_override_cleared`
- [ ] Frontend: P1/P2/P3/P4 selector in management panel, pin icon when manually set
- [ ] Update sort options to use effective_priority

### 3.4 Due dates
- [ ] Add `due_date TIMESTAMPTZ` to issues table (in migration with other Block 3 fields)
- [ ] API: set via `PUT /issues/{id}` or management panel
- [ ] Frontend: date picker in management panel
- [ ] Visual indicators: amber badge (< 24h), red badge (overdue)
- [ ] Show on issue list rows and board cards

### 3.5 List quick actions
- [ ] Add context menu (right-click or `...` button) on each issue row in `IssueList.tsx`
- [ ] Actions: Change status (valid transitions), Assign, Set priority tier
- [ ] Uses same API endpoints and validation as detail page
- [ ] Activity log entries are identical regardless of entry point

### 3.6 Kanban board view
- [ ] Create `web/src/pages/IssueBoard.tsx`
- [ ] Route: `/projects/{id}/board`
- [ ] Columns mapped to statuses (extended mode: Triage/Acknowledged/In Progress/In Review/Done; simple mode: Open/Resolved)
- [ ] `escalated` column: distinct, non-draggable, shows external ticket badge
- [ ] `ignored`/`snoozed`: hidden by default, toggle to show
- [ ] Drag-and-drop between columns (use existing dnd-kit dependency):
  - Validate transition before applying
  - Show error toast if invalid transition
  - Record activity on success
- [ ] Cards show: title (truncated), level badge, priority tier, assignee avatar, event count, due date indicator
- [ ] Same filters as list view (level, search, tag, assignee, release, priority)
- [ ] Toggle between List/Board in the project navigation (tabs or toggle button)

---

## Block 4: Collaboration

### 4.1 Follower notifications on changes
- [ ] Extend `internal/alert/service.go` (or create `internal/notification/service.go`) to notify followers on:
  - Status changes
  - New comments
  - Assignment changes
- [ ] Use existing SMTP and Slack channels
- [ ] Add per-user notification preference: `all` / `mentions_only` / `none`
  - Migration: `ALTER TABLE users ADD COLUMN notification_preference TEXT NOT NULL DEFAULT 'all'`
  - Settings UI in user profile

### 4.2 @Mentions in comments
- [ ] Parse `@username` in comment body on save
- [ ] Resolve usernames to user IDs
- [ ] Send notification to mentioned users (regardless of follow status)
- [ ] Frontend: `@` trigger in comment input with user search dropdown
- [ ] Render mentions as styled spans in comment display

### 4.3 Markdown comments
- [ ] Add markdown renderer in frontend (e.g., `react-markdown` or `marked`)
- [ ] Sanitize output (prevent XSS)
- [ ] Render comments as markdown in the timeline
- [ ] Add preview toggle in comment editor
- [ ] **Image uploads (separate sub-task):**
  - Storage backend (S3 or local disk)
  - Upload endpoint
  - Paste-to-upload in comment editor
  - Image display in rendered markdown

---

## Block 5: Planning and Reporting

### 5.1 Resolution tracking
- [ ] Resolution type dropdown when resolving (from Block 3.1 — may already be done)
- [ ] Fix reference field (commit SHA, PR URL, release)
- [ ] Verification: after resolving, show "Still seeing events?" indicator if events arrive in next 24h
- [ ] Resolution data displayed in issue detail and timeline

### 5.2 SLA rules
- [ ] New table: `sla_rules (id, project_id, priority_tier, ack_minutes, resolve_minutes, enabled)`
- [ ] SLA evaluation: compare current time vs first_seen (or status change timestamps from activity log)
- [ ] Visual: overdue badge on issues that breach SLA
- [ ] Alerts: email/Slack when SLA is about to breach or has breached

### 5.3 Reporting dashboard
- [ ] New page: `/projects/{id}/reports`
- [ ] **MTTA** (Mean Time to Acknowledge): `open → acknowledged` from activity log
- [ ] **MTTR** (Mean Time to Resolve): `open → resolved` from activity log
- [ ] **Resolution rate**: issues resolved vs opened per week (chart)
- [ ] **SLA compliance**: percentage of issues within SLA targets
- [ ] **Throughput by status**: cumulative flow diagram
- [ ] **Assignee workload**: open issues per person, average resolution time
- [ ] All metrics query the `issue_activities` table — this is why activity log must be source of truth

---

## Execution Order

```
Phase 1 (MVP):
  Block 1 (1.1 → 1.4 → 1.3 → 1.5 → 1.6 → 1.7 → 1.8 → 1.9 → 1.10)
  Block 2 (2.1 → 2.2 → 2.3 → 2.4 → 2.5 → 2.6)
  Block 3.1 (Management panel)
  Block 3.2 (Escalation flow)

Phase 2:
  Block 3.3 (Priority override)
  Block 3.4 (Due dates)
  Block 3.5 (List quick actions)
  Block 3.6 (Kanban board)

Phase 3:
  Block 4.1 (Follower notifications)
  Block 5.1 (Resolution tracking)

Phase 4:
  Block 4.2 (@Mentions)
  Block 4.3 (Markdown comments)
  Block 5.2 (SLA rules)
  Block 5.3 (Reporting dashboard)
```

Phase 1 delivers: **complete incident lifecycle with audit trail and escalation to external trackers**.
Phase 2 delivers: **visual management (board, quick actions, priority, due dates)**.
Phase 3 delivers: **team awareness (notifications, resolution quality)**.
Phase 4 delivers: **collaboration polish and metrics**.
