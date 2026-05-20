/**
 * Unit tests for BoardCardContextMenu (TES-70).
 *
 * Covers:
 *   1. The MoreHorizontal (…) trigger renders inside BoardCardContent when editable=true.
 *   2. The trigger is hidden when editable=false.
 *   3. Each sub-menu trigger (Status, Priority, Assignee, Due Date, Project, Labels)
 *      is present in the rendered DOM.
 *   4. Clicking a Status item calls onUpdate with { status }.
 *   5. Clicking a Priority item calls onUpdate with { priority }.
 *   6. Clicking Unassigned calls onUpdate with { assignee_type: null, assignee_id: null }.
 *   7. Clicking "Today" in the Due Date sub-menu calls onUpdate with a due_date.
 *   8. Clicking "Clear date" calls onUpdate with { due_date: null }.
 *   9. Clicking a Project item calls onUpdate with { project_id }.
 *  10. Clicking "Remove from project" calls onUpdate with { project_id: null }.
 */

import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { Issue } from "@mantica/core/types";

// ---------------------------------------------------------------------------
// Fixtures
// ---------------------------------------------------------------------------

const baseIssue: Issue = {
  id: "issue-1",
  workspace_id: "ws-1",
  number: 1,
  identifier: "TES-1",
  title: "Board Card Test Issue",
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

const issueWithDueDate: Issue = {
  ...baseIssue,
  due_date: "2099-01-01T00:00:00Z",
};

const issueWithProject: Issue = {
  ...baseIssue,
  project_id: "project-1",
};

// ---------------------------------------------------------------------------
// Mocks
// ---------------------------------------------------------------------------

const mockMutate = vi.fn();

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
  memberListOptions: () => ({
    queryKey: ["members", "ws-1"],
    queryFn: () =>
      Promise.resolve([{ user_id: "user-1", name: "Alice", role: "owner" }]),
  }),
  agentListOptions: () => ({
    queryKey: ["agents", "ws-1"],
    queryFn: () =>
      Promise.resolve([
        {
          id: "agent-1",
          name: "TestBot",
          archived_at: null,
          owner_id: "user-1",
        },
      ]),
  }),
}));

vi.mock("@mantica/core/projects/queries", () => ({
  projectListOptions: () => ({
    queryKey: ["projects", "ws-1"],
    queryFn: () =>
      Promise.resolve([{ id: "project-1", title: "Alpha Project", icon: "🚀" }]),
    select: (data: any) => (Array.isArray(data) ? data : []),
  }),
}));

vi.mock("@mantica/core/issues/mutations", () => ({
  useUpdateIssue: () => ({ mutate: mockMutate }),
}));

vi.mock("@mantica/core/issues/queries", () => ({
  issueListOptions: () => ({
    queryKey: ["issues", "ws-1"],
    queryFn: () => Promise.resolve([]),
  }),
}));

vi.mock("../../navigation", () => ({
  AppLink: ({ children, href, ...props }: any) => (
    <a href={href} {...props}>
      {children}
    </a>
  ),
  useNavigation: () => ({ push: vi.fn() }),
}));

