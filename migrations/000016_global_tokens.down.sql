ALTER TABLE api_tokens DROP COLUMN IF EXISTS scope;
-- Cannot safely re-add NOT NULL if there are global tokens with NULL project_id
-- DELETE FROM api_tokens WHERE project_id IS NULL;
-- ALTER TABLE api_tokens ALTER COLUMN project_id SET NOT NULL;
