-- Add AI classification support to tag rules
ALTER TABLE tag_rules ADD COLUMN rule_type TEXT NOT NULL DEFAULT 'pattern';
ALTER TABLE tag_rules ADD COLUMN threshold INT NOT NULL DEFAULT 0;

-- Guard table for AI tag evaluations (mirrors ai_priority_evaluations)
CREATE TABLE ai_tag_evaluations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    issue_id UUID NOT NULL REFERENCES issues(id) ON DELETE CASCADE,
    rule_id UUID NOT NULL REFERENCES tag_rules(id) ON DELETE CASCADE,
    status TEXT NOT NULL DEFAULT 'success',
    tag_value TEXT NOT NULL DEFAULT '',
    reason TEXT NOT NULL DEFAULT '',
    retries INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(issue_id, rule_id)
);
CREATE INDEX idx_ai_tag_eval_issue ON ai_tag_evaluations(issue_id);
