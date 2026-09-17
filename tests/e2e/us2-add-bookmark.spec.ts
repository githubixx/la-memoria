import { test, expect } from "@playwright/test";

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

test.describe("User Story 2 - Add a Bookmark with Tags and Screenshot", () => {
  test("first sign-in reaches the protected Add view", async ({ page }) => {
    await signIn(page);
    await page.goto("/bookmarks/new");
    await expect(page.locator("#add-url")).toBeVisible();
  });

  test("capturing a private fixture URL shows a preview before saving", async ({ page, baseURL }) => {
    await signIn(page);
    await page.goto("/bookmarks/new");
    const targetURL = new URL("/capture-target", process.env.BOOKMARKER_CAPTURE_BASE_URL ?? baseURL!).toString();
    await page.locator("#add-url").fill(targetURL);
    await page.locator("#add-url").dispatchEvent("change");
    await expect(page.locator(".screenshot-preview")).toBeVisible({ timeout: 20000 });
  });

  test("tags can be added and removed before saving", async ({ page }) => {
    await signIn(page);
    await page.goto("/bookmarks/new");
    await page.locator("[data-tag-input]").fill("Go");
    await page.locator("[data-tag-add]").click();
    await page.locator("[data-tag-input]").fill("go");
    await page.locator("[data-tag-add]").click();
    await expect(page.locator("[data-tag-list] li")).toHaveCount(2);
    await page.locator("[data-tag-list] li").first().getByRole("button", { name: /Remove/ }).click();
    await expect(page.locator("[data-tag-list] li")).toHaveCount(1);
  });

  test("saving after a successful capture creates the bookmark", async ({ page, baseURL }) => {
    await signIn(page);
    await page.goto("/bookmarks/new");
    const targetURL = new URL("/capture-target", process.env.BOOKMARKER_CAPTURE_BASE_URL ?? baseURL!).toString();
    await page.locator("#add-url").fill(targetURL);
    await page.locator("#add-url").dispatchEvent("change");
    await expect(page.locator(".screenshot-preview")).toBeVisible({ timeout: 20000 });
    await page.locator("#add-description").fill("A capture fixture page");
    await page.getByRole("button", { name: "Save bookmark" }).click();
    await expect(page).toHaveURL(/\/bookmarks$/);
    await expect(page.locator(".bookmark-item", { hasText: "A capture fixture page" }).first()).toBeVisible();
  });

  test("a failed capture allows retry or explicit continuation without a screenshot", async ({ page, baseURL }) => {
    await signIn(page);
    await page.goto("/bookmarks/new");
    const unreachableURL = new URL("/capture-target/unreachable", process.env.BOOKMARKER_CAPTURE_BASE_URL ?? baseURL!).toString();
    await page.locator("#add-url").fill(unreachableURL);
    await page.locator("#add-url").dispatchEvent("change");
    await expect(page.locator(".capture-status")).toHaveAttribute("data-state", "failed", { timeout: 20000 });
    await expect(page.getByRole("button", { name: "Retry capture" })).toBeVisible();
    await expect(page.getByRole("button", { name: "Discard screenshot" })).toBeVisible();
    await page.getByRole("button", { name: "Discard screenshot" }).click();
    await page.locator("#add-description").fill("Saved without a screenshot");
    await page.locator("[name=save_without_screenshot]").check();
    await page.getByRole("button", { name: "Save bookmark" }).click();
    await expect(page).toHaveURL(/\/bookmarks$/);
  });

  test("an expired browser session requires signing in again before a write completes", async ({ page, context }) => {
    await signIn(page);
    await page.goto("/bookmarks/new");
    await context.clearCookies();
    await page.reload();
    await expect(page).toHaveURL(/\/login/);
  });
});
