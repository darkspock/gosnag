-- name: CreateIssueAlias :exec
INSERT INTO issue_aliases (project_id, fingerprint, primary_issue_id)
VALUES ($1, $2, $3)
ON CONFLICT (project_id, fingerprint) DO UPDATE SET primary_issue_id = $3;

-- name: GetIssueAlias :one
SELECT * FROM issue_aliases WHERE project_id = $1 AND fingerprint = $2;

-- name: RepointAliases :exec
UPDATE issue_aliases SET primary_issue_id = $2 WHERE primary_issue_id = $1;

-- name: MoveEventsToIssue :execresult
UPDATE events SET issue_id = $2 WHERE issue_id = $1;

-- name: DeleteIssue :exec
DELETE FROM issues WHERE id = $1;

-- name: RecalcIssueStats :one
UPDATE issues SET
    event_count = (SELECT count(*)::int FROM events e WHERE e.issue_id = issues.id),
    first_seen = COALESCE((SELECT min(e.timestamp) FROM events e WHERE e.issue_id = issues.id), issues.first_seen),
    last_seen = COALESCE((SELECT max(e.timestamp) FROM events e WHERE e.issue_id = issues.id), issues.last_seen),
    updated_at = now()
WHERE issues.id = $1
RETURNING *;
