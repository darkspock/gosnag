DROP TABLE IF EXISTS jira_rules;

ALTER TABLE issues
    DROP COLUMN IF EXISTS jira_ticket_key,
    DROP COLUMN IF EXISTS jira_ticket_url;

ALTER TABLE projects
    DROP COLUMN IF EXISTS jira_base_url,
    DROP COLUMN IF EXISTS jira_email,
    DROP COLUMN IF EXISTS jira_api_token,
    DROP COLUMN IF EXISTS jira_project_key,
    DROP COLUMN IF EXISTS jira_issue_type;
