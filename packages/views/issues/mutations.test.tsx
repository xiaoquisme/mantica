import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, act, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { ReactNode } from "react";
import type { Issue, Label, ListIssuesResponse } from "@multica/core/types";
import { useUpdateIssueLabels } from "@multica/core/issues/mutations";
import { issueKeys } from "@multica/core/issues/queries";

// ---------------------------------------------------------------------------
// Mocks
// ---------------------------------------------------------------------------

vi.mock("@multica/core/hooks", () => ({
  useWorkspaceId: () => "ws-1",
}));

const mockUpdateIssueLabels = vi.fn();

vi.mock("@multica/core/api", () => ({
  api: {
    updateIssueLabels: (...args: unknown[]) => mockUpdateIssueLabels(...args),
  },
}));

// ---------------------------------------------------------------------------
// Fixtures
// ---------------------------------------------------------------------------

const WS_ID = "ws-1";
const ISSUE_ID = "issue-1";

function makeIssue(overrides?: Partial<Issue>): Issue {
  return {
    id: ISSUE_ID,
    workspace_id: WS_ID,
    number: 1,
    identifier: "TES-1",
    title: "Test issue",
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
    labels: [],
    created_at: "2024-01-01T00:00:00Z",
    updated_at: "2024-01-01T00:00:00Z",
    ...overrides,
  };
}

const CONFIRMED_LABELS: Label[] = [
  { id: "label-1", workspace_id: WS_ID, name: "Bug", color: "#ef4444" },
];

// ---------------------------------------------------------------------------
// Render helper
// ---------------------------------------------------------------------------

function makeWrapper(qc: QueryClient) {
  return function Wrapper({ children }: { children: ReactNode }) {
    return <QueryClientProvider client={qc}>{children}</QueryClientProvider>;
  };
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe("useUpdateIssueLabels — onSuccess cache update", () => {
  let qc: QueryClient;

  beforeEach(() => {
    qc = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } });
    mockUpdateIssueLabels.mockReset();
  });

  it("AC1: writes server-confirmed labels into the detail cache on success", async () => {
    const issue = makeIssue({ labels: [] });
    qc.setQueryData<Issue>(issueKeys.detail(WS_ID, ISSUE_ID), issue);
    mockUpdateIssueLabels.mockResolvedValue(CONFIRMED_LABELS);

    const { result } = renderHook(() => useUpdateIssueLabels(), { wrapper: makeWrapper(qc) });

    await act(async () => {
      result.current.mutate({ issueId: ISSUE_ID, labelIds: ["label-1"] });
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    const cached = qc.getQueryData<Issue>(issueKeys.detail(WS_ID, ISSUE_ID));
    expect(cached?.labels).toEqual(CONFIRMED_LABELS);
  });

  it("AC1: writes server-confirmed labels into the list cache on success", async () => {
    const issue = makeIssue({ labels: [] });
    const list: ListIssuesResponse = { issues: [issue], total: 1 };
    qc.setQueryData<ListIssuesResponse>(issueKeys.list(WS_ID), list);
    mockUpdateIssueLabels.mockResolvedValue(CONFIRMED_LABELS);

    const { result } = renderHook(() => useUpdateIssueLabels(), { wrapper: makeWrapper(qc) });

    await act(async () => {
      result.current.mutate({ issueId: ISSUE_ID, labelIds: ["label-1"] });
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    const cached = qc.getQueryData<ListIssuesResponse>(issueKeys.list(WS_ID));
    const cachedIssue = cached?.issues.find((i) => i.id === ISSUE_ID);
    expect(cachedIssue?.labels).toEqual(CONFIRMED_LABELS);
  });

  it("AC3: cache reflects server-confirmed labels, not just optimistic ones", async () => {
    const serverLabels: Label[] = [
      { id: "label-server", workspace_id: WS_ID, name: "ServerOnly", color: "#00ff00" },
    ];
    const issue = makeIssue({ labels: [] });
    qc.setQueryData<Issue>(issueKeys.detail(WS_ID, ISSUE_ID), issue);
    // Server returns different labels than what was optimistically applied
    mockUpdateIssueLabels.mockResolvedValue(serverLabels);

    const { result } = renderHook(() => useUpdateIssueLabels(), { wrapper: makeWrapper(qc) });

    await act(async () => {
      result.current.mutate({ issueId: ISSUE_ID, labelIds: ["label-1"] });
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    const cached = qc.getQueryData<Issue>(issueKeys.detail(WS_ID, ISSUE_ID));
    expect(cached?.labels).toEqual(serverLabels);
  });
});
