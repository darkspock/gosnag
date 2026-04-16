ALTER TABLE projects
    ADD COLUMN analysis_db_enabled BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN analysis_db_driver TEXT NOT NULL DEFAULT '',
    ADD COLUMN analysis_db_dsn TEXT NOT NULL DEFAULT '',
    ADD COLUMN analysis_db_name TEXT NOT NULL DEFAULT '',
    ADD COLUMN analysis_db_schema TEXT NOT NULL DEFAULT '',
    ADD COLUMN analysis_db_notes TEXT NOT NULL DEFAULT '';
