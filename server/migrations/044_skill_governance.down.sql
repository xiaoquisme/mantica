ALTER TABLE skill
DROP COLUMN IF EXISTS quality_score,
DROP COLUMN IF EXISTS source_task_id,
DROP COLUMN IF EXISTS usage_count,
DROP COLUMN IF EXISTS success_count,
DROP COLUMN IF EXISTS failure_count,
DROP COLUMN IF EXISTS last_used_at,
DROP COLUMN IF EXISTS pinned,
DROP COLUMN IF EXISTS archived_at;

DROP INDEX IF EXISTS idx_skill_quality;
DROP INDEX IF EXISTS idx_skill_archived;
