-- name: ListPriorityRules :many
SELECT * FROM priority_rules WHERE project_id = $1 ORDER BY position, created_at;

-- name: ListEnabledPriorityRules :many
SELECT * FROM priority_rules WHERE project_id = $1 AND enabled = true ORDER BY position;

-- name: CreatePriorityRule :one
INSERT INTO priority_rules (project_id, name, rule_type, pattern, operator, threshold, points, enabled, position, conditions)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, COALESCE((SELECT max(position) + 1 FROM priority_rules WHERE project_id = $1), 0), $9)
RETURNING *;

-- name: UpdatePriorityRule :one
UPDATE priority_rules
SET name = $3, rule_type = $4, pattern = $5, operator = $6, threshold = $7, points = $8, enabled = $9, conditions = $10, updated_at = now()
WHERE id = $1 AND project_id = $2
RETURNING *;

-- name: DeletePriorityRule :exec
DELETE FROM priority_rules WHERE id = $1 AND project_id = $2;

-- name: UpdateIssuePriority :exec
UPDATE issues SET priority = $2, updated_at = now() WHERE id = $1;

-- name: GetIssueVelocity1h :one
SELECT COUNT(*)::int as count FROM events
WHERE issue_id = $1 AND timestamp >= now() - interval '1 hour';

-- name: GetIssueVelocity24h :one
SELECT COUNT(*)::int as count FROM events
WHERE issue_id = $1 AND timestamp >= now() - interval '24 hours';

-- name: ListIssueIDsByProject :many
SELECT id FROM issues WHERE project_id = $1;

-- name: GetAIPriorityEvaluation :one
SELECT * FROM ai_priority_evaluations WHERE issue_id = $1 AND rule_id = $2;

-- name: UpsertAIPriorityEvaluation :one
INSERT INTO ai_priority_evaluations (issue_id, rule_id, status, points, reason, retries)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (issue_id, rule_id) DO UPDATE
SET status = $3, points = $4, reason = $5, retries = $6, updated_at = now()
RETURNING *;

-- name: DeleteAIPriorityEvaluationsByProject :exec
DELETE FROM ai_priority_evaluations
WHERE rule_id IN (SELECT id FROM priority_rules WHERE project_id = $1);

-- name: ListRecentIssuesForContext :many
SELECT id, title, level, platform, event_count, culprit, first_seen, last_seen
FROM issues WHERE project_id = $1 AND status != 'resolved'
ORDER BY last_seen DESC LIMIT 20;
