ALTER TABLE project_issue_settings
    ADD COLUMN error_grouping_mode TEXT NOT NULL DEFAULT 'normal',
    ADD COLUMN warning_grouping_mode TEXT NOT NULL DEFAULT 'normal';

UPDATE project_issue_settings
SET
    error_grouping_mode = info_grouping_mode,
    warning_grouping_mode = info_grouping_mode
WHERE
    error_grouping_mode = 'normal'
    AND warning_grouping_mode = 'normal';
