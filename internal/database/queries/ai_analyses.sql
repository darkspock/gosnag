-- name: CreateAIAnalysis :one
INSERT INTO ai_analyses (issue_id, project_id, summary, evidence, suggested_fix, model, version)
VALUES ($1, $2, $3, $4, $5, $6,
    COALESCE((SELECT MAX(version) FROM ai_analyses WHERE issue_id = $1), 0) + 1
)
RETURNING *;

-- name: GetLatestAIAnalysis :one
SELECT * FROM ai_analyses
WHERE issue_id = $1
ORDER BY created_at DESC
LIMIT 1;

-- name: ListAIAnalyses :many
SELECT * FROM ai_analyses
WHERE issue_id = $1
ORDER BY created_at DESC;
