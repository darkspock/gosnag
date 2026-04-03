-- name: CreateEvent :one
INSERT INTO events (issue_id, project_id, event_id, timestamp, platform, level, message, release, environment, server_name, data, user_identifier)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
RETURNING *;

-- name: GetEvent :one
SELECT * FROM events WHERE id = $1;

-- name: ListEventsByIssue :many
SELECT * FROM events
WHERE issue_id = $1
ORDER BY timestamp DESC
LIMIT $2 OFFSET $3;

-- name: CountEventsByIssue :one
SELECT count(*) FROM events WHERE issue_id = $1;

-- name: GetUniqueUserCountsByIssues :many
SELECT issue_id, COUNT(DISTINCT user_identifier)::int as user_count
FROM events
WHERE issue_id = ANY($1::uuid[])
  AND user_identifier != ''
GROUP BY issue_id;

-- name: GetEventTrendByIssues :many
SELECT issue_id,
       date_trunc('hour', timestamp)::timestamptz as bucket,
       COUNT(*)::int as count
FROM events
WHERE issue_id = ANY($1::uuid[])
  AND timestamp >= now() - interval '24 hours'
GROUP BY issue_id, bucket
ORDER BY issue_id, bucket;

-- name: DeleteEventsOlderThan :execresult
DELETE FROM events WHERE created_at < $1;
