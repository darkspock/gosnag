-- name: ListJiraRules :many
SELECT * FROM jira_rules WHERE project_id = $1 ORDER BY created_at;

-- name: GetJiraRule :one
SELECT * FROM jira_rules WHERE id = $1;

-- name: CreateJiraRule :one
INSERT INTO jira_rules (project_id, name, enabled, level_filter, min_events, min_users, title_pattern)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: UpdateJiraRule :one
UPDATE jira_rules
SET name = $3, enabled = $4, level_filter = $5, min_events = $6, min_users = $7, title_pattern = $8, updated_at = now()
WHERE id = $1 AND project_id = $2
RETURNING *;

-- name: DeleteJiraRule :exec
DELETE FROM jira_rules WHERE id = $1 AND project_id = $2;

-- name: ListEnabledJiraRules :many
SELECT * FROM jira_rules WHERE project_id = $1 AND enabled = true;

-- name: UpdateIssueJiraTicket :exec
UPDATE issues SET jira_ticket_key = $2, jira_ticket_url = $3, updated_at = now() WHERE id = $1;
