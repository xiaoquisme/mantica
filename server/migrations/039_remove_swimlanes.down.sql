CREATE TABLE swimlane (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    name TEXT NOT NULL CHECK (char_length(name) <= 255),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_swimlane_workspace ON swimlane(workspace_id);

ALTER TABLE issue ADD COLUMN swimlane_id UUID REFERENCES swimlane(id) ON DELETE SET NULL;
CREATE INDEX idx_issue_swimlane ON issue(swimlane_id);
