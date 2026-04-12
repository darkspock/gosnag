-- name: CreateMergeSuggestion :one
INSERT INTO ai_merge_suggestions (issue_id, target_issue_id, project_id, confidence, reason)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetPendingMergeSuggestion :one
SELECT s.*, i.title AS target_issue_title
FROM ai_merge_suggestions s
JOIN issues i ON i.id = s.target_issue_id
WHERE s.issue_id = $1
  AND s.status = 'pending'
LIMIT 1;

-- name: AcceptMergeSuggestion :exec
UPDATE ai_merge_suggestions
SET status = 'accepted'
WHERE id = $1;

-- name: DismissMergeSuggestion :exec
UPDATE ai_merge_suggestions
SET status = 'dismissed'
WHERE id = $1;

-- name: ListIssuesWithPendingSuggestions :many
SELECT DISTINCT s.issue_id FROM ai_merge_suggestions s WHERE s.project_id = $1 AND s.status = 'pending'
UNION
SELECT DISTINCT s.target_issue_id FROM ai_merge_suggestions s WHERE s.project_id = $1 AND s.status = 'pending';

-- name: GetMergeSuggestionByID :one
SELECT * FROM ai_merge_suggestions WHERE id = $1;

-- name: DismissSuggestionsForIssue :exec
UPDATE ai_merge_suggestions
SET status = 'dismissed'
WHERE (issue_id = $1 OR target_issue_id = $1)
  AND status = 'pending';
