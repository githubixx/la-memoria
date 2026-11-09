import { execFileSync } from "node:child_process";
import { expect, test } from "@playwright/test";
import { createRestartFixture } from "./restart-fixture";

test("real configuration picker, validation, swap and save", async ({ page }, testInfo) => {
  test.setTimeout(180_000);
  // Only use a locally cached image; the fixture owns a UUID container and
  // a fresh temporary directory, never the project's tracked fixtures.
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
    const theme = page.locator("#theme");
    const view = page.locator("#default-view");
    const tls = page.locator("#database-tls-mode");
    const themeTrigger = page.getByRole("combobox", { name: "Theme", exact: true });
    const viewTrigger = page.getByRole("combobox", { name: "Default view", exact: true });
    await expect(page.locator("form select")).toHaveCount(3);
    await viewTrigger.click();
    await expect(viewTrigger).toHaveAttribute("aria-expanded", "true");
    await page.getByRole("option", { name: "Add", exact: true }).click();
    await expect(view).toHaveValue("add");
    await theme.evaluate(el => {
      (el as HTMLSelectElement).add(new Option("Unavailable", "missing-theme", true, true));
    });
    let response = page.waitForResponse(r => r.url().endsWith("/configuration") && r.request().method() === "POST");
    await page.getByRole("button", { name: "Save configuration" }).click();
    expect((await response).status()).toBe(422);
    await expect(theme).toHaveAttribute("aria-invalid", "true");
    await expect(page.locator("#theme-help")).toBeVisible();
    await expect(theme.locator("option:checked")).toBeDisabled();
    expect(await theme.evaluate(el => new FormData((el as HTMLSelectElement).form!).has("theme"))).toBe(false);
    // Omitted disabled selected values must not silently save.
    response = page.waitForResponse(r => r.url().endsWith("/configuration") && r.request().method() === "POST");
    await page.getByRole("button", { name: "Save configuration" }).click();
    expect((await response).status()).toBe(422);
    await expect(theme).toHaveAttribute("aria-invalid", "true");
    await theme.selectOption("cartoon-1");
    // The isolated PostgreSQL fixture does not offer TLS; keep its actual
    // connection valid while exercising the TLS select's submitted value.
    await tls.selectOption("disable");
    response = page.waitForResponse(r => r.url().endsWith("/configuration") && r.request().method() === "POST");
    await page.getByRole("button", { name: "Save configuration" }).click();
    expect((await response).status()).toBe(200);
    await expect(page).toHaveURL(/\/configuration/);
    await expect(theme).toHaveValue("cartoon-1");
    await expect(tls).toHaveValue("disable");
    await themeTrigger.click();
    await expect(themeTrigger).toHaveAttribute("aria-expanded", "true");
    await expect(page.getByRole("option", { name: "Cartoon 1", exact: true })).toHaveAttribute("aria-selected", "true");
    await page.screenshot({ path: testInfo.outputPath("cartoon-open-picker.png") });
    await page.keyboard.press("Escape");
  } finally {
    await fixture.close();
  }
});

test("real normal form submission without JavaScript", async ({ browser }) => {
  test.setTimeout(180_000);
  execFileSync("docker", ["image", "inspect", "postgres:18.6"], { stdio: "ignore" });
  const fixture = await createRestartFixture();
  const context = await browser.newContext({ javaScriptEnabled: false });
  try {
    await context.route("https://**", route => route.abort());
    const page = await context.newPage();
    await page.goto(`${fixture.baseURL}/login`);
    await page.getByLabel("Username").fill("admin");
    await page.getByLabel("Password").fill("bookmarker-fixture-password");
    await page.getByRole("button", { name: "Sign in" }).click();
    await page.goto(`${fixture.baseURL}/configuration`);
    await page.getByLabel("Theme", { exact: true }).selectOption("cartoon-1");
    await page.getByLabel("Default view", { exact: true }).selectOption("search");
    const post = page.waitForResponse(r => r.url().endsWith("/configuration") && r.request().method() === "POST");
    await page.getByRole("button", { name: "Save configuration" }).click();
    expect((await post).status()).toBe(303);
    await expect(page.getByLabel("Theme", { exact: true })).toHaveValue("cartoon-1");
    await expect(page.getByLabel("Default view", { exact: true })).toHaveValue("search");
  } finally {
    await context.close();
    await fixture.close();
  }
});
