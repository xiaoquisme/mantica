-- Post-task analysis: stores structured execution insights
-- Generated automatically after each task completes or fails.

CREATE TABLE IF NOT EXISTS task_analysis (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id UUID NOT NULL REFERENCES agent_task_queue(id) ON DELETE CASCADE,

    -- Execution metrics
    tool_count INT NOT NULL DEFAULT 0,
    error_count INT NOT NULL DEFAULT 0,
    unique_tools INT NOT NULL DEFAULT 0,
    total_duration_ms BIGINT NOT NULL DEFAULT 0,
    message_count INT NOT NULL DEFAULT 0,

    -- Failure analysis (NULL for successful tasks)
    failure_class TEXT,  -- timeout | runtime_error | empty_output | build_error | test_fail | logic_error
    failure_detail TEXT, -- human-readable description

    -- Tool usage breakdown (JSONB for flexible querying)
    tool_usage JSONB,    -- {"terminal": {"count": 5, "errors": 1}, "read_file": {"count": 3, "errors": 0}}

    -- Quality signals
    has_retry_pattern BOOLEAN DEFAULT FALSE,  -- same tool+input called multiple times
    has_error_recovery BOOLEAN DEFAULT FALSE, -- error followed by successful tool call
    longest_tool_ms BIGINT,                   -- slowest tool call duration

    -- Raw summary for LLM analysis (optional, filled by background job)
    summary TEXT,
    improvement_hint TEXT,

    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_task_analysis_task_id ON task_analysis(task_id);
CREATE INDEX IF NOT EXISTS idx_task_analysis_failure_class ON task_analysis(failure_class) WHERE failure_class IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_task_analysis_created_at ON task_analysis(created_at DESC);
