-- Task Analysis CRUD

-- name: CreateTaskAnalysis :one
INSERT INTO task_analysis (
    task_id, tool_count, error_count, unique_tools,
    total_duration_ms, message_count,
    failure_class, failure_detail,
    tool_usage, has_retry_pattern, has_error_recovery,
    longest_tool_ms, summary, improvement_hint,
    output_language, output_length, tool_efficiency,
    first_attempt_success, communication_quality
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19)
RETURNING *;

-- name: GetTaskAnalysis :one
SELECT * FROM task_analysis WHERE task_id = $1;

-- name: ListTaskAnalysesByAgent :many
SELECT ta.*, atq.agent_id, atq.issue_id, atq.status as task_status
FROM task_analysis ta
JOIN agent_task_queue atq ON ta.task_id = atq.id
WHERE atq.agent_id = $1
ORDER BY ta.created_at DESC
LIMIT $2;

-- name: ListFailedAnalyses :many
SELECT ta.*, atq.agent_id
FROM task_analysis ta
JOIN agent_task_queue atq ON ta.task_id = atq.id
WHERE ta.failure_class IS NOT NULL
ORDER BY ta.created_at DESC
LIMIT $1;

-- name: GetFailureClassCounts :many
SELECT failure_class, COUNT(*) as cnt
FROM task_analysis
WHERE failure_class IS NOT NULL
AND created_at > $1
GROUP BY failure_class
ORDER BY cnt DESC;

-- name: GetToolUsageStats :many
SELECT
    tool_name,
    SUM((tool_value->>'count')::int) as total_count,
    SUM((tool_value->>'errors')::int) as total_errors
FROM task_analysis,
     jsonb_each(tool_usage) AS tool_entry(tool_name, tool_value)
WHERE created_at > $1
GROUP BY tool_name
ORDER BY total_count DESC;

-- name: UpdateTaskAnalysisSummary :exec
UPDATE task_analysis
SET summary = $2, improvement_hint = $3
WHERE task_id = $1;
