-- name: LogAICall :exec
INSERT INTO ai_usage_log (project_id, feature, model, input_tokens, output_tokens, latency_ms, prompt_hash, cached_response)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8);

-- name: GetDailyTokenUsage :one
SELECT COALESCE(SUM(input_tokens + output_tokens), 0)::bigint
FROM ai_usage_log
WHERE project_id = $1
  AND created_at >= date_trunc('day', now() AT TIME ZONE 'UTC');

-- name: GetCallsInLastMinute :one
SELECT COUNT(*)::bigint
FROM ai_usage_log
WHERE project_id = $1
  AND created_at >= now() - interval '1 minute';

-- name: GetCachedResponse :one
SELECT cached_response
FROM ai_usage_log
WHERE project_id = $1
  AND prompt_hash = $2
  AND cached_response IS NOT NULL
  AND created_at >= now() - interval '5 minutes'
ORDER BY created_at DESC
LIMIT 1;

-- name: ClearExpiredAICache :exec
UPDATE ai_usage_log
SET cached_response = NULL
WHERE cached_response IS NOT NULL
  AND created_at < now() - interval '5 minutes';

-- name: GetProjectAIUsageToday :one
SELECT COALESCE(SUM(input_tokens + output_tokens), 0)::bigint AS total_tokens,
       COUNT(*)::bigint AS total_calls
FROM ai_usage_log
WHERE project_id = $1
  AND created_at >= date_trunc('day', now() AT TIME ZONE 'UTC');

-- name: GetProjectAIUsageWeek :one
SELECT COALESCE(SUM(input_tokens + output_tokens), 0)::bigint AS total_tokens,
       COUNT(*)::bigint AS total_calls
FROM ai_usage_log
WHERE project_id = $1
  AND created_at >= date_trunc('week', now() AT TIME ZONE 'UTC');
