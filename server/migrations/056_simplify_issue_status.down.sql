-- Revert to original status values
-- This is a lossy migration - cannot recover original granular statuses

ALTER TABLE issue DROP CONSTRAINT IF EXISTS issue_status_check;

ALTER TABLE issue ADD CONSTRAINT issue_status_check 
  CHECK (status IN (
    'backlog', 'classifying', 'ready_analyze', 'in_analyze',
    'ready_arch_design', 'in_arch_design', 'ready_dev', 'doing',
    'ready_review', 'in_review', 'ready_test', 'in_test',
    'done', 'blocked', 'cancelled'
  ));
