import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { Issue } from "@multica/core/types";
import { WorkspaceIdProvider } from "@multica/core/hooks";
import { ParentSubMenuContent } from "./parent-picker";

// ---------------------------------------------------------------------------
// Mocks
// ---------------------------------------------------------------------------

const mockIssues: Issue[] = [
  {
    id: "issue-1",
    workspace_id: "ws-1",
    number: 1,
    identifier: "TES-1",
    title: "Root issue",
    description: null,
    status: "backlog",
    priority: "none",
    assignee_type: null,
    assignee_id: null,
    creator_type: "member",
    creator_id: "user-1",
    parent_issue_id: null,
    project_id: null,
    position: 0,
    due_date: null,
    created_at: "2024-01-01T00:00:00Z",
    updated_at: "2024-01-01T00:00:00Z",
  },
  {
    id: "issue-2",
    workspace_id: "ws-1",
    number: 2,
    identifier: "TES-2",
    title: "Child of root",
    description: null,
    status: "backlog",
    priority: "none",
    assignee_type: null,
    assignee_id: null,
    creator_type: "member",
    creator_id: "user-1",
    parent_issue_id: "issue-1",
    project_id: null,
    position: 0,
    due_date: null,
    created_at: "2024-01-01T00:00:00Z",
    updated_at: "2024-01-01T00:00:00Z",
  },
  {
    id: "issue-3",
    workspace_id: "ws-1",
    number: 3,
    identifier: "TES-3",
    title: "Grandchild of root",
    description: null,
    status: "backlog",
    priority: "none",
    assignee_type: null,
    assignee_id: null,
    creator_type: "member",
    creator_id: "user-1",
    parent_issue_id: "issue-2",
    project_id: null,
    position: 0,
    due_date: null,
    created_at: "2024-01-01T00:00:00Z",
    updated_at: "2024-01-01T00:00:00Z",
  },
  {
    id: "issue-4",
    workspace_id: "ws-1",
    number: 4,
    identifier: "TES-4",
    title: "Unrelated issue",
    description: null,
    status: "backlog",
    priority: "none",
    assignee_type: null,
    assignee_id: null,
    creator_type: "member",
    creator_id: "user-1",
    parent_issue_id: null,
    project_id: null,
    position: 0,
    due_date: null,
    created_at: "2024-01-01T00:00:00Z",
    updated_at: "2024-01-01T00:00:00Z",
  },
];

vi.mock("@multica/core/issues/queries", () => ({
  issueListOptions: () => ({ queryKey: ["issues"], queryFn: () => mockIssues }),
}));

vi.mock("@multica/core/hooks", () => ({
  useWorkspaceId: () => "ws-1",
  WorkspaceIdProvider: ({ children }: { children: React.ReactNode }) => children,
}));

function renderInProvider(ui: React.ReactElement) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  qc.setQueryData(["issues"], mockIssues);
  return render(
    <QueryClientProvider client={qc}>
      <WorkspaceIdProvider workspaceId="ws-1">{ui}</WorkspaceIdProvider>
    </QueryClientProvider>,
  );
}

// ---------------------------------------------------------------------------
// Tests: AC7 — circular dependency prevention
// ---------------------------------------------------------------------------

describe("ParentSubMenuContent", () => {
  it("excludes the current issue from the selectable list", () => {
    renderInProvider(
      <ParentSubMenuContent
        currentIssueId="issue-1"
        parentIssueId={null}
        onUpdate={vi.fn()}
      />,
    );
    expect(screen.queryByText("Root issue")).not.toBeInTheDocument();
  });

  it("excludes direct children of the current issue to prevent circular dependency", () => {
    renderInProvider(
      <ParentSubMenuContent
        currentIssueId="issue-1"
        parentIssueId={null}
        onUpdate={vi.fn()}
      />,
    );
    expect(screen.queryByText("Child of root")).not.toBeInTheDocument();
  });

  it("excludes grandchildren of the current issue to prevent circular dependency", () => {
    renderInProvider(
      <ParentSubMenuContent
        currentIssueId="issue-1"
        parentIssueId={null}
        onUpdate={vi.fn()}
      />,
    );
    expect(screen.queryByText("Grandchild of root")).not.toBeInTheDocument();
  });

  it("still shows unrelated issues in the selectable list", () => {
    renderInProvider(
      <ParentSubMenuContent
        currentIssueId="issue-1"
        parentIssueId={null}
        onUpdate={vi.fn()}
      />,
    );
    expect(screen.getByText("Unrelated issue")).toBeInTheDocument();
  });
});
