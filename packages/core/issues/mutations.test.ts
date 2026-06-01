import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, act } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import React from "react";
import { issueKeys } from "./queries";
import type { Issue, Label } from "../types";
import type { ListIssuesResponse } from "../types";

// ---------------------------------------------------------------------------
// Mocks
// ---------------------------------------------------------------------------

vi.mock("../hooks", () => ({
  useWorkspaceId: () => "ws-1",
}));

const mockUpdateIssueLabels = vi.hoisted(() => vi.fn());

vi.mock("../api", () => ({
  api: { updateIssueLabels: mockUpdateIssueLabels },
}));

import { useUpdateIssueLabels } from "./mutations";

// ---------------------------------------------------------------------------
// Fixtures
// ---------------------------------------------------------------------------

const WS_ID = "ws-1";
const ISSUE_ID = "issue-1";

const oldLabel: Label = { id: "label-old", workspace_id: WS_ID, name: "Old", color: "#aaa" };
const newLabel: Label = { id: "label-new", workspace_id: WS_ID, name: "New", color: "#bbb" };

function makeIssue(labels: Label[]): Issue {
  return {
    id: ISSUE_ID,
    workspace_id: WS_ID,
    number: 1,
    identifier: "TES-1",
    title: "Test Issue",
    description: null,
    status: "doing",
    priority: "high",
    assignee_type: null,
    assignee_id: null,
    creator_type: "member",
    creator_id: "user-1",
    parent_issue_id: null,
    project_id: null,
    position: 0,
    due_date: null,
    created_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-01T00:00:00Z",
    labels,
  };
}

function makeListResponse(issue: Issue): ListIssuesResponse {
  return { issues: [issue], total: 1 };
}

function createQc() {
  return new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
}

function makeWrapper(qc: QueryClient) {
  return ({ children }: { children: React.ReactNode }) =>
    React.createElement(QueryClientProvider, { client: qc }, children);
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe("useUpdateIssueLabels — onSuccess cache update", () => {
  let qc: QueryClient;

  beforeEach(() => {
    vi.clearAllMocks();
    qc = createQc();
  });

  it("writes server-confirmed labels into the detail cache (AC1)", async () => {
    qc.setQueryData(issueKeys.detail(WS_ID, ISSUE_ID), makeIssue([oldLabel]));
    mockUpdateIssueLabels.mockResolvedValue([newLabel]);

    const { result } = renderHook(() => useUpdateIssueLabels(), {
      wrapper: makeWrapper(qc),
    });

    await act(async () => {
      await result.current.mutateAsync({ issueId: ISSUE_ID, labelIds: [newLabel.id] });
    });

    const cached = qc.getQueryData<Issue>(issueKeys.detail(WS_ID, ISSUE_ID));
    expect(cached?.labels).toEqual([newLabel]);
  });

  it("writes server-confirmed labels into the list cache (AC1)", async () => {
    qc.setQueryData(issueKeys.list(WS_ID), makeListResponse(makeIssue([oldLabel])));
    mockUpdateIssueLabels.mockResolvedValue([newLabel]);

    const { result } = renderHook(() => useUpdateIssueLabels(), {
      wrapper: makeWrapper(qc),
    });

    await act(async () => {
      await result.current.mutateAsync({ issueId: ISSUE_ID, labelIds: [newLabel.id] });
    });

    const listData = qc.getQueryData<ListIssuesResponse>(issueKeys.list(WS_ID));
    const updatedIssue = listData?.issues.find((i) => i.id === ISSUE_ID);
    expect(updatedIssue?.labels).toEqual([newLabel]);
  });

  it("replaces optimistic labels with server-confirmed labels (AC3 — optimistic and server state in sync)", async () => {
    qc.setQueryData(issueKeys.detail(WS_ID, ISSUE_ID), makeIssue([oldLabel]));
    qc.setQueryData(issueKeys.list(WS_ID), makeListResponse(makeIssue([oldLabel])));
    const serverLabels = [newLabel];
    mockUpdateIssueLabels.mockResolvedValue(serverLabels);

    const { result } = renderHook(() => useUpdateIssueLabels(), {
      wrapper: makeWrapper(qc),
    });

    await act(async () => {
      await result.current.mutateAsync({ issueId: ISSUE_ID, labelIds: [newLabel.id] });
    });

    const detail = qc.getQueryData<Issue>(issueKeys.detail(WS_ID, ISSUE_ID));
    const listData = qc.getQueryData<ListIssuesResponse>(issueKeys.list(WS_ID));
    const listIssue = listData?.issues.find((i) => i.id === ISSUE_ID);

    expect(detail?.labels).toEqual(serverLabels);
    expect(listIssue?.labels).toEqual(serverLabels);
  });

  it("does not initiate background fetches when there are no active query observers", async () => {
    qc.setQueryData(issueKeys.detail(WS_ID, ISSUE_ID), makeIssue([oldLabel]));
    qc.setQueryData(issueKeys.list(WS_ID), makeListResponse(makeIssue([oldLabel])));
    mockUpdateIssueLabels.mockResolvedValue([newLabel]);

    const fetchingKeys: string[] = [];
    qc.getQueryCache().subscribe((event) => {
      if (event.query.state.fetchStatus === "fetching") {
        fetchingKeys.push(JSON.stringify(event.query.queryKey));
      }
    });

    const { result } = renderHook(() => useUpdateIssueLabels(), {
      wrapper: makeWrapper(qc),
    });

    await act(async () => {
      await result.current.mutateAsync({ issueId: ISSUE_ID, labelIds: [newLabel.id] });
    });

    // invalidateQueries marks queries stale but without active observers no fetch is initiated
    expect(fetchingKeys).toHaveLength(0);
  });
});
