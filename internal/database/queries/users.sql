-- name: UpsertUserByGoogle :one
INSERT INTO users (email, name, google_id, avatar_url)
VALUES ($1, $2, $3, $4)
ON CONFLICT (email) DO UPDATE
SET name = EXCLUDED.name, google_id = EXCLUDED.google_id, avatar_url = EXCLUDED.avatar_url, updated_at = now()
RETURNING *;

-- name: GetUser :one
SELECT * FROM users WHERE id = $1;

-- name: GetUserByEmail :one
SELECT * FROM users WHERE email = $1;

-- name: ListUsers :many
SELECT * FROM users ORDER BY created_at;

-- name: UpdateUserRole :one
UPDATE users SET role = $2, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: UpdateUserStatus :one
UPDATE users SET status = $2, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: ActivateUser :one
UPDATE users SET status = 'active', name = $2, google_id = $3, avatar_url = $4, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: CreateInvitedUser :one
INSERT INTO users (email, role, status)
VALUES ($1, $2, 'invited')
RETURNING *;

-- name: CountUsers :one
SELECT count(*) FROM users;
