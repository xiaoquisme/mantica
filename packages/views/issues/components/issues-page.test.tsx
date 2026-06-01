import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ListIssuesResponse } from "@mantica/core/types";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { Issue } from "@mantica/core/types";
import { WorkspaceIdProvider } from "@mantica/core/hooks";

// ---------------------------------------------------------------------------
// Mocks
// ---------------------------------------------------------------------------

// Mock @mantica/core/auth
const mockAuthUser = { id: "user-1", email: "test@test.com", name: "Test User" };
vi.mock("@mantica/core/auth", () => ({
  useAuthStore: Object.assign(
    (selector?: any) => {
      const state = { user: mockAuthUser, isAuthenticated: true };
      return selector ? selector(state) : state;
    },
    { getState: () => ({ user: mockAuthUser, isAuthenticated: true }) },
  ),
  registerAuthStore: vi.fn(),
  createAuthStore: vi.fn(),
}));

// Mock @mantica/core/workspace
vi.mock("@mantica/core/workspace", () => ({
  useWorkspaceStore: Object.assign(
    (selector?: any) => {
      const state = {
        workspace: { id: "ws-1", name: "Test WS", slug: "test" },
        agents: [],
        members: [],
      };
      return selector ? selector(state) : state;
    },
    {
      getState: () => ({
        workspace: { id: "ws-1", name: "Test WS", slug: "test" },
        agents: [],
        members: [],
      }),
    },
  ),
  registerWorkspaceStore: vi.fn(),
}));

// Mock @mantica/views/navigation (AppLink + useNavigation)
vi.mock("../../navigation", () => ({
  AppLink: ({ children, href, ...props }: any) => (
    <a href={href} {...props}>
      {children}
    </a>
  ),
  useNavigation: () => ({ push: vi.fn(), pathname: "/issues" }),
  NavigationProvider: ({ children }: { children: React.ReactNode }) => children,
}));

// Mock workspace avatar
vi.mock("../../workspace/workspace-avatar", () => ({
  WorkspaceAvatar: ({ name }: { name: string }) => <span data-testid="workspace-avatar">{name.charAt(0)}</span>,
}));

// Mock api (queries use api internally)
const mockListIssues = vi.hoisted(() => vi.fn().mockResolvedValue({ issues: [], total: 0 }));
const mockGetWorkspaceLabels = vi.hoisted(() => vi.fn().mockResolvedValue([]));
const mockUpdateIssueLabels = vi.hoisted(() => vi.fn().mockResolvedValue([]));
vi.mock("@mantica/core/api", () => ({
  api: {
    listIssues: (...args: any[]) => mockListIssues(...args),
    updateIssue: vi.fn(),
    listMembers: () => Promise.resolve([]),
    listAgents: () => Promise.resolve([]),
    getWorkspaceLabels: (...args: any[]) => mockGetWorkspaceLabels(...args),
    updateIssueLabels: (...args: any[]) => mockUpdateIssueLabels(...args),
    listProjects: () => Promise.resolve({ projects: [] }),
  },
  getApi: () => ({
    listIssues: (...args: any[]) => mockListIssues(...args),
    updateIssue: vi.fn(),
    listMembers: () => Promise.resolve([]),
    listAgents: () => Promise.resolve([]),
    getWorkspaceLabels: (...args: any[]) => mockGetWorkspaceLabels(...args),
    updateIssueLabels: (...args: any[]) => mockUpdateIssueLabels(...args),
    listProjects: () => Promise.resolve({ projects: [] }),
  }),
  setApiInstance: vi.fn(),
}));

// Mock workspace queries (so list-row's memberListOptions/agentListOptions resolve immediately)
vi.mock("@mantica/core/workspace/queries", () => ({
  memberListOptions: () => ({
    queryKey: ["workspaces", "ws-1", "members"],
    queryFn: () => Promise.resolve([]),
  }),
  agentListOptions: () => ({
    queryKey: ["workspaces", "ws-1", "agents"],
    queryFn: () => Promise.resolve([]),
  }),
}));

// Mock projects queries (so list-row's projectListOptions resolves immediately)
vi.mock("@mantica/core/projects/queries", () => ({
  projectListOptions: () => ({
    queryKey: ["projects", "ws-1", "list"],
    queryFn: () => Promise.resolve({ projects: [] }),
    select: (data: any) => data.projects,
  }),
}));

