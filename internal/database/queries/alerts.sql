-- name: CreateAlertConfig :one
INSERT INTO alert_configs (project_id, alert_type, config, enabled, level_filter, title_pattern, min_events, min_velocity_1h, exclude_pattern)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: ListAlertConfigs :many
SELECT * FROM alert_configs WHERE project_id = $1 ORDER BY created_at;

-- name: GetEnabledAlerts :many
SELECT * FROM alert_configs WHERE project_id = $1 AND enabled = true;

-- name: UpdateAlertConfig :one
UPDATE alert_configs
SET config = $3, enabled = $4, level_filter = $5, title_pattern = $6, min_events = $7, min_velocity_1h = $8, exclude_pattern = $9, updated_at = now()
WHERE id = $1 AND project_id = $2
RETURNING *;

-- name: DeleteAlertConfig :exec
DELETE FROM alert_configs WHERE id = $1 AND project_id = $2;
