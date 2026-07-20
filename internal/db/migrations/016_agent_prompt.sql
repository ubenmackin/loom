-- Migration 016: Add prompt column to agent_profiles
-- Stores the per-profile system prompt text (the EXPRESSION of the role
-- prompt previously hard-coded as Go consts in internal/gateway/prompts.go).
-- Blank means "use the static role-keyed default" (see ProfilePrompt in
-- internal/gateway/prompts.go). Existing rows are backfilled by the seed
-- pass in internal/db/seed.go on the next startup after this migration
-- lands, so the column ships with NOT NULL DEFAULT ''.
ALTER TABLE agent_profiles ADD COLUMN prompt TEXT NOT NULL DEFAULT '';
