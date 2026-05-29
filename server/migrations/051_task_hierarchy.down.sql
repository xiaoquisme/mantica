-- Migration 051: Task Hierarchy (down)
-- Removes parent task reference and subagent role

DROP INDEX IF EXISTS idx_agent_task_queue_parent;
DROP INDEX IF EXISTS idx_agent_task_queue_depth;

ALTER TABLE agent_task_queue DROP COLUMN IF EXISTS parent_task_id;
ALTER TABLE agent_task_queue DROP COLUMN IF EXISTS subagent_role;
ALTER TABLE agent_task_queue DROP COLUMN IF EXISTS task_depth;