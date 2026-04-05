-- name: ListFavorites :many
SELECT project_id FROM project_favorites WHERE user_id = $1;

-- name: AddFavorite :exec
INSERT INTO project_favorites (user_id, project_id) VALUES ($1, $2) ON CONFLICT DO NOTHING;

-- name: RemoveFavorite :exec
DELETE FROM project_favorites WHERE user_id = $1 AND project_id = $2;
