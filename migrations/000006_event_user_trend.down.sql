DROP INDEX IF EXISTS idx_events_user_identifier;
ALTER TABLE events DROP COLUMN IF EXISTS user_identifier;
