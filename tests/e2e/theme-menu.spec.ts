import { execFileSync } from "node:child_process";
import { expect, test, type Page, type TestInfo } from "@playwright/test";
import { createRestartFixture } from "./restart-fixture";

test.setTimeout(600_000);

async function setTheme(page: Page, id: string, scheme: "light" | "dark") {
  await page.emulateMedia({ colorScheme: scheme, reducedMotion: "reduce" });
  await page.evaluate(async id => {
    document.querySelector("#menu-test-theme")?.remove();
    if (id === "system") {
      document.documentElement.removeAttribute("data-theme");
    } else {
      document.documentElement.dataset.theme = id;
      const link = document.createElement("link");
      link.id = "menu-test-theme";
      link.rel = "stylesheet";
      link.href = `/ui/themes/${id}/theme.css`;
      const ready = new Promise<void>((resolve, reject) => {
        link.onload = () => resolve();
        // The app's CSP blocks remote font imports, which can fire onerror
        // even though the same-origin theme stylesheet itself loaded.
        link.onerror = () => link.sheet?.cssRules.length
          ? resolve() : reject(new Error(`Unable to load ${link.href}`));
      });
      document.querySelector('link[href^="/assets/css/chroma-compat.css"]')!.before(link);
      await ready;
    }
    await document.fonts.ready;
  }, id);
}

// Compare a real rendering to the same scene with only menu glyphs hidden.
// Restrict samples to actual text-range pixels that changed and lie close to
// the declared foreground: this excludes antialiased edge pixels and checks
// that every label painted visible glyphs. Contrast uses the *rendered* glyph
// pixel and its corresponding composited background pixel.
async function glyphContrast(page: Page) {
  const labels = await page.locator(".menu-list a, .menu-list button").evaluateAll(els => els.map(el => {
    const range = document.createRange();
    range.selectNodeContents(el);
    const r = range.getBoundingClientRect();
    return { name: el.textContent!.trim(), x: r.x, y: r.y, width: r.width, height: r.height, color: getComputedStyle(el).color };
  }));
  const rendered = (await page.screenshot({ animations: "disabled" })).toString("base64");
  await page.addStyleTag({ content: `.menu-list a, .menu-list button { color: transparent !important; -webkit-text-fill-color: transparent !important; text-shadow: none !important }`, });
  const background = (await page.screenshot({ animations: "disabled" })).toString("base64");
  await page.locator("style").last().evaluate(el => el.remove());
  return page.evaluate(async ({ rendered, background, labels }) => {
    async function pixels(src: string) {
      const image = new Image();
      image.src = `data:image/png;base64,${src}`;
      await image.decode();
      const canvas = document.createElement("canvas");
      canvas.width = image.width; canvas.height = image.height;
      const context = canvas.getContext("2d")!;
      context.drawImage(image, 0, 0);
      return { data: context.getImageData(0, 0, image.width, image.height).data, width: image.width, scale: image.width / innerWidth };
    }
    const a = await pixels(rendered), b = await pixels(background);
    const luminance = (rgb: number[]) => rgb.map(c => {
      c /= 255;
      return c <= .04045 ? c / 12.92 : ((c + .055) / 1.055) ** 2.4;
    }).reduce((sum, c, i) => sum + c * [0.2126, 0.7152, 0.0722][i], 0);
    return labels.map(label => {
      const ink = label.color.match(/\d+/g)!.slice(0, 3).map(Number);
      let minimum = Infinity, count = 0;
      const x0 = Math.max(0, Math.floor(label.x * a.scale)), y0 = Math.max(0, Math.floor(label.y * a.scale));
      const x1 = Math.min(a.width, Math.ceil((label.x + label.width) * a.scale));
      const y1 = Math.min(a.data.length / 4 / a.width, Math.ceil((label.y + label.height) * a.scale));
      for (let y = y0; y < y1; y++) for (let x = x0; x < x1; x++) {
        const i = (y * a.width + x) * 4;
        const fg = [a.data[i], a.data[i + 1], a.data[i + 2]];
        const bg = [b.data[i], b.data[i + 1], b.data[i + 2]];
        if (Math.max(...fg.map((v, j) => Math.abs(v - ink[j]))) > 20 ||
            Math.max(...fg.map((v, j) => Math.abs(v - bg[j]))) < 30) continue;
        const l1 = luminance(fg), l2 = luminance(bg);
        minimum = Math.min(minimum, (Math.max(l1, l2) + .05) / (Math.min(l1, l2) + .05));
        count++;
      }
      return { name: label.name, minimum, count };
    });
  }, { rendered, background, labels });
}

