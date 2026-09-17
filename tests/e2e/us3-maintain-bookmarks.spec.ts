import { test, expect } from "./fixtures";

const ADMIN_USERNAME = "admin";
const ADMIN_PASSWORD = "bookmarker-fixture-password";

async function signIn(page: import("@playwright/test").Page) {
  await page.goto("/login");
  await page.getByLabel("Username").fill(ADMIN_USERNAME);
  await page.getByLabel("Password").fill(ADMIN_PASSWORD);
  await Promise.all([
    page.waitForURL(/\/bookmarks$/),
    page.getByRole("button", { name: "Sign in" }).click(),
  ]);
}

test.describe("User Story 3 - Maintain Existing Bookmarks", () => {
  test("administrators can edit, cancel, then confirm deletion", async ({ page }) => {
    await signIn(page);
    await page.goto("/bookmarks");
    const item = page.locator(".bookmark-item").first();
    const originalDate = await item.locator("time").getAttribute("datetime");
    const originalDescription = await item.locator(".description").textContent();
    await item.getByRole("link", { name: "Edit" }).click();
    await expect(page.getByRole("heading", { name: "Edit bookmark" })).toBeVisible();
    await page.locator("#edit-description").fill("US3 edited bookmark");
    await page.getByRole("button", { name: "Save bookmark" }).click();
    await expect(page).toHaveURL(/\/bookmarks$/);
    await expect(page.locator(".bookmark-item", { hasText: "US3 edited bookmark" })).toContainText(originalDate!.slice(0, 10));

    const editedItem = page.locator(".bookmark-item", { hasText: "US3 edited bookmark" });
    await editedItem.getByRole("link", { name: "Delete" }).click();
    await expect(page.getByRole("heading", { name: "Delete bookmark" })).toBeVisible();
    await page.getByRole("link", { name: "Cancel" }).click();
    await expect(page).toHaveURL(/\/bookmarks$/);
    await expect(page.locator(".bookmark-item", { hasText: "US3 edited bookmark" })).toBeVisible();

    await page.locator(".bookmark-item", { hasText: "US3 edited bookmark" }).getByRole("link", { name: "Delete" }).click();
    await page.getByRole("button", { name: "Delete" }).click();
    await expect(page).toHaveURL(/\/bookmarks$/);
    await expect(page.locator(".bookmark-item", { hasText: "US3 edited bookmark" })).toHaveCount(0);
    await page.goto(`/search?q=${encodeURIComponent("US3 edited bookmark")}`);
    await expect(page.locator(".bookmark-item", { hasText: "US3 edited bookmark" })).toHaveCount(0);
    void originalDescription;
  });
});