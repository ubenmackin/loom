-- Migration 006: Add projects table, agent fields to stories and tasks
CREATE TABLE IF NOT EXISTS projects (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT,
    repo_path TEXT,
    language TEXT,
    build_command TEXT,
    created_at DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

ALTER TABLE stories ADD COLUMN project_id TEXT REFERENCES projects(id) ON DELETE SET NULL;
ALTER TABLE stories ADD COLUMN agent_session_id TEXT;
ALTER TABLE stories ADD COLUMN agent_type TEXT;

ALTER TABLE tasks ADD COLUMN agent_session_id TEXT;
ALTER TABLE tasks ADD COLUMN agent_type TEXT;