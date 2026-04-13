DROP TABLE IF EXISTS ai_tag_evaluations;
ALTER TABLE tag_rules DROP COLUMN IF EXISTS threshold;
ALTER TABLE tag_rules DROP COLUMN IF EXISTS rule_type;
