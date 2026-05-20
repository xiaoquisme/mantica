/**
 * Tests for TES-120 AC1 — Parent submenu trigger renders in ListRowContextMenu
 * and opening it mounts ParentSubMenuContent.
 */
import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { Issue } from "@mantica/core/types";

// ---------------------------------------------------------------------------
// Fixtures
// ---------------------------------------------------------------------------

const mockIssue: Issue = {
  id: "issue-1",
  workspace_id: "ws-1",
  number: 1,
  identifier: "TES-1",
  title: "Test Issue",
  description: null,
  status: "backlog",
  priority: "medium",
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
};

// ---------------------------------------------------------------------------
// Mocks
// ---------------------------------------------------------------------------

vi.mock("@mantica/core/auth", () => ({
  useAuthStore: (selector?: any) => {
    const state = { user: { id: "user-1" } };
    return selector ? selector(state) : state;
  },
}));

vi.mock("@mantica/core/hooks", () => ({
  useWorkspaceId: () => "ws-1",
  WorkspaceIdProvider: ({ children }: { children: React.ReactNode }) => children,
}));

vi.mock("@mantica/core/workspace/queries", () => ({
  memberListOptions: () => ({ queryKey: ["members"], queryFn: () => Promise.resolve([]) }),
  agentListOptions: () => ({ queryKey: ["agents"], queryFn: () => Promise.resolve([]) }),
}));

vi.mock("@mantica/core/projects/queries", () => ({
  projectListOptions: () => ({ queryKey: ["projects"], queryFn: () => Promise.resolve([]) }),
}));

vi.mock("@mantica/core/issues/mutations", () => ({
  useUpdateIssue: () => ({ mutate: vi.fn() }),
}));

vi.mock("@mantica/core/issues/stores/selection-store", () => ({
  useIssueSelectionStore: (selector: any) => {
    const state = { selectedIds: new Set(), toggle: vi.fn() };
    return selector(state);
  },
}));

vi.mock("../../navigation", () => ({
  AppLink: ({ children, href, ...props }: any) => (
    <a href={href} {...props}>
      {children}
    </a>
  ),
}));

vi.mock("../../common/actor-avatar", () => ({
  ActorAvatar: () => null,
}));

vi.mock("sonner", () => ({
  toast: { error: vi.fn(), success: vi.fn() },
}));

vi.mock("@mantica/core/issues/config", () => ({
  ALL_STATUSES: ["backlog", "in_dev", "done"],
  STATUS_CONFIG: {
    backlog: { label: "Backlog" },
    in_dev: { label: "In Dev" },
    done: { label: "Done" },
  },
  PRIORITY_ORDER: ["none", "low", "medium", "high", "urgent"],
  PRIORITY_CONFIG: {
    none: { label: "None", badgeBg: "", badgeText: "" },
    low: { label: "Low", badgeBg: "", badgeText: "" },
    medium: { label: "Medium", badgeBg: "", badgeText: "" },
    high: { label: "High", badgeBg: "", badgeText: "" },
    urgent: { label: "Urgent", badgeBg: "", badgeText: "" },
  },
}));

// LabelPicker has complex internal dependencies unrelated to what we're testing.
// ParentSubMenuContent is stubbed with a minimal test double that exposes just
// the search input — the real component is thoroughly tested in parent-picker.test.tsx.
vi.mock("./pickers", () => ({
  LabelPicker: () => null,
  canAssignAgent: () => false,
  ParentSubMenuContent: () => (
    <div>
      <input
        type="text"
        placeholder="Search issues..."
        aria-label="Search issues"
        data-testid="parent-search-input"
      />
    </div>
  ),
}));

// Stateful dropdown mock: DropdownMenuSubContent is hidden until its sibling
// DropdownMenuSubTrigger is clicked, matching real open/close behavior.
// Each DropdownMenuSub manages its own open state via React context.
vi.mock("@mantica/ui/components/ui/dropdown-menu", () => {
  const React = require("react");
  const SubCtx = React.createContext({ open: false, toggle: () => {} });
  return {
    DropdownMenu: ({ children }: any) => <div data-testid="dropdown-menu">{children}</div>,
    DropdownMenuTrigger: ({ render: r, children, ...props }: any) => (
      <div data-testid="dropdown-trigger" {...props}>
        {r ?? children}
      </div>
    ),
    DropdownMenuContent: ({ children }: any) => (
      <div data-testid="dropdown-content">{children}</div>
    ),
    DropdownMenuItem: ({ children, onClick }: any) => (
      <button onClick={onClick}>{children}</button>
    ),
    DropdownMenuSeparator: () => <hr />,
    DropdownMenuSub: ({ children }: any) => {
      const [open, setOpen] = React.useState(false);
      return (
        <SubCtx.Provider value={{ open, toggle: () => setOpen((o: boolean) => !o) }}>
          <div>{children}</div>
        </SubCtx.Provider>
      );
    },
    DropdownMenuSubTrigger: ({ children }: any) => {
      const { toggle } = React.useContext(SubCtx);
      return (
        <button data-testid="dropdown-sub-trigger" onClick={toggle}>
          {children}
        </button>
      );
    },
    DropdownMenuSubContent: ({ children }: any) => {
      const { open } = React.useContext(SubCtx);
      return open ? <div data-testid="dropdown-sub-content">{children}</div> : null;
    },
    DropdownMenuGroup: ({ children }: any) => <div>{children}</div>,
    DropdownMenuLabel: ({ children }: any) => <div>{children}</div>,
  };
});

// ---------------------------------------------------------------------------
// Render helper
// ---------------------------------------------------------------------------

import { ListRow } from "./list-row";

function renderRow(issue: Issue = mockIssue) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={qc}>
      <ListRow issue={issue} />
    </QueryClientProvider>,
  );
}

// ---------------------------------------------------------------------------
// Tests: TES-120 — AC1: Parent submenu trigger renders in ListRowContextMenu
// ---------------------------------------------------------------------------

describe("ListRowContextMenu — Parent submenu (TES-102 AC1)", () => {
  it("AC1: renders a Parent sub-menu trigger in the context menu", () => {
    renderRow();
    expect(screen.getByText("Parent")).toBeInTheDocument();
  });

  it("AC1: the Parent trigger is a DropdownMenuSubTrigger button", () => {
    renderRow();
    const triggers = screen.getAllByTestId("dropdown-sub-trigger");
    const parentTrigger = triggers.find((el) => el.textContent?.includes("Parent"));
    expect(parentTrigger).toBeDefined();
  });

  it("AC1: ParentSubMenuContent search input is not visible before opening the submenu", () => {
    renderRow();
    expect(screen.queryByTestId("parent-search-input")).not.toBeInTheDocument();
  });

  it("AC1: clicking the Parent trigger mounts ParentSubMenuContent with search input", async () => {
    const user = userEvent.setup();
    renderRow();

    const triggers = screen.getAllByTestId("dropdown-sub-trigger");
    const parentTrigger = triggers.find((el) => el.textContent?.includes("Parent"))!;
    await user.click(parentTrigger);

    expect(screen.getByTestId("parent-search-input")).toBeInTheDocument();
  });

  it("AC1: the search input inside ParentSubMenuContent is accessible by role", async () => {
    const user = userEvent.setup();
    renderRow();

    const triggers = screen.getAllByTestId("dropdown-sub-trigger");
    const parentTrigger = triggers.find((el) => el.textContent?.includes("Parent"))!;
    await user.click(parentTrigger);

    expect(screen.getByRole("textbox", { name: /search issues/i })).toBeInTheDocument();
  });
});
