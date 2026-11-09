import { execFileSync } from "node:child_process";
import { expect, test, type Page } from "@playwright/test";
import { createRestartFixture } from "./restart-fixture";

const fields = [
  ["theme", "Theme"], ["default-view", "Default view"], ["database-tls-mode", "Database TLS mode"],
] as const;

async function expectEnhancedFields(page: Page) {
  await expect(page.locator("form select.themed-select-native")).toHaveCount(3);
  await expect(page.locator(".themed-select")).toHaveCount(3);
  await expect(page.getByRole("combobox")).toHaveCount(3);
  for (const [id, label] of fields) {
    const native = page.locator(`#${id}`);
    const trigger = page.getByRole("combobox", { name: label, exact: true });
    await expect(trigger).toHaveAttribute("id", `${id}-trigger`);
    const state = await native.evaluate((el: HTMLSelectElement) => {
      const wrapper = el.parentElement!;
      const button = wrapper.querySelector<HTMLButtonElement>(".themed-select-trigger")!;
      const style = getComputedStyle(el);
      return {
        position: style.position, opacity: Number(style.opacity), pointerEvents: style.pointerEvents,
        hidden: el.getAttribute("aria-hidden"), tabindex: el.getAttribute("tabindex"),
        wrapperHeight: wrapper.getBoundingClientRect().height, buttonHeight: button.getBoundingClientRect().height,
      };
    });
    expect(state, id).toMatchObject({
      position: "absolute", opacity: 0, pointerEvents: "none", hidden: "true", tabindex: "-1",
    });
    expect(state.buttonHeight, id).toBeGreaterThan(0);
    expect(Math.abs(state.wrapperHeight - state.buttonHeight), `${id}: extra layout row`).toBeLessThanOrEqual(1);
  }
}

async function expectNativeFields(page: Page) {
  await expect(page.locator(".themed-select")).toHaveCount(0);
  await expect(page.getByRole("combobox")).toHaveCount(3);
  for (const [id, label] of fields) {
    const native = page.getByRole("combobox", { name: label, exact: true });
    await expect(native).toHaveAttribute("id", id);
    await expect(native).toBeVisible();
    await expect(native).toBeEnabled();
    await expect(native).not.toHaveAttribute("aria-hidden", "true");
    await expect(native).not.toHaveAttribute("tabindex", "-1");
    const style = await native.evaluate(el => ({
      opacity: Number(getComputedStyle(el).opacity), pointerEvents: getComputedStyle(el).pointerEvents,
    }));
    expect(style.opacity, id).toBeGreaterThan(0);
    expect(style.pointerEvents, id).not.toBe("none");
  }
}

test("desktop dropdowns preserve keyboard, native values and HTMX recovery", async ({ page }, testInfo) => {
  test.setTimeout(180_000);
  execFileSync("docker", ["image", "inspect", "postgres:18.6"], { stdio: "ignore" });
  const fixture = await createRestartFixture();
  try {
    await page.route("https://**", route => route.abort());
    await page.goto(`${fixture.baseURL}/login`);
    await page.getByLabel("Username").fill("admin");
    await page.getByLabel("Password").fill("bookmarker-fixture-password");
    await page.getByRole("button", { name: "Sign in" }).click();
    await expect(page).toHaveURL(/\/bookmarks$/);
    await page.goto(`${fixture.baseURL}/configuration`);

    // Exercise the theme reported in the screenshot on the actual page.
    await page.locator("#theme").selectOption("spaceship-red-dark");
    const themeSaved = page.waitForResponse(r => r.url().endsWith("/configuration") && r.request().method() === "POST");
    await page.getByRole("button", { name: "Save configuration" }).click();
    expect((await themeSaved).status()).toBe(200);
    await expect(page.locator("html")).toHaveAttribute("data-theme", "spaceship-red-dark");
    await expectEnhancedFields(page);
    // Keep every widget in view while comparing pixels: this theme has a
    // viewport-fixed gradient, so scrolling between shots changes its pixels.
    await page.setViewportSize({ width: 1280, height: 2400 });
    await page.evaluate(() => window.scrollTo(0, 0));
    for (const [id] of fields) {
      const wrapper = page.locator(`#${id}`).locator("..");
      const before = await wrapper.screenshot({ animations: "disabled" });
      const hideNative = await page.addStyleTag({ content: ".themed-select-native { visibility: hidden !important }" });
      const after = await wrapper.screenshot({ animations: "disabled" });
      await hideNative.evaluate(el => (el as Element).remove());
      if (!before.equals(after)) {
        await testInfo.attach(`${id}-before`, { body: before, contentType: "image/png" });
        await testInfo.attach(`${id}-native-hidden`, { body: after, contentType: "image/png" });
      }
      expect(before.equals(after), `${id}: native select paints anywhere inside its widget`).toBe(true);
    }
    await page.setViewportSize({ width: 1280, height: 720 });
    await expectEnhancedFields(page);

    const trigger = page.getByRole("combobox", { name: "Default view" });
    await expect(trigger).toHaveClass(/themed-select-trigger/);
    await page.locator("label", { hasText: "Default view" }).click();
    await expect(trigger).toBeFocused();
    await trigger.press("ArrowDown");
    await expect(trigger).toHaveAttribute("aria-expanded", "true");
    await trigger.press("End");
    await expect(page.getByRole("option", { name: "Configuration" })).toHaveAttribute("data-active", "true");
    await trigger.press("Home");
    await trigger.press("s");
    await trigger.press("Enter");
    await expect(page.locator("#default-view")).toHaveValue("search");
    await expect(trigger).toHaveAttribute("aria-expanded", "false");
    await trigger.press("ArrowDown");
    await trigger.press("End");
    await trigger.press("Escape");
    await expect(page.locator("#default-view")).toHaveValue("search");

    const theme = page.locator("#theme");
    await theme.evaluate(el => (el as HTMLSelectElement).add(new Option("Unavailable", "missing-theme", true, true)));
    await theme.dispatchEvent("change");
    const response = page.waitForResponse(r => r.url().endsWith("/configuration") && r.request().method() === "POST");
    await page.getByRole("button", { name: "Save configuration" }).click();
    expect((await response).status()).toBe(422);
    const recovery = page.getByRole("combobox", { name: "Theme", exact: true });
    await expect(recovery).toHaveClass(/themed-select-trigger/);
    await expectEnhancedFields(page);
    await expect(recovery).toHaveAttribute("aria-invalid", "true");
    await recovery.click();
    await expect(page.getByRole("option", { name: /unavailable/ })).toHaveAttribute("aria-disabled", "true");
    await page.getByRole("option", { name: "System", exact: true }).click();
    await expect(page.locator("#theme")).toHaveValue("system");
    const saved = page.waitForResponse(r => r.url().endsWith("/configuration") && r.request().method() === "POST");
    await page.getByRole("button", { name: "Save configuration" }).click();
    expect((await saved).status()).toBe(200);
  } finally {
    await fixture.close();
  }
});

