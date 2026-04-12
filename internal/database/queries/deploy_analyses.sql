-- name: CreateDeployAnalysis :one
INSERT INTO deploy_analyses (deploy_id, project_id, severity, summary, details, likely_deploy_caused, recommended_action, new_issues_count, spiked_issues_count, reopened_issues_count)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING *;

-- name: GetDeployAnalysis :one
SELECT * FROM deploy_analyses WHERE deploy_id = $1;

-- name: GetLatestDeployAnalysisByProject :one
SELECT * FROM deploy_analyses WHERE project_id = $1 ORDER BY created_at DESC LIMIT 1;

-- name: ListDeployAnalysesByProject :many
SELECT * FROM deploy_analyses WHERE project_id = $1 ORDER BY created_at DESC LIMIT $2;

-- name: DeleteDeployAnalysis :exec
DELETE FROM deploy_analyses WHERE deploy_id = $1;
