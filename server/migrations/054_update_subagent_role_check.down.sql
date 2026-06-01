-- Revert subagent_role check constraint to original values

ALTER TABLE agent_task_queue DROP CONSTRAINT IF EXISTS agent_task_queue_subagent_role_check;

ALTER TABLE agent_task_queue ADD CONSTRAINT agent_task_queue_subagent_role_check
    CHECK (subagent_role IN ('kanban', 'main', 'terminal', 'summary', 'orchestrator'));
