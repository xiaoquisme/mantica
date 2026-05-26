-- Context cache operations for sharing context across agent stages

-- name: GetTaskContextCache :one
SELECT context_cache FROM agent_task_queue
WHERE id = $1;

-- name: UpdateTaskContextCache :one
UPDATE agent_task_queue
SET context_cache = $2
WHERE id = $1
RETURNING *;

-- name: GetPriorTaskContextCache :one
-- Gets the context cache from the most recent completed task for the same issue
-- This allows the current task to inherit context from previous stages
SELECT atq.context_cache FROM agent_task_queue atq
WHERE atq.issue_id = $1 
  AND atq.status = 'completed' 
  AND atq.context_cache IS NOT NULL
ORDER BY atq.completed_at DESC
LIMIT 1;
