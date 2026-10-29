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
    await page.keyboard.press("Escape");
    await expect(page.getByRole("button", { name: "Menu", exact: true })).toBeFocused();
    await expect(page.locator(".menu-list")).toBeHidden();
  });

  for (const width of [320, 390, 1280]) {
    test(`burger dropdown is upper-right, contained, and usable on public views at ${width}px`, async ({ page }, testInfo) => {
      await page.setViewportSize({ width, height: 844 });
      for (const path of ["/bookmarks", "/search", "/login"]) {
        await page.goto(path);
        const toggle = page.getByRole("button", { name: "Menu", exact: true });
        const list = page.locator(".menu-list");
        await expect(toggle).toBeVisible();
        await expect(toggle).toHaveAttribute("title", "Menu");
        await expect(toggle.locator(".menu-icon > span")).toHaveCount(3);
        await expect(toggle).toHaveAttribute("aria-expanded", "false");
        await expect(list).toBeHidden();
        const bounds = await toggle.boundingBox();
        expect(bounds!.x + bounds!.width).toBeGreaterThanOrEqual(width - 24);
        expect(bounds!.y).toBeLessThan(56);
        expect(bounds!.width).toBeGreaterThanOrEqual(44);
        expect(bounds!.height).toBeGreaterThanOrEqual(44);
        await toggle.click();
        await expect(toggle).toHaveAttribute("aria-expanded", "true");
        for (const name of ["List", "Add", "Search", "Configuration", "Sign in"]) {
          await expect(list.getByRole("link", { name, exact: true })).toBeVisible();
        }
        if (path === "/bookmarks") {
          await page.screenshot({ path: testInfo.outputPath(`burger-menu-${width}.png`) });
        }
        await list.getByRole("link", { name: "Configuration", exact: true }).evaluate((element) => {
          element.textContent = "ConfigurationWithAnUnusuallyLongUnbrokenLabel";
        });
        const panel = await list.boundingBox();
        expect(panel!.x).toBeGreaterThanOrEqual(0);
        expect(panel!.x + panel!.width).toBeLessThanOrEqual(width);
        expect(panel!.y + panel!.height).toBeLessThanOrEqual(844);
        expect(await list.evaluate((element) => element.scrollWidth <= element.clientWidth)).toBeTruthy();
        expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBeTruthy();
        await page.locator(".site-title").click();
        await expect(toggle).toHaveAttribute("aria-expanded", "false");
        await expect(list).toBeHidden();
        await toggle.click();
        await toggle.click();
        await expect(list).toBeHidden();
      }
      await page.setViewportSize({ width, height: 240 });
      await page.goto("/bookmarks");
      await page.getByRole("button", { name: "Menu", exact: true }).click();
      const shortPanel = await page.locator(".menu-list").boundingBox();
      expect(shortPanel!.y + shortPanel!.height).toBeLessThanOrEqual(240);
      await page.getByRole("navigation", { name: "Primary" }).getByRole("link", { name: "Sign in" }).scrollIntoViewIfNeeded();
      await expect(page.getByRole("navigation", { name: "Primary" }).getByRole("link", { name: "Sign in" })).toBeInViewport();
    });
  }

  test("menu closes after enhanced and ordinary navigation and protects administrative destinations", async ({ page }) => {
    await page.goto("/bookmarks");
    await page.evaluate(() => { document.body.dataset.menuNavigationProbe = "retained"; });
    const toggle = page.getByRole("button", { name: "Menu", exact: true });
    const menu = page.getByRole("navigation", { name: "Primary" });
    await toggle.click();
    await menu.getByRole("link", { name: "Search", exact: true }).click();
    await expect(page).toHaveURL(/\/search$/);
    await expect(page.locator("body")).toHaveAttribute("data-menu-navigation-probe", "retained");
    await expect(toggle).toHaveAttribute("aria-expanded", "false");
    await expect(page.locator(".menu-list")).toBeHidden();

    await page.route("**/assets/vendor/htmx.min.js", (route) => route.abort());
    await page.goto("/search");
    await page.evaluate(() => { document.body.dataset.menuNavigationProbe = "retained"; });
    await toggle.click();
    await menu.getByRole("link", { name: "List", exact: true }).click();
    await expect(page).toHaveURL(/\/bookmarks$/);
    await expect(page.locator("body")).not.toHaveAttribute("data-menu-navigation-probe");
    await expect(toggle).toHaveAttribute("aria-expanded", "false");
    await page.unroute("**/assets/vendor/htmx.min.js");
    for (const name of ["Add", "Configuration"]) {
      await page.goto("/bookmarks");
      await toggle.click();
      await menu.getByRole("link", { name, exact: true }).click();
      await expect(page).toHaveURL(/\/login\?return_to=/);
      await expect(page.getByLabel("Password")).toBeVisible();
    }
  });

  test("administrative views share the menu and forms initialize once after htmx swaps", async ({ page }) => {
    await page.goto("/login");
    await page.getByLabel("Username").fill("admin");
    await page.getByLabel("Password").fill("bookmarker-fixture-password");
    await page.getByRole("button", { name: "Sign in", exact: true }).click();
    await expect(page).toHaveURL(/\/bookmarks$/);
    const editPath = await page.locator(".bookmark-item").first().getByRole("link", { name: "Edit" }).getAttribute("href");
    const toggle = page.getByRole("button", { name: "Menu", exact: true });
    const menu = page.getByRole("navigation", { name: "Primary" });
    let captures = 0;
    await page.route("**/captures", async (route) => {
      expect(route.request().method()).toBe("POST");
      expect(route.request().postData()).toContain("csrf_token=");
      captures++;
      await route.fulfill({ contentType: "text/html", body: '<div id="capture-status" class="capture-status" data-state="failed">Fixture capture response</div>' });
    });

    for (const path of ["/bookmarks/new", editPath!, "/configuration"]) {
      await page.goto(path);
      await expect(toggle).toBeVisible();
      await expect(page.locator(".menu-list")).toBeHidden();
      await toggle.click();
      await expect(menu.getByRole("button", { name: "Sign out" })).toBeVisible();
      await page.keyboard.press("Escape");
      if (path !== "/configuration") {
        const initialTags = await page.locator("[data-tag-list] li").count();
        await page.locator("[data-tag-input]").fill("Direct-load tag");
        await page.locator("[data-tag-add]").click();
        await expect(page.locator("[data-tag-list] li")).toHaveCount(initialTags + 1);
        const beforeCapture = captures;
        await page.locator("[data-capture-trigger]").fill("https://example.com/direct-capture");
        await page.locator("[data-capture-trigger]").dispatchEvent("change");
        await expect(page.locator("#capture-status")).toHaveAttribute("data-state", "failed");
        expect(captures).toBe(beforeCapture + 1);
      }
    }

    for (const iteration of [1, 2]) {
      await toggle.click();
      await menu.getByRole("link", { name: "Add", exact: true }).click();
      await expect(page).toHaveURL(/\/bookmarks\/new$/);
      await expect(toggle).toHaveAttribute("aria-expanded", "false");
      await page.locator("[data-tag-input]").fill(`Swapped tag ${iteration}`);
      await page.locator("[data-tag-add]").click();
      await expect(page.locator("[data-tag-list] li")).toHaveCount(1);
      await page.getByRole("button", { name: `Remove tag Swapped tag ${iteration}` }).click();
      await expect(page.locator("[data-tag-list] li")).toHaveCount(0);
      const beforeCapture = captures;
      await page.locator("[data-capture-trigger]").fill(`https://example.com/swapped-capture-${iteration}`);
      await page.locator("[data-capture-trigger]").dispatchEvent("change");
      await expect(page.locator("#capture-status")).toHaveAttribute("data-state", "failed");
      expect(captures).toBe(beforeCapture + 1);
      await toggle.click();
      await menu.getByRole("link", { name: "List", exact: true }).click();
      await expect(page).toHaveURL(/\/bookmarks$/);
      await page.locator(".bookmark-item").first().getByRole("link", { name: "Edit" }).click();
      await expect(page.getByRole("heading", { name: "Edit bookmark" })).toBeVisible();
      const initialTags = await page.locator("[data-tag-list] li").count();
      await page.locator("[data-tag-input]").fill("Swapped edit tag");
      await page.locator("[data-tag-add]").click();
      await expect(page.locator("[data-tag-list] li")).toHaveCount(initialTags + 1);
    }

    await toggle.click();
    await menu.getByRole("button", { name: "Sign out" }).click();
    await expect(page).toHaveURL(/\/bookmarks$/);
    await toggle.click();
    await expect(menu.getByRole("link", { name: "Sign in" })).toBeVisible();
    await expect(menu.getByRole("button", { name: "Sign out" })).toHaveCount(0);
    await page.goto("/configuration");
    await expect(page).toHaveURL(/\/login\?return_to=/);
  });

  test("direct navigation works without JavaScript and text remains contained", async ({ browser, baseURL }) => {
    const context = await browser.newContext({ javaScriptEnabled: false, viewport: { width: 390, height: 844 } });
    const page = await context.newPage();
    await page.goto(`${baseURL}/search?q=database`);
    await expect(page.getByRole("heading")).toBeVisible();
    const menu = page.getByRole("navigation", { name: "Primary" });
    await expect(menu.getByRole("link", { name: "List", exact: true })).toBeVisible();
    await menu.getByRole("link", { name: "List", exact: true }).click();
    await expect(page).toHaveURL(/\/bookmarks$/);
    await menu.getByRole("link", { name: "Search", exact: true }).click();
    await expect(page).toHaveURL(/\/search$/);
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBeTruthy();
    await context.close();
  });
});
