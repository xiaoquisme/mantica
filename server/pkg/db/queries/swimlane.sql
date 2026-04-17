-- name: ListSwimlanes :many
SELECT * FROM swimlane
WHERE workspace_id = $1
ORDER BY created_at ASC;

-- name: GetSwimlane :one
SELECT * FROM swimlane
WHERE id = $1;

-- name: GetSwimlaneInWorkspace :one
SELECT * FROM swimlane
WHERE id = $1 AND workspace_id = $2;

-- name: CreateSwimlane :one
INSERT INTO swimlane (workspace_id, name)
VALUES ($1, $2)
RETURNING *;

-- name: UpdateSwimlane :one
UPDATE swimlane SET
    name = COALESCE(sqlc.narg('name'), name),
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: DeleteSwimlane :exec
DELETE FROM swimlane WHERE id = $1;
