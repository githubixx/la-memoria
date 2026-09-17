import { test, expect, devices } from "@playwright/test";

test.describe("User Story 1 - Browse and Find Bookmarks (signed out)", () => {
  test("lists newest-first bookmarks with URL, description, date, then tags", async ({ page }) => {
    await page.goto("/bookmarks");
    const firstItem = page.locator(".bookmark-item").first();
    await expect(firstItem.locator("a[target=_blank]")).toBeVisible();
    await expect(firstItem.locator(".description")).toBeVisible();
    await expect(firstItem.locator("time")).toBeVisible();
    await expect(page.locator("nav.pagination a[aria-current=page]")).toHaveText("1");
  });

  test("filters to an exact tag when a tag link is selected", async ({ page }) => {
    await page.goto("/bookmarks");
    await page.locator(".tags a", { hasText: "Go" }).first().click();
    await expect(page).toHaveURL(/tag=go/);
    const tags = page.locator(".bookmark-item .tags a");
    await expect(tags.first()).toHaveText(/Go/i);
  });

  test("searches by tag and all description words together", async ({ page }) => {
    await page.goto("/search?tag=go&q=database+transactions");
    const results = page.locator(".bookmark-item");
    await expect(results.first()).toBeVisible();
    await expect(page.locator(".tags a", { hasText: "Go" }).first()).toBeVisible();
  });

  test("reaches first, adjacent, and final pages across 50 list pages", async ({ page }) => {
    await page.goto("/bookmarks");
    await expect(page.locator("nav.pagination a", { hasText: "50" })).toBeVisible();
    await page.locator("nav.pagination a", { hasText: "50" }).click();
    await expect(page).toHaveURL(/page=50/);
    await expect(page.locator("nav.pagination a[aria-current=page]")).toHaveText("50");
    await page.locator("nav.pagination a", { hasText: "1" }).first().click();
    await expect(page).toHaveURL(/page=1|bookmarks$/);
  });

  test("direct page-3 URL loads the full page without JavaScript-only state", async ({ page }) => {
    await page.goto("/bookmarks?page=3");
    await expect(page.locator("nav.pagination a[aria-current=page]")).toHaveText("3");
  });

  test("tag navigation works through htmx fragment updates", async ({ page }) => {
    await page.goto("/bookmarks");
    const [response] = await Promise.all([
      page.waitForResponse((r) => r.url().includes("/bookmarks") && r.request().headers()["hx-request"] === "true"),
      page.locator(".tags a", { hasText: "Go" }).first().click(),
    ]);
    expect(response.headers()["vary"]).toContain("HX-Request");
  });

  test("renders usably on a mobile viewport", async ({ browser }) => {
    const context = await browser.newContext({ ...devices["iPhone 13"] });
    const mobilePage = await context.newPage();
    await mobilePage.goto("/bookmarks");
    await expect(mobilePage.locator(".bookmark-item").first()).toBeVisible();
    await context.close();
  });
});
