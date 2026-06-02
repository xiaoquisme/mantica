-- Allow runtime_id to be nullable for agents that don't need a runtime yet (e.g. KANBAN)
ALTER TABLE agent ALTER COLUMN runtime_id DROP NOT NULL;
