-- Migration 050: Agent Composition
-- Defines parent-child relationships between agents (main, terminal, summary)

CREATE TABLE agent_composition (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    parent_agent_id UUID NOT NULL REFERENCES agent(id) ON DELETE CASCADE,
    child_agent_id UUID NOT NULL REFERENCES agent(id) ON DELETE CASCADE,
    role TEXT NOT NULL CHECK (role IN ('main', 'terminal', 'summary', 'orchestrator')),
    priority INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(parent_agent_id, child_agent_id, role)
);

-- Index for faster lookups
CREATE INDEX idx_agent_composition_parent ON agent_composition(parent_agent_id);
CREATE INDEX idx_agent_composition_child ON agent_composition(child_agent_id);