-- name: CreateScheduledTask :one
INSERT INTO scheduled_task (workspace_id, name, agent_id, schedule, prompt, enabled, next_run_at, created_by)
VALUES ($1, $2, $3, $4, $5, $6, sqlc.narg(next_run_at), $7)
RETURNING *;

-- name: GetScheduledTask :one
SELECT * FROM scheduled_task WHERE id = $1;

-- name: ListScheduledTasks :many
SELECT * FROM scheduled_task
WHERE workspace_id = $1
ORDER BY created_at ASC;

-- name: UpdateScheduledTask :one
UPDATE scheduled_task SET
    name = COALESCE(sqlc.narg('name'), name),
    agent_id = COALESCE(sqlc.narg('agent_id'), agent_id),
    schedule = COALESCE(sqlc.narg('schedule'), schedule),
    prompt = COALESCE(sqlc.narg('prompt'), prompt),
    enabled = COALESCE(sqlc.narg('enabled'), enabled),
    next_run_at = sqlc.narg('next_run_at'),
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: DeleteScheduledTask :exec
DELETE FROM scheduled_task WHERE id = $1;

-- name: ListDueScheduledTasks :many
SELECT * FROM scheduled_task
WHERE enabled = true AND next_run_at <= now()
ORDER BY next_run_at ASC;

-- name: MarkScheduledTaskRun :one
UPDATE scheduled_task SET
    last_run_at = now(),
    next_run_at = sqlc.narg(next_run_at),
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: CreateScheduledAgentTask :one
INSERT INTO agent_task_queue (agent_id, runtime_id, issue_id, status, priority, scheduled_task_id)
VALUES ($1, $2, NULL, 'queued', $3, $4)
RETURNING *;
