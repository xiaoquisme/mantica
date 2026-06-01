-- Simplify issue status to 6 values: backlog, todo, doing, done, blocked, cancelled
-- Map old statuses to new ones:
--   classifying, ready_analyze, in_analyze, ready_arch_design, in_arch_design → todo
--   ready_dev, ready_review, in_review, ready_test, in_test → doing

-- First, migrate existing data
UPDATE issue SET status = 'todo' WHERE status IN (
  'classifying', 'ready_analyze', 'in_analyze', 
  'ready_arch_design', 'in_arch_design'
);

UPDATE issue SET status = 'doing' WHERE status IN (
  'ready_dev', 'ready_review', 'in_review', 
  'ready_test', 'in_test'
);

-- Update the CHECK constraint
ALTER TABLE issue DROP CONSTRAINT IF EXISTS issue_status_check;

ALTER TABLE issue ADD CONSTRAINT issue_status_check 
  CHECK (status IN ('backlog', 'todo', 'doing', 'done', 'blocked', 'cancelled'));
