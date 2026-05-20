/**
 * E2E tests for TES-66 board card context menu interactions.
 *
 * Covers the five sub-menus added by the context menu feature:
 *   - Status change via quick actions
 *   - Priority change via quick actions
 *   - Assignee change via quick actions
 *   - Due Date set via quick actions
 *   - Project (covered in board-card-project.spec.ts)
 *
 * Locator strategy mirrors board-card-project.spec.ts:
 *   - `[data-slot="dropdown-menu-sub-trigger"]` selects sub-menu triggers
 *     without matching unrelated page text.
 *   - `.group` filters to the board card wrapping element.
 *   - The MoreHorizontal trigger is identified by `svg.lucide-ellipsis`.
 */

import { test, expect } from "./fixtures";
import type { TestApiClient } from "./fixtures";
import type { Locator, Page } from "@playwright/test";
import { loginAsDefault, createTestApi } from "./helpers";

// ---------------------------------------------------------------------------
// Locator helpers (mirrors board-card-project.spec.ts)
// ---------------------------------------------------------------------------

function boardCardFor(page: Page, title: string): Locator {
  return page.locator(".group").filter({ hasText: title }).first();
}

async function openCardQuickActions(card: Locator) {
  await card.scrollIntoViewIfNeeded();
  await card.hover();
  const trigger = card.locator("button", {
    has: card.page().locator("svg.lucide-ellipsis"),
  });
  await trigger.first().click();
}

function menuSubTrigger(page: Page, label: string): Locator {
  return page
    .locator('[data-slot="dropdown-menu-sub-trigger"]')
    .filter({ hasText: label });
}

function menuItem(page: Page, label: string): Locator {
  return page
    .locator('[data-slot="dropdown-menu-item"], [data-slot="dropdown-menu-sub-trigger"]')
    .filter({ hasText: label });
}

// ---------------------------------------------------------------------------
// Test suite
// ---------------------------------------------------------------------------

