-- Revert to include 'in_dev' for backwards compatibility
ALTER TABLE issue DROP CONSTRAINT IF EXISTS issue_status_check;

ALTER TABLE issue ADD CONSTRAINT issue_status_check 
  CHECK (status IN ('backlog', 'todo', 'in_dev', 'done', 'blocked', 'cancelled'));
