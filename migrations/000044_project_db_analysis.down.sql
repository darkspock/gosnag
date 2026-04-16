ALTER TABLE projects
    DROP COLUMN IF EXISTS analysis_db_notes,
    DROP COLUMN IF EXISTS analysis_db_schema,
    DROP COLUMN IF EXISTS analysis_db_name,
    DROP COLUMN IF EXISTS analysis_db_dsn,
    DROP COLUMN IF EXISTS analysis_db_driver,
    DROP COLUMN IF EXISTS analysis_db_enabled;