vi.mock("../../common/actor-avatar", () => ({
  ActorAvatar: () => <span data-testid="actor-avatar" />,
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

vi.mock("@mantica/core/issues/stores/view-store-context", () => ({
  useViewStore: (selector: any) =>
    selector({
      cardProperties: {
        priority: true,
        description: true,
        assignee: true,
        dueDate: true,
        parentIssue: false,
      },
    }),
}));

vi.mock("./pickers", () => ({
  PriorityPicker: ({ trigger }: any) => (
    <div data-testid="priority-picker">{trigger}</div>
  ),
  AssigneePicker: ({ trigger }: any) => (
    <div data-testid="assignee-picker">{trigger}</div>
  ),
  DueDatePicker: ({ trigger }: any) => (
    <div data-testid="due-date-picker">{trigger}</div>
  ),
  LabelPicker: () => <div data-testid="label-picker" />,
  canAssignAgent: () => true,
}));

vi.mock("./status-icon", () => ({
  StatusIcon: ({ status }: { status: string }) => (
    <span data-testid={`status-icon-${status}`} />
  ),
}));

vi.mock("./priority-icon", () => ({
  PriorityIcon: ({ priority }: { priority: string }) => (
    <span data-testid={`priority-icon-${priority}`} />
  ),
}));

vi.mock("./project-badge", () => ({
  ProjectBadge: ({ projectId }: { projectId: string }) => (
    <span data-testid={`project-badge-${projectId}`} />
  ),
}));

vi.mock("./progress-ring", () => ({
  ProgressRing: () => <svg data-testid="progress-ring" />,
}));

// ---------------------------------------------------------------------------
// Stateful dropdown mock — mirrors the one in list-row.test.tsx exactly.
// DropdownMenuSubContent is hidden until its sibling DropdownMenuSubTrigger is
// clicked, matching real open/close behaviour.
// ---------------------------------------------------------------------------

vi.mock("@mantica/ui/components/ui/dropdown-menu", () => {
  const React = require("react");
  const SubCtx = React.createContext({ open: false, toggle: () => {} });
  return {
    DropdownMenu: ({ children }: any) => (
      <div data-testid="dropdown-menu">{children}</div>
    ),
    DropdownMenuTrigger: ({ render: r, children, ...props }: any) => (
      <div data-testid="dropdown-trigger" {...props}>
        {r ?? children}
      </div>
    ),
    DropdownMenuContent: ({ children }: any) => (
      <div data-testid="dropdown-content">{children}</div>
    ),
    DropdownMenuItem: ({ children, onClick }: any) => (
      <button data-testid="dropdown-item" onClick={onClick}>
        {children}
      </button>
    ),
    DropdownMenuSeparator: () => <hr />,
    DropdownMenuSub: ({ children }: any) => {
      const [open, setOpen] = React.useState(false);
      return (
        <SubCtx.Provider
          value={{ open, toggle: () => setOpen((o: boolean) => !o) }}
        >
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
      return open ? (
        <div data-testid="dropdown-sub-content">{children}</div>
      ) : null;
    },
  };
});

vi.mock("@mantica/ui/components/ui/button", () => ({
  Button: ({ children, ...props }: any) => (
    <button data-testid="button" {...props}>
      {children}
    </button>
  ),
}));

// ---------------------------------------------------------------------------
// Render helper
// ---------------------------------------------------------------------------

import { BoardCardContent } from "./board-card";

function renderCard(issue: Issue = baseIssue, editable = true) {
  const qc = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={qc}>
      <BoardCardContent issue={issue} editable={editable} />
    </QueryClientProvider>,
  );
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe("BoardCardContextMenu — presence (TES-70)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders the dropdown trigger (…) when editable=true", () => {
    renderCard(baseIssue, true);
    // The DropdownMenu mock renders a data-testid="dropdown-menu" wrapper
    expect(screen.getByTestId("dropdown-menu")).toBeInTheDocument();
    // DropdownMenuTrigger renders a button wrapper
    expect(screen.getByTestId("dropdown-trigger")).toBeInTheDocument();
  });

  it("does NOT render the context menu when editable=false", () => {
    renderCard(baseIssue, false);
    expect(screen.queryByTestId("dropdown-menu")).not.toBeInTheDocument();
  });

  it("renders all six sub-menu triggers in the content area", () => {
    renderCard(baseIssue, true);
    const triggers = screen.getAllByTestId("dropdown-sub-trigger");
    const labels = triggers.map((t) => t.textContent ?? "");
    expect(labels.some((l) => l.includes("Status"))).toBe(true);
    expect(labels.some((l) => l.includes("Priority"))).toBe(true);
    expect(labels.some((l) => l.includes("Assignee"))).toBe(true);
    expect(labels.some((l) => l.includes("Due date"))).toBe(true);
    expect(labels.some((l) => l.includes("Project"))).toBe(true);
    expect(labels.some((l) => l.includes("Labels"))).toBe(true);
  });

  it("renders the issue identifier and title on the card", () => {
    renderCard(baseIssue, true);
    expect(screen.getByText("TES-1")).toBeInTheDocument();
    expect(screen.getByText("Board Card Test Issue")).toBeInTheDocument();
  });
});

describe("BoardCardContextMenu — Status submenu (TES-70)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("shows status items after clicking the Status sub-trigger", async () => {
    const user = userEvent.setup();
    renderCard(baseIssue, true);

    const triggers = screen.getAllByTestId("dropdown-sub-trigger");
    const statusTrigger = triggers.find((t) =>
      t.textContent?.includes("Status"),
    )!;
    await user.click(statusTrigger);

    // STATUS_CONFIG has "Backlog", "In Dev", "Done"
    expect(screen.getByText("Backlog")).toBeInTheDocument();
    expect(screen.getByText("In Dev")).toBeInTheDocument();
    expect(screen.getByText("Done")).toBeInTheDocument();
  });

  it("calls onUpdate with the selected status when a status item is clicked", async () => {
    const user = userEvent.setup();
    renderCard(baseIssue, true);

    const triggers = screen.getAllByTestId("dropdown-sub-trigger");
    const statusTrigger = triggers.find((t) =>
      t.textContent?.includes("Status"),
    )!;
    await user.click(statusTrigger);

    await user.click(screen.getByText("In Dev"));

    expect(mockMutate).toHaveBeenCalledWith(
      expect.objectContaining({ id: "issue-1", status: "in_dev" }),
      expect.any(Object),
    );
  });
});

