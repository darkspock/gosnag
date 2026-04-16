-- name: ListProjectGroups :many
SELECT id, name, position, created_at, default_slack_webhook_url
FROM project_groups
ORDER BY position, name;

-- name: GetProjectGroup :one
SELECT id, name, position, created_at, default_slack_webhook_url
FROM project_groups
WHERE id = $1;

-- name: CreateProjectGroup :one
INSERT INTO project_groups (name, position, default_slack_webhook_url)
VALUES ($1, COALESCE((SELECT max(position) + 1 FROM project_groups), 0), $2)
RETURNING id, name, position, created_at, default_slack_webhook_url;

-- name: UpdateProjectGroup :one
UPDATE project_groups
SET name = $2,
    default_slack_webhook_url = $3
WHERE id = $1
RETURNING id, name, position, created_at, default_slack_webhook_url;

-- name: DeleteProjectGroup :exec
DELETE FROM project_groups WHERE id = $1;

-- name: SetProjectGroup :exec
UPDATE projects SET group_id = $2, updated_at = now() WHERE id = $1;