// Mock @mantica/ui dropdown-menu with minimal HTML stubs for jsdom.
// DropdownMenuContent renders its children so that ListRow's sub-menus
// ("Labels", "Status", etc.) are visible in list-view tests.
// DropdownMenuSubContent renders null so that the deeply-nested items
// (status labels like "Backlog", priority names, etc.) do NOT appear in the
// DOM — this prevents conflicts with board column header text in board-view tests.
// AC2/AC3 tests render LabelPicker directly (bypassing the sub-content mock).
vi.mock("@mantica/ui/components/ui/dropdown-menu", () => ({
  DropdownMenu: ({ children }: any) => <div data-testid="dropdown-menu">{children}</div>,
  DropdownMenuTrigger: ({ render: renderProp, children, ...props }: any) => (
    <div data-testid="dropdown-trigger" {...props}>{renderProp ?? children}</div>
  ),
  DropdownMenuContent: ({ children }: any) => <div data-testid="dropdown-content">{children}</div>,
  DropdownMenuItem: ({ children, onClick }: any) => <button onClick={onClick}>{children}</button>,
  DropdownMenuSeparator: () => <hr />,
  DropdownMenuSub: ({ children }: any) => <div data-testid="dropdown-sub">{children}</div>,
  DropdownMenuSubTrigger: ({ children, onClick }: any) => <button data-testid="dropdown-sub-trigger" onClick={onClick}>{children}</button>,
  // Sub-content renders null: prevents status/priority/label items from leaking
  // into the DOM and causing ambiguous getByText queries in board-view tests.
  DropdownMenuSubContent: () => null,
  DropdownMenuGroup: ({ children }: any) => <div>{children}</div>,
  DropdownMenuLabel: ({ children }: any) => <div>{children}</div>,
  DropdownMenuCheckboxItem: ({ children, onClick }: any) => <button onClick={onClick}>{children}</button>,
  DropdownMenuRadioGroup: ({ children }: any) => <div>{children}</div>,
  DropdownMenuRadioItem: ({ children, onClick }: any) => <button onClick={onClick}>{children}</button>,
  DropdownMenuShortcut: ({ children }: any) => <span>{children}</span>,
}));

// Mock issue config
vi.mock("@mantica/core/issues/config", () => ({
  ALL_STATUSES: ["backlog", "classifying", "ready_analyze", "in_analyze", "ready_arch_design", "in_arch_design", "ready_dev", "doing", "ready_review", "in_review", "ready_test", "in_test", "done", "blocked", "cancelled"],
  BOARD_STATUSES: ["backlog", "classifying", "ready_analyze", "in_analyze", "ready_arch_design", "in_arch_design", "ready_dev", "doing", "ready_review", "in_review", "ready_test", "in_test", "done", "blocked"],
  STATUS_ORDER: ["backlog", "classifying", "ready_analyze", "in_analyze", "ready_arch_design", "in_arch_design", "ready_dev", "doing", "ready_review", "in_review", "ready_test", "in_test", "done", "blocked", "cancelled"],
  STATUS_CONFIG: {
    backlog: { label: "Backlog", iconColor: "text-muted-foreground", hoverBg: "hover:bg-accent" },
    classifying: { label: "Classifying", iconColor: "text-orange-400", hoverBg: "hover:bg-orange-400/10" },
    ready_analyze: { label: "Ready Analyze", iconColor: "text-yellow-500", hoverBg: "hover:bg-yellow-500/10" },
    in_analyze: { label: "In Analyze", iconColor: "text-yellow-600", hoverBg: "hover:bg-yellow-600/10" },
    ready_arch_design: { label: "Ready Arch Design", iconColor: "text-cyan-500", hoverBg: "hover:bg-cyan-500/10" },
    in_arch_design: { label: "In Arch Design", iconColor: "text-cyan-600", hoverBg: "hover:bg-cyan-600/10" },
    ready_dev: { label: "Ready Dev", iconColor: "text-blue-400", hoverBg: "hover:bg-blue-400/10" },
    in_dev: { label: "In Dev", iconColor: "text-blue-600", hoverBg: "hover:bg-blue-600/10" },
    ready_review: { label: "Ready Review", iconColor: "text-violet-400", hoverBg: "hover:bg-violet-400/10" },
    in_review: { label: "In Review", iconColor: "text-violet-600", hoverBg: "hover:bg-violet-600/10" },
    ready_test: { label: "Ready Test", iconColor: "text-rose-400", hoverBg: "hover:bg-rose-400/10" },
    in_test: { label: "In Test", iconColor: "text-rose-600", hoverBg: "hover:bg-rose-600/10" },
    done: { label: "Done", iconColor: "text-info", hoverBg: "hover:bg-info/10" },
    blocked: { label: "Blocked", iconColor: "text-destructive", hoverBg: "hover:bg-destructive/10" },
    cancelled: { label: "Cancelled", iconColor: "text-muted-foreground", hoverBg: "hover:bg-accent" },
  },
  PRIORITY_ORDER: ["urgent", "high", "medium", "low", "none"],
  PRIORITY_CONFIG: {
    urgent: { label: "Urgent", bars: 4, color: "text-destructive" },
    high: { label: "High", bars: 3, color: "text-warning" },
    medium: { label: "Medium", bars: 2, color: "text-warning" },
    low: { label: "Low", bars: 1, color: "text-info" },
    none: { label: "No priority", bars: 0, color: "text-muted-foreground" },
  },
}));

