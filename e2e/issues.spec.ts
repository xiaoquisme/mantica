import { test, expect } from "./fixtures";
import type { TestApiClient } from "./fixtures";
import { loginAsDefault, createTestApi } from "./helpers";

test.describe("Issues", () => {
  let api: TestApiClient;

  test.beforeEach(async ({ page }) => {
    api = await createTestApi();
    await loginAsDefault(page);
  });

  test.afterEach(async () => {
    await api.cleanup();
  });

  test("issues page loads with board view", async ({ page }) => {
    await expect(page.locator("text=All Issues")).toBeVisible();

    // Board columns should be visible
    await expect(page.locator("text=Backlog")).toBeVisible();
    await expect(page.locator("text=Todo")).toBeVisible();
    await expect(page.locator("text=In Progress")).toBeVisible();
  });

  test("can switch between board and list view", async ({ page }) => {
    await expect(page.locator("text=All Issues")).toBeVisible();

    // Switch to list view
    await page.click("text=List");
    await expect(page.locator("text=All Issues")).toBeVisible();

    // Switch back to board view
    await page.click("text=Board");
    await expect(page.locator("text=Backlog")).toBeVisible();
  });

  test("can create a new issue", async ({ page }) => {
    await page.click("text=New Issue");

    const title = "E2E Created " + Date.now();
    await page.fill('input[placeholder="Issue title..."]', title);
    await page.click("text=Create");

    // New issue should appear on the page
    await expect(page.locator(`text=${title}`).first()).toBeVisible({
      timeout: 10000,
    });
  });

  test("can navigate to issue detail page", async ({ page }) => {
    // Create a known issue via API so the test controls its own fixture
    const issue = await api.createIssue("E2E Detail Test " + Date.now());

    // Reload to see the new issue
    await page.reload();
    await expect(page.locator("text=All Issues")).toBeVisible();

    // Navigate to the issue detail
    const issueLink = page.locator(`a[href="/issues/${issue.id}"]`);
    await expect(issueLink).toBeVisible({ timeout: 5000 });
    await issueLink.click();

    await page.waitForURL(/\/issues\/[\w-]+/);

    // Should show Properties panel
    await expect(page.locator("text=Properties")).toBeVisible();
    // Should show breadcrumb link back to Issues
    await expect(
      page.locator("a", { hasText: "Issues" }).first(),
    ).toBeVisible();
  });

  // TES-71: when a previous session collapsed the sidebar, the persisted
  // {sidebar: 0} layout used to make the right-side action buttons disappear
  // on every subsequent visit. The page must always render the sidebar so
  // users can manage issue properties.
  test("issue detail sidebar stays visible even when previously collapsed", async ({
    page,
  }) => {
    const issue = await api.createIssue("E2E Sidebar TES-71 " + Date.now());

    // Pre-seed the persisted resizable layout with a fully collapsed sidebar,
    // mimicking the user state from the bug report. The /issues page is
    // already loaded by loginAsDefault, so we set localStorage in-place.
    await page.evaluate(() => {
      localStorage.setItem(
        "react-resizable-panels:multica_issue_detail_layout",
        JSON.stringify({ content: 1, sidebar: 0 }),
      );
    });

    await page.goto(`/issues/${issue.id}`);
    await page.waitForURL(/\/issues\/[\w-]+/);

    // Sidebar Properties section + action rows must be visible despite the
    // persisted collapsed-sidebar layout.
    await expect(page.locator("text=Properties")).toBeVisible();
    await expect(page.locator("text=Status")).toBeVisible();
    await expect(page.locator("text=Assignee")).toBeVisible();
  });

  test("can cancel issue creation", async ({ page }) => {
    await page.click("text=New Issue");

    await expect(
      page.locator('input[placeholder="Issue title..."]'),
    ).toBeVisible();

    await page.click("text=Cancel");

    await expect(
      page.locator('input[placeholder="Issue title..."]'),
    ).not.toBeVisible();
    await expect(page.locator("text=New Issue")).toBeVisible();
  });
});