async function checkContrast(page: Page, context: string) {
  const results = await glyphContrast(page);
  expect(results.map(r => r.name), context).toEqual(["List", "Search", "Add", "Configuration", "Sign out"]);
  for (const result of results) {
    expect(result.count, `${context} ${result.name}: painted glyph pixels`).toBeGreaterThanOrEqual(3);
    expect(result.minimum, `${context} ${result.name}: rendered glyph/background contrast`).toBeGreaterThanOrEqual(4.5);
  }
  return Math.min(...results.map(r => r.minimum));
}

async function transmission(page: Page) {
  // Use two uniform backgrounds and compare the same unlabeled menu pixel.
  // This measures actual compositing, rather than a computed alpha value.
  const sample = async (color: string) => {
    await page.locator("#menu-overlap").evaluate((el, color) => {
      (el as HTMLElement).style.background = color;
      el.classList.add("flat");
    }, color);
    const box = (await page.locator(".menu-list").boundingBox())!;
    const png = (await page.screenshot({ animations: "disabled" })).toString("base64");
    return page.evaluate(async ({ png, x, y }) => {
      const image = new Image(); image.src = `data:image/png;base64,${png}`; await image.decode();
      const canvas = document.createElement("canvas"); canvas.width = image.width; canvas.height = image.height;
      const ctx = canvas.getContext("2d")!; ctx.drawImage(image, 0, 0);
      return Array.from(ctx.getImageData(Math.round(x * image.width / innerWidth), Math.round(y * image.width / innerWidth), 1, 1).data).slice(0, 3);
    }, { png, x: box.x + box.width - 5, y: box.y + box.height / 2 });
  };
  const black = await sample("black");
  const white = await sample("white");
  await page.locator("#menu-overlap").evaluate(el => {
    el.classList.remove("flat");
    (el as HTMLElement).style.background = "";
  });
  const rates = black.map((v, i) => (white[i] - v) / 255);
  if (rates.some(rate => rate < .07 || rate > .13)) console.log("Transmission samples", { black, white, rates });
  return rates;
}