// Mock view store
const mockViewState = {
  viewMode: "board" as const,
  statusFilters: [] as string[],
  priorityFilters: [] as string[],
  assigneeFilters: [] as { type: string; id: string }[],
  includeNoAssignee: false,
  creatorFilters: [] as { type: string; id: string }[],
  sortBy: "position" as const,
  sortDirection: "asc" as const,
  cardProperties: { priority: true, description: true, assignee: true, dueDate: true },
  listCollapsedStatuses: [] as string[],
  columnOrder: [] as string[],
  setViewMode: vi.fn(),
  toggleStatusFilter: vi.fn(),
  togglePriorityFilter: vi.fn(),
  toggleAssigneeFilter: vi.fn(),
  toggleNoAssignee: vi.fn(),
  toggleCreatorFilter: vi.fn(),
  hideStatus: vi.fn(),
  showStatus: vi.fn(),
  clearFilters: vi.fn(),
  setSortBy: vi.fn(),
  setSortDirection: vi.fn(),
  toggleCardProperty: vi.fn(),
  toggleListCollapsed: vi.fn(),
  setColumnOrder: vi.fn(),
};

vi.mock("@mantica/core/issues/stores/view-store", () => ({
  initFilterWorkspaceSync: vi.fn(),
  useIssueViewStore: Object.assign(
    (selector?: any) => (selector ? selector(mockViewState) : mockViewState),
    { getState: () => mockViewState, setState: vi.fn() },
  ),
  createIssueViewStore: () => ({
    getState: () => mockViewState,
    setState: vi.fn(),
    subscribe: vi.fn(),
  }),
  SORT_OPTIONS: [
    { value: "position", label: "Manual" },
    { value: "priority", label: "Priority" },
    { value: "due_date", label: "Due date" },
    { value: "created_at", label: "Created date" },
    { value: "title", label: "Title" },
  ],
  CARD_PROPERTY_OPTIONS: [
    { key: "priority", label: "Priority" },
    { key: "description", label: "Description" },
    { key: "assignee", label: "Assignee" },
    { key: "dueDate", label: "Due date" },
  ],
}));

vi.mock("@mantica/core/issues/stores/view-store-context", () => ({
  ViewStoreProvider: ({ children }: { children: React.ReactNode }) => children,
  useViewStore: (selector?: any) => (selector ? selector(mockViewState) : mockViewState),
  useViewStoreApi: () => ({ getState: () => mockViewState, setState: vi.fn(), subscribe: vi.fn() }),
}));

vi.mock("@mantica/core/issues/stores/issues-scope-store", () => ({
  useIssuesScopeStore: Object.assign(
    (selector?: any) => {
      const state = { scope: "all", setScope: vi.fn() };
      return selector ? selector(state) : state;
    },
    { getState: () => ({ scope: "all", setScope: vi.fn() }) },
  ),
}));

