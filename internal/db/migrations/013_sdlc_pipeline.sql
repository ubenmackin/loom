-- Migration 013: Add SDLC pipeline columns to stories and tasks
ALTER TABLE stories ADD COLUMN requires_security INTEGER NOT NULL DEFAULT 1;
ALTER TABLE stories ADD COLUMN failure_count INTEGER NOT NULL DEFAULT 0;
ALTER TABLE stories ADD COLUMN branch_name TEXT NOT NULL DEFAULT '';
ALTER TABLE tasks ADD COLUMN target_files TEXT NOT NULL DEFAULT '[]';
