-- Add context_cache column to store shared context across agent stages
-- This reduces redundant API calls and context fetching

ALTER TABLE agent_task_queue 
ADD COLUMN IF NOT EXISTS context_cache JSONB DEFAULT NULL;

-- Create index for efficient context lookups
CREATE INDEX IF NOT EXISTS idx_task_context_cache 
ON agent_task_queue USING GIN (context_cache)
WHERE context_cache IS NOT NULL;

-- Add comment explaining the cache structure
COMMENT ON COLUMN agent_task_queue.context_cache IS 'Shared context cache across agent stages. Contains: {issue: {}, comments: [], code_snippets: [], memory: {}}';
