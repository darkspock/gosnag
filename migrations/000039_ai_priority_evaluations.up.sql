CREATE TABLE ai_priority_evaluations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    issue_id UUID NOT NULL REFERENCES issues(id) ON DELETE CASCADE,
    rule_id UUID NOT NULL REFERENCES priority_rules(id) ON DELETE CASCADE,
    status TEXT NOT NULL DEFAULT 'success',
    points INT NOT NULL DEFAULT 0,
    reason TEXT NOT NULL DEFAULT '',
    retries INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(issue_id, rule_id)
);
CREATE INDEX idx_ai_priority_eval_issue ON ai_priority_evaluations(issue_id);
