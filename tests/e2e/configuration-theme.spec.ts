import { execFileSync } from "node:child_process";
import { expect, test, type Page } from "@playwright/test";
import { createRestartFixture } from "./restart-fixture";

test.setTimeout(600_000);

const fields = [
  ["Page title", "page-title", "page_title"],
  ["Favicon path", "favicon-path", "favicon_path"],
  ["Screenshot root", "screenshot-root", "screenshot_root"],
  ["Database host", "database-host", "database_host"],
  ["Database name", "database-name", "database_name"],
  ["Database user", "database-user", "database_user"],
  ["Database password environment variable", "database-password-env", "database_password_env"],
] as const;

async function setTheme(page: Page, id: string, scheme: "light" | "dark") {
  await page.emulateMedia({ colorScheme: scheme, reducedMotion: "reduce" });
  await page.evaluate(async id => {
    document.querySelector("#configuration-test-theme")?.remove();
    if (id === "system") document.documentElement.removeAttribute("data-theme");
    else {
      document.documentElement.dataset.theme = id;
      const link = document.createElement("link");
      link.id = "configuration-test-theme";
      link.rel = "stylesheet";
      link.href = `/ui/themes/${id}/theme.css`;
      const ready = new Promise<void>((resolve, reject) => {
        link.onload = () => resolve();
        link.onerror = () => link.sheet?.cssRules.length ? resolve() : reject(new Error(`Unable to load ${link.href}`));
      });
      document.querySelector('link[href^="/assets/css/chroma-compat.css"]')!.before(link);
      await ready;
    }
    await document.fonts.ready;
  }, id);
}

async function compareFields(page: Page, context: string) {
  const styles = await page.locator("#configuration-reference, " + fields.map(([, id]) => `#${id}`).join(", ")).evaluateAll(els => els.map(el => {
    const s = getComputedStyle(el);
    return {
      id: el.id, backgroundColor: s.backgroundColor, backgroundImage: s.backgroundImage,
      color: s.color, borderColor: s.borderColor, borderWidth: s.borderWidth,
      borderRadius: s.borderRadius, fontSize: s.fontSize, height: s.height, boxShadow: s.boxShadow,
    };
  }));
  const reference = styles.find(s => s.id === "configuration-reference");
  expect(reference, context).toBeTruthy();
  expect(styles, context).toHaveLength(8);
  for (const style of styles.filter(s => s.id !== "configuration-reference")) {
    expect({ ...style, id: "reference" }, `${context} ${style.id}`)
      .toEqual({ ...reference!, id: "reference" });
  }
}

async function addSearchReference(page: Page) {
  // Use an actual Search control in the same form/layout for per-theme comparison.
  await page.evaluate(async () => {
    const html = await (await fetch("/search")).text();
    const search = new DOMParser().parseFromString(html, "text/html").querySelector("#search-q")!;
    search.id = "configuration-reference";
    search.removeAttribute("name");
    document.querySelector('form[action="/configuration"]')!.append(search);
  });
}