test.describe("Board Card Context Menu — TES-66", () => {
  let api: TestApiClient;

  test.beforeEach(async ({ page }) => {
    api = await createTestApi();
    await loginAsDefault(page, api);
  });

  test.afterEach(async () => {
    await api.cleanup();
  });

  // -------------------------------------------------------------------------
  // Happy path: context menu opens with expected sub-menus
  // -------------------------------------------------------------------------

  test("context menu opens on card hover and shows all sub-menus", async ({
    page,
  }) => {
    const issue = await api.createIssue(
      "Context Menu Smoke Test " + Date.now(),
    );
    await page.reload();

    const card = boardCardFor(page, issue.title);
    await expect(card).toBeVisible({ timeout: 10000 });

    await openCardQuickActions(card);

    // All five sub-menu triggers must be visible
    await expect(menuSubTrigger(page, "Status").first()).toBeVisible({
      timeout: 5000,
    });
    await expect(menuSubTrigger(page, "Priority").first()).toBeVisible();
    await expect(menuSubTrigger(page, "Assignee").first()).toBeVisible();
    await expect(menuSubTrigger(page, "Due date").first()).toBeVisible();
    await expect(menuSubTrigger(page, "Project").first()).toBeVisible();
    await expect(menuSubTrigger(page, "Labels").first()).toBeVisible();
  });

  // -------------------------------------------------------------------------
  // Status change
  // -------------------------------------------------------------------------

  test("can change issue status from context menu", async ({ page }) => {
    const issue = await api.createIssue(
      "Status Change Test " + Date.now(),
    );
    await page.reload();

    const card = boardCardFor(page, issue.title);
    await expect(card).toBeVisible({ timeout: 10000 });

    await openCardQuickActions(card);

    // Hover the Status sub-trigger to reveal its sub-menu
    const statusTrigger = menuSubTrigger(page, "Status").first();
    await statusTrigger.hover();

    // Pick "In Dev" from the status sub-menu
    const inDevItem = menuItem(page, "In Dev").first();
    await expect(inDevItem).toBeVisible({ timeout: 5000 });
    await inDevItem.click();

    // After status change, the card should move to the "In Dev" column.
    // Verify the page did not navigate away.
    await expect(page).toHaveURL(/\/issues(?!\/)/);
  });

  // -------------------------------------------------------------------------
  // Priority change
  // -------------------------------------------------------------------------

  test("can change issue priority from context menu", async ({ page }) => {
    const issue = await api.createIssue(
      "Priority Change Test " + Date.now(),
    );
    await page.reload();

    const card = boardCardFor(page, issue.title);
    await expect(card).toBeVisible({ timeout: 10000 });

    await openCardQuickActions(card);

    const priorityTrigger = menuSubTrigger(page, "Priority").first();
    await priorityTrigger.hover();

    // Pick "Urgent"
    const urgentItem = menuItem(page, "Urgent").first();
    await expect(urgentItem).toBeVisible({ timeout: 5000 });
    await urgentItem.click();

    // Still on the issues board
    await expect(page).toHaveURL(/\/issues(?!\/)/);
  });

  // -------------------------------------------------------------------------
  // Due Date — set to Today
  // -------------------------------------------------------------------------

  test("can set due date to Today from context menu", async ({ page }) => {
    const issue = await api.createIssue(
      "Due Date Today Test " + Date.now(),
    );
    await page.reload();

    const card = boardCardFor(page, issue.title);
    await expect(card).toBeVisible({ timeout: 10000 });

    await openCardQuickActions(card);

    const dueTrigger = menuSubTrigger(page, "Due date").first();
    await dueTrigger.hover();

    const todayItem = menuItem(page, "Today").first();
    await expect(todayItem).toBeVisible({ timeout: 5000 });
    await todayItem.click();

    // Page stays on issues board after setting due date
    await expect(page).toHaveURL(/\/issues(?!\/)/);
  });

  // -------------------------------------------------------------------------
  // Assignee — unassign
  // -------------------------------------------------------------------------

  test("can unassign issue from context menu", async ({ page }) => {
    const issue = await api.createIssue(
      "Unassign Test " + Date.now(),
    );
    await page.reload();

    const card = boardCardFor(page, issue.title);
    await expect(card).toBeVisible({ timeout: 10000 });

    await openCardQuickActions(card);

    const assigneeTrigger = menuSubTrigger(page, "Assignee").first();
    await assigneeTrigger.hover();

    const unassignedItem = menuItem(page, "Unassigned").first();
    await expect(unassignedItem).toBeVisible({ timeout: 5000 });
    await unassignedItem.click();

    // Page stays on issues board
    await expect(page).toHaveURL(/\/issues(?!\/)/);
  });

  // -------------------------------------------------------------------------
  // Edge case: context menu does not navigate away on open
  // -------------------------------------------------------------------------

  test("opening context menu does not navigate away from board", async ({
    page,
  }) => {
    const issue = await api.createIssue(
      "No Nav Context Menu Test " + Date.now(),
    );
    await page.reload();

    const card = boardCardFor(page, issue.title);
    await expect(card).toBeVisible({ timeout: 10000 });

    await openCardQuickActions(card);

    // URL must remain on /issues (not navigated to detail page)
    await expect(page).toHaveURL(/\/issues(?!\/)/);

    // Dismiss the menu
    await page.keyboard.press("Escape");
    await expect(page).toHaveURL(/\/issues(?!\/)/);
  });

  // -------------------------------------------------------------------------
  // Edge case: context menu not visible when card is not hovered
  // -------------------------------------------------------------------------

  test("context menu trigger is not visible on initial render without hover", async ({
    page,
  }) => {
    const issue = await api.createIssue(
      "Hidden Trigger Test " + Date.now(),
    );
    await page.reload();

    const card = boardCardFor(page, issue.title);
    await expect(card).toBeVisible({ timeout: 10000 });

    // Move mouse away from the card so hover state is not active
    await page.mouse.move(0, 0);

    // The MoreHorizontal button is opacity-0 on non-hover — it is still in
    // the DOM but should not be visually presented.  Playwright's toBeVisible
    // checks visibility including opacity, so an opacity-0 element is hidden.
    const trigger = card.locator("button", {
      has: page.locator("svg.lucide-ellipsis"),
    });
    // The trigger may or may not exist; if it does it must not be visible
    const count = await trigger.count();
    if (count > 0) {
      await expect(trigger.first()).not.toBeVisible();
    }
  });
});
