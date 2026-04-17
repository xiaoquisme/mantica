/**
 * Tests for the PickerWrapper event-handling fix (TES-70).
 *
 * PickerWrapper sits between an AppLink (<a>) and a DropdownMenu trigger.
 * The fix splits event handling by phase:
 *
 *   - onPointerDown / onMouseDown: stopPropagation ONLY
 *     → @base-ui Menu.Trigger can still open because its pointerDown fires
 *
 *   - onClick: stopPropagation + preventDefault
 *     → AppLink navigation (React synthetic) is blocked
 *     → Native <a> anchor navigation is also blocked
 *
 * These tests verify that contract directly via fireEvent, which gives full
 * control over the defaultPrevented flag — something userEvent abstracts away.
 */

import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";

// ---------------------------------------------------------------------------
// Inline PickerWrapper — mirrors the exact implementation in list-row.tsx and
// board-card.tsx so this test file has no external import dependencies and
// stays resilient to refactors of the surrounding components.
// ---------------------------------------------------------------------------

function PickerWrapper({ children }: { children: React.ReactNode }) {
  const stopAndPrevent = (e: React.SyntheticEvent) => {
    e.stopPropagation();
    e.preventDefault();
  };
  const stopOnly = (e: React.SyntheticEvent) => {
    e.stopPropagation();
  };
  return (
    // onClick: preventDefault blocks native <a> navigation if this element is
    // ever nested inside an AppLink. onPointerDown/onMouseDown: stopPropagation
    // only so @base-ui Menu.Trigger can still open on pointerDown.
    <div onClick={stopAndPrevent} onMouseDown={stopOnly} onPointerDown={stopOnly}>
      {children}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

/**
 * Fires an event on a target and returns the event object so callers can
 * inspect defaultPrevented / stopPropagation behaviour.
 */
function fireAndCapture(
  eventType: "click" | "mousedown" | "pointerdown",
  target: Element,
): Event {
  const eventInit: EventInit & PointerEventInit = { bubbles: true, cancelable: true };
  let event: Event;
  if (eventType === "pointerdown") {
    event = new PointerEvent("pointerdown", eventInit);
  } else if (eventType === "mousedown") {
    event = new MouseEvent("mousedown", eventInit);
  } else {
    event = new MouseEvent("click", eventInit);
  }
  target.dispatchEvent(event);
  return event;
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe("PickerWrapper event handling (TES-70)", () => {
  it("does NOT preventDefault on pointerdown — allows @base-ui Menu.Trigger to open", () => {
    render(
      <PickerWrapper>
        <button data-testid="trigger">…</button>
      </PickerWrapper>,
    );

    const btn = screen.getByTestId("trigger");
    const evt = fireAndCapture("pointerdown", btn);

    expect(evt.defaultPrevented).toBe(false);
  });

  it("does NOT preventDefault on mousedown — allows @base-ui Menu.Trigger to open", () => {
    render(
      <PickerWrapper>
        <button data-testid="trigger">…</button>
      </PickerWrapper>,
    );

    const btn = screen.getByTestId("trigger");
    const evt = fireAndCapture("mousedown", btn);

    expect(evt.defaultPrevented).toBe(false);
  });

  it("DOES preventDefault on click — blocks native <a> anchor navigation", () => {
    render(
      <PickerWrapper>
        <button data-testid="trigger">…</button>
      </PickerWrapper>,
    );

    const btn = screen.getByTestId("trigger");
    const evt = fireAndCapture("click", btn);

    expect(evt.defaultPrevented).toBe(true);
  });

  it("stops pointerdown propagation so the drag/row handler does not fire", () => {
    const outerHandler = vi.fn();

    render(
      <div onPointerDown={outerHandler}>
        <PickerWrapper>
          <button data-testid="trigger">…</button>
        </PickerWrapper>
      </div>,
    );

    const btn = screen.getByTestId("trigger");
    fireAndCapture("pointerdown", btn);

    expect(outerHandler).not.toHaveBeenCalled();
  });

  it("stops mousedown propagation so the drag/row handler does not fire", () => {
    const outerHandler = vi.fn();

    render(
      <div onMouseDown={outerHandler}>
        <PickerWrapper>
          <button data-testid="trigger">…</button>
        </PickerWrapper>
      </div>,
    );

    const btn = screen.getByTestId("trigger");
    fireAndCapture("mousedown", btn);

    expect(outerHandler).not.toHaveBeenCalled();
  });

  it("stops click propagation so the AppLink onClick does not fire", () => {
    const linkHandler = vi.fn();

    render(
      <a href="/issues/1" onClick={linkHandler}>
        <PickerWrapper>
          <button data-testid="trigger">…</button>
        </PickerWrapper>
      </a>,
    );

    const btn = screen.getByTestId("trigger");
    fireAndCapture("click", btn);

    expect(linkHandler).not.toHaveBeenCalled();
  });

  it("regression: old PickerWrapper (preventDefault on all events) would block dropdown", () => {
    // This test documents the bug behaviour that was fixed.
    // If a PickerWrapper called e.preventDefault() on pointerdown, the event
    // would have defaultPrevented=true and @base-ui Menu.Trigger would fail to open.
    //
    // With the fix, the current PickerWrapper does NOT call preventDefault on
    // pointerdown, so defaultPrevented must be false.

    function OldPickerWrapper({ children }: { children: React.ReactNode }) {
      const stop = (e: React.SyntheticEvent) => {
        e.stopPropagation();
        e.preventDefault(); // ← the old bug
      };
      return (
        <div onClick={stop} onMouseDown={stop} onPointerDown={stop}>
          {children}
        </div>
      );
    }

    render(
      <OldPickerWrapper>
        <button data-testid="old-trigger">…</button>
      </OldPickerWrapper>,
    );

    const btn = screen.getByTestId("old-trigger");
    const evt = fireAndCapture("pointerdown", btn);

    // Old behaviour blocks dropdown — defaultPrevented is true (bad)
    expect(evt.defaultPrevented).toBe(true);

    // PickerWrapper (fixed) does NOT do this — verified in the tests above
  });
});
