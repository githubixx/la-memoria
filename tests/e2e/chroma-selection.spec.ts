import { execFileSync } from "node:child_process";
import { expect, test, type Page } from "@playwright/test";
import { createRestartFixture } from "./restart-fixture";

test.setTimeout(900_000);

async function signIn(page: Page, base: string) {
  await page.goto(`${base}/login`);
  await page.getByLabel("Username").fill("admin");
  await page.getByLabel("Password").fill("bookmarker-fixture-password");
  await page.getByRole("button", { name: "Sign in" }).click();
  await expect(page).toHaveURL(/\/bookmarks$/);
}

async function expectAppearance(page: Page, id: string) {
  if (id === "system") {
    await expect(page.locator("html")).not.toHaveAttribute("data-theme");
    await expect(page.locator('link[href^="/ui/themes/"]')).toHaveCount(0);
  } else {
    await expect(page.locator("html")).toHaveAttribute("data-theme", id);
    await expect(page.locator(`link[href="/ui/themes/${id}/theme.css"]`)).toHaveCount(1);
  }
}

async function checkKeyboard(page: Page) {
  const base = new URL(page.url()).origin;
  const menu = page.getByRole("button", { name: "Menu", exact: true });
  await page.keyboard.press("Tab");
  await expect(menu).toBeFocused();
  await page.keyboard.press("Enter");
  const nav = page.getByRole("navigation", { name: "Primary" });
  for (const name of ["List", "Search", "Add", "Configuration", "Sign out"]) {
    const control = nav.getByRole(name === "Sign out" ? "button" : "link", { name, exact: true });
    await expect(control).toBeVisible();
  }
  const reached = new Set<string>();
  for (let i = 0; i < 6; i++) {
    await page.keyboard.press("Tab");
    const focus = page.locator(":focus");
    expect(await focus.evaluate(el => getComputedStyle(el).outlineStyle)).not.toBe("none");
    reached.add((await focus.textContent())?.trim() ?? "");
  }
  for (const name of ["List", "Search", "Add", "Configuration", "Sign out"]) expect(reached.has(name)).toBeTruthy();
  // Exercise each primary action by keyboard, reopening the menu from a fresh page.
  for (const [name, destination] of [
    ["List", "/bookmarks"], ["Search", "/search"], ["Add", "/bookmarks/new"],
    ["Configuration", "/configuration"], ["Sign out", "/bookmarks"],
  ]) {
    await page.goto(`${base}/configuration`);
    await page.keyboard.press("Tab");
    await expect(menu).toBeFocused();
    await page.keyboard.press("Enter");
    for (let i = 0; i < 5; i++) {
      await page.keyboard.press("Tab");
      if ((await page.locator(":focus").textContent())?.trim() === name) break;
    }
    await expect(page.locator(":focus")).toHaveText(name);
    if (name === "Configuration") {
      await page.keyboard.press("Enter");
      await expect(page.getByRole("heading", { name: "Configuration" })).toBeVisible();
    } else {
      await Promise.all([page.waitForURL(url => url.pathname === destination), page.keyboard.press("Enter")]);
    }
    if (name === "Sign out") await signIn(page, base);
  }
  await page.goto(`${base}/configuration`);
  const controls = page.locator('form[action="/configuration"] input:not([type=hidden]), form[action="/configuration"] select, form[action="/configuration"] button');
  const expected = await controls.count();
  const seen = new Set<string>();
  await page.locator("body").click({ position: { x: 1, y: 1 } });
  for (let i = 0; i < expected + 12 && seen.size < expected; i++) {
    await page.keyboard.press("Tab");
    const focus = page.locator(":focus");
    const inside = await focus.evaluate(el => !!el.closest('form[action="/configuration"]'));
    if (!inside) continue;
    expect(await focus.evaluate(el => getComputedStyle(el).outlineStyle)).not.toBe("none");
    const key = await focus.evaluate(el => (el as HTMLInputElement).name || el.textContent?.trim() || "");
    const labelled = await focus.evaluate(el => el.tagName === "BUTTON" || (el as HTMLInputElement).labels?.length);
    expect(labelled, key).toBeTruthy();
    const tag = await focus.evaluate(el => el.tagName);
    if (tag === "SELECT") {
      const before = await focus.inputValue();
      await page.keyboard.press("ArrowDown");
      if (await focus.inputValue() === before) {
        await page.keyboard.press("ArrowUp");
        await page.keyboard.press("ArrowDown");
      } else {
        await page.keyboard.press("ArrowUp");
      }
      await expect(focus).toHaveValue(before);
    } else if (tag === "INPUT") {
      const before = await focus.inputValue();
      const type = await focus.getAttribute("type");
      if (type === "number") {
        await page.keyboard.press("ArrowUp");
        await page.keyboard.press("ArrowDown");
      } else {
        await page.keyboard.press("End");
        await page.keyboard.type("x");
        await page.keyboard.press("Backspace");
      }
      await expect(focus).toHaveValue(before);
    }
    seen.add(key);
  }
  expect(seen.size).toBe(expected);
  await expect(page.getByRole("button", { name: "Save configuration" })).toBeFocused();
  await Promise.all([
    page.waitForResponse(response => response.url().endsWith("/configuration") && response.request().method() === "POST"),
    page.keyboard.press("Enter"),
  ]);
  await expect(page.getByRole("heading", { name: "Configuration" })).toBeVisible();
}

test("every bundled theme and System persists and remains keyboard accessible across restart", async ({ page, browser }) => {
  const options = JSON.parse(execFileSync("go", ["run", "./tests/e2e/catalog"], { encoding: "utf8" })) as { ID: string }[];
  const ids = ["system", ...options.map(option => option.ID)];
  expect(new Set(ids).size).toBe(ids.length);
  const fixture = await createRestartFixture();
  const visitor = await browser.newContext();
  const second = await visitor.newPage();
  try {
    await signIn(page, fixture.baseURL);
    await page.goto(`${fixture.baseURL}/configuration`);
    const rendered = await page.getByLabel("Theme", { exact: true }).locator("option").evaluateAll(nodes => nodes.map(node => (node as HTMLOptionElement).value));
    expect(rendered).toEqual(ids);
    for (const id of ids) {
      await page.getByLabel("Theme", { exact: true }).selectOption(id);
      await Promise.all([
        page.waitForResponse(response => response.url().endsWith("/configuration") && response.request().method() === "POST"),
        page.getByRole("button", { name: "Save configuration" }).click(),
      ]);
      await expectAppearance(page, id);
      await page.reload();
      await expectAppearance(page, id);
      await expect(page.getByLabel("Theme", { exact: true })).toHaveValue(id);
      await second.goto(`${fixture.baseURL}/bookmarks`);
      await expectAppearance(second, id);
      if (id === "system") {
        await page.emulateMedia({ colorScheme: "light" });
        const light = await page.locator("body").evaluate(el => getComputedStyle(el).backgroundColor);
        await page.emulateMedia({ colorScheme: "dark" });
        await expect.poll(() => page.locator("body").evaluate(el => getComputedStyle(el).backgroundColor)).not.toBe(light);
      }
      await checkKeyboard(page);
      // Cancel in-flight decorative resource requests before graceful shutdown.
      await page.goto("about:blank");
      await second.goto("about:blank");
      await fixture.stop();
      await fixture.start();
      await page.goto(`${fixture.baseURL}/configuration`);
      await expectAppearance(page, id);
      await expect(page.getByLabel("Theme", { exact: true })).toHaveValue(id);
    }
  } finally {
    await visitor.close();
    await fixture.close();
  }
});
