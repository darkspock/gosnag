ALTER TABLE alert_configs ADD COLUMN level_filter TEXT NOT NULL DEFAULT '';
ALTER TABLE alert_configs ADD COLUMN title_pattern TEXT NOT NULL DEFAULT '';
