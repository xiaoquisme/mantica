-- Scheduled tasks: cron-triggered agent tasks managed via UI.
CREATE TABLE scheduled_task (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    agent_id UUID NOT NULL REFERENCES agent(id) ON DELETE CASCADE,
    schedule TEXT NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT true,
    last_run_at TIMESTAMPTZ,
    next_run_at TIMESTAMPTZ,
    created_by UUID NOT NULL REFERENCES "user"(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(workspace_id, name)
);

CREATE INDEX idx_scheduled_task_workspace ON scheduled_task(workspace_id);
CREATE INDEX idx_scheduled_task_due ON scheduled_task(next_run_at) WHERE enabled = true;

-- Link tasks back to the scheduled_task that triggered them.
ALTER TABLE agent_task_queue ADD COLUMN scheduled_task_id UUID REFERENCES scheduled_task(id) ON DELETE SET NULL;
