import { test, expect } from "./fixtures";
import type { TestApiClient } from "./fixtures";
import { createTestApi, loginAsDefault } from "./helpers";

test.describe("Comments", () => {
  let api: TestApiClient;

  test.beforeEach(async ({ page }) => {
    api = await createTestApi();
    await api.createIssue("E2E Comment Test " + Date.now());
    await loginAsDefault(page);
  });

  test.afterEach(async () => {
    await api.cleanup();
  });

  test("can add a comment on an issue", async ({ page }) => {
    // Wait for issues to load and click first one
    const issueLink = page.locator('a[href^="/issues/"]').first();
    await expect(issueLink).toBeVisible({ timeout: 5000 });
    await issueLink.click();
    await page.waitForURL(/\/issues\/[\w-]+/);

    // Wait for issue detail to load
    await expect(page.locator("text=Properties")).toBeVisible();

    // The comment input is a Tiptap ContentEditor, NOT an <input>. The
    // ContentEditor itself is a contenteditable <div> with no role/aria
    // hooks; Tiptap's Placeholder extension renders the placeholder string
    // as `data-placeholder` on the first child node, so target the
    // contenteditable wrapper that contains it.
    const commentText = "E2E comment " + Date.now();
    const commentInput = page.locator(
      '[contenteditable="true"]:has([data-placeholder="Leave a comment..."])',
    );
    await commentInput.click();
    await commentInput.fill(commentText);

    // The submit button enables once the editor is non-empty; click it.
    // Locate it as the only enabled button adjacent to the comment editor
    // (icon-only — no accessible name to match by).
    const submitBtn = page
      .locator(
        'button:has(svg.lucide-arrow-up), button:has(svg.lucide-loader-2)',
      )
      .last();
    await expect(submitBtn).toBeEnabled();
    await submitBtn.click();

    // Comment should appear in the activity section
    await expect(page.locator(`text=${commentText}`)).toBeVisible({
      timeout: 5000,
    });
  });

  test("comment submit button is disabled when empty", async ({ page }) => {
    const issueLink = page.locator('a[href^="/issues/"]').first();
    await expect(issueLink).toBeVisible({ timeout: 5000 });
    await issueLink.click();
    await page.waitForURL(/\/issues\/[\w-]+/);

    await expect(page.locator("text=Properties")).toBeVisible();

    // The comment submit is an icon-only button with the ArrowUp lucide
    // icon, disabled while the Tiptap editor has no content.
    const submitBtn = page
      .locator('button:has(svg.lucide-arrow-up)')
      .last();
    await expect(submitBtn).toBeDisabled();
  });
});
