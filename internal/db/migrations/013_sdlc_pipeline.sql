-- Migration 013: Add SDLC pipeline columns to stories and tasks
ALTER TABLE stories ADD COLUMN requires_security INTEGER NOT NULL DEFAULT 1;
ALTER TABLE stories ADD COLUMN failure_count INTEGER NOT NULL DEFAULT 0;
ALTER TABLE stories ADD COLUMN branch_name TEXT NOT NULL DEFAULT '';
-- TODO: The DEFAULT '[]' on target_files is never exercised in practice — store.Create
-- always supplies an explicit value for target_files (empty string for non-code tasks).
-- Both "" and "[]" are handled identically by parseTargetFiles, so this is not a bug,
-- but a future developer may be surprised the DEFAULT never fires. Consider removing
-- the DEFAULT or aligning the INSERT to omit the column for non-code tasks.
ALTER TABLE tasks ADD COLUMN target_files TEXT NOT NULL DEFAULT '[]';
