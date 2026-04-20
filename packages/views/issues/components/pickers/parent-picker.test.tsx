import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
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

  // AC1 — search input renders
  it("AC1: renders a visible search input for filtering issues", () => {
    renderInProvider(
      <ParentSubMenuContent
        currentIssueId="issue-3"
        parentIssueId={null}
        onUpdate={vi.fn()}
      />,
    );
    expect(screen.getByRole("textbox", { name: /search issues/i })).toBeInTheDocument();
  });

  // AC4 — selecting calls onUpdate with parent_issue_id
  it("AC4: clicking an issue calls onUpdate with the correct parent_issue_id", async () => {
    const user = userEvent.setup();
    const onUpdate = vi.fn();
    renderInProvider(
      <ParentSubMenuContent
        currentIssueId="issue-3"
        parentIssueId={null}
        onUpdate={onUpdate}
      />,
    );
    await user.click(screen.getByRole("button", { name: /unrelated issue/i }));
    expect(onUpdate).toHaveBeenCalledWith({ parent_issue_id: "issue-4" });
  });

  // AC5 — selected parent shows check indicator
  it("AC5: the currently selected parent issue renders a check indicator; others do not", () => {
    renderInProvider(
      <ParentSubMenuContent
        currentIssueId="issue-3"
        parentIssueId="issue-1"
        onUpdate={vi.fn()}
      />,
    );
    const selectedButton = screen.getByRole("button", { name: /root issue/i });
    const otherButton = screen.getByRole("button", { name: /unrelated issue/i });
    expect(selectedButton.querySelectorAll("svg").length).toBeGreaterThan(
      otherButton.querySelectorAll("svg").length,
    );
  });
});

// ---------------------------------------------------------------------------
// Tests: AC1 — picker renders and shows search input
// ---------------------------------------------------------------------------

describe("ParentSubMenuContent — AC1: rendering", () => {
  it("AC1: renders a search input for filtering issues", () => {
    renderInProvider(
      <ParentSubMenuContent currentIssueId="issue-1" parentIssueId={null} onUpdate={vi.fn()} />,
    );
    expect(screen.getByPlaceholderText("Search issues...")).toBeInTheDocument();
  });

  it("AC1: renders candidate issues in the list on mount", () => {
    renderInProvider(
      <ParentSubMenuContent currentIssueId="issue-1" parentIssueId={null} onUpdate={vi.fn()} />,
    );
    expect(screen.getByText("Unrelated issue")).toBeInTheDocument();
  });
});

// ---------------------------------------------------------------------------
// Tests: AC4 — selecting a parent saves the relationship immediately
// ---------------------------------------------------------------------------

describe("ParentSubMenuContent — AC4: selection saves parent relationship", () => {
  it("AC4: clicking an issue calls onUpdate with the selected parent_issue_id", async () => {
    const onUpdate = vi.fn();
    const user = userEvent.setup();
    renderInProvider(
      <ParentSubMenuContent currentIssueId="issue-1" parentIssueId={null} onUpdate={onUpdate} />,
    );
    await user.click(screen.getByText("Unrelated issue"));
    expect(onUpdate).toHaveBeenCalledWith({ parent_issue_id: "issue-4" });
  });

  it("AC4: onUpdate is not called before any selection is made", () => {
    const onUpdate = vi.fn();
    renderInProvider(
      <ParentSubMenuContent currentIssueId="issue-1" parentIssueId={null} onUpdate={onUpdate} />,
    );
    expect(onUpdate).not.toHaveBeenCalled();
  });
});

// ---------------------------------------------------------------------------
// Tests: AC5 — success feedback: selected parent shows a visual indicator
// ---------------------------------------------------------------------------

