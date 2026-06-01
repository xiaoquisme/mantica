-- Update unique index to exclude subtasks (parent_task_id IS NOT NULL)
-- This allows kanban agent to create parallel subtasks for the same issue
-- while still preventing duplicate top-level tasks.

DROP INDEX IF EXISTS idx_one_pending_task_per_issue;

CREATE UNIQUE INDEX idx_one_pending_task_per_issue
    ON agent_task_queue (issue_id)
    WHERE status IN ('queued', 'dispatched') AND parent_task_id IS NULL;
