-- Add role column to users table
-- Values: 'admin' | 'normal'
ALTER TABLE users ADD COLUMN role TEXT NOT NULL DEFAULT 'normal';

-- Promote the earliest-created user to admin (handles existing single-user installs)
UPDATE users SET role = 'admin' WHERE rowid = (SELECT MIN(rowid) FROM users);
