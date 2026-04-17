DROP INDEX IF EXISTS idx_issue_swimlane;
ALTER TABLE issue DROP COLUMN IF EXISTS swimlane_id;
DROP INDEX IF EXISTS idx_swimlane_workspace;
DROP TABLE IF EXISTS swimlane;
