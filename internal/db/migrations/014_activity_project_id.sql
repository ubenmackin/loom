-- Migration 002: Add project_id column to activity_log
-- Each activity log entry is now scoped to a project so that activity can be
-- filtered and grouped by project without joining through the work_item_id.
--
-- For existing rows, project_id is backfilled from the associated story:
--   - If the work item is a story, use stories.project_id.
--   - If the work item is a task, look up its parent story's project_id.
-- Rows that cannot be resolved (orphan activity entries) remain '' (the
-- DEFAULT), which matches the empty-string convention used elsewhere in the
-- schema.
ALTER TABLE activity_log ADD COLUMN project_id TEXT DEFAULT '';

-- Backfill project_id from stories (direct case).
UPDATE activity_log
   SET project_id = (SELECT s.project_id
                       FROM stories s
                      WHERE s.id = activity_log.work_item_id
                        AND activity_log.work_item_type = 'story')
 WHERE project_id = ''
   AND work_item_type = 'story'
   AND EXISTS (SELECT 1
                 FROM stories s
                WHERE s.id = activity_log.work_item_id);

-- Backfill project_id from tasks → stories (two-hop case).
UPDATE activity_log
   SET project_id = (SELECT s.project_id
                       FROM stories s
                       JOIN tasks t ON t.story_id = s.id
                      WHERE t.id = activity_log.work_item_id
                        AND activity_log.work_item_type = 'task')
 WHERE project_id = ''
   AND work_item_type = 'task'
   AND EXISTS (SELECT 1
                 FROM stories s
                 JOIN tasks t ON t.story_id = s.id
                WHERE t.id = activity_log.work_item_id);

CREATE INDEX IF NOT EXISTS idx_activity_log_project_id ON activity_log(project_id);
