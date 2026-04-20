import { describe, it, expect } from "vitest";
import { getDescendantIds, filterParentCandidates } from "./parent-picker";
import type { Issue } from "@multica/core/types";

function makeIssue(id: string, parent_issue_id: string | null = null, overrides: Partial<Issue> = {}): Issue {
  return {
    id,
    workspace_id: "ws-1",
    number: 1,
    identifier: id.toUpperCase(),
    title: `Issue ${id}`,
    description: null,
    status: "backlog",
    priority: "none",
    assignee_type: null,
    assignee_id: null,
    creator_type: "member",
    creator_id: "user-1",
    parent_issue_id,
    project_id: null,
    position: 0,
    due_date: null,
    created_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-01T00:00:00Z",
    ...overrides,
  };
}

describe("getDescendantIds (TES-108)", () => {
  it("returns empty set when issue has no children", () => {
    const issues = [makeIssue("a"), makeIssue("b"), makeIssue("c")];
    expect(getDescendantIds(issues, "a").size).toBe(0);
  });

  it("returns direct children", () => {
    const issues = [makeIssue("a"), makeIssue("b", "a"), makeIssue("c", "a"), makeIssue("d")];
    const result = getDescendantIds(issues, "a");
    expect(result).toEqual(new Set(["b", "c"]));
  });

  it("returns nested descendants (grandchildren)", () => {
    const issues = [makeIssue("a"), makeIssue("b", "a"), makeIssue("c", "b"), makeIssue("d")];
    const result = getDescendantIds(issues, "a");
    expect(result).toEqual(new Set(["b", "c"]));
  });

  it("returns deeply nested descendants", () => {
    const issues = [
      makeIssue("root"),
      makeIssue("child", "root"),
      makeIssue("grandchild", "child"),
      makeIssue("great-grandchild", "grandchild"),
      makeIssue("unrelated"),
    ];
    const result = getDescendantIds(issues, "root");
    expect(result).toEqual(new Set(["child", "grandchild", "great-grandchild"]));
  });

  it("does not include the root issue itself", () => {
    const issues = [makeIssue("a"), makeIssue("b", "a")];
    const result = getDescendantIds(issues, "a");
    expect(result.has("a")).toBe(false);
  });

  it("does not include unrelated issues", () => {
    const issues = [makeIssue("a"), makeIssue("b", "a"), makeIssue("c")];
    const result = getDescendantIds(issues, "a");
    expect(result.has("c")).toBe(false);
  });
});

// ---------------------------------------------------------------------------
// filterParentCandidates — covers TES-102 ACs 2, 3, 6, 7
// ---------------------------------------------------------------------------

describe("filterParentCandidates (TES-102)", () => {
  const issues = [
    makeIssue("current", null),
    makeIssue("alpha", null, { title: "Alpha Feature", identifier: "TES-1" }),
    makeIssue("beta", null, { title: "Beta Fix", identifier: "TES-2" }),
    makeIssue("child-of-current", "current", { title: "Child Issue", identifier: "TES-3" }),
    makeIssue("grandchild", "child-of-current", { title: "Grandchild Issue", identifier: "TES-4" }),
    makeIssue("unrelated", null, { title: "Unrelated Task", identifier: "TES-5" }),
  ];

  // AC6 — self-parent prevention
  it("AC6: excludes the current issue itself", () => {
    const result = filterParentCandidates(issues, "current", "");
    expect(result.some((i) => i.id === "current")).toBe(false);
  });

  // AC7 — circular dependency prevention
  it("AC7: excludes direct children of the current issue", () => {
    const result = filterParentCandidates(issues, "current", "");
    expect(result.some((i) => i.id === "child-of-current")).toBe(false);
  });

  it("AC7: excludes grandchildren (deep descendants) of the current issue", () => {
    const result = filterParentCandidates(issues, "current", "");
    expect(result.some((i) => i.id === "grandchild")).toBe(false);
  });

  it("AC7: includes issues that are ancestors (not descendants)", () => {
    const result = filterParentCandidates(issues, "child-of-current", "");
    expect(result.some((i) => i.id === "current")).toBe(true);
  });

  // AC2 — search by title
  it("AC2: filters by title substring (case-insensitive)", () => {
    const result = filterParentCandidates(issues, "current", "alpha");
    expect(result.length).toBe(1);
    expect(result[0].id).toBe("alpha");
  });

  // AC2 — search by identifier
  it("AC2: filters by identifier substring (case-insensitive)", () => {
    const result = filterParentCandidates(issues, "current", "tes-2");
    expect(result.length).toBe(1);
    expect(result[0].id).toBe("beta");
  });

  it("AC2: returns no results when query matches nothing", () => {
    const result = filterParentCandidates(issues, "current", "zzznomatch");
    expect(result.length).toBe(0);
  });

  // AC3 — results include identifier and title
  it("AC3: result items expose both identifier and title fields", () => {
    const result = filterParentCandidates(issues, "current", "alpha");
    expect(result[0].identifier).toBe("TES-1");
    expect(result[0].title).toBe("Alpha Feature");
  });

  // Empty query returns all valid candidates
  it("returns all valid candidates when query is empty", () => {
    const result = filterParentCandidates(issues, "current", "");
    // excludes: current, child-of-current, grandchild → leaves alpha, beta, unrelated
    expect(result.length).toBe(3);
  });
});
