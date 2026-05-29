-- Migration 053: Add failed_subagents column (rollback)

ALTER TABLE agent_task_queue DROP COLUMN failed_subagents;