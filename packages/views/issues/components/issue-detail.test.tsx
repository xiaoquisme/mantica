import { forwardRef, useRef, useState, useImperativeHandle } from "react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { Issue, TimelineEntry } from "@multica/core/types";
import { WorkspaceIdProvider } from "@multica/core/hooks";

// ---------------------------------------------------------------------------
// Mocks
// ---------------------------------------------------------------------------

// Mock @multica/core/auth
const mockAuthUser = { id: "user-1", email: "test@test.com", name: "Test User" };
vi.mock("@multica/core/auth", () => ({
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

// Mock @multica/core/workspace
vi.mock("@multica/core/workspace", () => ({
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

// Mock @multica/core/workspace/hooks
vi.mock("@multica/core/workspace/hooks", () => ({
  useActorName: () => ({
    getMemberName: (id: string) => (id === "user-1" ? "Test User" : "Unknown"),
    getAgentName: (id: string) => (id === "agent-1" ? "Claude Agent" : "Unknown Agent"),
    getActorName: (type: string, id: string) => {
      if (type === "member" && id === "user-1") return "Test User";
      if (type === "agent" && id === "agent-1") return "Claude Agent";
      return "Unknown";
    },
    getActorInitials: (type: string) => (type === "member" ? "TU" : "CA"),
    getActorAvatarUrl: () => null,
  }),
}));

// Mock workspace queries
vi.mock("@multica/core/workspace/queries", () => ({
  memberListOptions: () => ({
    queryKey: ["workspaces", "ws-1", "members"],
    queryFn: () => Promise.resolve([{ user_id: "user-1", name: "Test User", email: "test@test.com", role: "admin" }]),
  }),
  agentListOptions: () => ({
    queryKey: ["workspaces", "ws-1", "agents"],
    queryFn: () => Promise.resolve([]),
  }),
}));

// Stable router mock — shared across tests so call history can be inspected
const mockRouter = vi.hoisted(() => ({
  push: vi.fn(),
  replace: vi.fn(),
  pathname: "/issues/issue-1",
  getShareableUrl: undefined as undefined,
}));

// Mock navigation
vi.mock("../../navigation", () => ({
  AppLink: ({ children, href, ...props }: any) => (
    <a href={href} {...props}>
      {children}
    </a>
  ),
  useNavigation: () => mockRouter,
  NavigationProvider: ({ children }: { children: React.ReactNode }) => children,
}));

// Mock editor components (Tiptap requires real DOM)
vi.mock("../../editor", () => ({
  ReadonlyContent: ({ content }: { content: string }) => (
    <div data-testid="readonly-content">{content}</div>
  ),
  ContentEditor: forwardRef(({ defaultValue, onUpdate, placeholder }: any, ref: any) => {
    const valueRef = useRef(defaultValue || "");
    const [value, setValue] = useState(defaultValue || "");
    useImperativeHandle(ref, () => ({
      getMarkdown: () => valueRef.current,
      clearContent: () => { valueRef.current = ""; setValue(""); },
      focus: () => {},
      uploadFile: () => {},
    }));
    return (
      <textarea
        value={value}
        onChange={(e) => {
          valueRef.current = e.target.value;
          setValue(e.target.value);
          onUpdate?.(e.target.value);
        }}
        placeholder={placeholder}
        data-testid="rich-text-editor"
      />
    );
  }),
  TitleEditor: forwardRef(({ defaultValue, placeholder, onBlur, onChange }: any, ref: any) => {
    const valueRef = useRef(defaultValue || "");
    const [value, setValue] = useState(defaultValue || "");
    useImperativeHandle(ref, () => ({
      getText: () => valueRef.current,
      focus: () => {},
    }));
    return (
      <input
        value={value}
        onChange={(e) => {
          valueRef.current = e.target.value;
          setValue(e.target.value);
          onChange?.(e.target.value);
        }}
        onBlur={() => onBlur?.(valueRef.current)}
        placeholder={placeholder}
        data-testid="title-editor"
      />
    );
  }),
}));

// Mock common components
vi.mock("../../common/actor-avatar", () => ({
  ActorAvatar: ({ actorType, actorId }: any) => (
    <span data-testid="actor-avatar">
      {actorType}:{actorId}
    </span>
  ),
}));

vi.mock("../../projects/components/project-picker", () => ({
  ProjectPicker: () => <span data-testid="project-picker">Project</span>,
}));

// Mock api
const mockApiObj = vi.hoisted(() => ({
  getIssue: vi.fn(),
  listTimeline: vi.fn().mockResolvedValue([]),
  listComments: vi.fn().mockResolvedValue([]),
  createComment: vi.fn(),
  updateComment: vi.fn(),
  deleteComment: vi.fn(),
  deleteIssue: vi.fn(),
  updateIssue: vi.fn(),
  listIssueSubscribers: vi.fn().mockResolvedValue([]),
  subscribeToIssue: vi.fn().mockResolvedValue(undefined),
  unsubscribeFromIssue: vi.fn().mockResolvedValue(undefined),
  getActiveTasksForIssue: vi.fn().mockResolvedValue({ tasks: [] }),
  listTasksByIssue: vi.fn().mockResolvedValue([]),
  listTaskMessages: vi.fn().mockResolvedValue([]),
  listChildIssues: vi.fn().mockResolvedValue({ issues: [] }),
  listIssues: vi.fn().mockResolvedValue({ issues: [], total: 0 }),
  uploadFile: vi.fn(),
  listIssueReactions: vi.fn().mockResolvedValue([]),
  addIssueReaction: vi.fn(),
  removeIssueReaction: vi.fn(),
  addCommentReaction: vi.fn(),
  removeCommentReaction: vi.fn(),
  listMembers: vi.fn().mockResolvedValue([{ user_id: "user-1", name: "Test User", email: "test@test.com", role: "admin" }]),
  listAgents: vi.fn().mockResolvedValue([]),
  getWorkspaceLabels: vi.fn().mockResolvedValue([]),
  updateIssueLabels: vi.fn().mockResolvedValue([]),
}));

vi.mock("@multica/core/api", () => ({
  api: mockApiObj,
  getApi: () => mockApiObj,
  setApiInstance: vi.fn(),
}));

// Mock issue config
vi.mock("@multica/core/issues/config", () => ({
  ALL_STATUSES: ["backlog", "classifying", "ready_analyze", "in_analyze", "ready_arch_design", "in_arch_design", "ready_dev", "in_dev", "ready_review", "in_review", "ready_test", "in_test", "done", "blocked", "cancelled"],
  BOARD_STATUSES: ["backlog", "classifying", "ready_analyze", "in_analyze", "ready_arch_design", "in_arch_design", "ready_dev", "in_dev", "ready_review", "in_review", "ready_test", "in_test", "done", "blocked"],
  STATUS_ORDER: ["backlog", "classifying", "ready_analyze", "in_analyze", "ready_arch_design", "in_arch_design", "ready_dev", "in_dev", "ready_review", "in_review", "ready_test", "in_test", "done", "blocked", "cancelled"],
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
    urgent: { label: "Urgent", bars: 4, color: "text-destructive", badgeBg: "bg-destructive/10", badgeText: "text-destructive" },
    high: { label: "High", bars: 3, color: "text-warning", badgeBg: "bg-warning/10", badgeText: "text-warning" },
    medium: { label: "Medium", bars: 2, color: "text-warning", badgeBg: "bg-warning/10", badgeText: "text-warning" },
    low: { label: "Low", bars: 1, color: "text-info", badgeBg: "bg-info/10", badgeText: "text-info" },
    none: { label: "No priority", bars: 0, color: "text-muted-foreground", badgeBg: "bg-muted", badgeText: "text-muted-foreground" },
  },
}));

// Mock modals
vi.mock("@multica/core/modals", () => ({
  useModalStore: Object.assign(
    () => ({ open: vi.fn() }),
    { getState: () => ({ open: vi.fn() }) },
  ),
}));

// Mock core/utils
vi.mock("@multica/core/utils", () => ({
  timeAgo: () => "1d ago",
}));

// Mock core/hooks/use-file-upload
vi.mock("@multica/core/hooks/use-file-upload", () => ({
  useFileUpload: () => ({ uploadWithToast: vi.fn().mockResolvedValue("https://example.com/file.png") }),
}));

// Mock realtime
vi.mock("@multica/core/realtime", () => ({
  useWSEvent: vi.fn(),
  useWSReconnect: vi.fn(),
  useWS: () => ({ subscribe: vi.fn(() => () => {}), onReconnect: vi.fn(() => () => {}) }),
  WSProvider: ({ children }: { children: React.ReactNode }) => children,
  useRealtimeSync: () => {},
}));

// Mock sonner
vi.mock("sonner", () => ({
  toast: { error: vi.fn(), success: vi.fn() },
}));

// Mock react-resizable-panels (used by @multica/ui/components/ui/resizable)
vi.mock("react-resizable-panels", () => ({
  Group: ({ children, ...props }: any) => <div data-testid="panel-group" {...props}>{children}</div>,
  Panel: ({ children, ...props }: any) => <div data-testid="panel" {...props}>{children}</div>,
  Separator: ({ children, ...props }: any) => <div data-testid="panel-handle" {...props}>{children}</div>,
  useDefaultLayout: () => ({ defaultLayout: undefined, onLayoutChanged: vi.fn() }),
  usePanelRef: () => ({ current: { isCollapsed: () => false, expand: vi.fn(), collapse: vi.fn() } }),
}));

// ---------------------------------------------------------------------------
// Test data
// ---------------------------------------------------------------------------

const mockIssue: Issue = {
  id: "issue-1",
  workspace_id: "ws-1",
  number: 1,
  identifier: "TES-1",
  title: "Implement authentication",
  description: "Add JWT auth to the backend",
  status: "in_dev",
  priority: "high",
  assignee_type: "member",
  assignee_id: "user-1",
  creator_type: "member",
  creator_id: "user-1",
  parent_issue_id: null,
  project_id: null,
  position: 0,
  due_date: "2026-06-01T00:00:00Z",
  created_at: "2026-01-15T00:00:00Z",
  updated_at: "2026-01-20T00:00:00Z",
};

const mockTimeline: TimelineEntry[] = [
  {
    type: "comment",
    id: "comment-1",
    actor_type: "member",
    actor_id: "user-1",
    content: "Started working on this",
    parent_id: null,
    created_at: "2026-01-16T00:00:00Z",
    updated_at: "2026-01-16T00:00:00Z",
    comment_type: "comment",
  },
  {
    type: "comment",
    id: "comment-2",
    actor_type: "agent",
    actor_id: "agent-1",
    content: "I can help with this",
    parent_id: null,
    created_at: "2026-01-17T00:00:00Z",
    updated_at: "2026-01-17T00:00:00Z",
    comment_type: "comment",
  },
];

// ---------------------------------------------------------------------------
// Import component under test (after mocks)
// ---------------------------------------------------------------------------

import { IssueDetail } from "./issue-detail";

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function createTestQueryClient() {
  return new QueryClient({
    defaultOptions: {
      queries: { retry: false, gcTime: 0 },
      mutations: { retry: false },
    },
  });
}

function renderIssueDetail(issueId = "issue-1") {
  const queryClient = createTestQueryClient();
  return render(
    <QueryClientProvider client={queryClient}>
      <WorkspaceIdProvider wsId="ws-1">
        <IssueDetail issueId={issueId} />
      </WorkspaceIdProvider>
    </QueryClientProvider>,
  );
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe("IssueDetail (shared)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    // Default: issue loads successfully
    mockApiObj.getIssue.mockResolvedValue(mockIssue);
    mockApiObj.listTimeline.mockResolvedValue(mockTimeline);
    mockApiObj.listIssueReactions.mockResolvedValue([]);
    mockApiObj.listIssueSubscribers.mockResolvedValue([]);
    mockApiObj.listChildIssues.mockResolvedValue({ issues: [] });
    mockApiObj.listIssues.mockResolvedValue({ issues: [], total: 0 });
    mockApiObj.getActiveTasksForIssue.mockResolvedValue({ tasks: [] });
    mockApiObj.listTasksByIssue.mockResolvedValue([]);
    mockApiObj.listMembers.mockResolvedValue([
      { user_id: "user-1", name: "Test User", email: "test@test.com", role: "admin" },
    ]);
    mockApiObj.listAgents.mockResolvedValue([]);
    mockApiObj.getWorkspaceLabels.mockResolvedValue([]);
    mockApiObj.updateIssueLabels.mockResolvedValue([]);
  });

  it("shows loading skeleton while data is loading", () => {
    // Make the API hang to keep loading state
    mockApiObj.getIssue.mockReturnValue(new Promise(() => {}));
    renderIssueDetail();

    expect(
      screen.getAllByRole("generic").some((el) => el.getAttribute("data-slot") === "skeleton"),
    ).toBe(true);
  });

  it("renders issue title and description after loading", async () => {
    renderIssueDetail();

    await waitFor(() => {
      expect(screen.getByDisplayValue("Implement authentication")).toBeInTheDocument();
    });

    expect(screen.getByDisplayValue("Add JWT auth to the backend")).toBeInTheDocument();
  });

  it("renders issue identifier in the breadcrumb", async () => {
    renderIssueDetail();

    await waitFor(() => {
      expect(screen.getByText("TES-1")).toBeInTheDocument();
    });
  });

  it("renders workspace name as breadcrumb link", async () => {
    renderIssueDetail();

    await waitFor(() => {
      expect(screen.getByText("Test WS")).toBeInTheDocument();
    });

    const wsLink = screen.getByText("Test WS");
    expect(wsLink.closest("a")).toHaveAttribute("href", "/issues");
  });

  it("renders properties sidebar with status, priority, assignee, due date", async () => {
    renderIssueDetail();

    await waitFor(() => {
      expect(screen.getByText("Properties")).toBeInTheDocument();
    });

    expect(screen.getByText("Status")).toBeInTheDocument();
    expect(screen.getByText("Priority")).toBeInTheDocument();
    expect(screen.getByText("Assignee")).toBeInTheDocument();
    expect(screen.getByText("Due date")).toBeInTheDocument();
  });

  it("renders Details section with Created by and dates", async () => {
    renderIssueDetail();

    await waitFor(() => {
      expect(screen.getByText("Details")).toBeInTheDocument();
    });

    expect(screen.getByText("Created by")).toBeInTheDocument();
    expect(screen.getByText("Created")).toBeInTheDocument();
    expect(screen.getByText("Updated")).toBeInTheDocument();
  });

  it("shows 'not found' message when issue does not exist", async () => {
    mockApiObj.getIssue.mockRejectedValue(new Error("Not found"));

    renderIssueDetail("nonexistent-id");

    await waitFor(() => {
      expect(
        screen.getByText("This issue does not exist or has been deleted in this workspace."),
      ).toBeInTheDocument();
    });
  });

  it("shows 'Back to Issues' button when issue is not found and no onDelete prop", async () => {
    mockApiObj.getIssue.mockRejectedValue(new Error("Not found"));

    renderIssueDetail("nonexistent-id");

    await waitFor(() => {
      expect(screen.getByText("Back to Issues")).toBeInTheDocument();
    });
  });

  it("renders Activity section header", async () => {
    renderIssueDetail();

    await waitFor(() => {
      expect(screen.getAllByText("Activity").length).toBeGreaterThanOrEqual(1);
    });
  });

  it("renders comments from timeline", async () => {
    renderIssueDetail();

    await waitFor(() => {
      expect(screen.getByText("Started working on this")).toBeInTheDocument();
    });

    expect(screen.getByText("I can help with this")).toBeInTheDocument();
  });

  it("renders parent issue link in properties panel when parent_issue_id is set", async () => {
    const mockParentIssue: Issue = {
      id: "parent-issue-1",
      workspace_id: "ws-1",
      number: 5,
      identifier: "TES-5",
      title: "Parent Feature",
      description: "Parent issue description",
      status: "in_dev",
      priority: "high",
      assignee_type: null,
      assignee_id: null,
      creator_type: "member",
      creator_id: "user-1",
      parent_issue_id: null,
      project_id: null,
      position: 0,
      due_date: null,
      created_at: "2026-01-10T00:00:00Z",
      updated_at: "2026-01-10T00:00:00Z",
    };

    const mockIssueWithParent: Issue = { ...mockIssue, parent_issue_id: "parent-issue-1" };

    mockApiObj.getIssue.mockImplementation((id: string) =>
      Promise.resolve(id === "parent-issue-1" ? mockParentIssue : mockIssueWithParent),
    );

    renderIssueDetail();

    await waitFor(() => {
      expect(screen.getAllByText("Parent Feature").length).toBeGreaterThan(0);
    });

    const parentLinks = screen
      .getAllByText("Parent Feature")
      .map((el) => el.closest("a"))
      .filter((a): a is HTMLAnchorElement => a !== null);
    expect(parentLinks.length).toBeGreaterThan(0);
    parentLinks.forEach((link) => expect(link).toHaveAttribute("href", "/issues/TES-5"));
  });

  it("does not render parent issue section when parent_issue_id is null", async () => {
    renderIssueDetail();

    await waitFor(() => {
      expect(screen.getByText("Implement authentication")).toBeInTheDocument();
    });

    expect(screen.queryByText("Parent Feature")).not.toBeInTheDocument();
  });

  // ---------------------------------------------------------------------------
  // AC1 & AC5 — Display assigned labels with their configured colors
  // ---------------------------------------------------------------------------

  it("renders Labels field in the properties panel", async () => {
    renderIssueDetail();

    await waitFor(() => {
      expect(screen.getByText("Labels")).toBeInTheDocument();
    });
  });

  it("displays each assigned label name as a badge (AC1)", async () => {
    const mockLabels = [
      { id: "label-1", workspace_id: "ws-1", name: "Bug", color: "#ef4444" },
      { id: "label-2", workspace_id: "ws-1", name: "Feature", color: "#3b82f6" },
    ];
    mockApiObj.getIssue.mockResolvedValue({ ...mockIssue, labels: mockLabels });

    renderIssueDetail();

    await waitFor(() => {
      expect(screen.getByText("Bug")).toBeInTheDocument();
    });
    expect(screen.getByText("Feature")).toBeInTheDocument();
  });

  it("shows 'No labels' placeholder when issue has no labels (AC1)", async () => {
    mockApiObj.getIssue.mockResolvedValue({ ...mockIssue, labels: [] });

    renderIssueDetail();

    await waitFor(() => {
      expect(screen.getByText("No labels")).toBeInTheDocument();
    });
  });

  it("renders label badges with color-derived styles (AC5)", async () => {
    const mockLabels = [
      { id: "label-1", workspace_id: "ws-1", name: "Critical", color: "#ef4444" },
    ];
    mockApiObj.getIssue.mockResolvedValue({ ...mockIssue, labels: mockLabels });

    renderIssueDetail();

    await waitFor(() => {
      const badge = screen.getByText("Critical");
      expect(badge).toBeInTheDocument();
      // Badge uses inline color style derived from label.color
      expect(badge).toHaveStyle({ color: "#ef4444" });
    });
  });

  // ---------------------------------------------------------------------------
  // AC2 — Clicking the Labels field opens the picker listing available labels
  // ---------------------------------------------------------------------------

  it("opens label picker showing workspace labels when Labels field is clicked (AC2)", async () => {
    const allLabels = [
      { id: "label-1", workspace_id: "ws-1", name: "Bug", color: "#ef4444" },
      { id: "label-2", workspace_id: "ws-1", name: "Enhancement", color: "#22c55e" },
    ];
    mockApiObj.getWorkspaceLabels.mockResolvedValue(allLabels);
    mockApiObj.getIssue.mockResolvedValue({ ...mockIssue, labels: [] });

    const { container: _c } = renderIssueDetail();

    await waitFor(() => {
      expect(screen.getByText("No labels")).toBeInTheDocument();
    });

    // Click the trigger to open picker
    const user = userEvent.setup();
    await user.click(screen.getByText("No labels"));

    await waitFor(() => {
      expect(screen.getByText("Bug")).toBeInTheDocument();
      expect(screen.getByText("Enhancement")).toBeInTheDocument();
    });
  });

  // ---------------------------------------------------------------------------
  // AC3 — Selecting/deselecting a label in the picker calls updateIssueLabels
  // ---------------------------------------------------------------------------

  it("calls updateIssueLabels when a label is toggled in the picker (AC3)", async () => {
    const allLabels = [
      { id: "label-1", workspace_id: "ws-1", name: "Bug", color: "#ef4444" },
    ];
    mockApiObj.getWorkspaceLabels.mockResolvedValue(allLabels);
    mockApiObj.getIssue.mockResolvedValue({ ...mockIssue, labels: [] });
    mockApiObj.updateIssueLabels.mockResolvedValue(allLabels);

    renderIssueDetail();

    await waitFor(() => {
      expect(screen.getByText("No labels")).toBeInTheDocument();
    });

    const user = userEvent.setup();
    await user.click(screen.getByText("No labels"));

    await waitFor(() => {
      expect(screen.getByText("Bug")).toBeInTheDocument();
    });

    await user.click(screen.getByText("Bug"));

    await waitFor(() => {
      expect(mockApiObj.updateIssueLabels).toHaveBeenCalledWith("issue-1", ["label-1"]);
    });
  });

  it("calls updateIssueLabels with empty array when selected label is deselected (AC3)", async () => {
    const mockLabels = [
      { id: "label-1", workspace_id: "ws-1", name: "Bug", color: "#ef4444" },
    ];
    mockApiObj.getWorkspaceLabels.mockResolvedValue(mockLabels);
    mockApiObj.getIssue.mockResolvedValue({ ...mockIssue, labels: mockLabels });
    mockApiObj.updateIssueLabels.mockResolvedValue([]);

    renderIssueDetail();

    await waitFor(() => {
      expect(screen.getAllByText("Bug").length).toBeGreaterThan(0);
    });

    const user = userEvent.setup();
    // Click the badge area to open picker
    await user.click(screen.getAllByText("Bug")[0]!);

    await waitFor(() => {
      // Picker should be open showing Bug as a selectable item
      expect(mockApiObj.getWorkspaceLabels).toHaveBeenCalled();
    });

    // After picker opens, find and click Bug to deselect
    const bugItems = screen.getAllByText("Bug");
    await user.click(bugItems[bugItems.length - 1]!);

    await waitFor(() => {
      expect(mockApiObj.updateIssueLabels).toHaveBeenCalledWith("issue-1", []);
    });
  });

  // ---------------------------------------------------------------------------
  // TES-125 AC — UUID URL triggers silent redirect to identifier-based URL
  // ---------------------------------------------------------------------------

  it("calls router.replace with identifier path when rendered with a UUID issueId (TES-125 AC)", async () => {
    const uuid = "550e8400-e29b-41d4-a716-446655440000";

    renderIssueDetail(uuid);

    await waitFor(() => {
      expect(mockRouter.replace).toHaveBeenCalledWith("/issues/TES-1");
    });

    expect(mockRouter.push).not.toHaveBeenCalled();
  });
});
