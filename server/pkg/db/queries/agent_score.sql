-- Agent Score CRUD

-- name: GetAgentScore :one
SELECT * FROM agent_score
WHERE agent_id = $1 AND workspace_id = $2;

-- name: UpsertAgentScore :one
INSERT INTO agent_score (
    agent_id, workspace_id, overall_score, task_type_scores,
    total_tasks, successful_tasks, failed_tasks, success_rate,
    avg_tool_count, avg_error_count, avg_error_rate,
    score_trend, trend_samples, updated_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, now())
ON CONFLICT (agent_id, workspace_id) DO UPDATE SET
    overall_score = EXCLUDED.overall_score,
    task_type_scores = EXCLUDED.task_type_scores,
    total_tasks = EXCLUDED.total_tasks,
    successful_tasks = EXCLUDED.successful_tasks,
    failed_tasks = EXCLUDED.failed_tasks,
    success_rate = EXCLUDED.success_rate,
    avg_tool_count = EXCLUDED.avg_tool_count,
    avg_error_count = EXCLUDED.avg_error_count,
    avg_error_rate = EXCLUDED.avg_error_rate,
    score_trend = EXCLUDED.score_trend,
    trend_samples = EXCLUDED.trend_samples,
    updated_at = now()
RETURNING *;

-- name: ListAgentScoresByWorkspace :many
SELECT as2.*, a.name as agent_name
FROM agent_score as2
JOIN agent a ON as2.agent_id = a.id
WHERE as2.workspace_id = $1
ORDER BY as2.overall_score DESC;

-- name: CreateScoreHistory :one
INSERT INTO agent_score_history (
    agent_id, workspace_id, task_id, task_type,
    score_before, score_after, score_delta,
    success, tool_count, error_count
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING *;

-- name: ListScoreHistory :many
SELECT * FROM agent_score_history
WHERE agent_id = $1
ORDER BY created_at DESC
LIMIT $2;

-- name: GetScoreHistorySince :many
SELECT * FROM agent_score_history
WHERE agent_id = $1 AND created_at > $2
ORDER BY created_at ASC;
