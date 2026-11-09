import { execFileSync } from "node:child_process";
import { mkdirSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import { expect, test, type Page } from "@playwright/test";
import { createRestartFixture } from "./restart-fixture";

test.setTimeout(300_000);

async function theme(page: Page, id: string, scheme: "light" | "dark") {
  await page.emulateMedia({ colorScheme: scheme, reducedMotion: "reduce" });
  await page.evaluate(async id => {
    document.querySelector("#width-test-theme")?.remove();
    if (id === "system") document.documentElement.removeAttribute("data-theme");
    else {
      document.documentElement.dataset.theme = id;
      const link = document.createElement("link");
      link.id = "width-test-theme";
      link.rel = "stylesheet";
      link.href = `/ui/themes/${id}/theme.css`;
      const loaded = new Promise<void>((resolve, reject) => {
        link.onload = () => resolve();
        link.onerror = () => link.sheet?.cssRules.length
          ? resolve() : reject(new Error(`Unable to load ${link.href}`));
      });
      document.querySelector('link[href^="/assets/css/chroma-compat.css"]')!.before(link);
      await loaded;
    }
    await document.fonts.ready;
  }, id);
}

async function aligned(page: Page, left: number, width: number, context: string) {
  const form = page.locator("main#content > form.form-grid");
  await expect(form, context).toBeVisible();
  const box = (await form.boundingBox())!;
  expect(Math.abs(box.x - left), context).toBeLessThanOrEqual(1);
  expect(Math.abs(box.width - width), context).toBeLessThanOrEqual(1);
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= document.documentElement.clientWidth), context).toBe(true);
}