test("configuration fields follow every Chroma theme and preserve form behavior", async ({ page }, testInfo) => {
  // Avoid a network pull if the isolated database image is not already present.
  execFileSync("docker", ["image", "inspect", "postgres:18.6"], { stdio: "ignore" });
  const fixture = await createRestartFixture({ temporaryRoot: process.cwd() });
  try {
    // Theme CSS is local; block external fonts/images without touching the user's browser.
    await page.route("https://**", route => route.request().resourceType() === "stylesheet"
      ? route.fulfill({ status: 200, contentType: "text/css", body: "" }) : route.abort());
    await page.goto(`${fixture.baseURL}/login`);
    await page.getByLabel("Username").fill("admin");
    await page.getByLabel("Password").fill("bookmarker-fixture-password");
    await page.getByRole("button", { name: "Sign in" }).click();
    await expect(page).toHaveURL(/\/bookmarks$/);
    await page.goto(`${fixture.baseURL}/configuration`);

    await addSearchReference(page);
    for (const [label, id, name] of fields) {
      const input = page.getByLabel(label, { exact: true });
      await expect(input).toHaveAttribute("id", id);
      await expect(input).toHaveAttribute("name", name);
      await expect(input).toHaveAttribute("type", "text");
      await expect(input).toBeVisible();
    }
    const themes = JSON.parse(execFileSync("go", ["run", "./tests/e2e/catalog"], { encoding: "utf8" })) as { ID: string }[];
    const cases = [{ id: "system", scheme: "light" as const }, { id: "system", scheme: "dark" as const },
      ...themes.map(theme => ({ id: theme.ID, scheme: "light" as const }))];
    for (const viewport of [{ width: 375, height: 812 }, { width: 1280, height: 800 }]) {
      await page.setViewportSize(viewport);
      await page.locator("#favicon-path").fill("long-value-".repeat(40));
      for (const { id, scheme } of cases) {
        const context = `${testInfo.project.name} ${viewport.width} ${id}/${scheme}`;
        await page.evaluate(() => (document.activeElement as HTMLElement | null)?.blur());
        await setTheme(page, id, scheme);
        await compareFields(page, context);
        expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), context).toBe(true);
        for (const selector of ["#database-port", "#search-page-size", "#maximum-search-results",
          "#theme", "#default-view", "#database-tls-mode"]) {
          const control = page.locator(selector);
          await expect(control, context).toBeVisible();
          const box = await control.boundingBox();
          expect(box!.width, `${context} ${selector}`).toBeGreaterThan(0);
          expect(box!.x + box!.width, `${context} ${selector}`).toBeLessThanOrEqual(viewport.width);
        }
        await page.locator("#theme").focus();
        await page.keyboard.press("Tab");
        await expect(page.locator("#page-title"), context).toBeFocused();
        const focus = await page.locator("#page-title").evaluate(el => ({
          visible: el.matches(":focus-visible"), outline: getComputedStyle(el).outlineStyle,
          width: getComputedStyle(el).outlineWidth,
        }));
        expect(focus.visible, context).toBe(true);
        expect(focus.outline, context).not.toBe("none");
        expect(parseFloat(focus.width), context).toBeGreaterThan(0);
        if (id === "system" || id === "aurora-glass-1" || id === "spaceship-blue-dark") {
          const screenshot = testInfo.outputPath(`${id}-${scheme}-${viewport.width}.png`);
          await page.screenshot({ path: screenshot, fullPage: true, animations: "disabled" });
          console.log(`Screenshot: ${screenshot}`);
        }
      }
    }
    console.log(`Compared ${cases.length} theme/System cases at two viewport widths in ${testInfo.project.name}`);
    await page.locator("#configuration-reference").evaluate(el => el.remove());
    await page.goto(`${fixture.baseURL}/configuration`);

    // Invalid CSRF must not save, even through the real HTMX form.
    const csrf = page.locator('#content input[name="csrf_token"]');
    const token = await csrf.inputValue();
    await csrf.evaluate(el => (el as HTMLInputElement).value = "invalid");
    let response = page.waitForResponse(r => r.url().endsWith("/configuration") && r.request().method() === "POST");
    await page.getByRole("button", { name: "Save configuration" }).click();
    expect((await response).status()).toBe(403);
    await csrf.evaluate((el, value) => (el as HTMLInputElement).value = value, token);

    // A server-side validation error is swapped into the live page by HTMX.
    await page.getByLabel("Page title").fill(" ");
    response = page.waitForResponse(r => r.url().endsWith("/configuration") && r.request().method() === "POST");
    await page.getByRole("button", { name: "Save configuration" }).click();
    expect((await response).status()).toBe(422);
    await expect(page.getByRole("alert")).toContainText("page_title");
    await expect(page.getByLabel("Page title")).toHaveValue(" ");
    for (const [label] of fields) await expect(page.getByLabel(label, { exact: true })).toHaveAttribute("type", "text");
    await addSearchReference(page);
    await page.evaluate(() => (document.activeElement as HTMLElement | null)?.blur());
    await compareFields(page, "HTMX validation error");
    await page.locator("#configuration-reference").evaluate(el => el.remove());

    // Submit an unavailable theme to exercise its server-rendered error state.
    await page.getByLabel("Page title").fill("Valid configuration title");
    await page.locator("#theme").evaluate(el => {
      const option = new Option("Unavailable", "not-a-real-theme", true, true);
      (el as HTMLSelectElement).add(option);
    });
    response = page.waitForResponse(r => r.url().endsWith("/configuration") && r.request().method() === "POST");
    await page.getByRole("button", { name: "Save configuration" }).click();
    expect((await response).status()).toBe(422);
    await expect(page.locator("#theme")).toHaveAttribute("aria-invalid", "true");
    await expect(page.locator("#theme-help")).toBeVisible();
    for (const [label] of fields) await expect(page.getByLabel(label, { exact: true })).toHaveAttribute("type", "text");
    await addSearchReference(page);
    await page.evaluate(() => (document.activeElement as HTMLElement | null)?.blur());
    await compareFields(page, "HTMX unavailable theme");
    await page.locator("#configuration-reference").evaluate(el => el.remove());

    // Actual persisted selection and successful submission, including a long value.
    const title = "Configuration theme " + "long ".repeat(20);
    await page.getByLabel("Page title").fill(title);
    await page.getByLabel("Theme", { exact: true }).selectOption("aurora-glass-1");
    response = page.waitForResponse(r => r.url().endsWith("/configuration") && r.request().method() === "POST");
    await page.getByRole("button", { name: "Save configuration" }).click();
    expect((await response).status()).toBe(200);
    await expect(page.locator("html")).toHaveAttribute("data-theme", "aurora-glass-1");
    await expect(page.getByLabel("Page title")).toHaveValue(title.trim());
    await expect(page.getByLabel("Theme", { exact: true })).toHaveValue("aurora-glass-1");
    await expect(page.locator('link[href="/ui/themes/aurora-glass-1/theme.css"]')).toHaveCount(1);
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  } finally {
    await fixture.close();
  }
});
