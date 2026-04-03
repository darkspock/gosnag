ALTER TABLE events ADD COLUMN user_identifier TEXT NOT NULL DEFAULT '';

CREATE INDEX idx_events_user_identifier ON events(issue_id, user_identifier) WHERE user_identifier != '';
