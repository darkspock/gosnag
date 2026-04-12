-- name: UpsertReleaseCommit :one
INSERT INTO release_commits (project_id, release_version, commit_sha, commit_url, committed_at)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (project_id, release_version) DO UPDATE
SET commit_sha = EXCLUDED.commit_sha, commit_url = EXCLUDED.commit_url, committed_at = EXCLUDED.committed_at
RETURNING *;

-- name: GetReleaseCommit :one
SELECT * FROM release_commits WHERE project_id = $1 AND release_version = $2;

-- name: ListReleaseCommits :many
SELECT * FROM release_commits WHERE project_id = $1 ORDER BY created_at DESC LIMIT $2;

-- name: GetPreviousRelease :one
SELECT * FROM release_commits rc
WHERE rc.project_id = $1 AND rc.created_at < (SELECT rc2.created_at FROM release_commits rc2 WHERE rc2.project_id = $1 AND rc2.release_version = $2)
ORDER BY rc.created_at DESC LIMIT 1;

-- name: CreateDeploy :one
INSERT INTO deploys (project_id, release_version, commit_sha, environment, url, deployed_at)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: ListDeploys :many
SELECT * FROM deploys WHERE project_id = $1 ORDER BY deployed_at DESC LIMIT $2;

-- name: GetLatestDeployForRelease :one
SELECT * FROM deploys WHERE project_id = $1 AND release_version = $2 ORDER BY deployed_at DESC LIMIT 1;

-- name: GetDeploy :one
SELECT * FROM deploys WHERE id = $1;

-- name: GetLatestDeploy :one
SELECT * FROM deploys WHERE project_id = $1 ORDER BY deployed_at DESC LIMIT 1;

-- name: ListRecentDeploys :many
SELECT * FROM deploys WHERE project_id = $1 ORDER BY deployed_at DESC LIMIT $2;
