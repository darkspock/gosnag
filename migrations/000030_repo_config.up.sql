-- Source code repository config per project
ALTER TABLE projects
    ADD COLUMN repo_provider TEXT NOT NULL DEFAULT '',
    ADD COLUMN repo_owner TEXT NOT NULL DEFAULT '',
    ADD COLUMN repo_name TEXT NOT NULL DEFAULT '',
    ADD COLUMN repo_default_branch TEXT NOT NULL DEFAULT 'main',
    ADD COLUMN repo_token TEXT NOT NULL DEFAULT '',
    ADD COLUMN repo_path_strip TEXT NOT NULL DEFAULT '';
