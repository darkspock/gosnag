-- name: CreateAPIToken :one
INSERT INTO api_tokens (project_id, token_hash, name, permission, expires_at, created_by, scope)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: ListAPITokensByProject :many
SELECT * FROM api_tokens WHERE project_id = $1 ORDER BY created_at DESC;

-- name: ListGlobalTokens :many
SELECT * FROM api_tokens WHERE scope = 'global' AND created_by = $1 ORDER BY created_at DESC;

-- name: GetAPITokenByHash :one
SELECT * FROM api_tokens WHERE token_hash = $1;

-- name: DeleteAPIToken :exec
DELETE FROM api_tokens WHERE id = $1 AND project_id = $2;

-- name: DeleteGlobalToken :exec
DELETE FROM api_tokens WHERE id = $1 AND scope = 'global' AND created_by = $2;

-- name: UpdateAPITokenLastUsed :exec
UPDATE api_tokens SET last_used_at = now() WHERE id = $1;
