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
  test("editing preserves, adds and removes persisted tags on direct and swapped forms", async ({ page }) => {
    const id = "a0000000-0000-0000-0000-000000000001";
    const edit = `/bookmarks/${id}/edit`;
    const description = "Tagged bookmark updated without changing its URL";
    const tags = () => page.locator("[data-tag-list] input[name=tags]");
    const save = async () => {
      const navigation = page.waitForNavigation({ url: /\/bookmarks$/ });
      await page.getByRole("button", { name: "Save bookmark" }).click();
      await navigation;
    };

    await signIn(page);
    await page.goto(edit);
    await expect(tags()).toHaveValues(["Go"]);
    await page.locator("#edit-description").fill(description);
    await save();
    await page.goto("/bookmarks?tag=go");
    await expect(page.locator(".bookmark-item", { hasText: description })).toContainText("Go");
    await page.reload();
    await expect(page.locator(".bookmark-item", { hasText: description })).toContainText("Go");

    await page.goto(edit);
    await expect(tags()).toHaveValues(["Go"]);
    await page.locator("[data-tag-input]").fill("Rust");
    await page.getByRole("button", { name: "Add tag" }).click();
    await expect(tags()).toHaveValues(["Go", "Rust"]);
    await page.getByRole("button", { name: "Remove tag Rust" }).click();
    await expect(tags()).toHaveValues(["Go"]);
    await page.locator("[data-tag-input]").fill("Rust");
    await page.getByRole("button", { name: "Add tag" }).click();
    await save();

    await page.evaluate(async (url) => {
      await (window as any).htmx.ajax("GET", url, { target: "#content", swap: "innerHTML" });
    }, edit);
    await expect(tags()).toHaveValues(["Go", "Rust"]);
    await page.getByRole("button", { name: "Remove tag Rust" }).click();
    await expect(tags()).toHaveValues(["Go"]);
    await save();
    await page.goto(edit);
    await expect(tags()).toHaveValues(["Go"]);
    await page.getByRole("button", { name: "Remove tag Go" }).click();
    await expect(tags()).toHaveCount(0);
    await save();
    await page.goto(edit);
    await expect(tags()).toHaveCount(0);
    await page.goto("/bookmarks?tag=go");
    await expect(page.locator(".bookmark-item", { hasText: description })).toHaveCount(0);
  });

  test("administrators can edit, cancel, then confirm deletion", async ({ page }) => {
    await page.goto("/bookmarks");
    await expect(page.locator(".bookmark-actions")).toHaveCount(0);
    await signIn(page);
    await page.goto("/bookmarks");
    const item = page.locator(".bookmark-item").first();
    const originalDate = await item.locator("time").getAttribute("datetime");
    const originalDescription = await item.locator(".description").textContent();
    await expect(item.getByRole("link", { name: "Edit" })).toHaveClass(/\bbtn-secondary\b/);
    await expect(item.getByRole("link", { name: "Delete" })).toHaveClass(/\bbtn-danger\b/);
    await item.getByRole("link", { name: "Edit" }).click();
    await expect(page.getByRole("heading", { name: "Edit bookmark" })).toBeVisible();
    await page.locator("#edit-description").fill("US3 edited bookmark");
    await page.getByRole("button", { name: "Save bookmark" }).click();
    await expect(page).toHaveURL(/\/bookmarks$/);
    await expect(page.locator(".bookmark-item", { hasText: "US3 edited bookmark" })).toContainText(originalDate!.slice(0, 10));

    await page.goto(`/search?q=${encodeURIComponent("US3 edited bookmark")}`);
    const searchItem = page.locator(".bookmark-item", { hasText: "US3 edited bookmark" });
    await expect(searchItem.getByRole("link", { name: "Edit" })).toHaveClass(/\bbtn-secondary\b/);
    await expect(searchItem.getByRole("link", { name: "Delete" })).toHaveClass(/\bbtn-danger\b/);
    await page.goto("/bookmarks");

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