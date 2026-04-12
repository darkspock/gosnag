-- name: UpsertIssue :one
INSERT INTO issues (project_id, title, fingerprint, level, platform, first_seen, last_seen, event_count, culprit, first_release)
VALUES ($1, $2, $3, $4, $5, $6, $6, 1, $7, $8)
ON CONFLICT (project_id, fingerprint) DO UPDATE
SET last_seen = $6,
    event_count = issues.event_count + 1,
    title = EXCLUDED.title,
    level = EXCLUDED.level,
    platform = EXCLUDED.platform,
    culprit = CASE WHEN EXCLUDED.culprit != '' THEN EXCLUDED.culprit ELSE issues.culprit END,
    updated_at = now()
RETURNING *;

-- name: GetIssue :one
SELECT * FROM issues WHERE id = $1;

-- Level filter macro (repeated in each query):
--   '' = all, 'errors' = error+fatal, 'errors_w' = error+fatal+warning,
--   'informational' = warning+info+debug, 'info_only' = info+debug,
--   or exact level name

-- name: ListIssuesByProject :many
SELECT i.*, EXISTS(SELECT 1 FROM issue_follows f WHERE f.issue_id = i.id AND f.user_id = sqlc.narg('follower_id')::uuid) AS followed
FROM issues i
WHERE i.project_id = $1
  AND ($2::text = '' OR i.status = $2::text)
  AND (NOT $5::bool OR i.first_seen >= CURRENT_DATE)
  AND (NOT $6::bool OR i.assigned_to IS NOT NULL)
  AND (sqlc.narg('assigned_to_user')::uuid IS NULL OR i.assigned_to = sqlc.narg('assigned_to_user'))
  AND (sqlc.arg('level_filter')::text = '' OR i.level = sqlc.arg('level_filter')::text
    OR (sqlc.arg('level_filter')::text = 'errors' AND i.level IN ('error', 'fatal'))
    OR (sqlc.arg('level_filter')::text = 'errors_w' AND i.level IN ('error', 'fatal', 'warning'))
    OR (sqlc.arg('level_filter')::text = 'informational' AND i.level IN ('warning', 'info', 'debug'))
    OR (sqlc.arg('level_filter')::text = 'info_only' AND i.level IN ('info', 'debug')))
  AND (sqlc.arg('search')::text = '' OR i.title ILIKE '%' || sqlc.arg('search')::text || '%')
  AND (sqlc.arg('release_filter')::text = '' OR i.first_release = sqlc.arg('release_filter')::text)
ORDER BY followed DESC, i.last_seen DESC
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
  AND (sqlc.arg('search')::text = '' OR title ILIKE '%' || sqlc.arg('search')::text || '%')
  AND (sqlc.arg('release_filter')::text = '' OR first_release = sqlc.arg('release_filter')::text);

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

-- name: ListIssuesByIDs :many
SELECT * FROM issues WHERE id = ANY(@ids::uuid[]) ORDER BY last_seen DESC;

-- name: GetIssueByFingerprint :one
SELECT * FROM issues WHERE project_id = $1 AND fingerprint = $2;

-- name: FollowIssue :exec
INSERT INTO issue_follows (user_id, issue_id) VALUES ($1, $2) ON CONFLICT DO NOTHING;

-- name: UnfollowIssue :exec
DELETE FROM issue_follows WHERE user_id = $1 AND issue_id = $2;

-- name: ListFollowedIssueIDs :many
SELECT issue_id FROM issue_follows WHERE user_id = $1;

-- name: IsFollowingIssue :one
SELECT EXISTS(SELECT 1 FROM issue_follows WHERE user_id = $1 AND issue_id = $2)::bool AS following;

-- name: ListIssueFollowers :many
SELECT u.id, u.name, u.email FROM issue_follows f JOIN users u ON u.id = f.user_id WHERE f.issue_id = $1 ORDER BY f.created_at;

-- name: ListOpenN1Issues :many
SELECT * FROM issues
WHERE project_id = $1
  AND status = 'open'
  AND fingerprint LIKE 'n1:%';

-- name: ListIssueReleases :many
SELECT DISTINCT first_release FROM issues
WHERE project_id = $1 AND first_release != ''
ORDER BY first_release;

-- name: ListNewIssuesSince :many
SELECT * FROM issues
WHERE project_id = $1
  AND first_seen >= $2
  AND status IN ('open', 'reopened')
ORDER BY first_seen DESC
LIMIT 50;

-- name: ListRecentOpenIssues :many
SELECT * FROM issues
WHERE project_id = $1
  AND status IN ('open', 'reopened')
  AND id != $2
ORDER BY last_seen DESC
LIMIT 10;

-- name: ListIssuesCreatedSince :many
SELECT * FROM issues
WHERE project_id = $1
  AND first_seen >= $2
ORDER BY first_seen DESC;

-- name: ListIssuesReopenedSince :many
SELECT * FROM issues
WHERE project_id = $1
  AND status = 'reopened'
  AND last_seen >= $2
ORDER BY last_seen DESC;

-- name: ListTopIssuesByEvents :many
SELECT * FROM issues
WHERE project_id = $1
  AND status IN ('open', 'reopened')
ORDER BY event_count DESC
LIMIT $2;
