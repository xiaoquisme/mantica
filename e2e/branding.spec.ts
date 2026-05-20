import { test, expect } from "./fixtures";
import { loginAsDefault, createTestApi } from "./helpers";
import type { TestApiClient } from "./fixtures";

let api: TestApiClient;

test.beforeEach(async ({ page }) => {
  api = await createTestApi();
  await loginAsDefault(page);
});

test.afterEach(async () => {
  await api.cleanup();
});

test.describe("Brand name — Mantica", () => {
  test("chat FAB shows Ask Mantica", async ({ page }) => {
    const fab = page.getByRole("button", { name: "Ask Mantica" });
    await expect(fab).toBeVisible();
  });

  test("chat window shows Welcome to Mantica", async ({ page }) => {
    // Open the chat window by clicking the FAB
    await page.getByRole("button", { name: "Ask Mantica" }).click();

    await expect(
      page.getByRole("heading", { name: "Welcome to Mantica" }),
    ).toBeVisible();
    await expect(
      page.getByText("tell Mantica what you need"),
    ).toBeVisible();
  });

  test("chat input placeholder says Ask Mantica", async ({ page }) => {
    // Open the chat window
    await page.getByRole("button", { name: "Ask Mantica" }).click();

    const input = page.getByPlaceholder("Ask Mantica...");
    await expect(input).toBeVisible();
  });

  test("no stale Multica text appears on dashboard", async ({ page }) => {
    // The old brand name should not appear anywhere on the page
    const body = await page.textContent("body");
    expect(body).not.toContain("Multica");
  });
});
