import { describe, it, expect } from "vitest";
import { getDescendantIds } from "./parent-picker";
import type { Issue } from "@multica/core/types";

function makeIssue(id: string, parent_issue_id: string | null = null): Issue {
  return {
    id,
    workspace_id: "ws-1",
    number: 1,
    identifier: id,
    title: id,
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
  };
}

describe("getDescendantIds (TES-108)", () => {
  it("returns empty set when issue has no children", () => {
    const issues = [makeIssue("A"), makeIssue("B"), makeIssue("C")];
    expect(getDescendantIds(issues, "A").size).toBe(0);
  });

  it("returns direct children", () => {
    const issues = [makeIssue("A"), makeIssue("B", "A"), makeIssue("C", "A"), makeIssue("D")];
    const result = getDescendantIds(issues, "A");
    expect(result).toEqual(new Set(["B", "C"]));
  });

  it("returns nested descendants (grandchildren)", () => {
    const issues = [makeIssue("A"), makeIssue("B", "A"), makeIssue("C", "B"), makeIssue("D")];
    const result = getDescendantIds(issues, "A");
    expect(result).toEqual(new Set(["B", "C"]));
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
    const issues = [makeIssue("A"), makeIssue("B", "A")];
    const result = getDescendantIds(issues, "A");
    expect(result.has("A")).toBe(false);
  });

  it("does not include unrelated issues", () => {
    const issues = [makeIssue("A"), makeIssue("B", "A"), makeIssue("C")];
    const result = getDescendantIds(issues, "A");
    expect(result.has("C")).toBe(false);
  });
});
