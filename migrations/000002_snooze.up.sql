-- Add snooze support to issues
ALTER TABLE issues DROP CONSTRAINT issues_status_check;
ALTER TABLE issues ADD CONSTRAINT issues_status_check
  CHECK (status IN ('open', 'resolved', 'reopened', 'ignored', 'snoozed'));

ALTER TABLE issues ADD COLUMN snooze_until TIMESTAMPTZ;
ALTER TABLE issues ADD COLUMN snooze_event_threshold INTEGER;
ALTER TABLE issues ADD COLUMN snooze_events_at_start INTEGER NOT NULL DEFAULT 0;
