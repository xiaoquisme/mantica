import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import type { Label } from "@multica/core/types";
import { LabelBadge } from "./label-badge";

// ---------------------------------------------------------------------------
// Inline label overflow renderer — mirrors the list-row.tsx label section so
// this file has no heavy external import dependencies (stores, queries, etc.).
// ---------------------------------------------------------------------------

const MAX_VISIBLE = 3;

function LabelList({ labels }: { labels: Label[] }) {
  if (!labels || labels.length === 0) return null;
  return (
    <div data-testid="label-list">
      {labels.slice(0, MAX_VISIBLE).map((l) => (
        <LabelBadge key={l.id} label={l} />
      ))}
      {labels.length > MAX_VISIBLE && (
        <span data-testid="overflow-indicator">+{labels.length - MAX_VISIBLE}</span>
      )}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Fixtures
// ---------------------------------------------------------------------------

function makeLabel(overrides?: Partial<Label>): Label {
  return {
    id: "label-1",
    workspace_id: "ws-1",
    name: "Bug",
    color: "#ef4444",
    ...overrides,
  };
}

// ---------------------------------------------------------------------------
// LabelBadge
// ---------------------------------------------------------------------------

describe("LabelBadge", () => {
  it("renders the label name", () => {
    render(<LabelBadge label={makeLabel({ name: "Feature" })} />);
    expect(screen.getByText("Feature")).toBeTruthy();
  });

  it("applies the configured color as inline text color and semi-transparent background", () => {
    render(<LabelBadge label={makeLabel({ color: "#3b82f6" })} />);
    const span = screen.getByText("Bug") as HTMLElement;
    expect(span.style.color).toBe("rgb(59, 130, 246)");
    // jsdom normalizes hex+alpha to rgba; 0x33/0xff = 0.2
    expect(span.style.backgroundColor).toBe("rgba(59, 130, 246, 0.2)");
    expect(span.style.borderColor).toBe("rgba(59, 130, 246, 0.333)");
  });

  it("renders different label colors independently", () => {
    const { rerender } = render(<LabelBadge label={makeLabel({ color: "#ef4444" })} />);
    expect((screen.getByText("Bug") as HTMLElement).style.color).toBe("rgb(239, 68, 68)");

    rerender(<LabelBadge label={makeLabel({ color: "#22c55e" })} />);
    expect((screen.getByText("Bug") as HTMLElement).style.color).toBe("rgb(34, 197, 94)");
  });
});

// ---------------------------------------------------------------------------
// Label overflow display (mirrors list-row.tsx label section)
// ---------------------------------------------------------------------------

describe("label overflow display", () => {
  it("renders nothing when there are no labels", () => {
    const { container } = render(<LabelList labels={[]} />);
    expect(container.firstChild).toBeNull();
  });

  it("renders all labels when count is within the max visible limit", () => {
    const labels = [
      makeLabel({ id: "l1", name: "Alpha" }),
      makeLabel({ id: "l2", name: "Beta" }),
      makeLabel({ id: "l3", name: "Gamma" }),
    ];
    render(<LabelList labels={labels} />);

    expect(screen.getByText("Alpha")).toBeTruthy();
    expect(screen.getByText("Beta")).toBeTruthy();
    expect(screen.getByText("Gamma")).toBeTruthy();
    expect(screen.queryByTestId("overflow-indicator")).toBeNull();
  });

  it("shows +1 overflow indicator when there are 4 labels", () => {
    const labels = [
      makeLabel({ id: "l1", name: "Alpha" }),
      makeLabel({ id: "l2", name: "Beta" }),
      makeLabel({ id: "l3", name: "Gamma" }),
      makeLabel({ id: "l4", name: "Delta" }),
    ];
    render(<LabelList labels={labels} />);

    expect(screen.getByText("Alpha")).toBeTruthy();
    expect(screen.getByText("Beta")).toBeTruthy();
    expect(screen.getByText("Gamma")).toBeTruthy();
    expect(screen.queryByText("Delta")).toBeNull();
    expect(screen.getByTestId("overflow-indicator").textContent).toBe("+1");
  });

  it("shows +N overflow indicator for many labels", () => {
    const labels = Array.from({ length: 7 }, (_, i) =>
      makeLabel({ id: `l${i}`, name: `Label ${i}` }),
    );
    render(<LabelList labels={labels} />);

    expect(screen.getByText("Label 0")).toBeTruthy();
    expect(screen.getByText("Label 1")).toBeTruthy();
    expect(screen.getByText("Label 2")).toBeTruthy();
    expect(screen.queryByText("Label 3")).toBeNull();
    expect(screen.getByTestId("overflow-indicator").textContent).toBe("+4");
  });

  it("does not show overflow indicator for exactly 3 labels", () => {
    const labels = Array.from({ length: 3 }, (_, i) =>
      makeLabel({ id: `l${i}`, name: `Label ${i}` }),
    );
    render(<LabelList labels={labels} />);
    expect(screen.queryByTestId("overflow-indicator")).toBeNull();
  });
});
