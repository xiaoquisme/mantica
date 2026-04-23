import { test, expect } from "./fixtures";
import { createTestApi, loginAsDefault } from "./helpers";
import type { TestApiClient } from "./fixtures";

test.describe("Sub-task progress overview", () => {
  let api: TestApiClient;

  test.beforeEach(async ({ page }) => {
    api = await createTestApi();
    await loginAsDefault(page);
  });

  test.afterEach(async () => {
    await api.cleanup();
  });

  test("parent issue shows per-child status, summary count, and clickable rows", async ({
    page,
  }) => {
    const stamp = Date.now();
    const parent = await api.createIssue(`E2E Parent ${stamp}`);
    // Status values must match the current enum
    // (packages/core/issues/config/status.ts) — the prior "in_progress"
    // and "todo" labels were dropped in the workflow redesign and the
    // server's status_check constraint silently rejects them, so only the
    // first child would land and the chip read "1 of 1" instead of "1 of 3".
    const childA = await api.createIssue(`E2E Child A ${stamp}`, {
      parent_issue_id: parent.id,
      status: "done",
    });
    const childB = await api.createIssue(`E2E Child B ${stamp}`, {
      parent_issue_id: parent.id,
      status: "in_dev",
    });
    const childC = await api.createIssue(`E2E Child C ${stamp}`, {
      parent_issue_id: parent.id,
      status: "backlog",
    });

    await page.goto(`/issues/${parent.identifier}`);
    await expect(page.locator("text=Properties")).toBeVisible();

    // AC3 — summary count "X of Y sub-tasks complete" exposed to a11y
    const chip = page.locator(
      'span[aria-label$="sub-tasks complete"], div[aria-label$="sub-tasks complete"]',
    );
    await expect(chip.first()).toHaveAttribute(
      "aria-label",
      "1 of 3 sub-tasks complete",
    );
    await expect(chip.first()).toContainText("1/3");

    // AC1 — each child row is rendered and visible under the parent
    await expect(page.locator(`text=${childA.identifier}`)).toBeVisible();
    await expect(page.locator(`text=${childB.identifier}`)).toBeVisible();
    await expect(page.locator(`text=${childC.identifier}`)).toBeVisible();

    // AC2 — each child row links to /issues/{identifier}
    const linkB = page.locator(`a[href="/issues/${childB.identifier}"]`).first();
    await expect(linkB).toBeVisible();
    await linkB.click();
    await page.waitForURL(`**/issues/${childB.identifier}`);
    await expect(page).toHaveURL(new RegExp(`/issues/${childB.identifier}$`));
  });

  test("parent issue with no sub-issues shows the add affordance instead of the chip", async ({
    page,
  }) => {
    const stamp = Date.now();
    const parent = await api.createIssue(`E2E Lonely Parent ${stamp}`);

    await page.goto(`/issues/${parent.identifier}`);
    await expect(page.locator("text=Properties")).toBeVisible();

    // No chip should render when there are no children
    await expect(
      page.locator('[aria-label$="sub-tasks complete"]'),
    ).toHaveCount(0);

    // The "Add sub-issues" affordance is present instead
    await expect(page.locator("text=Add sub-issues")).toBeVisible();
  });
});
