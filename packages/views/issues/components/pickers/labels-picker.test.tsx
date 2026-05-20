import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { Label } from "@mantica/core/types";
import { LabelsPicker } from "./labels-picker";

// ---------------------------------------------------------------------------
// Fixtures
// ---------------------------------------------------------------------------

const MOCK_LABELS: Label[] = [
  { id: "label-1", workspace_id: "ws-1", name: "Bug", color: "#ef4444" },
  { id: "label-2", workspace_id: "ws-1", name: "Feature", color: "#3b82f6" },
];

function makeLabel(overrides?: Partial<Label>): Label {
  return { id: "label-1", workspace_id: "ws-1", name: "Bug", color: "#ef4444", ...overrides };
}

// ---------------------------------------------------------------------------
// Mocks
// ---------------------------------------------------------------------------

vi.mock("@mantica/core/issues/queries", () => ({
  labelListOptions: () => ({ queryKey: ["labels", "ws-1"], queryFn: () => MOCK_LABELS }),
}));

vi.mock("@mantica/core/hooks", () => ({
  useWorkspaceId: () => "ws-1",
}));

const mockMutate = vi.hoisted(() => vi.fn());

vi.mock("@mantica/core/issues/mutations", () => ({
  useCreateLabel: () => ({ mutate: mockMutate, isPending: false }),
}));

// Mock PropertyPicker to avoid @base-ui portal complexity in jsdom.
// The mock renders children and footer inline so tests can interact with them
// without triggering the popover open/close mechanism.
vi.mock("./property-picker", () => ({
  PropertyPicker: ({
    children,
    footer,
  }: {
    children: React.ReactNode;
    footer?: React.ReactNode;
    open: boolean;
    onOpenChange: (v: boolean) => void;
    trigger: React.ReactNode;
    searchable?: boolean;
    onSearchChange?: (q: string) => void;
    width?: string;
    align?: "start" | "center" | "end";
  }) => (
    <div data-testid="property-picker">
      <div data-testid="picker-items">{children}</div>
      {footer && <div data-testid="picker-footer">{footer}</div>}
    </div>
  ),
  PickerItem: ({
    children,
    onClick,
    selected,
  }: {
    children: React.ReactNode;
    onClick: () => void;
    selected: boolean;
  }) => (
    <button type="button" onClick={onClick} data-selected={String(selected)}>
      {children}
    </button>
  ),
  PickerEmpty: () => <div>No results</div>,
}));

// ---------------------------------------------------------------------------
// Render helper
// ---------------------------------------------------------------------------

function renderPicker(labels: Label[] = [], onUpdate = vi.fn()) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  qc.setQueryData(["labels", "ws-1"], MOCK_LABELS);
  return {
    onUpdate,
    ...render(
      <QueryClientProvider client={qc}>
        <LabelsPicker labels={labels} onUpdate={onUpdate} />
      </QueryClientProvider>,
    ),
  };
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe("LabelsPicker — inline create form", () => {
  beforeEach(() => {
    mockMutate.mockReset();
  });

  // AC1: '+ Create label' footer button is always visible in the picker
  it("AC1: renders '+ Create label' button in the picker footer", () => {
    renderPicker();
    expect(screen.getByRole("button", { name: /create label/i })).toBeInTheDocument();
  });

  // AC2: clicking the button reveals the name input and color swatches
  it("AC2: clicking '+ Create label' shows the label name input", async () => {
    const user = userEvent.setup();
    renderPicker();
    await user.click(screen.getByRole("button", { name: /create label/i }));
    expect(screen.getByPlaceholderText("Label name")).toBeInTheDocument();
  });

  it("AC2: clicking '+ Create label' shows color swatches", async () => {
    const user = userEvent.setup();
    renderPicker();
    await user.click(screen.getByRole("button", { name: /create label/i }));
    // 10 color swatches, each a round button with a background-color style
    const footer = screen.getByTestId("picker-footer");
    const colorButtons = footer.querySelectorAll("button[style*='background-color']");
    expect(colorButtons.length).toBe(10);
  });

  // AC3: valid submission calls the mutation and propagates the new label via onUpdate
  it("AC3: submitting with a valid name calls useCreateLabel with name and color", async () => {
    const user = userEvent.setup();
    renderPicker();
    await user.click(screen.getByRole("button", { name: /create label/i }));
    await user.type(screen.getByPlaceholderText("Label name"), "New Label");
    await user.click(screen.getByRole("button", { name: /^create$/i }));
    expect(mockMutate).toHaveBeenCalledWith(
      expect.objectContaining({ name: "New Label" }),
      expect.any(Object),
    );
  });

  it("AC3: onUpdate is called with existing label IDs plus the new label ID on success", async () => {
    const newLabel: Label = { id: "new-id", workspace_id: "ws-1", name: "New Label", color: "#ef4444" };
    mockMutate.mockImplementation(
      (_: unknown, options: { onSuccess?: (label: Label) => void }) => {
        options?.onSuccess?.(newLabel);
      },
    );
    const user = userEvent.setup();
    const onUpdate = vi.fn();
    renderPicker([makeLabel({ id: "label-1" })], onUpdate);
    await user.click(screen.getByRole("button", { name: /create label/i }));
    await user.type(screen.getByPlaceholderText("Label name"), "New Label");
    await user.click(screen.getByRole("button", { name: /^create$/i }));
    expect(onUpdate).toHaveBeenCalledWith(["label-1", "new-id"]);
  });

  // Cancel hides the form without calling the mutation
  it("cancel button hides the create form and restores the '+ Create label' button", async () => {
    const user = userEvent.setup();
    renderPicker();
    await user.click(screen.getByRole("button", { name: /create label/i }));
    expect(screen.getByPlaceholderText("Label name")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: /^cancel$/i }));
    expect(screen.queryByPlaceholderText("Label name")).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: /create label/i })).toBeInTheDocument();
    expect(mockMutate).not.toHaveBeenCalled();
  });

  // Submit disabled when name is empty
  it("submit button is disabled when the name input is empty", async () => {
    const user = userEvent.setup();
    renderPicker();
    await user.click(screen.getByRole("button", { name: /create label/i }));
    expect(screen.getByRole("button", { name: /^create$/i })).toBeDisabled();
  });
});
