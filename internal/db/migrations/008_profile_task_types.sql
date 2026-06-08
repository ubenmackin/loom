-- Migration 008: Create profile_task_types table
CREATE TABLE IF NOT EXISTS profile_task_types (
    profile_id TEXT NOT NULL REFERENCES agent_profiles(id) ON DELETE CASCADE,
    task_type TEXT NOT NULL,
    PRIMARY KEY (profile_id, task_type)
);