test("real configuration falls back when the native-hiding rule is missing", async ({ page }) => {
  test.setTimeout(180_000);
  execFileSync("docker", ["image", "inspect", "postgres:18.6"], { stdio: "ignore" });
  const fixture = await createRestartFixture();
  try {
    await page.route("https://**", route => route.abort());
    await page.goto(`${fixture.baseURL}/login`);
    await page.getByLabel("Username").fill("admin");
    await page.getByLabel("Password").fill("bookmarker-fixture-password");
    await page.getByRole("button", { name: "Sign in" }).click();
    await expect(page).toHaveURL(/\/bookmarks$/);

    await page.route("**/assets/css/chroma-compat.css*", async route => {
      const response = await route.fetch();
      const css = await response.text();
      const withoutRule = css.replace(/^\s*\.form-grid \.themed-select-native\s*\{[^}]*\}/m, "");
      expect(withoutRule, "the live response must contain the native-hiding rule").not.toBe(css);
      await route.fulfill({ response, body: withoutRule });
    });
    await page.goto(`${fixture.baseURL}/configuration`);
    await expectNativeFields(page);
    await page.locator('label[for="default-view"]').click();
    await expect(page.locator("#default-view")).toBeFocused();
    await page.locator("#default-view").selectOption("search");
    expect(await page.locator('form[action="/configuration"]').evaluate((form: HTMLFormElement) => Object.fromEntries(new FormData(form))))
      .toMatchObject({ theme: "system", default_view: "search", database_tls_mode: "disable" });

    await page.locator("#theme").evaluate((el: HTMLSelectElement) => {
      el.add(new Option("Unavailable", "missing-theme", true, true));
    });
    for (let attempt = 0; attempt < 2; attempt++) {
      const response = page.waitForResponse(r => r.url().endsWith("/configuration") && r.request().method() === "POST");
      await page.getByRole("button", { name: "Save configuration" }).click();
      expect((await response).status()).toBe(422);
      await expect(page.locator("#configuration-error")).toBeVisible();
      await expectNativeFields(page);
      await expect(page.locator("#theme")).toHaveAttribute("aria-invalid", "true");
    }
    await page.locator("#theme").selectOption("system");
    const saved = page.waitForResponse(r => r.url().endsWith("/configuration") && r.request().method() === "POST");
    await page.getByRole("button", { name: "Save configuration" }).click();
    expect((await saved).status()).toBe(200);
  } finally {
    await fixture.close();
  }
});

test("native select remains available in forced-colors mode", async ({ page }) => {
  test.setTimeout(180_000);
  execFileSync("docker", ["image", "inspect", "postgres:18.6"], { stdio: "ignore" });
  const fixture = await createRestartFixture();
  try {
    await page.emulateMedia({ forcedColors: "active" });
    await page.route("https://**", route => route.abort());
    await page.goto(`${fixture.baseURL}/login`);
    await page.getByLabel("Username").fill("admin");
    await page.getByLabel("Password").fill("bookmarker-fixture-password");
    await page.getByRole("button", { name: "Sign in" }).click();
    await expect(page).toHaveURL(/\/bookmarks$/);
    await page.goto(`${fixture.baseURL}/configuration`);
    await expectNativeFields(page);
  } finally {
    await fixture.close();
  }
});
