ALTER TABLE agent_task_queue DROP COLUMN IF EXISTS scheduled_task_id;
DROP TABLE IF EXISTS scheduled_task;
