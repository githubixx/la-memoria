import { expect, test } from "./fixtures";

test("System shell supports public and administrator journeys without decorative resources", async ({ page }) => {
  await page.route(/\.(woff2?|ttf|png|jpe?g|svg)(\?.*)?$/i, route => route.abort());
  await page.route("https://fonts.googleapis.com/**", route => route.abort());
  await page.emulateMedia({ colorScheme: "light" });
  await page.goto("/bookmarks");
  await expect(page.locator('link[href="/ui/css/base.css"]')).toHaveCount(1);
  await expect(page.locator('link[href^="/ui/themes/"]')).toHaveCount(0);
  const light = await page.locator("body").evaluate(el => getComputedStyle(el).backgroundColor);
  await page.emulateMedia({ colorScheme: "dark" });
  await expect.poll(() => page.locator("body").evaluate(el => getComputedStyle(el).backgroundColor)).not.toBe(light);
  await page.keyboard.press("Tab");
  await expect(page.locator(":focus")).toBeVisible();
  await page.getByRole("button", { name: "Menu", exact: true }).focus();
  await page.keyboard.press("Enter");
  await expect(page.getByRole("navigation", { name: "Primary" }).getByRole("link", { name: "Search", exact: true })).toBeVisible();
  await page.keyboard.press("Tab");
  expect(await page.locator(":focus").evaluate(el => getComputedStyle(el).outlineStyle)).not.toBe("none");
  await page.keyboard.press("Escape");
  await page.goto("/search?q=database");
  await expect(page.getByRole("heading")).toBeVisible();
  await page.goto("/login");
  await page.getByLabel("Username").fill("admin");
  await page.getByLabel("Password").fill("bookmarker-fixture-password");
  await page.getByRole("button", { name: "Sign in" }).click();
  await expect(page).toHaveURL(/\/bookmarks$/);
  await page.goto("/bookmarks/new");
  await expect(page.getByLabel("URL", { exact: true })).toBeVisible();
  await page.goto("/configuration?restart_required=true");
  await expect(page.getByRole("heading", { name: /configuration/i })).toBeVisible();
  await expect(page.getByLabel("Page title")).toBeVisible();
  await expect(page.locator(".feedback")).toContainText("Restart");
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBeTruthy();
});

test("existing bookmark maintenance remains available in the refreshed shell", async ({ page }) => {
  await page.goto("/login");
  await page.getByLabel("Username").fill("admin");
  await page.getByLabel("Password").fill("bookmarker-fixture-password");
  await page.getByRole("button", { name: "Sign in" }).click();
  await expect(page).toHaveURL(/\/bookmarks$/);
  await page.locator(".bookmark-item").first().getByRole("link", { name: "Edit" }).click();
  await page.locator("#edit-description").fill("Chroma journey edit");
  await page.getByRole("button", { name: "Save bookmark" }).click();
  const item = page.locator(".bookmark-item", { hasText: "Chroma journey edit" });
  await expect(item).toBeVisible();
  await item.getByRole("link", { name: "Delete" }).click();
  await page.getByRole("button", { name: "Delete", exact: true }).click();
  await expect(page.locator(".bookmark-item", { hasText: "Chroma journey edit" })).toHaveCount(0);
});

test("capture and create remain available", async ({ page, baseURL }) => {
  await page.goto("/login");
  await page.getByLabel("Username").fill("admin");
  await page.getByLabel("Password").fill("bookmarker-fixture-password");
  await page.getByRole("button", { name: "Sign in" }).click();
  await expect(page).toHaveURL(/\/bookmarks$/);
  await page.goto("/bookmarks/new");
  await page.locator("#add-url").fill(new URL("/capture-target", process.env.BOOKMARKER_CAPTURE_BASE_URL ?? baseURL!).toString());
  await page.locator("#add-url").dispatchEvent("change");
  await expect(page.locator(".screenshot-preview")).toBeVisible({ timeout: 20000 });
  await page.locator("#add-description").fill("Chroma captured bookmark");
  await page.getByRole("button", { name: "Save bookmark" }).click();
  await expect(page.locator(".bookmark-item", { hasText: "Chroma captured bookmark" })).toBeVisible();
});
