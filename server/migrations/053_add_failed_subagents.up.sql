-- Migration 053: Add failed_subagents column
-- Adds failed sub-agents tracking column that was missing from migration 052

ALTER TABLE agent_task_queue ADD COLUMN failed_subagents INT NOT NULL DEFAULT 0;