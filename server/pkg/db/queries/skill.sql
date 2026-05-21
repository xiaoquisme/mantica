-- Skill CRUD

-- name: ListSkillsByWorkspace :many
SELECT * FROM skill
WHERE workspace_id = $1
ORDER BY name ASC;

-- name: GetSkill :one
SELECT * FROM skill
WHERE id = $1;

-- name: GetSkillInWorkspace :one
SELECT * FROM skill
WHERE id = $1 AND workspace_id = $2;

-- name: CreateSkill :one
INSERT INTO skill (workspace_id, name, description, content, config, created_by)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: UpdateSkill :one
UPDATE skill SET
    name = COALESCE(sqlc.narg('name'), name),
    description = COALESCE(sqlc.narg('description'), description),
    content = COALESCE(sqlc.narg('content'), content),
    config = COALESCE(sqlc.narg('config'), config),
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: DeleteSkill :exec
DELETE FROM skill WHERE id = $1;

-- Skill File CRUD

-- name: ListSkillFiles :many
SELECT * FROM skill_file
WHERE skill_id = $1
ORDER BY path ASC;

-- name: GetSkillFile :one
SELECT * FROM skill_file
WHERE id = $1;

-- name: UpsertSkillFile :one
INSERT INTO skill_file (skill_id, path, content)
VALUES ($1, $2, $3)
ON CONFLICT (skill_id, path) DO UPDATE SET
    content = EXCLUDED.content,
    updated_at = now()
RETURNING *;

-- name: DeleteSkillFile :exec
DELETE FROM skill_file WHERE id = $1;

-- name: DeleteSkillFilesBySkill :exec
DELETE FROM skill_file WHERE skill_id = $1;

-- Agent-Skill junction

-- name: ListAgentSkills :many
SELECT s.* FROM skill s
JOIN agent_skill ask ON ask.skill_id = s.id
WHERE ask.agent_id = $1
ORDER BY s.name ASC;

-- name: AddAgentSkill :exec
INSERT INTO agent_skill (agent_id, skill_id)
VALUES ($1, $2)
ON CONFLICT DO NOTHING;

-- name: RemoveAgentSkill :exec
DELETE FROM agent_skill
WHERE agent_id = $1 AND skill_id = $2;

-- name: RemoveAllAgentSkills :exec
DELETE FROM agent_skill WHERE agent_id = $1;

-- name: ListAgentSkillsByWorkspace :many
SELECT ask.agent_id, s.id, s.name, s.description
FROM agent_skill ask
JOIN skill s ON s.id = ask.skill_id
WHERE s.workspace_id = $1
ORDER BY s.name ASC;

-- Skill Governance Queries

-- name: UpdateSkillQuality :exec
UPDATE skill SET quality_score = $2, updated_at = now() WHERE id = $1;

-- name: RecordSkillUsage :exec
UPDATE skill SET usage_count = usage_count + 1, last_used_at = now(), updated_at = now() WHERE id = $1;

-- name: RecordSkillSuccess :exec
UPDATE skill SET success_count = success_count + 1, quality_score = LEAST(100, quality_score + 1), updated_at = now() WHERE id = $1;

-- name: RecordSkillFailure :exec
UPDATE skill SET failure_count = failure_count + 1, quality_score = GREATEST(0, quality_score - 2), updated_at = now() WHERE id = $1;

-- name: PinSkill :exec
UPDATE skill SET pinned = TRUE, updated_at = now() WHERE id = $1;

-- name: UnpinSkill :exec
UPDATE skill SET pinned = FALSE, updated_at = now() WHERE id = $1;

-- name: ArchiveSkill :exec
UPDATE skill SET archived_at = now(), updated_at = now() WHERE id = $1 AND pinned = FALSE;

-- name: ListStaleSkills :many
SELECT * FROM skill
WHERE archived_at IS NULL
AND pinned = FALSE
AND (quality_score < 30 OR (last_used_at IS NOT NULL AND last_used_at < now() - interval '30 days' AND quality_score < 60))
ORDER BY quality_score ASC;

-- name: ListSimilarSkills :many
SELECT * FROM skill
WHERE workspace_id = $1
AND archived_at IS NULL
AND id != $2
AND (name ILIKE '%' || $3 || '%' OR $3 = ANY(string_to_array(name, '/')));

-- name: ListSkillsByQuality :many
SELECT s.*, as2.agent_id IS NOT NULL as assigned_to_agent
FROM skill s
LEFT JOIN agent_skill as2 ON as2.skill_id = s.id
WHERE s.workspace_id = $1
AND s.archived_at IS NULL
ORDER BY s.quality_score DESC
LIMIT $2;
