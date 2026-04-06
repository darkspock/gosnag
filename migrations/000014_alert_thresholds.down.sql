ALTER TABLE alert_configs DROP COLUMN IF EXISTS min_events;
ALTER TABLE alert_configs DROP COLUMN IF EXISTS min_velocity_1h;
ALTER TABLE alert_configs DROP COLUMN IF EXISTS exclude_pattern;
