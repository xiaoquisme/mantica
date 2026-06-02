-- Fix CHECK constraint to use 'doing' instead of 'in_dev'
-- and migrate any remaining old status values

-- First, update any remaining old status values
UPDATE issue SET status = 'doing' WHERE status = 'in_dev';

-- Drop the old constraint
ALTER TABLE issue DROP CONSTRAINT IF EXISTS issue_status_check;

-- Create new constraint with correct values
ALTER TABLE issue ADD CONSTRAINT issue_status_check 
  CHECK (status IN ('backlog', 'todo', 'doing', 'done', 'blocked', 'cancelled'));
