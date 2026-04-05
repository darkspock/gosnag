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
SELECT issue_id FROM issue_tags WHERE key = $1 AND value = $2;

-- name: ListTagRules :many
SELECT * FROM tag_rules WHERE project_id = $1 ORDER BY created_at;

-- name: ListEnabledTagRules :many
SELECT * FROM tag_rules WHERE project_id = $1 AND enabled = true;

-- name: CreateTagRule :one
INSERT INTO tag_rules (project_id, name, pattern, tag_key, tag_value, enabled)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: UpdateTagRule :one
UPDATE tag_rules
SET name = $3, pattern = $4, tag_key = $5, tag_value = $6, enabled = $7, updated_at = now()
WHERE id = $1 AND project_id = $2
RETURNING *;

-- name: DeleteTagRule :exec
DELETE FROM tag_rules WHERE id = $1 AND project_id = $2;
