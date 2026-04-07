-- Phase 2 prerequisite: idempotent event forwarding from gosnag-agent.
-- First remove any existing duplicates (keep the earliest row per project_id + event_id).
DELETE FROM events e
USING (
    SELECT project_id, event_id, MIN(ctid) AS keep_ctid
    FROM events
    GROUP BY project_id, event_id
    HAVING COUNT(*) > 1
) dups
WHERE e.project_id = dups.project_id
  AND e.event_id = dups.event_id
  AND e.ctid <> dups.keep_ctid;

-- Now create the unique index. CONCURRENTLY cannot run inside a transaction,
-- and golang-migrate wraps each file in a transaction, so we use a regular CREATE.
CREATE UNIQUE INDEX IF NOT EXISTS events_project_event_unique
ON events (project_id, event_id);

-- Drop the old non-unique index (now redundant).
DROP INDEX IF EXISTS idx_events_event_id;
