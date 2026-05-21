-- Loom User Authentication & Global Numeric ID Schema Updates

-- Alter users table to support secure email & password login
ALTER TABLE users ADD COLUMN email TEXT;
ALTER TABLE users ADD COLUMN password_hash TEXT;

-- Create unique index on users.email
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email ON users(email);

-- Create user_sessions table for token-based authentication
CREATE TABLE IF NOT EXISTS user_sessions (
    token TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at DATETIME NOT NULL DEFAULT (datetime('now')),
    expires_at DATETIME NOT NULL
);

-- Index for session lookups and expiration cleanup
CREATE INDEX IF NOT EXISTS idx_user_sessions_user ON user_sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_user_sessions_expires ON user_sessions(expires_at);

-- Create shared sequencer for global numeric IDs across all work items (stories and tasks)
CREATE TABLE IF NOT EXISTS work_item_sequence (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    type TEXT NOT NULL -- 'story' or 'task'
);

-- Add numeric_id to stories
ALTER TABLE stories ADD COLUMN numeric_id INTEGER;
CREATE UNIQUE INDEX IF NOT EXISTS idx_stories_numeric_id ON stories(numeric_id);

-- Add numeric_id to tasks
ALTER TABLE tasks ADD COLUMN numeric_id INTEGER;
CREATE UNIQUE INDEX IF NOT EXISTS idx_tasks_numeric_id ON tasks(numeric_id);
