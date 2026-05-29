-- Migration 052: Sub-agent Task Status (down)
-- Removes status tracking for sub-agent coordination

ALTER TABLE agent_task_queue DROP COLUMN IF EXISTS waiting_for_subagents;
ALTER TABLE agent_task_queue DROP COLUMN IF EXISTS completed_subagents;
ALTER TABLE agent_task_queue DROP COLUMN IF EXISTS total_subagents;