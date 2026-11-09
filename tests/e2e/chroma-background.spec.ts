import { execFileSync } from "node:child_process";
import { expect, test, type Page } from "@playwright/test";
import { createRestartFixture } from "./restart-fixture";

test.setTimeout(120_000);

async function signIn(page: Page, base: string) {
  await page.goto(`${base}/login`);
  await page.getByLabel("Username").fill("admin");
  await page.getByLabel("Password").fill("bookmarker-fixture-password");
  await page.getByRole("button", { name: "Sign in" }).click();
  await expect(page).toHaveURL(/\/bookmarks$/);
}

async function chooseTheme(page: Page, id: string) {
  await page.getByLabel("Theme", { exact: true }).selectOption(id);
  await Promise.all([
    page.waitForResponse(response => response.url().endsWith("/configuration") && response.request().method() === "POST"),
    page.getByRole("button", { name: "Save configuration" }).click(),
  ]);
  if (id === "system") await expect(page.locator("html")).not.toHaveAttribute("data-theme");
  else await expect(page.locator("html")).toHaveAttribute("data-theme", id);
}

async function navigate(page: Page, name: string) {
  await page.getByRole("button", { name: "Menu", exact: true }).click();
  await page.getByRole("navigation", { name: "Primary" }).getByRole("link", { name, exact: true }).click();
  await expect(page.getByRole("heading", { name, exact: true })).toBeVisible();
}

async function background(page: Page) {
  return page.locator("body").evaluate(el => {
    const style = getComputedStyle(el);
    return {
      image: style.backgroundImage,
      attachment: style.backgroundAttachment,
      color: style.backgroundColor,
      size: style.backgroundSize,
      position: style.backgroundPosition,
      repeat: style.backgroundRepeat,
    };
  });
}

async function backgroundPixels(page: Page) {
  // Capture the viewport first: Playwright's clipped screenshots can scroll
  // the page to the clip rectangle, changing the position being tested.
  const image = (await page.screenshot({ animations: "disabled" })).toString("base64");
  return page.evaluate(async data => {
    const screenshot = new Image();
    screenshot.src = `data:image/png;base64,${data}`;
    await screenshot.decode();
    const scale = screenshot.width / innerWidth;
    const canvas = document.createElement("canvas");
    canvas.width = 8 * scale;
    canvas.height = 50 * scale;
    canvas.getContext("2d")!.drawImage(screenshot, 0, 200 * scale, canvas.width, canvas.height, 0, 0, canvas.width, canvas.height);
    return Array.from(canvas.getContext("2d")!.getImageData(0, 0, canvas.width, canvas.height).data);
  }, image);
}

test("Aurora background stays fixed across menu navigation, scrolling, reload and asset failure", async ({ page }) => {
  const fixture = await createRestartFixture({ temporaryRoot: process.cwd() });
  try {
    await page.route("https://**", route => route.abort());
    await signIn(page, fixture.baseURL);
    await page.goto(`${fixture.baseURL}/configuration`);
    await chooseTheme(page, "aurora-glass-1");

    for (const unavailable of [false, true]) {
      let blockedAssets = 0;
      if (unavailable) {
        await page.route("**/ui/themes/aurora-glass-1/assets/**", route => {
          blockedAssets++;
          return route.abort();
        });
      }
      const imageResponses: number[] = [];
      const onResponse = (response: import("@playwright/test").Response) => {
        if (/\/aurora-(desktop|mobile)\.(avif|webp)$/.test(new URL(response.url()).pathname)) imageResponses.push(response.status());
      };
      page.on("response", onResponse);
      await page.goto(`${fixture.baseURL}/search`, { waitUntil: "networkidle" });
      page.off("response", onResponse);
      if (!unavailable) expect(imageResponses, "bundled background image loads").toContain(200);
      else expect(blockedAssets, "decorative requests were actually blocked").toBeGreaterThan(0);
      const style = await background(page);
      expect(style.image).toMatch(/aurora-(desktop|mobile)/);
      expect(style.attachment).toBe("fixed");
      expect(style.color).toBe("rgb(5, 10, 32)");
      const short = await page.evaluate(() => ({
        contentWidth: document.body.getBoundingClientRect().width,
        scrollHeight: document.documentElement.scrollHeight,
        height: innerHeight,
      }));
      expect(short.scrollHeight).toBeLessThanOrEqual(short.height);
      const reference = await backgroundPixels(page);

      await navigate(page, "Configuration");
      const tall = await page.evaluate(() => ({
        contentWidth: document.body.getBoundingClientRect().width,
        scrollHeight: document.documentElement.scrollHeight,
        height: innerHeight,
      }));
      expect(tall.scrollHeight).toBeGreaterThan(tall.height);
      expect((await backgroundPixels(page)).every((value, i) => value === reference[i])).toBe(true);
      expect(tall.contentWidth).toBe(short.contentWidth);
      await page.evaluate(() => scrollTo(0, 300));
      const scrollY = await page.evaluate(() => window.scrollY);
      expect(scrollY).toBeGreaterThan(0);
      const scrolled = await backgroundPixels(page);
      // Some engines produce tiny compositing differences after scrolling.
      const meanDelta = scrolled.reduce((total, value, i) => total + Math.abs(value - reference[i]), 0) / scrolled.length;
      expect(meanDelta).toBeLessThan(1);
      await page.evaluate(() => scrollTo(0, 0));
      await page.reload({ waitUntil: "networkidle" });
      expect((await backgroundPixels(page)).every((value, i) => value === reference[i])).toBe(true);
      await navigate(page, "Search");
      expect((await backgroundPixels(page)).every((value, i) => value === reference[i])).toBe(true);
    }
  } finally {
    await fixture.close();
  }
});

test("System and bundled themes retain Chroma backgrounds and fallback colors", async ({ page }) => {
  const fixture = await createRestartFixture({ temporaryRoot: process.cwd() });
  try {
    await page.route("https://**", route => route.abort());
    await signIn(page, fixture.baseURL);
    await page.goto(`${fixture.baseURL}/configuration`);
    const themes = JSON.parse(execFileSync("go", ["run", "./tests/e2e/catalog"], { encoding: "utf8" })) as { ID: string }[];
    for (const id of ["system", ...themes.map(theme => theme.ID)]) {
      await chooseTheme(page, id);
      const actual = await background(page);
      const withoutBookmarker = await page.evaluate(() => {
        const sheet = document.querySelector<HTMLLinkElement>('link[href^="/assets/css/bookmarker.css"]')!;
        sheet.disabled = true;
        const style = getComputedStyle(document.body);
        const result = {
          image: style.backgroundImage, attachment: style.backgroundAttachment,
          color: style.backgroundColor, size: style.backgroundSize,
          position: style.backgroundPosition, repeat: style.backgroundRepeat,
        };
        sheet.disabled = false;
        return result;
      });
      expect(actual, id).toEqual(withoutBookmarker);
      expect(actual.color, id).not.toBe("rgba(0, 0, 0, 0)");
    }
  } finally {
    await fixture.close();
  }
});
