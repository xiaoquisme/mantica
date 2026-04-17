DROP INDEX IF EXISTS idx_issue_swimlane;
ALTER TABLE issue DROP COLUMN IF EXISTS swimlane_id;
DROP TABLE IF EXISTS swimlane;
