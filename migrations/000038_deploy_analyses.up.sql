CREATE TABLE IF NOT EXISTS deploy_analyses (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    deploy_id UUID NOT NULL REFERENCES deploys(id) ON DELETE CASCADE,
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    severity TEXT NOT NULL DEFAULT 'none',
    summary TEXT NOT NULL DEFAULT '',
    details TEXT NOT NULL DEFAULT '',
    likely_deploy_caused BOOLEAN NOT NULL DEFAULT false,
    recommended_action TEXT NOT NULL DEFAULT 'monitor',
    new_issues_count INT NOT NULL DEFAULT 0,
    spiked_issues_count INT NOT NULL DEFAULT 0,
    reopened_issues_count INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_deploy_analyses_deploy ON deploy_analyses (deploy_id);
CREATE INDEX idx_deploy_analyses_project ON deploy_analyses (project_id, created_at DESC);
