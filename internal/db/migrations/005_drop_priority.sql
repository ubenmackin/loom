-- Migration 005: Drop priority column from stories and tasks
ALTER TABLE stories DROP COLUMN priority;
ALTER TABLE tasks DROP COLUMN priority;
