-- name: CreateProject :one
INSERT INTO projects (name, slug, default_cooldown_minutes)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetProject :one
SELECT * FROM projects WHERE id = $1;

-- name: GetProjectByNumericID :one
SELECT * FROM projects WHERE numeric_id = $1;

-- name: GetProjectBySlug :one
SELECT * FROM projects WHERE slug = $1;

-- name: ListProjects :many
SELECT * FROM projects ORDER BY position, created_at DESC;

-- name: UpdateProject :one
UPDATE projects
SET name = $2, slug = $3, default_cooldown_minutes = $4, warning_as_error = $5,
    jira_base_url = $6, jira_email = $7, jira_api_token = $8, jira_project_key = $9, jira_issue_type = $10,
    max_events_per_issue = $11,
    icon = $12, color = $13,
    issue_display_mode = $14,
    info_grouping_mode = $15,
    max_info_issues = $16,
    github_token = $17, github_owner = $18, github_repo = $19, github_labels = $20,
    workflow_mode = $21,
    repo_provider = $22, repo_owner = $23, repo_name = $24,
    repo_default_branch = $25, repo_token = $26, repo_path_strip = $27,
    ai_enabled = $28, ai_model = $29, ai_merge_suggestions = $30, ai_auto_merge = $31,
    ai_anomaly_detection = $32, ai_ticket_description = $33, ai_root_cause = $34, ai_triage = $35,
    stacktrace_rules = $36,
    analysis_db_enabled = $37,
    analysis_db_driver = $38,
    analysis_db_dsn = $39,
    analysis_db_name = $40,
    analysis_db_schema = $41,
    analysis_db_notes = $42,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: UpdateProjectPosition :exec
UPDATE projects SET position = $2 WHERE id = $1;

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
  count(*) FILTER (WHERE status IN ('open', 'reopened') AND level IN ('error', 'fatal'))::int AS open_issues,
  max(last_seen) AS latest_event
FROM issues
GROUP BY project_id;

-- name: GetProjectEventTrend :many
SELECT project_id,
       date_trunc('day', timestamp)::timestamptz as bucket,
       COUNT(*)::int as count
FROM events
WHERE timestamp >= now() - interval '14 days'
  AND level IN ('error', 'fatal')
GROUP BY project_id, bucket
ORDER BY project_id, bucket;

-- name: ListAIEnabledProjects :many
SELECT * FROM projects WHERE ai_enabled = true AND ai_merge_suggestions = true;

-- name: GetProjectLatestRelease :many
SELECT DISTINCT ON (project_id) project_id, release
FROM events
WHERE release != ''
  AND level IN ('error', 'fatal')
ORDER BY project_id, timestamp DESC;

-- name: GetProjectWeeklyErrors :many
SELECT project_id,
       COUNT(*) FILTER (WHERE timestamp >= now() - interval '7 days')::int as this_week,
       COUNT(*) FILTER (WHERE timestamp >= now() - interval '14 days' AND timestamp < now() - interval '7 days')::int as last_week
FROM events
WHERE timestamp >= now() - interval '14 days'
  AND level IN ('error', 'fatal')
GROUP BY project_id;
