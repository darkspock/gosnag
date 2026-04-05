-- Jira connection config per project
ALTER TABLE projects
    ADD COLUMN jira_base_url TEXT NOT NULL DEFAULT '',
    ADD COLUMN jira_email TEXT NOT NULL DEFAULT '',
    ADD COLUMN jira_api_token TEXT NOT NULL DEFAULT '',
    ADD COLUMN jira_project_key TEXT NOT NULL DEFAULT '',
    ADD COLUMN jira_issue_type TEXT NOT NULL DEFAULT 'Bug';

-- Track Jira ticket on issues
ALTER TABLE issues
    ADD COLUMN jira_ticket_key TEXT,
    ADD COLUMN jira_ticket_url TEXT;

-- Auto-creation rules
CREATE TABLE jira_rules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT true,
    level_filter TEXT NOT NULL DEFAULT '',
    min_events INT NOT NULL DEFAULT 0,
    min_users INT NOT NULL DEFAULT 0,
    title_pattern TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_jira_rules_project ON jira_rules(project_id);
