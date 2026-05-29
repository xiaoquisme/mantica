-- Migration 051: Task Hierarchy
-- Adds parent task reference and subagent role for task hierarchy

-- Add parent task reference for task hierarchy
ALTER TABLE agent_task_queue ADD COLUMN parent_task_id UUID REFERENCES agent_task_queue(id) ON DELETE CASCADE;

-- Add subagent role to track which subagent is executing the task
ALTER TABLE agent_task_queue ADD COLUMN subagent_role TEXT CHECK (subagent_role IN ('kanban', 'main', 'terminal', 'summary', 'orchestrator'));

-- Add task depth to prevent infinite recursion
ALTER TABLE agent_task_queue ADD COLUMN task_depth INT NOT NULL DEFAULT 0;

-- Index for parent task queries
CREATE INDEX idx_agent_task_queue_parent ON agent_task_queue(parent_task_id);
CREATE INDEX idx_agent_task_queue_depth ON agent_task_queue(task_depth);