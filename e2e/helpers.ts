import { type Page } from "@playwright/test";
import { TestApiClient } from "./fixtures";

const DEFAULT_E2E_NAME = "E2E User";
const DEFAULT_E2E_EMAIL = "e2e@mantica.ai";
const DEFAULT_E2E_WORKSPACE = "e2e-workspace";

/**
 * Log in as the default E2E user and ensure the workspace exists first.
 * Authenticates via API (send-code → DB read → verify-code), then injects
 * the token into localStorage so the browser session is authenticated.
 *
 * Pass an existing `api` from `createTestApi()` to reuse its token instead
 * of doing a second login — back-to-back logins for the same user inside
 * the 10s send-code rate-limit window otherwise fail because the previous
 * code is already marked used and no new one is created.
 */
export async function loginAsDefault(page: Page, api?: TestApiClient) {
  let token = api?.getToken() ?? null;
  if (!token) {
    const fresh = new TestApiClient();
    await fresh.login(DEFAULT_E2E_EMAIL, DEFAULT_E2E_NAME);
    await fresh.ensureWorkspace("E2E Workspace", DEFAULT_E2E_WORKSPACE);
    token = fresh.getToken();
  }

  // Inject the token via addInitScript so it is present on the very first
  // load — the auth store's initialize() runs once on app boot, and Next.js
  // client-side nav between /login and /issues does not re-initialize, so
  // setting localStorage between two page.goto() calls intermittently leaves
  // the auth store with `user: null` and trips DashboardGuard's redirect.
  await page.addInitScript((t) => {
    localStorage.setItem("mantica_token", t);
  }, token);
  await page.goto("/issues");
  await page.waitForURL("**/issues", { timeout: 10000 });
  // Wait for the authenticated shell to render before returning. Without this,
  // an immediate `page.reload()` in the test body can race with the app's
  // client-side auth init and land back on /login.
  await page.getByRole("button", { name: "New Issue" }).waitFor({
    state: "visible",
    timeout: 10000,
  });
}

/**
 * Create a TestApiClient logged in as the default E2E user.
 * Call api.cleanup() in afterEach to remove test data created during the test.
 */
export async function createTestApi(): Promise<TestApiClient> {
  const api = new TestApiClient();
  await api.login(DEFAULT_E2E_EMAIL, DEFAULT_E2E_NAME);
  await api.ensureWorkspace("E2E Workspace", DEFAULT_E2E_WORKSPACE);
  return api;
}

export async function openWorkspaceMenu(page: Page) {
  // The workspace switcher is the first dropdown-menu-trigger inside the
  // sidebar header. Note: `[data-slot="sidebar-menu-button"]` does NOT match
  // the workspace switcher button — Base UI's DropdownMenuTrigger overrides
  // the SidebarMenuButton's data-slot when rendered via its `render` prop.
  // The implicit wait inside the caller's next .click() catches the open
  // dropdown, so no separate popover wait is needed here.
  await page
    .locator('[data-slot="sidebar-header"] [data-slot="dropdown-menu-trigger"]')
    .first()
    .click();
}