test("all bundled menus remain readable over page content", async ({ page }, testInfo: TestInfo) => {
  const fixture = await createRestartFixture({ temporaryRoot: process.cwd() });
  let worst = Infinity, minTransmission = Infinity, maxTransmission = 0, cases = 0;
  const imageAssets = new Set<string>();
  try {
    await page.route("https://**", route => route.request().resourceType() === "stylesheet"
      ? route.fulfill({ status: 200, contentType: "text/css", body: "" })
      : route.abort());
    page.on("response", response => {
      if (response.status() === 200 && /\/ui\/themes\/(aurora-glass-1|crt-starfield-1)\/assets\/.*\.(avif|webp)$/.test(response.url())) {
        imageAssets.add(new URL(response.url()).pathname);
      }
    });
    await page.goto(`${fixture.baseURL}/login`);
    await page.getByLabel("Username").fill("admin");
    await page.getByLabel("Password").fill("bookmarker-fixture-password");
    await page.getByRole("button", { name: "Sign in" }).click();
    await expect(page).toHaveURL(/\/bookmarks$/);
    await page.goto(`${fixture.baseURL}/configuration`);
    // Keep actual configuration content, plus adversarial text and controls
    // behind the popup. All theme CSS/assets continue to load from the server.
    await page.evaluate(() => {
      const overlay = document.createElement("div");
      overlay.id = "menu-overlap";
      overlay.setAttribute("aria-hidden", "true");
      overlay.innerHTML = `<h1>OVERLAPPING HEADING TEXT</h1><input value="Underlying control text"><p>HIGH FREQUENCY TEXT /////// WWWWWWWWW</p><button>Underlying control</button><p>BLACK WHITE TEXT BLACK WHITE TEXT</p>`;
      overlay.querySelectorAll("input, button").forEach(el => el.setAttribute("tabindex", "-1"));
      document.body.append(overlay);
    });
    await page.addStyleTag({ content: `
      *, *::before, *::after { animation: none !important; transition: none !important }
      .site-header { position: relative; z-index: 20 }
      #menu-overlap { position: fixed; z-index: 10; inset: 3.5rem 0 0; pointer-events: none;
        background: repeating-linear-gradient(45deg, #fff 0 4px, #000 4px 8px); color: #e11; font: bold 24px sans-serif; overflow: hidden }
      #menu-overlap > * { position: absolute; right: 0; width: 220px; margin: 0 }
      #menu-overlap > :nth-child(1) { top: 0 }
      #menu-overlap > :nth-child(2) { top: 55px }
      #menu-overlap > :nth-child(3) { top: 90px }
      #menu-overlap > :nth-child(4) { top: 165px }
      #menu-overlap > :nth-child(5) { top: 205px }
      #menu-overlap.flat { background-image: none !important }
      #menu-overlap.flat > * { visibility: hidden }
    ` });
    const themes = JSON.parse(execFileSync("go", ["run", "./tests/e2e/catalog"], { encoding: "utf8" })) as { ID: string }[];
    const casesToRun = [{ id: "system", scheme: "light" as const }, { id: "system", scheme: "dark" as const },
      ...themes.map(t => ({ id: t.ID, scheme: "light" as const }))];
    for (const viewport of [{ width: 403, height: 403 }, { width: 1280, height: 800 }]) {
      await page.setViewportSize(viewport);
      for (const { id, scheme } of casesToRun) {
        const context = `${testInfo.project.name} ${viewport.width}x${viewport.height} ${id}/${scheme}`;
        await setTheme(page, id, scheme);
        if (id === "aurora-glass-1" || id === "crt-starfield-1") await page.waitForLoadState("networkidle");
        const toggle = page.getByRole("button", { name: "Menu", exact: true });
        await toggle.focus();
        await page.keyboard.press("Enter");
        await expect(toggle, context).toHaveAttribute("aria-expanded", "true");
        const menu = page.locator(".menu-list");
        await expect(menu, context).toBeVisible();
        worst = Math.min(worst, await checkContrast(page, `${context} normal`));
        await page.locator("#menu-overlap").evaluate(el => (el as HTMLElement).style.visibility = "hidden");
        worst = Math.min(worst, await checkContrast(page, `${context} actual configuration/theme`));
        if ((id === "aurora-glass-1" || id === "crt-starfield-1") && viewport.width === 403) {
          const path = testInfo.outputPath(`${id}-configuration.png`);
          await page.screenshot({ path, animations: "disabled" });
          await testInfo.attach(`${id}-configuration`, { path, contentType: "image/png" });
        }
        await page.locator("#menu-overlap").evaluate(el => (el as HTMLElement).style.visibility = "");
        const rates = await transmission(page);
        minTransmission = Math.min(minTransmission, ...rates);
        maxTransmission = Math.max(maxTransmission, ...rates);
        for (const rate of rates) expect(rate, `${context} transmission`).toBeGreaterThanOrEqual(.07);
        for (const rate of rates) expect(rate, `${context} transmission`).toBeLessThanOrEqual(.13);
        await page.getByRole("link", { name: "Search", exact: true }).hover();
        worst = Math.min(worst, await checkContrast(page, `${context} hover`));
        await page.getByRole("button", { name: "Sign out" }).hover();
        worst = Math.min(worst, await checkContrast(page, `${context} sign-out hover`));
        await toggle.focus();
        await page.keyboard.press("Tab");
        await expect(page.getByRole("link", { name: "List", exact: true }), context).toBeFocused();
        worst = Math.min(worst, await checkContrast(page, `${context} link keyboard focus`));
        for (let i = 0; i < 4; i++) await page.keyboard.press("Tab");
        await expect(page.getByRole("button", { name: "Sign out" }), context).toBeFocused();
        expect(await page.getByRole("button", { name: "Sign out" }).evaluate(el => getComputedStyle(el).outlineStyle), context).not.toBe("none");
        worst = Math.min(worst, await checkContrast(page, `${context} keyboard focus`));
        if (id === "dark" && viewport.width === 403) {
          const path = testInfo.outputPath("dark-overlap.png");
          await page.screenshot({ path, animations: "disabled" });
          await testInfo.attach("dark-overlap", { path, contentType: "image/png" });
        }
        await page.keyboard.press("Escape");
        await expect(toggle, context).toBeFocused();
        await expect(toggle, context).toHaveAttribute("aria-expanded", "false");
        await toggle.click();
        await page.mouse.click(10, viewport.height - 10);
        await expect(toggle, context).toHaveAttribute("aria-expanded", "false");
        cases++;
      }
    }
    expect([...imageAssets].some(path => /aurora-glass-1\/assets\/aurora-/.test(path)), "Aurora background image loaded").toBe(true);
    expect([...imageAssets].some(path => /crt-starfield-1\/assets\/crt-/.test(path)), "CRT background image loaded").toBe(true);

    // No blur support: the color backing alone must preserve readable glyphs.
    await page.addStyleTag({ content: ".menu-ready .menu-list { backdrop-filter: none !important; -webkit-backdrop-filter: none !important }" });
    await page.getByRole("button", { name: "Menu", exact: true }).click();
    worst = Math.min(worst, await checkContrast(page, "without backdrop-filter"));
    await page.keyboard.press("Escape");

    // A very short viewport must scroll the menu to the keyboard-focused item.
    await page.setViewportSize({ width: 403, height: 220 });
    const shortToggle = page.getByRole("button", { name: "Menu", exact: true });
    await shortToggle.focus();
    await page.keyboard.press("Enter");
    // Some engines insert the scrollable list itself into the tab order.
    for (let i = 0; i < 7; i++) {
      await page.keyboard.press("Tab");
      if (await page.getByRole("button", { name: "Sign out" }).evaluate(el => el === document.activeElement)) break;
      expect(await page.locator(".site-menu").evaluate(el => el.contains(document.activeElement))).toBe(true);
    }
    await expect(page.getByRole("button", { name: "Sign out" })).toBeFocused();
    expect(await page.locator(".menu-list").evaluate(el => el.scrollTop)).toBeGreaterThan(0);
    const shortBox = (await page.locator(".menu-list").boundingBox())!;
    expect(shortBox.y + shortBox.height).toBeLessThanOrEqual(220);
    await page.keyboard.press("Escape");
    await page.setViewportSize({ width: 1280, height: 800 });

    // Exercise persisted Bookmarker selection and server-rendered load order,
    // not only the dynamic stylesheet switching used for the full catalog.
    for (const id of ["dark", "crt-starfield-1", "aurora-glass-1", "system"]) {
      await page.goto(`${fixture.baseURL}/configuration`);
      await page.getByLabel("Theme", { exact: true }).selectOption(id);
      const saved = page.waitForResponse(response => response.url().endsWith("/configuration") && response.request().method() === "POST");
      await page.getByRole("button", { name: "Save configuration" }).click();
      expect((await saved).ok(), `save ${id}`).toBe(true);
      await page.goto(`${fixture.baseURL}/configuration`);
      if (id === "system") await expect(page.locator("html")).not.toHaveAttribute("data-theme");
      else {
        await expect(page.locator("html")).toHaveAttribute("data-theme", id);
        await expect(page.locator(`link[href="/ui/themes/${id}/theme.css"]`)).toHaveCount(1);
      }
      await page.getByRole("button", { name: "Menu", exact: true }).click();
      worst = Math.min(worst, await checkContrast(page, `persisted ${id}`));
    }

    await page.getByRole("button", { name: "Menu", exact: true }).click();
    await page.evaluate(() => window.scrollTo(0, 250));
    expect(await page.evaluate(() => scrollY)).toBeGreaterThan(0);
    await page.getByRole("button", { name: "Menu", exact: true }).click();
    await expect(page.locator(".menu-list")).toBeVisible();
    await page.getByRole("button", { name: "Sign out" }).click();
    await expect(page).toHaveURL(/\/(login|bookmarks)/);
    await page.goto(`${fixture.baseURL}/configuration`);
    await expect(page.getByRole("button", { name: "Sign out" })).toHaveCount(0);
    console.log(`Measured ${cases} cases: minimum rendered core-glyph contrast ${worst.toFixed(2)}:1; transmission ${(100 * minTransmission).toFixed(1)}–${(100 * maxTransmission).toFixed(1)}%`);
  } finally {
    await fixture.close();
  }
});