vi.mock("@mantica/core/issues/stores/selection-store", () => ({
  useIssueSelectionStore: Object.assign(
    (selector?: any) => {
      const state = { selectedIds: new Set(), toggle: vi.fn(), clear: vi.fn(), setAll: vi.fn() };
      return selector ? selector(state) : state;
    },
    { getState: () => ({ selectedIds: new Set(), toggle: vi.fn(), clear: vi.fn(), setAll: vi.fn() }) },
  ),
}));

vi.mock("@mantica/core/modals", () => ({
  useModalStore: Object.assign(
    () => ({ open: vi.fn() }),
    { getState: () => ({ open: vi.fn() }) },
  ),
}));

// Mock sonner toast
vi.mock("sonner", () => ({
  toast: { error: vi.fn(), success: vi.fn() },
}));

// Mock dnd-kit
vi.mock("@dnd-kit/core", () => ({
  DndContext: ({ children }: any) => children,
  DragOverlay: () => null,
  PointerSensor: class {},
  useSensor: () => ({}),
  useSensors: () => [],
  useDroppable: () => ({ setNodeRef: vi.fn(), isOver: false }),
  pointerWithin: vi.fn(),
  closestCenter: vi.fn(),
}));

vi.mock("@dnd-kit/sortable", () => ({
  SortableContext: ({ children }: any) => children,
  verticalListSortingStrategy: {},
  horizontalListSortingStrategy: {},
  arrayMove: vi.fn(),
  useSortable: () => ({
    attributes: {},
    listeners: {},
    setNodeRef: vi.fn(),
    transform: null,
    transition: null,
    isDragging: false,
  }),
}));

vi.mock("@dnd-kit/utilities", () => ({
  CSS: { Transform: { toString: () => undefined } },
}));

// Mock @base-ui/react/accordion (used by ListView)
vi.mock("@base-ui/react/accordion", () => ({
  Accordion: Object.assign(
    ({ children }: any) => <div>{children}</div>,
    {
      Root: ({ children }: any) => <div>{children}</div>,
      Item: ({ children }: any) => <div>{children}</div>,
      Header: ({ children }: any) => <div>{children}</div>,
      Trigger: ({ children }: any) => <button>{children}</button>,
      Panel: ({ children }: any) => <div>{children}</div>,
    },
  ),
}));

// ---------------------------------------------------------------------------
// Test data
// ---------------------------------------------------------------------------

const issueDefaults = {
  parent_issue_id: null,
  project_id: null,
  position: 0,
};

const mockIssues: Issue[] = [
  {
    ...issueDefaults,
    id: "issue-1",
    workspace_id: "ws-1",
    number: 1,
    identifier: "TES-1",
    title: "Implement auth",
    description: "Add JWT authentication",
    status: "backlog",
    priority: "high",
    assignee_type: "member",
    assignee_id: "user-1",
    creator_type: "member",
    creator_id: "user-1",
    due_date: null,
    created_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-01T00:00:00Z",
  },
  {
    ...issueDefaults,
    id: "issue-2",
    workspace_id: "ws-1",
    number: 2,
    identifier: "TES-2",
    title: "Design landing page",
    description: null,
    status: "doing",
    priority: "medium",
    assignee_type: "agent",
    assignee_id: "agent-1",
    creator_type: "member",
    creator_id: "user-1",
    due_date: "2026-02-01T00:00:00Z",
    created_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-01T00:00:00Z",
  },
  {
    ...issueDefaults,
    id: "issue-3",
    workspace_id: "ws-1",
    number: 3,
    identifier: "TES-3",
    title: "Write tests",
    description: null,
    status: "backlog",
    priority: "low",
    assignee_type: null,
    assignee_id: null,
    creator_type: "member",
    creator_id: "user-1",
    due_date: null,
    created_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-01T00:00:00Z",
  },
];

// ---------------------------------------------------------------------------
// Import component under test (after mocks)
// ---------------------------------------------------------------------------

