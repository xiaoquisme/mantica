import { test, expect } from "./fixtures";
import { loginAsDefault } from "./helpers";

test.describe("Settings", () => {
  test("updating workspace name reflects in sidebar immediately", async ({
    page,
  }) => {
    await loginAsDefault(page);

    // The workspace name is displayed inside the sidebar header's workspace
    // switcher button. Base UI's DropdownMenuTrigger overrides the inner
    // SidebarMenuButton's data-slot, so the trigger is reachable as the
    // first `dropdown-menu-trigger` inside the sidebar header.
    const sidebarName = page
      .locator(
        '[data-slot="sidebar-header"] [data-slot="dropdown-menu-trigger"]',
      )
      .first();
    const originalName = await sidebarName.innerText();

    // Navigate to settings via the sidebar nav link (Settings was moved out
    // of the workspace dropdown into the Configure section of the sidebar).
    await page.getByRole("link", { name: "Settings" }).click();
    await page.waitForURL("**/settings");

    // /settings defaults to the Profile tab; switch to General to expose
    // the workspace name input + Save button.
    await page.getByRole("tab", { name: "General" }).click();

    // Change workspace name. The General tab now has multiple text inputs
    // (Name, Description, Context, Slug); scope to the General section
    // and target the Name input by its preceding Label text.
    const generalSection = page
      .locator("section")
      .filter({ has: page.getByRole("heading", { name: "General" }) });
    const nameInput = generalSection.locator("input[type='text']").first();
    await nameInput.clear();
    const newName = "Renamed WS " + Date.now();
    await nameInput.fill(newName);

    // Save (sonner toast "Workspace settings saved" replaced the old
    // inline "Saved!" indicator).
    await generalSection.getByRole("button", { name: "Save" }).click();
    await expect(
      page.locator("text=Workspace settings saved"),
    ).toBeVisible({ timeout: 5000 });

    // Sidebar should reflect the new name WITHOUT page refresh
    await expect(sidebarName).toContainText(newName);

    // Restore original name so other tests aren't affected
    await nameInput.clear();
    await nameInput.fill(originalName.trim());
    await generalSection.getByRole("button", { name: "Save" }).click();
    await expect(
      page.locator("text=Workspace settings saved"),
    ).toBeVisible({ timeout: 5000 });
  });
});
