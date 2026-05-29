-- Migration 052: Sub-agent Task Status
-- Adds status tracking for sub-agent coordination

-- Add status for sub-agent coordination
ALTER TABLE agent_task_queue ADD COLUMN waiting_for_subagents BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE agent_task_queue ADD COLUMN completed_subagents INT NOT NULL DEFAULT 0;
ALTER TABLE agent_task_queue ADD COLUMN total_subagents INT NOT NULL DEFAULT 0;