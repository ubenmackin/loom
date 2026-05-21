-- Loom Initial Schema
-- Agent-first JIT Kanban board database

CREATE TABLE IF NOT EXISTS users (
    id TEXT PRIMARY KEY,
    username TEXT UNIQUE NOT NULL,
    display_name TEXT,
    created_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS sessions (
    id TEXT PRIMARY KEY,
    harness_type TEXT NOT NULL,
    capabilities TEXT, -- JSON
    metadata TEXT, -- JSON
    last_seen_at DATETIME NOT NULL DEFAULT (datetime('now')),
    status TEXT NOT NULL DEFAULT 'active', -- "active" | "stale" | "disconnected"
    created_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS stories (
    id TEXT PRIMARY KEY, -- e.g., "STORY-001"
    title TEXT NOT NULL,
    description TEXT, -- markdown
    status TEXT NOT NULL DEFAULT 'new', -- "new" | "ready" | "in_progress" | "blocked" | "done"
    priority INTEGER NOT NULL DEFAULT 0, -- lower = higher priority
    requires_build BOOLEAN NOT NULL DEFAULT false,
    requires_review BOOLEAN NOT NULL DEFAULT false,
    assigned_to TEXT,
    assignee_type TEXT, -- "human" | "session"
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS tasks (
    id TEXT PRIMARY KEY, -- e.g., "TASK-001"
    story_id TEXT NOT NULL REFERENCES stories(id),
    title TEXT NOT NULL,
    description TEXT,
    status TEXT NOT NULL DEFAULT 'new', -- "new" | "ready" | "in_progress" | "blocked" | "done"
    priority INTEGER NOT NULL DEFAULT 0,
    task_type TEXT NOT NULL DEFAULT 'code', -- "code" | "build" | "review" | custom
    estimate INTEGER, -- nullable
    assigned_to TEXT,
    assignee_type TEXT, -- "human" | "session"
    sort_order INTEGER NOT NULL DEFAULT 0,
    context TEXT, -- JSON stored as text
    instructions TEXT,
    is_stale BOOLEAN NOT NULL DEFAULT false,
    created_at DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS task_dependencies (
    task_id TEXT NOT NULL REFERENCES tasks(id),
    depends_on_task_id TEXT NOT NULL REFERENCES tasks(id),
    PRIMARY KEY (task_id, depends_on_task_id)
);

CREATE TABLE IF NOT EXISTS comments (
    id TEXT PRIMARY KEY,
    work_item_id TEXT NOT NULL,
    work_item_type TEXT NOT NULL, -- "story" | "task"
    author_id TEXT NOT NULL,
    author_type TEXT NOT NULL, -- "human" | "session"
    body TEXT,
    created_at DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS activity_log (
    id TEXT PRIMARY KEY,
    work_item_id TEXT NOT NULL,
    work_item_type TEXT NOT NULL, -- "story" | "task"
    action TEXT NOT NULL,
    details TEXT, -- JSON as text
    created_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS prompt_templates (
    id TEXT PRIMARY KEY,
    task_type TEXT UNIQUE NOT NULL,
    template TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS unread_comments (
    session_id TEXT NOT NULL REFERENCES sessions(id),
    comment_id TEXT NOT NULL REFERENCES comments(id),
    PRIMARY KEY (session_id, comment_id)
);

-- Indexes for common queries
CREATE INDEX IF NOT EXISTS idx_sessions_status ON sessions(status);
CREATE INDEX IF NOT EXISTS idx_sessions_last_seen ON sessions(last_seen_at);

CREATE INDEX IF NOT EXISTS idx_stories_status ON stories(status);
CREATE INDEX IF NOT EXISTS idx_stories_assigned_to ON stories(assigned_to);
CREATE INDEX IF NOT EXISTS idx_stories_sort_order ON stories(sort_order);

CREATE INDEX IF NOT EXISTS idx_tasks_story_id ON tasks(story_id);
CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status);
CREATE INDEX IF NOT EXISTS idx_tasks_assigned_to ON tasks(assigned_to);
CREATE INDEX IF NOT EXISTS idx_tasks_task_type ON tasks(task_type);
CREATE INDEX IF NOT EXISTS idx_tasks_sort_order ON tasks(sort_order);

CREATE INDEX IF NOT EXISTS idx_task_dependencies_task_id ON task_dependencies(task_id);
CREATE INDEX IF NOT EXISTS idx_task_dependencies_depends_on ON task_dependencies(depends_on_task_id);

CREATE INDEX IF NOT EXISTS idx_comments_work_item ON comments(work_item_id, work_item_type);
CREATE INDEX IF NOT EXISTS idx_comments_author ON comments(author_id, author_type);

CREATE INDEX IF NOT EXISTS idx_activity_log_work_item ON activity_log(work_item_id, work_item_type);
CREATE INDEX IF NOT EXISTS idx_activity_log_created_at ON activity_log(created_at);

CREATE INDEX IF NOT EXISTS idx_unread_comments_session ON unread_comments(session_id);
