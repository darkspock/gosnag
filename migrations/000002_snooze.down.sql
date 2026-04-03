ALTER TABLE issues DROP COLUMN snooze_events_at_start;
ALTER TABLE issues DROP COLUMN snooze_event_threshold;
ALTER TABLE issues DROP COLUMN snooze_until;

ALTER TABLE issues DROP CONSTRAINT issues_status_check;
ALTER TABLE issues ADD CONSTRAINT issues_status_check
  CHECK (status IN ('open', 'resolved', 'reopened', 'ignored'));