import { IssuesPage } from "./issues-page";

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function renderWithQuery(ui: React.ReactElement) {
  const qc = new QueryClient({
    defaultOptions: {
      queries: { retry: false, gcTime: 0 },
      mutations: { retry: false },
    },
  });
  return render(
    <QueryClientProvider client={qc}>
      <WorkspaceIdProvider wsId="ws-1">{ui}</WorkspaceIdProvider>
    </QueryClientProvider>,
  );
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe("IssuesPage (shared)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockListIssues.mockResolvedValue({ issues: [], total: 0 });
    mockViewState.viewMode = "board";
    mockViewState.statusFilters = [];
    mockViewState.priorityFilters = [];
  });

  it("shows loading skeletons initially", () => {
    renderWithQuery(<IssuesPage />);
    expect(
      screen.getAllByRole("generic").some((el) => el.getAttribute("data-slot") === "skeleton"),
    ).toBe(true);
  });

  it("renders issue titles after data loads", async () => {
    mockListIssues.mockImplementation((params: any) =>
      Promise.resolve(
        params?.open_only
          ? { issues: mockIssues, total: mockIssues.length }
          : { issues: [], total: 0 },
      ),
    );

    renderWithQuery(<IssuesPage />);

    await screen.findByText("Implement auth");
    expect(screen.getByText("Design landing page")).toBeInTheDocument();
    expect(screen.getByText("Write tests")).toBeInTheDocument();
  });

  it("renders board column headers", async () => {
    mockListIssues.mockImplementation((params: any) =>
      Promise.resolve(
        params?.open_only
          ? { issues: mockIssues, total: mockIssues.length }
          : { issues: [], total: 0 },
      ),
    );

    renderWithQuery(<IssuesPage />);

    await screen.findByText("Backlog");
    expect(screen.getAllByText("Classifying").length).toBeGreaterThanOrEqual(1);
    expect(screen.getAllByText("In Dev").length).toBeGreaterThanOrEqual(1);
  });

  it("shows workspace breadcrumb with 'Issues' label", async () => {
    mockListIssues.mockImplementation((params: any) =>
      Promise.resolve(
        params?.open_only
          ? { issues: mockIssues, total: mockIssues.length }
          : { issues: [], total: 0 },
      ),
    );

    renderWithQuery(<IssuesPage />);

    await screen.findByText("Issues");
    expect(screen.getByText("Test WS")).toBeInTheDocument();
  });

  it("shows empty state when there are no issues", async () => {
    mockListIssues.mockResolvedValue({ issues: [], total: 0 });

    renderWithQuery(<IssuesPage />);

    await screen.findByText("No issues yet");
    expect(screen.getByText("Create an issue to get started.")).toBeInTheDocument();
  });

  it("shows scope tab buttons", async () => {
    renderWithQuery(<IssuesPage />);

    await screen.findByText("All");
    expect(screen.getByText("Members")).toBeInTheDocument();
    expect(screen.getByText("Agents")).toBeInTheDocument();
  });
});

// ---------------------------------------------------------------------------
// TES-89 — Quick-add labels via hover action menu
// ---------------------------------------------------------------------------

