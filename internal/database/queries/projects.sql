-- name: CreateProject :one
INSERT INTO projects (name, slug, default_cooldown_minutes)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetProject :one
SELECT * FROM projects WHERE id = $1;

-- name: GetProjectBySlug :one
SELECT * FROM projects WHERE slug = $1;

-- name: ListProjects :many
SELECT * FROM projects ORDER BY created_at DESC;

-- name: UpdateProject :one
UPDATE projects
SET name = $2, slug = $3, default_cooldown_minutes = $4, warning_as_error = $5,
    jira_base_url = $6, jira_email = $7, jira_api_token = $8, jira_project_key = $9, jira_issue_type = $10,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: DeleteProject :exec
DELETE FROM projects WHERE id = $1;

-- name: CreateProjectKey :one
INSERT INTO project_keys (project_id, public_key, secret_key, label)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetProjectKeyByPublic :one
SELECT * FROM project_keys WHERE public_key = $1;

-- name: ListProjectKeys :many
SELECT * FROM project_keys WHERE project_id = $1 ORDER BY created_at;

-- name: GetProjectStats :many
SELECT
  project_id,
  count(*)::int AS total_issues,
  count(*) FILTER (WHERE status IN ('open', 'reopened'))::int AS open_issues,
  max(last_seen) AS latest_event
FROM issues
GROUP BY project_id;

-- name: GetProjectEventTrend :many
SELECT project_id,
       date_trunc('day', timestamp)::timestamptz as bucket,
       COUNT(*)::int as count
FROM events
WHERE timestamp >= now() - interval '14 days'
GROUP BY project_id, bucket
ORDER BY project_id, bucket;

-- name: GetProjectLatestRelease :many
SELECT DISTINCT ON (project_id) project_id, release
FROM events
WHERE release != ''
ORDER BY project_id, timestamp DESC;

-- name: GetProjectWeeklyErrors :many
SELECT project_id,
       COUNT(*) FILTER (WHERE timestamp >= now() - interval '7 days')::int as this_week,
       COUNT(*) FILTER (WHERE timestamp >= now() - interval '14 days' AND timestamp < now() - interval '7 days')::int as last_week
FROM events
WHERE timestamp >= now() - interval '14 days'
  AND level IN ('error', 'fatal')
GROUP BY project_id;
