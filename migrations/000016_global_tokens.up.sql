-- Make project_id nullable to support global tokens
ALTER TABLE api_tokens ALTER COLUMN project_id DROP NOT NULL;

-- Add scope column: 'project' (default, existing) or 'global'
ALTER TABLE api_tokens ADD COLUMN scope TEXT NOT NULL DEFAULT 'project' CHECK (scope IN ('project', 'global'));
