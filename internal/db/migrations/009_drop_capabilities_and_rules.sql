-- Migration 009: Drop capabilities and trigger_rules
ALTER TABLE agent_profiles DROP COLUMN capabilities;
DROP TABLE IF EXISTS trigger_rules;
