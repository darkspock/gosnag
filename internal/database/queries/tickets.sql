-- name: CreateTicket :one
INSERT INTO tickets (issue_id, project_id, status, assigned_to, created_by, priority, title, description)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: GetTicket :one
SELECT * FROM tickets WHERE id = $1;

-- name: GetTicketByIssue :one
SELECT * FROM tickets WHERE issue_id = $1 AND status NOT IN ('done', 'wontfix') ORDER BY created_at DESC LIMIT 1;

-- name: GetTicketByIssueIncludingDone :one
SELECT * FROM tickets WHERE issue_id = $1 ORDER BY created_at DESC LIMIT 1;

-- name: UpdateTicketStatus :one
UPDATE tickets SET status = $2, updated_at = now() WHERE id = $1 RETURNING *;

-- name: UpdateTicket :one
UPDATE tickets SET
    status = $2,
    assigned_to = $3,
    priority = $4,
    due_date = $5,
    resolution_type = $6,
    resolution_notes = $7,
    fix_reference = $8,
    title = $9,
    description = $10,
    escalated_system = $11,
    escalated_key = $12,
    escalated_url = $13,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: ListTicketsByProject :many
SELECT t.*,
       COALESCE(i.title, t.title) AS issue_title,
       COALESCE(i.level, '') AS issue_level,
       COALESCE(i.event_count, 0) AS issue_event_count,
       i.first_seen AS issue_first_seen, i.last_seen AS issue_last_seen,
       u.name AS assignee_name, u.email AS assignee_email, u.avatar_url AS assignee_avatar
FROM tickets t
LEFT JOIN issues i ON i.id = t.issue_id
LEFT JOIN users u ON u.id = t.assigned_to
WHERE t.project_id = $1
  AND ($2::text = '' OR t.status = $2::text)
ORDER BY
    CASE t.priority WHEN 90 THEN 0 WHEN 70 THEN 1 WHEN 50 THEN 2 ELSE 3 END,
    t.updated_at DESC
LIMIT $3 OFFSET $4;

-- name: CountTicketsByProject :one
SELECT count(*) FROM tickets
WHERE project_id = $1
  AND ($2::text = '' OR status = $2::text);

-- name: CountTicketsByStatus :many
SELECT status, count(*)::int AS count
FROM tickets
WHERE project_id = $1
GROUP BY status;

-- name: CountPendingTicketsAssignedToUser :one
SELECT count(*)::int
FROM tickets
WHERE project_id = $1
  AND assigned_to = $2
  AND status NOT IN ('done', 'wontfix');
