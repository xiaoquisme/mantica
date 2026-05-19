ALTER TABLE task_analysis
DROP COLUMN IF EXISTS output_language,
DROP COLUMN IF EXISTS output_length,
DROP COLUMN IF EXISTS tool_efficiency,
DROP COLUMN IF EXISTS first_attempt_success,
DROP COLUMN IF EXISTS communication_quality;
