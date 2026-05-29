-- name: CreateSubTask :one
INSERT INTO agent_task_queue (
    agent_id, runtime_id, issue_id, status, priority,
    parent_task_id, subagent_role, task_depth
) VALUES ($1, $2, $3, 'queued', $4, $5, $6, $7)
RETURNING *;

-- name: GetSubTasksByParent :many
SELECT * FROM agent_task_queue
WHERE parent_task_id = $1
ORDER BY created_at ASC;

-- name: UpdateSubTaskStatus :one
UPDATE agent_task_queue
SET status = $2, updated_at = now()
WHERE id = $1 AND parent_task_id IS NOT NULL
RETURNING *;

-- name: CountCompletedSubTasks :one
SELECT count(*) FROM agent_task_queue
WHERE parent_task_id = $1 AND status = 'completed';

-- name: CountFailedSubTasks :one
SELECT count(*) FROM agent_task_queue
WHERE parent_task_id = $1 AND status = 'failed';

-- name: UpdateParentTaskSubAgentCounts :one
UPDATE agent_task_queue
SET waiting_for_subagents = $2,
    completed_subagents = $3,
    failed_subagents = $4,
    total_subagents = $5,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: GetParentTaskWithSubTasks :one
SELECT * FROM agent_task_queue
WHERE id = $1;

-- name: MarkParentTaskCompletedIfAllSubTasksDone :exec
UPDATE agent_task_queue
SET status = 'completed',
    updated_at = now()
WHERE id = $1
  AND waiting_for_subagents = true
  AND total_subagents > 0
  AND completed_subagents + failed_subagents = total_subagents;

-- name: ListTasksByParentAndRole :many
SELECT * FROM agent_task_queue
WHERE parent_task_id = $1 AND subagent_role = $2
ORDER BY created_at ASC;