import { describe, it, expect, vi } from "vitest";
import type { IssueStatus } from "@mantica/core/types";
import { deriveOrderedStatuses } from "./board-view";

describe("deriveOrderedStatuses", () => {
  it("returns visibleStatuses unchanged when columnOrder is empty", () => {
    const visible: IssueStatus[] = ["backlog", "doing", "done"];
    const result = deriveOrderedStatuses(visible, []);
    expect(result).toEqual(["backlog", "doing", "done"]);
  });

  it("reorders visible statuses according to columnOrder", () => {
    const visible: IssueStatus[] = ["backlog", "doing", "done"];
    const order: IssueStatus[] = ["doing", "backlog", "done"];
    const result = deriveOrderedStatuses(visible, order);
    expect(result).toEqual(["doing", "backlog", "done"]);
  });

  it("appends visible statuses not in columnOrder at the end", () => {
    const visible: IssueStatus[] = ["backlog", "doing", "done", "blocked"];
    const order: IssueStatus[] = ["done", "backlog"];
    const result = deriveOrderedStatuses(visible, order);
    // done, backlog come first (from order), then in_dev and blocked appended
    expect(result).toEqual(["done", "backlog", "doing", "blocked"]);
  });

  it("filters out statuses in columnOrder that are not visible", () => {
    const visible: IssueStatus[] = ["backlog", "doing"];
    // columnOrder has "done" which is not visible (hidden)
    const order: IssueStatus[] = ["done", "doing", "backlog"];
    const result = deriveOrderedStatuses(visible, order);
    // "done" is not visible, so filtered; in_dev and backlog in order
    expect(result).toEqual(["doing", "backlog"]);
  });

  it("handles visibleStatuses with single item", () => {
    const visible: IssueStatus[] = ["backlog"];
    const order: IssueStatus[] = ["doing", "backlog"];
    const result = deriveOrderedStatuses(visible, order);
    expect(result).toEqual(["backlog"]);
  });

  it("handles empty visibleStatuses", () => {
    const result = deriveOrderedStatuses([], ["backlog", "doing"]);
    expect(result).toEqual([]);
  });

  it("preserves all visible statuses even when none overlap with columnOrder", () => {
    const visible: IssueStatus[] = ["backlog", "doing", "done"];
    const order: IssueStatus[] = ["cancelled", "blocked"];
    const result = deriveOrderedStatuses(visible, order);
    // No overlap: all visible statuses appended as unordered
    expect(result).toEqual(["backlog", "doing", "done"]);
  });
});

// ---------------------------------------------------------------------------
// Mock @dnd-kit modules for integration-style behavior tests
// ---------------------------------------------------------------------------

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
  arrayMove: (arr: any[], from: number, to: number) => {
    const result = [...arr];
    const [item] = result.splice(from, 1);
    result.splice(to, 0, item);
    return result;
  },
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

describe("deriveOrderedStatuses integration — store sync scenario", () => {
  it("correctly handles show/hide column interaction with existing order", () => {
    // Simulate: user reordered to [in_dev, backlog, done], then hides in_dev
    const visible: IssueStatus[] = ["backlog", "done"]; // in_dev hidden
    const savedOrder: IssueStatus[] = ["doing", "backlog", "done"];
    const result = deriveOrderedStatuses(visible, savedOrder);
    // in_dev filtered out, backlog and done remain in saved order
    expect(result).toEqual(["backlog", "done"]);
  });

  it("correctly handles show column — appends newly shown column at end", () => {
    // Simulate: user shows in_review which wasn't in the saved order
    const visible: IssueStatus[] = ["backlog", "done", "doing"];
    const savedOrder: IssueStatus[] = ["done", "backlog"];
    const result = deriveOrderedStatuses(visible, savedOrder);
    // in_review not in savedOrder → appended at end
    expect(result).toEqual(["done", "backlog", "doing"]);
  });
});
