-- Migration 015: Add agent_role column to agent_profiles
-- Stores the system-prompt role key (e.g. "planner", "executor", "reviewer",
-- "security-auditor", "release-manager", "workspace-setup"). Blank means
-- "use the profile name" for back-compat with pre-migration rows.
ALTER TABLE agent_profiles ADD COLUMN agent_role TEXT NOT NULL DEFAULT '';