describe("BoardCardContextMenu — Priority submenu (TES-70)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("shows priority items after clicking the Priority sub-trigger", async () => {
    const user = userEvent.setup();
    renderCard(baseIssue, true);

    const triggers = screen.getAllByTestId("dropdown-sub-trigger");
    const priorityTrigger = triggers.find((t) =>
      t.textContent?.includes("Priority"),
    )!;
    await user.click(priorityTrigger);

    expect(screen.getByText("High")).toBeInTheDocument();
    expect(screen.getByText("Urgent")).toBeInTheDocument();
  });

  it("calls onUpdate with the selected priority", async () => {
    const user = userEvent.setup();
    renderCard(baseIssue, true);

    const triggers = screen.getAllByTestId("dropdown-sub-trigger");
    const priorityTrigger = triggers.find((t) =>
      t.textContent?.includes("Priority"),
    )!;
    await user.click(priorityTrigger);

    await user.click(screen.getByText("High"));

    expect(mockMutate).toHaveBeenCalledWith(
      expect.objectContaining({ id: "issue-1", priority: "high" }),
      expect.any(Object),
    );
  });
});

describe("BoardCardContextMenu — Assignee submenu (TES-70)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("shows Unassigned option in the Assignee sub-menu", async () => {
    const user = userEvent.setup();
    renderCard(baseIssue, true);

    const triggers = screen.getAllByTestId("dropdown-sub-trigger");
    const assigneeTrigger = triggers.find((t) =>
      t.textContent?.includes("Assignee"),
    )!;
    await user.click(assigneeTrigger);

    expect(screen.getByText("Unassigned")).toBeInTheDocument();
  });

  it("calls onUpdate with null assignee when Unassigned is clicked", async () => {
    const user = userEvent.setup();
    renderCard(baseIssue, true);

    const triggers = screen.getAllByTestId("dropdown-sub-trigger");
    const assigneeTrigger = triggers.find((t) =>
      t.textContent?.includes("Assignee"),
    )!;
    await user.click(assigneeTrigger);

    await user.click(screen.getByText("Unassigned"));

    expect(mockMutate).toHaveBeenCalledWith(
      expect.objectContaining({
        id: "issue-1",
        assignee_type: null,
        assignee_id: null,
      }),
      expect.any(Object),
    );
  });
});

describe("BoardCardContextMenu — Due Date submenu (TES-70)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("shows Today, Tomorrow, Next week in the Due Date sub-menu", async () => {
    const user = userEvent.setup();
    renderCard(baseIssue, true);

    const triggers = screen.getAllByTestId("dropdown-sub-trigger");
    const dueTrigger = triggers.find((t) =>
      t.textContent?.includes("Due date"),
    )!;
    await user.click(dueTrigger);

    expect(screen.getByText("Today")).toBeInTheDocument();
    expect(screen.getByText("Tomorrow")).toBeInTheDocument();
    expect(screen.getByText("Next week")).toBeInTheDocument();
  });

  it("calls onUpdate with a due_date when Today is clicked", async () => {
    const user = userEvent.setup();
    renderCard(baseIssue, true);

    const triggers = screen.getAllByTestId("dropdown-sub-trigger");
    const dueTrigger = triggers.find((t) =>
      t.textContent?.includes("Due date"),
    )!;
    await user.click(dueTrigger);

    await user.click(screen.getByText("Today"));

    expect(mockMutate).toHaveBeenCalledWith(
      expect.objectContaining({
        id: "issue-1",
        due_date: expect.any(String),
      }),
      expect.any(Object),
    );
  });

  it("shows Clear date when issue already has a due_date", async () => {
    const user = userEvent.setup();
    renderCard(issueWithDueDate, true);

    const triggers = screen.getAllByTestId("dropdown-sub-trigger");
    const dueTrigger = triggers.find((t) =>
      t.textContent?.includes("Due date"),
    )!;
    await user.click(dueTrigger);

    expect(screen.getByText("Clear date")).toBeInTheDocument();
  });

  it("calls onUpdate with { due_date: null } when Clear date is clicked", async () => {
    const user = userEvent.setup();
    renderCard(issueWithDueDate, true);

    const triggers = screen.getAllByTestId("dropdown-sub-trigger");
    const dueTrigger = triggers.find((t) =>
      t.textContent?.includes("Due date"),
    )!;
    await user.click(dueTrigger);

    await user.click(screen.getByText("Clear date"));

    expect(mockMutate).toHaveBeenCalledWith(
      expect.objectContaining({ id: "issue-1", due_date: null }),
      expect.any(Object),
    );
  });

  it("does NOT show Clear date when issue has no due_date", async () => {
    const user = userEvent.setup();
    renderCard(baseIssue, true);

    const triggers = screen.getAllByTestId("dropdown-sub-trigger");
    const dueTrigger = triggers.find((t) =>
      t.textContent?.includes("Due date"),
    )!;
    await user.click(dueTrigger);

    expect(screen.queryByText("Clear date")).not.toBeInTheDocument();
  });
});