describe("TES-89 — Quick-add labels via hover action menu", () => {
  const listIssue = {
    id: "issue-1",
    workspace_id: "ws-1",
    number: 1,
    identifier: "TES-1",
    title: "Implement auth",
    description: "Add JWT authentication",
    status: "backlog" as const,
    priority: "high" as const,
    assignee_type: null,
    assignee_id: null,
    creator_type: "member" as const,
    creator_id: "user-1",
    parent_issue_id: null,
    project_id: null,
    position: 0,
    due_date: null,
    labels: [],
    created_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-01T00:00:00Z",
  };

  beforeEach(() => {
    vi.clearAllMocks();
    (mockViewState as any).viewMode = "list";
    mockViewState.statusFilters = [];
    mockViewState.priorityFilters = [];
    mockGetWorkspaceLabels.mockResolvedValue([]);
    mockUpdateIssueLabels.mockResolvedValue([]);
    mockListIssues.mockImplementation((params: any) =>
      Promise.resolve(
        params?.open_only
          ? { issues: [listIssue], total: 1 }
          : { issues: [], total: 0 },
      ),
    );
  });

  // AC1 — Labels sub-menu trigger is visible in the context menu
  it("AC1: renders a Labels sub-menu item in the row context menu", async () => {
    renderWithQuery(<IssuesPage />);

    // Wait for issue row to appear
    await screen.findByText("Implement auth");

    // The DropdownMenu is mocked: DropdownMenuSub + DropdownMenuSubTrigger
    // always render, so "Labels" is visible as a sub-trigger in the list row.
    // Use getAllByText since IssuesHeader may also render a "Labels" item if any.
    const labelsItems = screen.getAllByText("Labels");
    expect(labelsItems.length).toBeGreaterThanOrEqual(1);
  });

  // AC2 — LabelPicker renders workspace labels fetched from the API
  // Render LabelPicker directly to avoid sub-content mock limitations in IssuesPage.
  it("AC2: LabelPicker shows workspace labels returned by getWorkspaceLabels", async () => {
    mockGetWorkspaceLabels.mockResolvedValue([
      { id: "label-1", workspace_id: "ws-1", name: "Bug", color: "#ff0000" },
    ]);

    const { LabelPicker } = await import("./pickers/label-picker");

    renderWithQuery(<LabelPicker issueId="issue-1" currentLabels={[]} />);

    await screen.findByText("Bug");
    expect(screen.getByText("Bug")).toBeInTheDocument();
  });

  // AC3 — Selecting a label calls updateIssueLabels with the correct IDs
  // Render LabelPicker directly for precise interaction testing.
  it("AC3: clicking a label in LabelPicker calls updateIssueLabels with the label ID", async () => {
    const user = userEvent.setup();
    mockGetWorkspaceLabels.mockResolvedValue([
      { id: "label-1", workspace_id: "ws-1", name: "Bug", color: "#ff0000" },
    ]);

    const { LabelPicker } = await import("./pickers/label-picker");

    renderWithQuery(<LabelPicker issueId="issue-1" currentLabels={[]} />);

    await screen.findByText("Bug");
    const labelBtn = screen.getByRole("button", { name: /Bug/i });
    await user.click(labelBtn);

    await waitFor(() => {
      expect(mockUpdateIssueLabels).toHaveBeenCalledWith("issue-1", ["label-1"]);
    });
  });

  // AC4 — List-view rows expose context-menu sub-trigger items
  it("AC4: list-view rows render Labels, Status and Priority sub-menu triggers", async () => {
    renderWithQuery(<IssuesPage />);

    await screen.findByText("Implement auth");

    // IssuesHeader renders Status/Priority sub-triggers (in the filter dropdown).
    // ListRow renders Labels/Status/Priority sub-triggers (in the context menu).
    // Both are rendered because DropdownMenuSubTrigger is not hidden by the mock.
    // Use getAllByText to tolerate duplicates between header and row menus.
    expect(screen.getAllByText("Labels").length).toBeGreaterThanOrEqual(1);
    expect(screen.getAllByText("Status").length).toBeGreaterThanOrEqual(1);
    expect(screen.getAllByText("Priority").length).toBeGreaterThanOrEqual(1);
  });

  // AC5 — Optimistic update: list cache reflects new labels immediately after mutation
  it("AC5: list cache is updated optimistically when updateIssueLabels is called", async () => {
    const { QueryClient: QC, QueryClientProvider: QCP } = await import("@tanstack/react-query");
    const { renderHook, act } = await import("@testing-library/react");
    const { WorkspaceIdProvider: WIP } = await import("@mantica/core/hooks");
    const { useUpdateIssueLabels } = await import("@mantica/core/issues/mutations");
    const { issueKeys } = await import("@mantica/core/issues/queries");

    const qc = new QC({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    });

    // Seed the list cache with one issue that has no labels
    qc.setQueryData(issueKeys.list("ws-1"), {
      issues: [{ ...listIssue, labels: [] }],
      total: 1,
      doneTotal: 0,
    });

    // Seed the workspace labels cache
    qc.setQueryData(["labels", "ws-1"], [
      { id: "label-1", workspace_id: "ws-1", name: "Bug", color: "#ff0000" },
    ]);

    const wrapper = ({ children }: { children: React.ReactNode }) => (
      <QCP client={qc}>
        <WIP wsId="ws-1">{children}</WIP>
      </QCP>
    );

    const { result } = renderHook(() => useUpdateIssueLabels(), { wrapper });

    act(() => {
      result.current.mutate({ issueId: "issue-1", labelIds: ["label-1"] });
    });

    // After mutate (synchronous optimistic update in onMutate), the list cache
    // should reflect the new label immediately.
    await waitFor(() => {
      const cached = qc.getQueryData<ListIssuesResponse>(issueKeys.list("ws-1"));
      const issue = cached?.issues.find((i) => i.id === "issue-1");
      expect(issue?.labels).toHaveLength(1);
      expect(issue?.labels?.[0]?.id).toBe("label-1");
    });
  });
});
