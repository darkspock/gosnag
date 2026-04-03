-- name: CreateAlertConfig :one
INSERT INTO alert_configs (project_id, alert_type, config, enabled, level_filter, title_pattern)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: ListAlertConfigs :many
SELECT * FROM alert_configs WHERE project_id = $1 ORDER BY created_at;

-- name: GetEnabledAlerts :many
SELECT * FROM alert_configs WHERE project_id = $1 AND enabled = true;

-- name: UpdateAlertConfig :one
UPDATE alert_configs
SET config = $2, enabled = $3, level_filter = $4, title_pattern = $5, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: DeleteAlertConfig :exec
DELETE FROM alert_configs WHERE id = $1;
