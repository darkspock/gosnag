-- name: ListIssueTags :many
SELECT * FROM issue_tags WHERE issue_id = $1 ORDER BY key, value;

-- name: AddIssueTag :exec
INSERT INTO issue_tags (issue_id, key, value) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING;

-- name: RemoveIssueTag :exec
DELETE FROM issue_tags WHERE issue_id = $1 AND key = $2 AND value = $3;

-- name: ListTagsByIssueIDs :many
SELECT * FROM issue_tags WHERE issue_id = ANY($1::uuid[]) ORDER BY key, value;

-- name: ListDistinctTags :many
SELECT DISTINCT key, value FROM issue_tags
WHERE issue_id IN (SELECT id FROM issues WHERE project_id = $1)
ORDER BY key, value;

-- name: ListIssueIDsByTag :many
SELECT it.issue_id FROM issue_tags it
JOIN issues i ON i.id = it.issue_id
WHERE it.key = $1 AND it.value = $2 AND i.project_id = $3;

-- name: ListTagRules :many
SELECT * FROM tag_rules WHERE project_id = $1 ORDER BY created_at;

-- name: ListEnabledTagRules :many
SELECT * FROM tag_rules WHERE project_id = $1 AND enabled = true;

-- name: CreateTagRule :one
INSERT INTO tag_rules (project_id, name, pattern, tag_key, tag_value, enabled, conditions, rule_type, threshold)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: UpdateTagRule :one
UPDATE tag_rules
SET name = $3, pattern = $4, tag_key = $5, tag_value = $6, enabled = $7, conditions = $8, rule_type = $9, threshold = $10, updated_at = now()
WHERE id = $1 AND project_id = $2
RETURNING *;

-- name: DeleteTagRule :exec
DELETE FROM tag_rules WHERE id = $1 AND project_id = $2;

-- name: GetAITagEvaluation :one
SELECT * FROM ai_tag_evaluations WHERE issue_id = $1 AND rule_id = $2;

-- name: UpsertAITagEvaluation :one
INSERT INTO ai_tag_evaluations (issue_id, rule_id, status, tag_value, reason, retries)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (issue_id, rule_id) DO UPDATE
SET status = $3, tag_value = $4, reason = $5, retries = $6, updated_at = now()
RETURNING *;

-- name: DeleteAITagEvaluationsByProject :exec
DELETE FROM ai_tag_evaluations
WHERE rule_id IN (SELECT id FROM tag_rules WHERE project_id = $1);