async function bookmarkTextContrast(page: Page) {
  const selector = ".bookmark-item:first-child :is(.bookmark-url, .description, time, .tags a)";
  const labels = await page.locator(selector).evaluateAll(elements => elements.map(element => {
    const range = document.createRange();
    range.selectNodeContents(element);
    const rect = range.getBoundingClientRect();
    return { name: element.textContent!.trim(), x: rect.x, y: rect.y, width: rect.width, height: rect.height, color: getComputedStyle(element).color };
  }));
  const rendered = (await page.screenshot({ animations: "disabled" })).toString("base64");
  const hidden = await page.addStyleTag({ content: `${selector} { color: transparent !important; -webkit-text-fill-color: transparent !important; text-shadow: none !important }` });
  const background = (await page.screenshot({ animations: "disabled" })).toString("base64");
  await hidden.evaluate(el => el.parentNode?.removeChild(el));
  return page.evaluate(async ({ rendered, background, labels }) => {
    async function pixels(src: string) {
      const image = new Image();
      image.src = `data:image/png;base64,${src}`;
      await image.decode();
      const canvas = document.createElement("canvas");
      canvas.width = image.width;
      canvas.height = image.height;
      const context = canvas.getContext("2d")!;
      context.drawImage(image, 0, 0);
      return { data: context.getImageData(0, 0, image.width, image.height).data, width: image.width, scale: image.width / innerWidth };
    }
    const a = await pixels(rendered), b = await pixels(background);
    const luminance = (rgb: number[]) => rgb.map(value => {
      value /= 255;
      return value <= .04045 ? value / 12.92 : ((value + .055) / 1.055) ** 2.4;
    }).reduce((sum, value, i) => sum + value * [0.2126, 0.7152, 0.0722][i], 0);
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

test("bookmark screenshots share the card width and show the top of the image", async ({ page, request }, testInfo) => {
  const fixture = await createRestartFixture();
  try {
    const config = readFileSync(fixture.configPath, "utf8");
    const port = config.match(/  port: (\d+)/)![1];
    const image = await page.evaluate(() => {
      const canvas = document.createElement("canvas");
      canvas.width = 400;
      canvas.height = 4000;
      const context = canvas.getContext("2d")!;
      context.fillStyle = "#ff0000";
      context.fillRect(0, 0, 400, 2000);
      context.fillStyle = "#0000ff";
      context.fillRect(0, 2000, 400, 2000);
      return canvas.toDataURL("image/png").split(",")[1];
    });
    const promoted = join(fixture.directory, "screenshots", "promoted");
    mkdirSync(promoted);
    const imagePath = join(promoted, "fixture.png");
    writeFileSync(imagePath, Buffer.from(image, "base64"));
    execFileSync("psql", ["-h", "127.0.0.1", "-p", port, "-U", "bookmarker", "-d", "bookmarker",
      "-v", "ON_ERROR_STOP=1", "-c", `
      INSERT INTO bookmarks (id, url, description, created_at, updated_at) VALUES
        ('with-shot', 'https://a.test/', 'Screenshot fixture', now(), now()),
        ('without-shot', 'https://a.test/', 'Screenshot fixture', now() - interval '1 minute', now());
      INSERT INTO screenshots (id, bookmark_id, storage_key, captured_url, captured_at, byte_size)
        VALUES ('shot-1', 'with-shot', 'promoted/fixture.png', 'https://a.test/', now(), 10);
      INSERT INTO tags (id, display_name, normalized_name) VALUES ('tag-1', 'Example', 'example');
      INSERT INTO bookmark_tags (bookmark_id, tag_id) VALUES ('with-shot', 'tag-1'), ('without-shot', 'tag-1');`],
    { env: { ...process.env, PGPASSWORD: "bookmarker-test" }, stdio: "ignore" });
    const response = await request.get(`${fixture.baseURL}/screenshots/shot-1`);
    expect(response.status()).toBe(200);
    expect(response.headers()["content-type"]).toBe("image/png");

    await page.route("https://**", route => route.request().resourceType() === "stylesheet"
      ? route.fulfill({ status: 200, contentType: "text/css", body: "" }) : route.abort());
    await page.goto(`${fixture.baseURL}/bookmarks`);
    const themes = JSON.parse(execFileSync("go", ["run", "./tests/e2e/catalog"], { encoding: "utf8" })) as { ID: string }[];
    const cases = [{ id: "system", scheme: "light" as const }, { id: "system", scheme: "dark" as const },
      ...themes.map(t => ({ id: t.ID, scheme: "light" as const }))];
    for (const { width, current } of [320, 1280].flatMap(width => cases.map(current => ({ width, current })))) {
      await page.setViewportSize({ width, height: 900 });
      await theme(page, current.id, current.scheme);
      const cards = page.locator(".bookmark-item");
      const preview = cards.first().locator("img.screenshot");
      await expect(preview).toBeVisible();
      await expect.poll(() => preview.evaluate((img: HTMLImageElement) => img.complete && img.naturalHeight)).toBe(4000);
      await expect(cards.nth(1).locator("img.screenshot")).toHaveCount(0);
      const withImage = (await cards.first().boundingBox())!;
      const imageBox = (await preview.boundingBox())!;
      const content = (await cards.first().locator(".bookmark-content").boundingBox())!;
      const frame = (await cards.first().locator(".bookmark-screenshot").boundingBox())!;
      const noImageContent = (await cards.nth(1).locator(".bookmark-content").boundingBox())!;
      expect(noImageContent.width, `${width} ${current.id} full-width content`).toBeGreaterThan(content.width * 1.8);
      await preview.evaluate(el => (el as HTMLElement).style.display = "none");
      expect(Math.abs((await cards.first().boundingBox())!.height - withImage.height), `${width} ${current.id} preview height`).toBeLessThanOrEqual(1);
      await preview.evaluate(el => (el as HTMLElement).style.removeProperty("display"));
      expect(Math.abs(content.width - frame.width), `${width} ${current.id} equal columns`).toBeLessThanOrEqual(1);
      expect(frame.x, `${width} ${current.id} image right of content`).toBeGreaterThanOrEqual(content.x + content.width);
      expect(Math.abs(frame.y - content.y), `${width} ${current.id} top alignment`).toBeLessThanOrEqual(1);
      expect(Math.abs(frame.height - content.height), `${width} ${current.id} constrained height`).toBeLessThanOrEqual(1);
      const tag = cards.first().getByRole("link", { name: "Example", exact: true });
      await expect(tag).toBeVisible();
      await expect(tag).toHaveAttribute("href", "/bookmarks?tag=example");
      const tagStyle = await tag.evaluate(el => {
        const style = getComputedStyle(el);
        return { border: parseFloat(style.borderTopWidth), radius: parseFloat(style.borderTopLeftRadius) };
      });
      expect(tagStyle.border, `${width} ${current.id} tag border`).toBeGreaterThan(0);
      expect(tagStyle.radius, `${width} ${current.id} tag pill`).toBeGreaterThan(10);
      expect(imageBox.x + imageBox.width).toBeLessThanOrEqual(withImage.x + withImage.width);
      expect(imageBox.y + imageBox.height).toBeLessThanOrEqual(withImage.y + withImage.height);
      expect(await preview.evaluate(el => getComputedStyle(el).objectPosition)).toBe("50% 0%");
      const screenshot = (await preview.screenshot()).toString("base64");
      const pixel = await page.evaluate(async data => {
        const img = new Image();
        img.src = `data:image/png;base64,${data}`;
        await img.decode();
        const canvas = document.createElement("canvas");
        canvas.width = img.width;
        canvas.height = img.height;
        const context = canvas.getContext("2d")!;
        context.drawImage(img, 0, 0);
        return Array.from(context.getImageData(Math.floor(img.width / 2), Math.floor(img.height / 2), 1, 1).data);
      }, screenshot);
      expect(pixel[0]).toBeGreaterThan(240);
      expect(pixel[2]).toBeLessThan(15);
      expect(await page.evaluate(() => document.documentElement.scrollWidth <= document.documentElement.clientWidth)).toBe(true);
      if (width === 1280 && ["system", "aurora-glass-1", "metal-1", "spaceship-2"].includes(current.id)) {
        const contrast = await bookmarkTextContrast(page);
        expect(contrast).toHaveLength(4);
        for (const label of contrast) {
          expect(label.count, `${current.id}/${current.scheme} ${label.name} painted pixels`).toBeGreaterThanOrEqual(3);
          expect(label.minimum, `${current.id}/${current.scheme} ${label.name} contrast`).toBeGreaterThanOrEqual(4.5);
        }
      }
      if (testInfo.project.name === "chromium" && width === 1280 && current.id === "system" && current.scheme === "dark") {
        const path = testInfo.outputPath("bookmark-card-dark.png");
        await cards.first().screenshot({ path, animations: "disabled" });
        console.log(`Card visual check: ${path}`);
      }
    }

    await page.setViewportSize({ width: 320, height: 900 });
    const longCard = page.locator(".bookmark-item").first();
    await longCard.locator(".bookmark-url").evaluate(el => { el.textContent = `https://a.test/${"x".repeat(240)}`; });
    await longCard.locator(".description").evaluate(el => { el.textContent = "description".repeat(40); });
    await longCard.locator(".tags a").evaluate(el => { el.textContent = "longtag".repeat(30); });
    const contentBox = (await longCard.locator(".bookmark-content").boundingBox())!;
    for (const selector of [".bookmark-url", ".description", ".tags a"]) {
      const box = (await longCard.locator(selector).boundingBox())!;
      expect(box.x + box.width, `${selector} wraps within the content column`).toBeLessThanOrEqual(contentBox.x + contentBox.width + 1);
    }
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= document.documentElement.clientWidth)).toBe(true);

    await page.goto(`${fixture.baseURL}/bookmarks`);
    const tagLink = page.locator(".bookmark-item").first().getByRole("link", { name: "Example", exact: true });
    await tagLink.focus();
    await expect(tagLink).toBeFocused();
    await Promise.all([
      page.waitForResponse(response => response.url().includes("tag=example") && response.request().headers()["hx-request"] === "true"),
      page.keyboard.press("Enter"),
    ]);
    await expect(page).toHaveURL(/tag=example/);
    await expect(page.locator(".bookmark-item")).toHaveCount(2);
    await page.goto(`${fixture.baseURL}/search`);
    await page.getByLabel("Words").fill("screenshot fixture");
    await Promise.all([
      page.waitForResponse(response => response.url().includes("/search?") && response.request().headers()["hx-request"] === "true"),
      page.getByRole("button", { name: "Search", exact: true }).click(),
    ]);
    await expect(page.locator(".bookmark-item").first().locator("img.screenshot")).toBeVisible();
    await page.goto(`${fixture.baseURL}/login`);
    await page.getByLabel("Username").fill("admin");
    await page.getByLabel("Password").fill("bookmarker-fixture-password");
    await page.getByRole("button", { name: "Sign in" }).click();
    await expect(page.locator(".bookmark-item").first().getByRole("link", { name: "Edit" })).toBeVisible();
    await expect(page.locator(".bookmark-item").first().locator("img.screenshot")).toBeVisible();
    for (const width of [320, 1280]) {
      await page.setViewportSize({ width, height: 900 });
      const card = page.locator(".bookmark-item").first();
      const screenshotBox = (await card.locator("img.screenshot").boundingBox())!;
      for (const name of ["Edit", "Delete"]) {
        const actionBox = (await card.getByRole("link", { name, exact: true }).boundingBox())!;
        expect(actionBox.x + actionBox.width).toBeLessThanOrEqual(screenshotBox.x);
      }
      expect(await page.evaluate(() => document.documentElement.scrollWidth <= document.documentElement.clientWidth)).toBe(true);
    }
    rmSync(imagePath);
    expect((await request.get(`${fixture.baseURL}/screenshots/shot-1`)).status()).toBe(404);
    await page.reload();
    await expect(page.locator(".bookmark-item").first().locator("img.screenshot")).toHaveCount(0);
  } finally {
    await fixture.close();
  }
});

test("bookmark action links use themed button styles", async ({ page }, testInfo) => {
  const fixture = await createRestartFixture();
  try {
    const config = readFileSync(fixture.configPath, "utf8");
    const port = config.match(/  port: (\d+)/)![1];
    execFileSync("psql", ["-h", "127.0.0.1", "-p", port, "-U", "bookmarker", "-d", "bookmarker",
      "-v", "ON_ERROR_STOP=1", "-c",
      "INSERT INTO bookmarks (id, url, description, created_at, updated_at) VALUES ('button-test', 'https://example.com/fixture', 'Button fixture bookmark', now(), now())"],
    { env: { ...process.env, PGPASSWORD: "bookmarker-test" }, stdio: "ignore" });
    await page.route("https://**", route => route.request().resourceType() === "stylesheet"
      ? route.fulfill({ status: 200, contentType: "text/css", body: "" }) : route.abort());
    await page.goto(`${fixture.baseURL}/bookmarks`);
    await expect(page.locator(".bookmark-actions")).toHaveCount(0);
    await page.goto(`${fixture.baseURL}/login`);
    await page.getByLabel("Username").fill("admin");
    await page.getByLabel("Password").fill("bookmarker-fixture-password");
    await page.getByRole("button", { name: "Sign in" }).click();
    await expect(page).toHaveURL(/\/bookmarks$/);
    await page.addStyleTag({ content: "*, *::before, *::after, :root[data-theme] a.btn, :root[data-theme] button.btn { animation: none !important; transition: none !important }" });

    const themes = JSON.parse(execFileSync("go", ["run", "./tests/e2e/catalog"], { encoding: "utf8" })) as { ID: string }[];
    const cases = [{ id: "system", scheme: "light" as const }, { id: "system", scheme: "dark" as const },
      ...themes.map(t => ({ id: t.ID, scheme: "light" as const }))];
    for (const viewport of [320, 1280]) {
      await page.setViewportSize({ width: viewport, height: 900 });
      for (const current of cases) {
        const context = `${testInfo.project.name} ${viewport} ${current.id}/${current.scheme}`;
        await theme(page, current.id, current.scheme);
        const actions = page.locator(".bookmark-item").first().locator(".bookmark-actions");
        for (const [label, variant, path] of [["Edit", "secondary", "edit"], ["Delete", "danger", "delete"]]) {
          const link = actions.getByRole("link", { name: label, exact: true });
          await expect(link, context).toBeVisible();
          await expect(link, context).toHaveAttribute("href", `/bookmarks/button-test/${path}`);
          const styles = await link.evaluate((el, variant) => {
            const reference = document.createElement("button");
            reference.className = `btn btn-${variant}`;
            el.parentElement!.appendChild(reference);
            const properties = ["color", "backgroundImage", "backgroundColor", "borderColor", "borderRadius"] as const;
            const actual = getComputedStyle(el), expected = getComputedStyle(reference);
            const result = properties.map(property => [property, actual[property], expected[property]]);
            reference.remove();
            return result;
          }, variant);
          for (const [property, actual, expected] of styles) {
            expect(actual, `${context} ${label} ${property}`).toBe(expected);
          }
          if (current.id === "spaceship-2") {
            await link.hover();
            const hover = await link.evaluate(el => {
              const style = getComputedStyle(el);
              return [style.color, style.backgroundColor, style.borderColor];
            });
            await link.evaluate((el, variant) => {
              const reference = document.createElement("button");
              reference.className = `btn btn-${variant}`;
              reference.id = "button-style-reference";
              reference.textContent = "Reference";
              el.parentElement!.appendChild(reference);
            }, variant);
            const reference = page.locator("#button-style-reference");
            await reference.hover();
            const referenceHover = await reference.evaluate(el => {
              const style = getComputedStyle(el);
              return [style.color, style.backgroundColor, style.borderColor];
            });
            expect(hover, `${context} ${label} hover`).toEqual(referenceHover);
            await reference.evaluate(el => el.remove());
            await page.mouse.move(0, 0);
          }
        }
        expect(await page.evaluate(() => document.documentElement.scrollWidth <= document.documentElement.clientWidth), context).toBe(true);
      }
    }
    await page.getByRole("link", { name: "Edit", exact: true }).click();
    await expect(page.getByRole("heading", { name: "Edit bookmark" })).toBeVisible();
    await page.goto(`${fixture.baseURL}/search?q=Button+fixture`);
    await expect(page.getByRole("link", { name: "Edit", exact: true })).toHaveClass(/\bbtn-secondary\b/);
    await page.getByRole("link", { name: "Delete", exact: true }).click();
    await expect(page.getByRole("heading", { name: "Delete bookmark" })).toBeVisible();
  } finally {
    await fixture.close();
  }
});

test("page forms align with list cards and preserve interactions", async ({ page }, testInfo) => {
  // Do not pull images or use the shared E2E setup.
  execFileSync("docker", ["image", "inspect", "postgres:18.6"], { stdio: "ignore" });
  const fixture = await createRestartFixture();
  try {
    const config = readFileSync(fixture.configPath, "utf8");
    const port = config.match(/  port: (\d+)/)![1];
    execFileSync("psql", ["-h", "127.0.0.1", "-p", port, "-U", "bookmarker", "-d", "bookmarker",
      "-v", "ON_ERROR_STOP=1", "-c",
      "INSERT INTO bookmarks (id, url, description, created_at, updated_at) VALUES ('width-test', 'https://example.com/fixture', 'Width fixture bookmark', now(), now())"],
    { env: { ...process.env, PGPASSWORD: "bookmarker-test" }, stdio: "ignore" });
    await page.route("https://**", route => route.abort());
    const cases = [
      { id: "system", scheme: "light" as const },
      { id: "system", scheme: "dark" as const },
      { id: "light", scheme: "light" as const },
      { id: "dark", scheme: "dark" as const },
      { id: "aurora-glass-1", scheme: "dark" as const },
    ];
    const sizes = [320, 390, 480, 481, 1514];
    for (const viewport of sizes) {
      await page.setViewportSize({ width: viewport, height: 900 });
      for (const current of cases) {
        const context = `${testInfo.project.name} ${viewport} ${current.id}/${current.scheme}`;
        await page.goto(`${fixture.baseURL}/bookmarks`);
        await theme(page, current.id, current.scheme);
        const card = (await page.locator(".bookmark-item").first().boundingBox())!;
        // Login is tested logged out, before the authenticated pages.
        await page.goto(`${fixture.baseURL}/login`);
        await theme(page, current.id, current.scheme);
        await aligned(page, card.x, card.width, `${context} login`);
        await page.getByLabel("Username").fill("admin");
        await page.getByLabel("Password").fill("bookmarker-fixture-password");
        await page.getByRole("button", { name: "Sign in" }).click();
        await expect(page).toHaveURL(/\/bookmarks$/);
        for (const [path, selector] of [
          ["/search?q=fixture", "#search-q"],
          ["/bookmarks/new", "#add-url"],
          ["/bookmarks/width-test/edit", "#edit-url"],
          ["/configuration", "#page-title"],
          ["/bookmarks/width-test/delete", 'button[type="submit"]'],
        ]) {
          await page.goto(`${fixture.baseURL}${path}`);
          await theme(page, current.id, current.scheme);
          await aligned(page, card.x, card.width, `${context} ${path}`);
          const field = (await page.locator(`main ${selector}`).first().boundingBox())!;
          expect(Math.abs(field.width - card.width), `${context} ${selector}`).toBeLessThanOrEqual(1);
          if (path.startsWith("/search")) {
            const result = (await page.locator(".bookmark-item").first().boundingBox())!;
            expect(Math.abs(result.width - card.width), context).toBeLessThanOrEqual(1);
          }
          if (path === "/configuration") {
            for (const selector of ["#theme", "#database-port"]) {
              const control = (await page.locator(selector).boundingBox())!;
              expect(Math.abs(control.width - card.width), `${context} ${selector}`).toBeLessThanOrEqual(1);
            }
          }
        }
        await page.getByRole("button", { name: "Menu", exact: true }).click();
        const menu = (await page.locator(".menu-list").boundingBox())!;
        const logout = (await page.locator('.menu-list form[action="/logout"]').boundingBox())!;
        expect(logout.x).toBeGreaterThanOrEqual(menu.x);
        expect(logout.x + logout.width).toBeLessThanOrEqual(menu.x + menu.width);
        expect(menu.x + menu.width).toBeLessThanOrEqual(viewport);
        await page.keyboard.press("Escape");
        await expect(page.getByRole("button", { name: "Menu", exact: true })).toBeFocused();
        await page.getByRole("button", { name: "Menu", exact: true }).click();
        await page.getByRole("button", { name: "Sign out" }).click();
        await expect(page).toHaveURL(/\/bookmarks$/);
        await page.getByRole("button", { name: "Menu", exact: true }).click();
        await expect(page.getByRole("link", { name: "Sign in", exact: true })).toBeVisible();
      }
    }

    await page.setViewportSize({ width: 320, height: 800 });
    await page.goto(`${fixture.baseURL}/bookmarks/new`);
    // An unauthenticated direct request must not expose the add form.
    await expect(page.locator("#add-url")).toHaveCount(0);
    await page.goto(`${fixture.baseURL}/login`);
    await page.getByLabel("Username").fill("admin");
    await page.getByLabel("Password").fill("bookmarker-fixture-password");
    await page.getByRole("button", { name: "Sign in" }).click();
    await expect(page).toHaveURL(/\/bookmarks$/);
    await page.getByRole("button", { name: "Menu", exact: true }).click();
    await page.getByRole("link", { name: "Add", exact: true }).click();
    await expect(page.locator("#add-url")).toBeVisible();
    const mainWidth = await page.locator("main").evaluate(el => {
      const s = getComputedStyle(el);
      return el.getBoundingClientRect().width - parseFloat(s.paddingLeft) - parseFloat(s.paddingRight);
    });
    expect(Math.abs((await page.locator("main form").boundingBox())!.width - mainWidth)).toBeLessThanOrEqual(1);
    expect(await page.locator("#add-url").evaluate((el: HTMLInputElement) => el.validity.valueMissing)).toBe(true);
    expect(await page.locator("#add-description").evaluate((el: HTMLInputElement) => el.validity.valueMissing)).toBe(true);
    await page.locator("#add-tag-input").fill("x".repeat(280));
    await page.getByRole("button", { name: "Add tag" }).click();
    await expect(page.locator("[data-tag-list] li")).toHaveCount(1);
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= document.documentElement.clientWidth)).toBe(true);
    await page.getByRole("button", { name: /^Remove tag/ }).click();
    await expect(page.locator("[data-tag-list] li")).toHaveCount(0);
    expect((await page.locator('input[type="checkbox"]').boundingBox())!.width).toBeLessThan(40);
    // Populate long values without firing the capture trigger (no external
    // capture target or network dependency), then submit the real form.
    await page.locator("#add-url").evaluate((el: HTMLInputElement) => {
      el.value = "https://example.com/" + "u".repeat(280);
    });
    await page.locator("#add-description").fill("Long description " + "d".repeat(280));
    await page.locator('input[name="save_without_screenshot"]').check();
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= document.documentElement.clientWidth)).toBe(true);
    await page.getByRole("button", { name: "Save bookmark" }).click();
    await expect(page.locator(".bookmark-item", { hasText: "Long description" })).toBeVisible();
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= document.documentElement.clientWidth)).toBe(true);

    await page.goto(`${fixture.baseURL}/search`);
    await page.locator("#search-q").fill("fixture");
    await page.getByRole("button", { name: "Search", exact: true }).click();
    await expect(page.locator(".bookmark-item")).toHaveCount(1);
    const searchForm = (await page.locator("main form").boundingBox())!;
    const searchCard = (await page.locator(".bookmark-item").boundingBox())!;
    expect(Math.abs(searchForm.width - searchCard.width)).toBeLessThanOrEqual(1);

    await page.goto(`${fixture.baseURL}/bookmarks/width-test/edit`);
    await page.locator("#edit-description").fill("e".repeat(280));
    await page.getByRole("button", { name: "Save bookmark" }).click();
    await expect(page.locator(".bookmark-item", { hasText: "e".repeat(280) })).toBeVisible();

    await page.goto(`${fixture.baseURL}/configuration`);
    await page.getByLabel("Page title").fill(" ");
    const response = page.waitForResponse(r => r.url().endsWith("/configuration") && r.request().method() === "POST");
    await page.getByRole("button", { name: "Save configuration" }).click();
    expect((await response).status()).toBe(422);
    await expect(page.getByRole("alert")).toContainText("page_title");
    const content = await page.locator("main").evaluate(el => {
      const rect = el.getBoundingClientRect();
      const s = getComputedStyle(el);
      return { x: rect.x + parseFloat(s.paddingLeft), width: rect.width - parseFloat(s.paddingLeft) - parseFloat(s.paddingRight) };
    });
    await aligned(page, content.x, content.width, "configuration validation swap");

    await page.goto(`${fixture.baseURL}/bookmarks/width-test/delete`);
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= document.documentElement.clientWidth)).toBe(true);
    await page.getByRole("button", { name: "Delete", exact: true }).click();
    await expect(page.locator(".bookmark-item", { hasText: "e".repeat(280) })).toHaveCount(0);
    await expect(page.locator(".bookmark-item", { hasText: "Long description" })).toBeVisible();
  } finally {
    await fixture.close();
  }
});
