-- Agent scoring: tracks per-agent performance metrics over time.
-- Updated after each task completes via the Analyzer service.

CREATE TABLE IF NOT EXISTS agent_score (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID NOT NULL REFERENCES agent(id) ON DELETE CASCADE,
    workspace_id UUID NOT NULL,

    -- Overall scoring (ELO-like, starts at 1000)
    overall_score DOUBLE PRECISION NOT NULL DEFAULT 1000.0,

    -- Per-task-type scores (JSONB for flexibility)
    -- e.g. {"bug_fix": 1050, "feature": 980, "review": 1100}
    task_type_scores JSONB NOT NULL DEFAULT '{}',

    -- Aggregate statistics
    total_tasks INT NOT NULL DEFAULT 0,
    successful_tasks INT NOT NULL DEFAULT 0,
    failed_tasks INT NOT NULL DEFAULT 0,
    success_rate DOUBLE PRECISION NOT NULL DEFAULT 0.0,

    -- Quality metrics (rolling average from task_analysis)
    avg_tool_count DOUBLE PRECISION NOT NULL DEFAULT 0.0,
    avg_error_count DOUBLE PRECISION NOT NULL DEFAULT 0.0,
    avg_error_rate DOUBLE PRECISION NOT NULL DEFAULT 0.0,

    -- Trend tracking
    score_trend TEXT NOT NULL DEFAULT 'stable'
        CHECK (score_trend IN ('improving', 'stable', 'declining')),
    trend_samples INT NOT NULL DEFAULT 0,  -- tasks in current trend window

    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    UNIQUE(agent_id, workspace_id)
);

CREATE INDEX IF NOT EXISTS idx_agent_score_agent ON agent_score(agent_id);
CREATE INDEX IF NOT EXISTS idx_agent_score_workspace ON agent_score(workspace_id);
CREATE INDEX IF NOT EXISTS idx_agent_score_overall ON agent_score(overall_score DESC);

-- Score history: tracks individual score changes for trend analysis
CREATE TABLE IF NOT EXISTS agent_score_history (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID NOT NULL REFERENCES agent(id) ON DELETE CASCADE,
    workspace_id UUID NOT NULL,
    task_id UUID NOT NULL REFERENCES agent_task_queue(id) ON DELETE CASCADE,
    task_type TEXT,

    -- Score delta from this task
    score_before DOUBLE PRECISION NOT NULL,
    score_after DOUBLE PRECISION NOT NULL,
    score_delta DOUBLE PRECISION NOT NULL,

    -- Task outcome
    success BOOLEAN NOT NULL,
    tool_count INT NOT NULL DEFAULT 0,
    error_count INT NOT NULL DEFAULT 0,

    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_score_history_agent ON agent_score_history(agent_id, created_at DESC);
