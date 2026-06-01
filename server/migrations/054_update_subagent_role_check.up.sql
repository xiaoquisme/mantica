-- Update subagent_role check constraint to include business role names
-- used by Kanban Agent decomposition rules (BA, TL, DEV, QA, Code Review)

ALTER TABLE agent_task_queue DROP CONSTRAINT IF EXISTS agent_task_queue_subagent_role_check;

ALTER TABLE agent_task_queue ADD CONSTRAINT agent_task_queue_subagent_role_check
    CHECK (subagent_role IN (
        'kanban', 'main', 'terminal', 'summary', 'orchestrator',
        'BA', 'TL', 'DEV', 'QA', 'Code Review'
    ));
