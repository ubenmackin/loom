-- Migration 007: Create agent_profiles and trigger_rules tables
CREATE TABLE IF NOT EXISTS agent_profiles (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT,
    capabilities TEXT,
    max_concurrency INTEGER DEFAULT 5,
    created_at DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS trigger_rules (
    id TEXT PRIMARY KEY,
    agent_profile_id TEXT REFERENCES agent_profiles(id) ON DELETE CASCADE,
    event_type TEXT NOT NULL,
    action TEXT NOT NULL,
    priority INTEGER DEFAULT 0,
    enabled BOOLEAN DEFAULT 1,
    created_at DATETIME NOT NULL DEFAULT (datetime('now'))
);