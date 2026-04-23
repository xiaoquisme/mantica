/**
 * E2E tests for TES-64: Change Project via Quick Actions on Board Cards
 *
 * Verifies AC1–AC5 from the story:
 *   AC1: Project option visible in quick actions menu
 *   AC2: Project picker opens from menu
 *   AC3: Change project successfully
 *   AC4: Remove from project
 *   AC5: Feedback on update (toast / badge change reflected on the card)
 *
 * Locator notes:
 * - The breadcrumb on /issues renders the literal text "Issues" (no
 *   "All Issues" string exists in the page); `loginAsDefault` already waits
 *   on the URL, so we anchor the page-loaded gate on a stable element near
 *   the board (an issue card or the empty-state).
 * - The dropdown menu items expose `data-slot="dropdown-menu-item"` /
 *   `data-slot="dropdown-menu-sub-trigger"` (see packages/ui/components/ui/
 *   dropdown-menu.tsx) — using these avoids matching unrelated "Project"
 *   text elsewhere on the page (sidebar nav, badges, etc.).
 */

import { test, expect } from "./fixtures";
import type { TestApiClient } from "./fixtures";
import type { Locator, Page } from "@playwright/test";
import { loginAsDefault, createTestApi } from "./helpers";

function boardCardFor(page: Page, title: string): Locator {
  // The card root is the AppLink with class "group" wrapping BoardCardContent.
  return page.locator(".group").filter({ hasText: title }).first();
}

async function openCardQuickActions(card: Locator) {
  await card.scrollIntoViewIfNeeded();
  await card.hover();
  // The MoreHorizontal trigger renders a lucide ellipsis SVG. Filtering by
  // that SVG distinguishes it from the priority/assignee/due-date pickers
  // that live in the same card.
  const trigger = card.locator("button", { has: card.page().locator("svg.lucide-ellipsis") });
  await trigger.first().click();
}

function menuItem(page: Page, label: string): Locator {
  // Match a real menu item by label, ignoring incidental "Project" text on
  // the page (e.g. sidebar nav, project badges).
  return page
    .locator('[data-slot="dropdown-menu-item"], [data-slot="dropdown-menu-sub-trigger"]')
    .filter({ hasText: label });
}

test.describe("Board Card — Change Project via Quick Actions (TES-64)", () => {
  let api: TestApiClient;

  test.beforeEach(async ({ page }) => {
    api = await createTestApi();
    await loginAsDefault(page, api);
  });

  test.afterEach(async () => {
    await api.cleanup();
  });

  test("project option visible in quick actions menu (AC1 + AC2)", async ({ page }) => {
    const issue = await api.createIssue("Quick Actions Project Test " + Date.now());

    await page.reload();

    const card = boardCardFor(page, issue.title);
    await expect(card).toBeVisible({ timeout: 10000 });

    await openCardQuickActions(card);

    // AC1: "Project" sub-trigger is present in the menu.
    const projectTrigger = menuItem(page, "Project").first();
    await expect(projectTrigger).toBeVisible({ timeout: 5000 });

    // AC2: hovering the sub-trigger opens the submenu and we did not navigate
    // off the issues page.
    await projectTrigger.hover();
    await expect(page).toHaveURL(/\/issues(?!\/)/);
  });

  test("can change issue project from quick actions menu (AC3 + AC5)", async ({ page }) => {
    const project = await api.createProject("E2E Project " + Date.now());
    const issue = await api.createIssue("Issue For Project Change " + Date.now());

    await page.reload();

    const card = boardCardFor(page, issue.title);
    await expect(card).toBeVisible({ timeout: 10000 });

    await openCardQuickActions(card);
    await menuItem(page, "Project").first().hover();

    // AC3: pick the project from the submenu.
    const option = menuItem(page, project.title).first();
    await expect(option).toBeVisible({ timeout: 5000 });
    await option.click();

    // AC5: the project badge appears on the card without a manual reload —
    // confirms the optimistic update + cache invalidation completed.
    await expect(boardCardFor(page, issue.title).getByText(project.title)).toBeVisible({
      timeout: 5000,
    });
  });

  test("can remove issue from project via quick actions menu (AC4)", async ({ page }) => {
    const project = await api.createProject("E2E Remove Project " + Date.now());
    const issue = await api.createIssue("Issue Already In Project " + Date.now(), {
      project_id: project.id,
    });

    await page.reload();

    // Sanity: the badge is on the card to start with.
    const card = boardCardFor(page, issue.title);
    await expect(card).toBeVisible({ timeout: 10000 });
    await expect(card.getByText(project.title)).toBeVisible();

    await openCardQuickActions(card);
    await menuItem(page, "Project").first().hover();

    const removeItem = menuItem(page, "Remove from project").first();
    await expect(removeItem).toBeVisible({ timeout: 5000 });
    await removeItem.click();

    // The project badge disappears from the card after the mutation settles.
    await expect(boardCardFor(page, issue.title).getByText(project.title)).toHaveCount(0, {
      timeout: 5000,
    });
  });

  test("quick actions menu does not navigate away from board (AC2 edge case)", async ({ page }) => {
    const issue = await api.createIssue("No Nav Test " + Date.now());

    await page.reload();

    const card = boardCardFor(page, issue.title);
    await expect(card).toBeVisible({ timeout: 10000 });

    await openCardQuickActions(card);

    await expect(page).toHaveURL(/\/issues(?!\/)/);
    await expect(menuItem(page, "Project").first()).toBeVisible();

    await page.keyboard.press("Escape");
    await expect(page).toHaveURL(/\/issues(?!\/)/);
  });
});