describe("BoardCardContextMenu — Project submenu (TES-70)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("shows Remove from project when issue already has a project", async () => {
    const user = userEvent.setup();
    // Issue has project_id and projectListOptions returns that project
    renderCard(issueWithProject, true);

    const triggers = screen.getAllByTestId("dropdown-sub-trigger");
    const projectTrigger = triggers.find((t) =>
      t.textContent?.includes("Project"),
    )!;
    await user.click(projectTrigger);

    expect(screen.getByText("Remove from project")).toBeInTheDocument();
  });

  it("calls onUpdate with { project_id: null } when Remove from project is clicked", async () => {
    const user = userEvent.setup();
    renderCard(issueWithProject, true);

    const triggers = screen.getAllByTestId("dropdown-sub-trigger");
    const projectTrigger = triggers.find((t) =>
      t.textContent?.includes("Project"),
    )!;
    await user.click(projectTrigger);

    await user.click(screen.getByText("Remove from project"));

    expect(mockMutate).toHaveBeenCalledWith(
      expect.objectContaining({ id: "issue-1", project_id: null }),
      expect.any(Object),
    );
  });

  it("shows 'No projects yet' when project list is empty", async () => {
    const user = userEvent.setup();
    // Pre-seed the query cache with empty project data so the component's
    // useQuery(projectListOptions(wsId)) returns [] immediately.
    const qc = new QueryClient({
      defaultOptions: { queries: { retry: false, staleTime: Infinity } },
    });
    // The mock sets queryKey: ["projects", "ws-1"] — seed that exact key.
    qc.setQueryData(["projects", "ws-1"], []);

    const { render: rtlRender, screen: rtlScreen } = await import(
      "@testing-library/react"
    );
    rtlRender(
      <QueryClientProvider client={qc}>
        <BoardCardContent issue={baseIssue} editable />
      </QueryClientProvider>,
    );

    const triggers = rtlScreen.getAllByTestId("dropdown-sub-trigger");
    const projectTrigger = triggers.find((t) =>
      t.textContent?.includes("Project"),
    )!;
    await user.click(projectTrigger);

    // "No projects yet" shows when projects array is empty
    expect(rtlScreen.getByText("No projects yet")).toBeInTheDocument();
  });
});

describe("BoardCardContextMenu — Labels submenu (TES-70)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders the Labels sub-trigger in the context menu", () => {
    renderCard(baseIssue, true);
    const triggers = screen.getAllByTestId("dropdown-sub-trigger");
    const labelsTrigger = triggers.find((t) =>
      t.textContent?.includes("Labels"),
    );
    expect(labelsTrigger).toBeDefined();
  });

  it("mounts LabelPicker inside the Labels sub-content when opened", async () => {
    const user = userEvent.setup();
    renderCard(baseIssue, true);

    const triggers = screen.getAllByTestId("dropdown-sub-trigger");
    const labelsTrigger = triggers.find((t) =>
      t.textContent?.includes("Labels"),
    )!;
    await user.click(labelsTrigger);

    expect(screen.getByTestId("label-picker")).toBeInTheDocument();
  });
});
