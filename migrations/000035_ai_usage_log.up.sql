CREATE TABLE ai_usage_log (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    feature TEXT NOT NULL,
    model TEXT NOT NULL,
    input_tokens INT NOT NULL DEFAULT 0,
    output_tokens INT NOT NULL DEFAULT 0,
    latency_ms INT NOT NULL DEFAULT 0,
    prompt_hash TEXT,
    cached_response TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_ai_usage_log_project_day ON ai_usage_log (project_id, created_at);
CREATE INDEX idx_ai_usage_log_prompt_hash ON ai_usage_log (project_id, prompt_hash, created_at) WHERE prompt_hash IS NOT NULL;
