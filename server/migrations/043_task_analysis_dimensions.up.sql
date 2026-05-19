-- Add new analysis dimensions
ALTER TABLE task_analysis ADD COLUMN IF NOT EXISTS output_language TEXT;
ALTER TABLE task_analysis ADD COLUMN IF NOT EXISTS output_length INT;
ALTER TABLE task_analysis ADD COLUMN IF NOT EXISTS tool_efficiency DOUBLE PRECISION;
ALTER TABLE task_analysis ADD COLUMN IF NOT EXISTS first_attempt_success BOOLEAN;
ALTER TABLE task_analysis ADD COLUMN IF NOT EXISTS communication_quality DOUBLE PRECISION;
