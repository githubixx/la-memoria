import { expect, test } from "./fixtures";

test.describe("Accessibility and responsive delivery", () => {
  test("public list supports keyboard navigation without horizontal overflow", async ({ page }) => {
    await page.goto("/bookmarks");
    await expect(page.getByRole("heading")).toBeVisible();
    await page.keyboard.press("Tab");
    await expect(page.locator(":focus")).toBeVisible();
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBeTruthy();
  });

  test("required local browser assets are served", async ({ request }) => {
    for (const path of ["/assets/css/bookmarker.css", "/assets/vendor/htmx.min.js"]) {
      const response = await request.get(path);
      expect(response.ok(), path).toBeTruthy();
      expect((await response.body()).byteLength, path).toBeGreaterThan(0);
    }
  });

  test("menu and visible keyboard focus support keyboard-only navigation", async ({ page }) => {
    await page.setViewportSize({ width: 390, height: 844 });
    await page.goto("/bookmarks");
    await page.getByRole("button", { name: "Menu" }).focus();
    await page.keyboard.press("Enter");
    await expect(page.getByRole("link", { name: "Search" })).toBeVisible();
    await page.keyboard.press("Tab");
    const outline = await page.locator(":focus").evaluate((element) => getComputedStyle(element).outlineStyle);
    expect(outline).not.toBe("none");
  });

  test("direct navigation works without JavaScript and text remains contained", async ({ browser }) => {
    const context = await browser.newContext({ javaScriptEnabled: false, viewport: { width: 390, height: 844 } });
    const page = await context.newPage();
    await page.goto("http://127.0.0.1:18080/search?q=database");
    await expect(page.getByRole("heading")).toBeVisible();
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBeTruthy();
    await context.close();
  });
});