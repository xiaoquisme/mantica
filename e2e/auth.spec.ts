import { test, expect } from "./fixtures";
import { loginAsDefault, openWorkspaceMenu } from "./helpers";

test.describe("Authentication", () => {
  test("login page renders correctly", async ({ page }) => {
    await page.goto("/login");

    // The page title is rendered inside shadcn's <CardTitle>, which is a <div>
    // (not a heading element) — target it via the data-slot attribute instead
    // of an h1 selector. Bumping CardTitle to a heading would be a global
    // shadcn-component change that affects every other Card in the app.
    await expect(page.locator('[data-slot="card-title"]')).toContainText(
      "Multica",
    );
    await expect(page.locator("#login-email")).toBeVisible();
    await expect(page.locator('button[type="submit"]')).toContainText(
      "Continue",
    );
  });

  test("login and redirect to /issues", async ({ page }) => {
    await loginAsDefault(page);

    await expect(page).toHaveURL(/\/issues/);
    // The previous `text=All Issues` heading no longer exists in the issues
    // page header; loginAsDefault already waits for the "New Issue" sidebar
    // button which is the canonical signal that the authenticated shell has
    // rendered, so re-asserting it here just confirms the shell stayed up.
    await expect(
      page.getByRole("button", { name: "New Issue" }),
    ).toBeVisible();
  });

  test("unauthenticated user is redirected to /login", async ({ page }) => {
    await page.goto("/login");
    await page.evaluate(() => {
      localStorage.removeItem("multica_token");
      localStorage.removeItem("multica_workspace_id");
    });

    await page.goto("/issues");
    await page.waitForURL("**/login", { timeout: 10000 });
  });

  test("logout redirects to /login", async ({ page }) => {
    await loginAsDefault(page);

    // Open the workspace dropdown menu
    await openWorkspaceMenu(page);

    // Click Log out (menu item label is "Log out", not "Sign out")
    await page.getByRole("menuitem", { name: "Log out" }).click();

    await page.waitForURL("**/login", { timeout: 10000 });
    await expect(page).toHaveURL(/\/login/);
  });
});