describe("ParentSubMenuContent — AC5: success feedback", () => {
  it("AC5: shows a check indicator next to the currently selected parent issue", () => {
    renderInProvider(
      <ParentSubMenuContent currentIssueId="issue-1" parentIssueId="issue-4" onUpdate={vi.fn()} />,
    );
    const parentButton = screen.getByText("Unrelated issue").closest("button")!;
    expect(parentButton.querySelector("svg.ml-auto")).toBeInTheDocument();
  });

  it("AC5: does not show a check indicator when no parent is set", () => {
    renderInProvider(
      <ParentSubMenuContent currentIssueId="issue-1" parentIssueId={null} onUpdate={vi.fn()} />,
    );
    const parentButton = screen.getByText("Unrelated issue").closest("button")!;
    expect(parentButton.querySelector("svg.ml-auto")).not.toBeInTheDocument();
  });

  it("AC5: check icon is present via data-testid for the selected parent issue", () => {
    renderInProvider(
      <ParentSubMenuContent
        currentIssueId="issue-3"
        parentIssueId="issue-1"
        onUpdate={vi.fn()}
      />,
    );
    expect(screen.getByTestId("parent-selected-check")).toBeInTheDocument();
  });

  it("AC5: check icon is absent via data-testid when no parent is selected", () => {
    renderInProvider(
      <ParentSubMenuContent currentIssueId="issue-3" parentIssueId={null} onUpdate={vi.fn()} />,
    );
    expect(screen.queryByTestId("parent-selected-check")).not.toBeInTheDocument();
  });
});

// ---------------------------------------------------------------------------
// Tests: AC2 — typing filters results by title and identifier
// ---------------------------------------------------------------------------

describe("ParentSubMenuContent — AC2: search filtering", () => {
  it("AC2: typing filters issues by title (case-insensitive)", () => {
    renderInProvider(
      <ParentSubMenuContent currentIssueId="issue-4" parentIssueId={null} onUpdate={vi.fn()} />,
    );
    fireEvent.change(screen.getByRole("textbox", { name: /search issues/i }), {
      target: { value: "grandchild" },
    });
    expect(screen.getByText("Grandchild of root")).toBeInTheDocument();
    expect(screen.queryByText("Root issue")).not.toBeInTheDocument();
    expect(screen.queryByText("Child of root")).not.toBeInTheDocument();
  });

  it("AC2: typing filters issues by identifier (case-insensitive)", () => {
    renderInProvider(
      <ParentSubMenuContent currentIssueId="issue-4" parentIssueId={null} onUpdate={vi.fn()} />,
    );
    fireEvent.change(screen.getByRole("textbox", { name: /search issues/i }), {
      target: { value: "TES-1" },
    });
    expect(screen.getByText("Root issue")).toBeInTheDocument();
    expect(screen.queryByText("Child of root")).not.toBeInTheDocument();
    expect(screen.queryByText("Grandchild of root")).not.toBeInTheDocument();
  });
});

// ---------------------------------------------------------------------------
// Tests: AC3 — results display identifier + title
// ---------------------------------------------------------------------------

describe("ParentSubMenuContent — AC3: result display", () => {
  it("AC3: each result displays both identifier and title", () => {
    renderInProvider(
      <ParentSubMenuContent currentIssueId="issue-4" parentIssueId={null} onUpdate={vi.fn()} />,
    );
    expect(screen.getByText("TES-1")).toBeInTheDocument();
    expect(screen.getByText("Root issue")).toBeInTheDocument();
  });
});

// ---------------------------------------------------------------------------
// Tests: AC6 — current issue excluded from results (self-parent prevention)
// ---------------------------------------------------------------------------

describe("ParentSubMenuContent — AC6: self-parent prevention", () => {
  it("AC6: the current issue does not appear in the selectable parent list", () => {
    renderInProvider(
      <ParentSubMenuContent currentIssueId="issue-2" parentIssueId={null} onUpdate={vi.fn()} />,
    );
    expect(screen.queryByText("Child of root")).not.toBeInTheDocument();
    expect(screen.getByText("Root issue")).toBeInTheDocument();
    expect(screen.getByText("Unrelated issue")).toBeInTheDocument();
  });
});
