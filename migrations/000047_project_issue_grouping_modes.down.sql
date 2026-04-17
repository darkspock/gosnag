ALTER TABLE project_issue_settings
    DROP COLUMN IF EXISTS warning_grouping_mode,
    DROP COLUMN IF EXISTS error_grouping_mode;
