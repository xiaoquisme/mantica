-- name: GetWorkspaceLabels :many
SELECT id, workspace_id, name, color
FROM issue_label
WHERE workspace_id = $1
ORDER BY name ASC;

-- name: GetIssueLabels :many
SELECT il.id, il.workspace_id, il.name, il.color
FROM issue_label il
JOIN issue_to_label itl ON itl.label_id = il.id
WHERE itl.issue_id = $1
ORDER BY il.name ASC;

-- name: GetLabelsByIssueIDs :many
SELECT itl.issue_id, il.id, il.workspace_id, il.name, il.color
FROM issue_label il
JOIN issue_to_label itl ON itl.label_id = il.id
WHERE itl.issue_id = ANY($1::uuid[])
ORDER BY il.name ASC;

-- name: DeleteIssueLabels :exec
DELETE FROM issue_to_label WHERE issue_id = $1;

-- name: AddIssueLabel :exec
INSERT INTO issue_to_label (issue_id, label_id) VALUES ($1, $2)
ON CONFLICT DO NOTHING;
