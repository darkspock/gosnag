-- AI per-project settings
ALTER TABLE projects
    ADD COLUMN ai_enabled BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN ai_model TEXT NOT NULL DEFAULT '',
    ADD COLUMN ai_merge_suggestions BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN ai_auto_merge BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN ai_anomaly_detection BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN ai_ticket_description BOOLEAN NOT NULL DEFAULT true,
    ADD COLUMN ai_root_cause BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN ai_triage BOOLEAN NOT NULL DEFAULT false;
