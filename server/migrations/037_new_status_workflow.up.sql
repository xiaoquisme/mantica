-- Delete issues with old statuses that no longer exist
DELETE FROM issue WHERE status IN ('todo', 'in_progress');

-- Update CHECK constraint
ALTER TABLE issue DROP CONSTRAINT IF EXISTS issue_status_check;
ALTER TABLE issue ADD CONSTRAINT issue_status_check
  CHECK (status IN ('backlog', 'classifying', 'ready_analyze', 'in_analyze', 'ready_arch_design', 'in_arch_design', 'ready_dev', 'in_dev', 'ready_review', 'in_review', 'ready_test', 'in_test', 'done', 'blocked', 'cancelled'));
