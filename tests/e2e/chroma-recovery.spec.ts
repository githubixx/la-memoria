import { execFileSync, spawn } from "node:child_process";
import { readFileSync, writeFileSync } from "node:fs";
import { expect, test } from "@playwright/test";
import { createRestartFixture } from "./restart-fixture";

test.setTimeout(120_000);

test("obsolete settings and failed saves recover without changing appearance", async ({ page }) => {
  const fixture = await createRestartFixture();
  try {
    await fixture.stop();
    const config = readFileSync(fixture.configPath, "utf8") + "\ntheme: retired-theme\n";
    writeFileSync(fixture.configPath, config, { mode: 0o600 });
    await fixture.start();
    await page.goto(`${fixture.baseURL}/login`);
    await page.getByLabel("Username").fill("admin");
    await page.getByLabel("Password").fill("bookmarker-fixture-password");
    await page.getByRole("button", { name: "Sign in" }).click();
    await expect(page).toHaveURL(/\/bookmarks$/);
    await page.goto(`${fixture.baseURL}/configuration`);
    await expect(page.locator("html")).not.toHaveAttribute("data-theme");
    const select = page.getByLabel("Theme", { exact: true });
    await expect(select).toHaveValue("retired-theme");
    await expect(select.locator('option[value="retired-theme"]')).toContainText(/unavailable/i);
    await page.getByLabel("Page title").fill("Candidate recovery");
    await page.locator('form[action="/configuration"]').evaluate((form: HTMLFormElement) => { form.noValidate = true; });
    await page.getByRole("button", { name: "Save configuration" }).click();
    await expect(page.locator(".field-error")).toBeVisible();
    await expect(page.getByLabel("Theme", { exact: true })).toHaveAttribute("aria-invalid", "true");
    expect(await page.getByLabel("Theme", { exact: true }).getAttribute("aria-describedby")).toBeTruthy();
    expect(readFileSync(fixture.configPath, "utf8")).toBe(config);
    const ids = JSON.parse(execFileSync("go", ["run", "./tests/e2e/catalog"], { encoding: "utf8" })) as { ID: string }[];
    const selected = ids[0].ID;
    await page.getByLabel("Theme", { exact: true }).selectOption(selected);
    await Promise.all([
      page.waitForResponse(response => response.url().endsWith("/configuration") && response.request().method() === "POST"),
      page.getByRole("button", { name: "Save configuration" }).click(),
    ]);
    await expect(page.locator("html")).toHaveAttribute("data-theme", selected);
    await expect(page.getByLabel("Theme", { exact: true })).toHaveValue(selected);
    const before = readFileSync(fixture.configPath, "utf8");

    // Hold the stable sibling lock in a fixture-owned process to force a
    // bounded pre-commit storage failure without touching permissions.
    const lockPath = `${fixture.configPath}.lock`;
    const holder = spawn("flock", ["--no-fork", "--exclusive", lockPath, "sleep", "20"], { stdio: "ignore" });
    try {
      let held = false;
      for (let i = 0; i < 50 && !held; i++) {
        await new Promise(resolve => setTimeout(resolve, 50));
        try { execFileSync("flock", ["--nonblock", lockPath, "true"], { stdio: "ignore" }); }
        catch { held = true; }
      }
      expect(held).toBeTruthy();
      await page.getByLabel("Theme", { exact: true }).selectOption("system");
      await page.getByLabel("Page title").fill("Uncommitted title");
      await page.getByRole("button", { name: "Save configuration" }).click();
      await expect(page.locator(".field-error")).toBeVisible({ timeout: 10_000 });
      await expect(page.getByLabel("Page title")).toHaveValue("Uncommitted title");
      await expect(page.locator("html")).toHaveAttribute("data-theme", selected);
      expect(readFileSync(fixture.configPath, "utf8")).toBe(before);
    } finally {
      if (holder.exitCode === null && holder.signalCode === null) {
        const exited = new Promise<void>(resolve => holder.once("exit", () => resolve()));
        holder.kill("SIGTERM");
        await exited;
      }
    }

    // A non-htmx tampered form returns a full error document using persisted CSS.
    await page.route("**/assets/vendor/htmx.min.js", route => route.abort());
    await page.goto(`${fixture.baseURL}/configuration`);
    await page.getByLabel("Theme", { exact: true }).evaluate((element: HTMLSelectElement) => {
      const option = new Option("Tampered", "../invalid");
      element.add(option);
      element.value = option.value;
    });
    await page.getByRole("button", { name: "Save configuration" }).click();
    await expect(page.locator(".field-error")).toBeVisible();
    await expect(page.locator("html")).toHaveAttribute("data-theme", selected);
    expect(readFileSync(fixture.configPath, "utf8")).toBe(before);
  } finally {
    await fixture.close();
  }
});
