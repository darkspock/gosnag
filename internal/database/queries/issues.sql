-- name: UpsertIssue :one
INSERT INTO issues (project_id, title, fingerprint, level, platform, first_seen, last_seen, event_count)
VALUES ($1, $2, $3, $4, $5, $6, $6, 1)
ON CONFLICT (project_id, fingerprint) DO UPDATE
SET last_seen = $6,
    event_count = issues.event_count + 1,
    title = EXCLUDED.title,
    level = EXCLUDED.level,
    platform = EXCLUDED.platform,
    updated_at = now()
RETURNING *;

-- name: GetIssue :one
SELECT * FROM issues WHERE id = $1;

-- Level filter macro (repeated in each query):
--   '' = all, 'errors' = error+fatal, 'errors_w' = error+fatal+warning,
--   'informational' = warning+info+debug, 'info_only' = info+debug,
--   or exact level name

-- name: ListIssuesByProject :many
SELECT * FROM issues
WHERE project_id = $1
  AND ($2::text = '' OR status = $2::text)
  AND (NOT $5::bool OR first_seen >= CURRENT_DATE)
  AND (NOT $6::bool OR assigned_to IS NOT NULL)
  AND (sqlc.narg('assigned_to_user')::uuid IS NULL OR assigned_to = sqlc.narg('assigned_to_user'))
  AND (sqlc.arg('level_filter')::text = '' OR level = sqlc.arg('level_filter')::text
    OR (sqlc.arg('level_filter')::text = 'errors' AND level IN ('error', 'fatal'))
    OR (sqlc.arg('level_filter')::text = 'errors_w' AND level IN ('error', 'fatal', 'warning'))
    OR (sqlc.arg('level_filter')::text = 'informational' AND level IN ('warning', 'info', 'debug'))
    OR (sqlc.arg('level_filter')::text = 'info_only' AND level IN ('info', 'debug')))
  AND (sqlc.arg('search')::text = '' OR title ILIKE '%' || sqlc.arg('search')::text || '%')
ORDER BY last_seen DESC
LIMIT $3 OFFSET $4;

-- name: CountIssuesByProject :one
SELECT count(*) FROM issues
WHERE project_id = $1
  AND ($2::text = '' OR status = $2::text)
  AND (NOT $3::bool OR first_seen >= CURRENT_DATE)
  AND (NOT $4::bool OR assigned_to IS NOT NULL)
  AND (sqlc.narg('assigned_to_user')::uuid IS NULL OR assigned_to = sqlc.narg('assigned_to_user'))
  AND (sqlc.arg('level_filter')::text = '' OR level = sqlc.arg('level_filter')::text
    OR (sqlc.arg('level_filter')::text = 'errors' AND level IN ('error', 'fatal'))
    OR (sqlc.arg('level_filter')::text = 'errors_w' AND level IN ('error', 'fatal', 'warning'))
    OR (sqlc.arg('level_filter')::text = 'informational' AND level IN ('warning', 'info', 'debug'))
    OR (sqlc.arg('level_filter')::text = 'info_only' AND level IN ('info', 'debug')))
  AND (sqlc.arg('search')::text = '' OR title ILIKE '%' || sqlc.arg('search')::text || '%');

-- name: UpdateIssueStatus :one
UPDATE issues
SET status = $2,
    resolved_at = $3,
    cooldown_until = $4,
    resolved_in_release = $5,
    snooze_until = $6,
    snooze_event_threshold = $7,
    snooze_events_at_start = $8,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: AssignIssue :one
UPDATE issues
SET assigned_to = $2, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: GetExpiredCooldownIssues :many
SELECT * FROM issues
WHERE status = 'resolved'
  AND cooldown_until IS NOT NULL
  AND cooldown_until < now();

-- name: GetExpiredSnoozeIssues :many
SELECT * FROM issues
WHERE status = 'snoozed'
  AND snooze_until IS NOT NULL
  AND snooze_until < now();

-- name: GetIssueCountsByStatus :many
SELECT status, count(*)::int as count
FROM issues
WHERE project_id = $1
  AND ($2::text = '' OR level = $2::text
    OR ($2::text = 'errors' AND level IN ('error', 'fatal'))
    OR ($2::text = 'errors_w' AND level IN ('error', 'fatal', 'warning'))
    OR ($2::text = 'informational' AND level IN ('warning', 'info', 'debug'))
    OR ($2::text = 'info_only' AND level IN ('info', 'debug')))
GROUP BY status;

-- name: CountIssuesToday :one
SELECT count(*)::int FROM issues
WHERE project_id = $1
  AND first_seen >= CURRENT_DATE
  AND ($2::text = '' OR level = $2::text
    OR ($2::text = 'errors' AND level IN ('error', 'fatal'))
    OR ($2::text = 'errors_w' AND level IN ('error', 'fatal', 'warning'))
    OR ($2::text = 'informational' AND level IN ('warning', 'info', 'debug'))
    OR ($2::text = 'info_only' AND level IN ('info', 'debug')));

-- name: CountIssuesAssignedToUser :one
SELECT count(*)::int FROM issues
WHERE project_id = $1
  AND assigned_to = $2
  AND ($3::text = '' OR level = $3::text
    OR ($3::text = 'errors' AND level IN ('error', 'fatal'))
    OR ($3::text = 'errors_w' AND level IN ('error', 'fatal', 'warning'))
    OR ($3::text = 'informational' AND level IN ('warning', 'info', 'debug'))
    OR ($3::text = 'info_only' AND level IN ('info', 'debug')));

-- name: DeleteIssues :execresult
DELETE FROM issues WHERE id = ANY(@ids::uuid[]) AND project_id = @project_id;

-- name: CountIssuesAssigned :one
SELECT count(*)::int FROM issues
WHERE project_id = $1
  AND assigned_to IS NOT NULL
  AND ($2::text = '' OR level = $2::text
    OR ($2::text = 'errors' AND level IN ('error', 'fatal'))
    OR ($2::text = 'errors_w' AND level IN ('error', 'fatal', 'warning'))
    OR ($2::text = 'informational' AND level IN ('warning', 'info', 'debug'))
    OR ($2::text = 'info_only' AND level IN ('info', 'debug')));
