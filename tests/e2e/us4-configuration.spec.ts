import { expect, test } from "./fixtures";

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

test.describe("User Story 4 - Customize Bookmarker", () => {
  test.describe.configure({ mode: "serial" });

  test("configuration is protected, redacted, and preserves invalid form input", async ({ page }) => {
    await page.goto("/configuration");
    await expect(page).toHaveURL(/\/login\?return_to=%2Fconfiguration$/);

    await signIn(page);
    await page.goto("/configuration");
    await expect(page.getByRole("heading", { name: "Configuration" })).toBeVisible();
    for (const label of [
      "Page title",
      "Favicon path",
      "Default view",
      "Screenshot root",
      "Database host",
      "Database port",
      "Database name",
      "Database user",
      "Database password environment variable",
      "Database TLS mode",
      "Search page size",
      "Maximum search results",
    ]) {
      await expect(page.getByLabel(label)).toBeVisible();
    }
    await expect(page.locator("form[action='/configuration']")).not.toContainText(/password hash|administrator username|resolved-secret/i);

    await page.getByLabel("Page title").fill(" ");
    await page.locator("form[action='/configuration']").evaluate((form) => {
      form.setAttribute("novalidate", "");
    });
    await Promise.all([
      page.waitForNavigation(),
      page.locator("form[action='/configuration']").evaluate((form) => {
        HTMLFormElement.prototype.submit.call(form);
      }),
    ]);
    await expect(page.locator(".field-error")).toContainText("page_title:");
    await expect(page.getByLabel("Page title")).toHaveValue(" ");
  });

  test("configuration rejects invalid paths and search limits without activation", async ({ page }) => {
    await signIn(page);
    await page.goto("/configuration");
    const form = page.locator("form[action='/configuration']");
    await page.getByLabel("Favicon path").fill("../outside.png");
    await form.evaluate((element) => {
      HTMLFormElement.prototype.submit.call(element);
    });
    await expect(page.locator(".field-error")).toContainText("favicon_path:");
    await expect(page.getByLabel("Favicon path")).toHaveValue("../outside.png");

    await page.getByLabel("Favicon path").fill("");
    await page.getByLabel("Maximum search results").fill("0");
    await form.evaluate((element) => {
      HTMLFormElement.prototype.submit.call(element);
    });
    await expect(page.locator(".field-error")).toContainText("search:");
    await expect(page.getByLabel("Maximum search results")).toHaveValue("0");

    await page.getByLabel("Maximum search results").fill("10000");
    await page.getByLabel("Database port").fill("0");
    await form.evaluate((element) => {
      HTMLFormElement.prototype.submit.call(element);
    });
    await expect(page.locator(".field-error")).toContainText("database:");
    await expect(page.getByLabel("Database port")).toHaveValue("0");
  });

  test("valid branding, default view, storage, database, and search values are accepted", async ({ page, request }) => {
    await signIn(page);
    await page.goto("/configuration");
    await page.getByLabel("Page title").fill("Bookmarker E2E configuration");
    await page.getByLabel("Favicon path").fill("images/bookmarker-favicon.png");
    await page.getByLabel("Default view").selectOption("search");
    await page.getByLabel("Screenshot root").fill("/tmp/bookmarker-e2e-screenshots-alternate");
    await page.getByLabel("Database host").fill("127.0.0.1");
    await page.getByLabel("Database port").fill("54329");
    await page.getByLabel("Database name").fill("bookmarker");
    await page.getByLabel("Database user").fill("bookmarker");
    await page.getByLabel("Database password environment variable").fill("BOOKMARKER_DB_PASSWORD");
    await page.getByLabel("Database TLS mode").selectOption("disable");
    await page.getByLabel("Search page size").fill("10");
    await page.getByLabel("Maximum search results").fill("10000");
    await Promise.all([
      page.waitForNavigation(),
      page.locator("form[action='/configuration']").evaluate((form) => {
        HTMLFormElement.prototype.submit.call(form);
      }),
    ]);
    await expect(page).toHaveURL(/\/configuration(?:\?restart_required=true)?$/);
    await expect(page.locator(".field-error")).toHaveCount(0);
    await expect(page.getByLabel("Page title")).toHaveValue("Bookmarker E2E configuration");
    await page.goto("/");
    await expect(page).toHaveURL(/\/search$/);
    const favicon = await request.get("/favicon");
    expect(favicon.ok()).toBeTruthy();
    expect((await favicon.body()).subarray(0, 8)).toEqual(Buffer.from([137, 80, 78, 71, 13, 10, 26, 10]));
  });

  test("configuration layout remains contained on mobile", async ({ page }) => {
    await signIn(page);
    await page.goto("/configuration");
    await expect(page.getByRole("heading", { name: "Configuration" })).toBeVisible();
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBeTruthy();
  });
});