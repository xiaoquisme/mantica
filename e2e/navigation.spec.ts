import { test, expect } from "./fixtures";
import { loginAsDefault } from "./helpers";

test.describe("Navigation", () => {
  test.beforeEach(async ({ page }) => {
    await loginAsDefault(page);
  });

  test("sidebar navigation works", async ({ page }) => {
    // Click Inbox
    await page.getByRole("link", { name: "Inbox" }).click();
    await page.waitForURL("**/inbox");
    await expect(page).toHaveURL(/\/inbox/);

    // Click Agents
    await page.getByRole("link", { name: "Agents" }).click();
    await page.waitForURL("**/agents");
    await expect(page).toHaveURL(/\/agents/);

    // Click Issues — exact match is required because "My Issues" also
    // contains the substring "Issues" and would otherwise match.
    await page
      .getByRole("link", { name: "Issues", exact: true })
      .click();
    await page.waitForURL("**/issues");
    await expect(page).toHaveURL(/\/issues/);
  });

  test("settings page loads from sidebar", async ({ page }) => {
    // Settings now lives in the sidebar's Configure section, not inside the
    // workspace dropdown — clicking the sidebar nav link is the only path.
    await page.getByRole("link", { name: "Settings" }).click();
    await page.waitForURL("**/settings");

    // Settings page renders an h1 "Settings" header in the left tab nav and
    // tab triggers for the per-section panes; assert both so we know the
    // tabbed shell rendered (the previous "Workspace"/"Members" headings
    // were replaced by tab labels in the redesign).
    await expect(
      page.getByRole("heading", { name: "Settings" }),
    ).toBeVisible();
    await expect(page.getByRole("tab", { name: "Members" })).toBeVisible();
  });

  test("agents page shows agent list", async ({ page }) => {
    await page.getByRole("link", { name: "Agents" }).click();
    await page.waitForURL("**/agents");

    // Should show "Agents" heading
    await expect(page.locator("text=Agents").first()).toBeVisible();
  });
});
