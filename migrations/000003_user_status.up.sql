ALTER TABLE users ADD COLUMN status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('invited', 'active', 'disabled'));

-- Existing users who have a google_id are active; those without are invited
UPDATE users SET status = 'invited' WHERE google_id IS NULL;
