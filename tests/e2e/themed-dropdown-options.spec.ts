import { execFileSync } from "node:child_process";
import { readFileSync, readdirSync } from "node:fs";
import { extname, join, resolve, sep } from "node:path";
import { expect, test, type BrowserContext, type Page } from "@playwright/test";

// In-memory HTTP fixture: actual pinned Chroma and app assets, representative
// configuration markup. It does not exercise Go templates, authentication,
// persistence, CSRF, or the real server's validation.
const origin = "http://picker.test";
const root = process.cwd();
const chroma = join(execFileSync("go", ["list", "-m", "-f", "{{.Dir}}", "github.com/githubixx/chroma-ui"], {
  cwd: root, encoding: "utf8", env: { ...process.env, GOPROXY: "off", GOSUMDB: "off" },
}).trim(), "assets");
const themes = readdirSync(join(chroma, "themes"), { withFileTypes: true })
  .filter(entry => entry.isDirectory()).map(entry => entry.name).sort();

function form(unavailable = false) {
  return `<h1>Configuration</h1>
    ${unavailable ? '<p id="configuration-error" role="alert">Choose an available theme</p>' : ""}
    <form class="form-grid" method="post" action="/configuration" hx-post="/configuration" hx-target="#content">
      <input type="hidden" name="csrf_token" value="fixture">
      <label for="theme">Theme</label>
      <select id="theme" name="theme" required ${unavailable ? 'aria-invalid="true" aria-describedby="configuration-error"' : ""}>
        ${unavailable ? '<option value="missing" selected disabled>Missing (unavailable — choose a replacement)</option>' : ""}
        <option value="system" ${unavailable ? "" : "selected"}>System</option>
        ${themes.map(id => `<option value="${id}">${id}</option>`).join("")}
      </select>
      <label for="default-view">Default view</label>
      <select id="default-view" name="default_view"><option value="list" selected>List</option><option value="add">Add</option><option value="search">Search</option><option value="configuration">Configuration</option></select>
      <label for="database-tls-mode">Database TLS mode</label>
      <select id="database-tls-mode" name="database_tls_mode"><option value="disable" selected>Disable</option><option value="require">Require</option><option value="verify-ca">Verify CA</option><option value="verify-full">Verify full</option></select>
      <button type="submit">Save configuration</button><button type="reset">Reset</button>
    </form>`;
}

function documentHTML(id = "system", unavailable = false, enhanced = false) {
  const attr = id === "system" ? "" : ` data-theme="${id}"`;
  const theme = id === "system" ? "" : `<link rel="stylesheet" href="/ui/themes/${id}/theme.css">`;
  return `<!doctype html><html lang="en"${attr}><head><meta charset="utf-8"><meta name="viewport" content="width=device-width">
    <link rel="stylesheet" href="/ui/css/base.css"><link rel="stylesheet" href="/assets/css/bookmarker.css">
    ${theme}<link rel="stylesheet" href="/assets/css/chroma-compat.css">
    <script defer src="/assets/vendor/htmx.min.js"></script><script defer src="/assets/js/configuration.js"></script>
    ${enhanced ? '<script defer src="/assets/js/themed-select.js"></script>' : ""}
    </head><body><main id="content">${form(unavailable)}</main></body></html>`;
}

async function fixture(context: BrowserContext) {
  await context.route("**/*", async route => {
    const request = route.request(), url = new URL(request.url());
    if (url.origin !== origin) return route.abort();
    if (url.pathname === "/configuration" && request.method() === "POST") {
      return route.fulfill({ status: 422, contentType: "text/html", body: form(true) });
    }
    if (url.pathname === "/configuration") {
      const id = url.searchParams.get("theme") || "system";
      if (id !== "system" && !themes.includes(id)) return route.abort();
      return route.fulfill({ contentType: "text/html", body: documentHTML(id, url.searchParams.has("unavailable"), url.searchParams.has("enhanced")) });
    }
    const isChroma = url.pathname.startsWith("/ui/");
    const base = isChroma ? chroma : join(root, "web/static");
    const relative = url.pathname.replace(isChroma ? /^\/ui\// : /^\/assets\//, "");
    const file = resolve(base, relative);
    if (!file.startsWith(base + sep)) return route.abort();
    try {
      const contentType = ({ ".css": "text/css", ".js": "text/javascript", ".webp": "image/webp", ".avif": "image/avif", ".svg": "image/svg+xml" } as Record<string, string>)[extname(file)] || "application/octet-stream";
      return route.fulfill({ status: 200, contentType, body: readFileSync(file) });
    } catch { return route.abort(); }
  });
}

async function open(page: Page, selector: string) {
  const select = page.locator(selector);
  await select.click();
  await expect.poll(() => select.evaluate(el => el.matches(":open"))).toBe(true);
}

// Compare the opened picker to the same rendered scene with option ink hidden.
// Samples are actual fully-painted glyph pixels against their composited backing,
// not the theme token or a transparent computed background.
async function contrast(page: Page, optionSelector: string, inkSelector = "select:open option") {
  const info = await page.locator(optionSelector).evaluate(el => {
    const r = el.getBoundingClientRect();
    return { x: r.x, y: r.y, width: r.width, height: r.height, color: getComputedStyle(el).color };
  });
  const ink = (await page.screenshot({ animations: "disabled" })).toString("base64");
  await page.addStyleTag({ content: `${inkSelector} { color: transparent !important; -webkit-text-fill-color: transparent !important; text-shadow: none !important }` });
  const background = (await page.screenshot({ animations: "disabled" })).toString("base64");
  await page.locator("style").last().evaluate(el => el.remove());
  return page.evaluate(async ({ ink, background, info }) => {
    async function pixels(data: string) {
      const image = new Image(); image.src = `data:image/png;base64,${data}`; await image.decode();
      const canvas = document.createElement("canvas"); canvas.width = image.width; canvas.height = image.height;
      const ctx = canvas.getContext("2d")!; ctx.drawImage(image, 0, 0);
      return { data: ctx.getImageData(0, 0, image.width, image.height).data, width: image.width, height: image.height, scale: image.width / innerWidth };
    }
    const a = await pixels(ink), b = await pixels(background);
    const expected = info.color.match(/[\d.]+/g)?.slice(0, 3).map(Number) || [];
    const luminance = (rgb: number[]) => rgb.map(c => {
      c /= 255; return c <= .04045 ? c / 12.92 : ((c + .055) / 1.055) ** 2.4;
    }).reduce((sum, c, i) => sum + c * [0.2126, 0.7152, 0.0722][i], 0);
    let minimum = Infinity, count = 0;
    for (let y = Math.max(0, Math.floor(info.y * a.scale)); y < Math.min(a.height, Math.ceil((info.y + info.height) * a.scale)); y++) {
      for (let x = Math.max(0, Math.floor(info.x * a.scale)); x < Math.min(a.width, Math.ceil((info.x + info.width) * a.scale)); x++) {
        const i = (y * a.width + x) * 4;
        const fg = Array.from(a.data.slice(i, i + 3)), bg = Array.from(b.data.slice(i, i + 3));
        if (expected.length !== 3 || Math.max(...fg.map((v, j) => Math.abs(v - expected[j]))) > 20 ||
            Math.max(...fg.map((v, j) => Math.abs(v - bg[j]))) < 20) continue;
        const l1 = luminance(fg), l2 = luminance(bg);
        minimum = Math.min(minimum, (Math.max(l1, l2) + .05) / (Math.min(l1, l2) + .05)); count++;
      }
    }
    return { count, minimum: count ? minimum : 0, ...info };
  }, { ink, background, info });
}

test.beforeEach(async ({ context }) => { await fixture(context); });

const pickerFields = [
  ["theme", "Theme"], ["default-view", "Default view"], ["database-tls-mode", "Database TLS mode"],
] as const;

async function expectNativeFallback(page: Page) {
  await expect(page.locator(".themed-select")).toHaveCount(0);
  await expect(page.getByRole("combobox")).toHaveCount(3);
  for (const [id, label] of pickerFields) {
    const native = page.locator(`#${id}`);
    await expect(native).toBeVisible();
    await expect(native).toBeEnabled();
    await expect(native).not.toHaveAttribute("aria-hidden", "true");
    await expect(native).not.toHaveAttribute("tabindex", "-1");
    await expect(page.getByRole("combobox", { name: label, exact: true })).toHaveAttribute("id", id);
    const state = await native.evaluate(el => {
      const style = getComputedStyle(el);
      return { opacity: Number(style.opacity), pointerEvents: style.pointerEvents };
    });
    expect(state.opacity, `${id} must paint`).toBeGreaterThan(0);
    expect(state.pointerEvents, `${id} must accept pointer input`).not.toBe("none");
    expect(await page.locator(`label[for="${id}"]`).evaluate((el: HTMLLabelElement, expected) =>
      el.control?.id === expected, id)).toBe(true);
  }
}

async function expectEnhancedDropdowns(page: Page, description: string) {
  await expect(page.locator("form select.themed-select-native")).toHaveCount(3);
  await expect(page.locator(".themed-select")).toHaveCount(3);
  await expect(page.getByRole("combobox")).toHaveCount(3);
  const states = await page.locator("form select").evaluateAll(elements => elements.map(element => {
    const select = element as HTMLSelectElement;
    const wrapper = select.parentElement!;
    const button = wrapper.querySelector<HTMLButtonElement>(":scope > .themed-select-trigger")!;
    const label = document.querySelector<HTMLLabelElement>(`label[for="${select.id}-trigger"]`);
    const nativeStyle = getComputedStyle(select);
    const buttonStyle = getComputedStyle(button);
    return {
      id: select.id,
      nativePosition: nativeStyle.position,
      nativeOpacity: Number(nativeStyle.opacity),
      nativePointerEvents: nativeStyle.pointerEvents,
      nativeDisplay: nativeStyle.display,
      nativeHidden: select.getAttribute("aria-hidden"),
      nativeTabindex: select.getAttribute("tabindex"),
      buttonVisibility: buttonStyle.visibility,
      buttonWidth: button.getBoundingClientRect().width,
      buttonHeight: button.getBoundingClientRect().height,
      wrapperHeight: wrapper.getBoundingClientRect().height,
      labelTargetsButton: label?.control === button,
    };
  }));
  expect(states.map(state => state.id), description).toEqual(pickerFields.map(([id]) => id));
  for (const state of states) {
    const context = `${description} ${state.id}`;
    expect(state.nativePosition, context).toBe("absolute");
    expect(state.nativeOpacity, context).toBe(0);
    expect(state.nativePointerEvents, context).toBe("none");
    expect(state.nativeDisplay, context).not.toBe("none");
    expect(state.nativeHidden, context).toBe("true");
    expect(state.nativeTabindex, context).toBe("-1");
    expect(state.buttonVisibility, context).toBe("visible");
    expect(state.buttonWidth, context).toBeGreaterThan(0);
    expect(state.buttonHeight, context).toBeGreaterThan(0);
    expect(Math.abs(state.wrapperHeight - state.buttonHeight), `${context}: extra layout row`).toBeLessThanOrEqual(1);
    expect(state.labelTargetsButton, context).toBe(true);
  }
}

test("missing native-hiding rule falls back to usable native dropdowns", async ({ page }) => {
  const css = readFileSync(join(root, "web/static/css/chroma-compat.css"), "utf8");
  const withoutRule = css.replace(/^\s*\.form-grid \.themed-select-native\s*\{[^}]*\}/m, "");
  expect(withoutRule, "the fixture must remove the actual native-hiding rule").not.toBe(css);
  await page.route("**/assets/css/chroma-compat.css*", route =>
    route.fulfill({ contentType: "text/css", body: withoutRule }));
  await page.goto(`${origin}/configuration?enhanced&theme=spaceship-red-dark`);

  expect(await page.evaluate(() =>
    CSS.supports("color", "light-dark(black, white)") &&
    CSS.supports("color", "color-mix(in srgb, red, white)") &&
    matchMedia("(hover: hover) and (pointer: fine) and (forced-colors: none)").matches
  )).toBe(true);
  await expectNativeFallback(page);
  expect(await page.locator("form select").evaluateAll(elements => elements.every(el => !el.hasAttribute("class"))))
    .toBe(true);
  await page.locator('label[for="default-view"]').click();
  const view = page.locator("#default-view");
  await expect(view).toBeFocused();
  await view.evaluate(el => {
    const counts = { input: 0, change: 0 };
    el.addEventListener("input", () => counts.input++);
    el.addEventListener("change", () => counts.change++);
    (el as HTMLSelectElement & { counts: typeof counts }).counts = counts;
  });
  if (await view.evaluate(el => getComputedStyle(el).appearance === "base-select")) {
    if (!await view.evaluate(el => el.matches(":open"))) await page.keyboard.press("Space");
    await expect.poll(() => view.evaluate(el => el.matches(":open"))).toBe(true);
  }
  await page.keyboard.press("Home");
  await page.keyboard.press("ArrowDown");
  await page.keyboard.press("Enter");
  await expect(view).toHaveValue("add");
  expect(await view.evaluate(el => (el as HTMLSelectElement & { counts: { input: number; change: number } }).counts))
    .toEqual({ input: 1, change: 1 });
  expect(await page.locator("form").evaluate((form: HTMLFormElement) => Object.fromEntries(new FormData(form))))
    .toMatchObject({ theme: "system", default_view: "add", database_tls_mode: "disable" });

  const theme = page.locator("#theme");
  await theme.evaluate((el: HTMLSelectElement) => { el.selectedIndex = -1; });
  await page.getByRole("button", { name: "Save configuration" }).click();
  await expect(theme).toBeFocused();
  expect(await theme.evaluate((el: HTMLSelectElement) => el.validity.valueMissing)).toBe(true);
  await theme.selectOption("system");
  await page.getByRole("button", { name: "Reset" }).click();
  await expect(view).toHaveValue("list");

  for (let attempt = 0; attempt < 2; attempt++) {
    const response = page.waitForResponse(r => r.url() === `${origin}/configuration` && r.request().method() === "POST");
    await page.getByRole("button", { name: "Save configuration" }).click();
    expect((await response).status()).toBe(422);
    await expect(page.getByRole("alert")).toHaveText("Choose an available theme");
    await expectNativeFallback(page);
    await expect(page.locator("#theme")).toHaveAttribute("aria-describedby", "configuration-error");
    await page.locator("#theme").selectOption("system");
  }
  await page.evaluate(() => {
    document.dispatchEvent(new Event("htmx:afterSwap"));
    document.dispatchEvent(new Event("htmx:afterSettle"));
    document.dispatchEvent(new Event("htmx:afterSettle"));
  });
  await expectNativeFallback(page);
});

for (const [description, override] of [
  ["the compatibility stylesheet is unavailable", null],
  ["the native select still consumes a layout row", ".form-grid .themed-select-native { position: static !important; }"],
  ["the native select can still paint", ".form-grid .themed-select-native { opacity: 1 !important; }"],
  ["the native select can intercept pointer input", ".form-grid .themed-select-native { pointer-events: auto !important; }"],
  ["the native select is removed with display none", ".form-grid .themed-select-native { display: none !important; }"],
  ["the replacement trigger is invisible", ".form-grid .themed-select-trigger { visibility: hidden !important; }"],
] as const) {
  test(`fallback when ${description}`, async ({ page }) => {
    const css = readFileSync(join(root, "web/static/css/chroma-compat.css"), "utf8");
    await page.route("**/assets/css/chroma-compat.css*", route => override === null
      ? route.abort("failed")
      : route.fulfill({ contentType: "text/css", body: `${css}\n${override}` }));
    await page.goto(`${origin}/configuration?enhanced&theme=spaceship-red-dark`);
    await expectNativeFallback(page);
  });
}

test("enhanced controls occupy one row across System and every bundled theme", async ({ page }) => {
  test.setTimeout(180_000);
  const cases: [string, "light" | "dark"][] = [
    ["system", "light"], ["system", "dark"], ...themes.map(id => [id, "light"] as [string, "light"]),
  ];
  expect(themes).toContain("spaceship-red-dark");
  for (const [id, scheme] of cases) {
    await page.emulateMedia({ colorScheme: scheme, reducedMotion: "reduce" });
    await page.goto(`${origin}/configuration?enhanced&theme=${id}`);
    await expectEnhancedDropdowns(page, `${id}/${scheme}`);
  }
});

test("Spaceship red dark has no native paint anywhere in the whole form", async ({ page }) => {
  await page.emulateMedia({ reducedMotion: "reduce" });
  await page.goto(`${origin}/configuration?enhanced&theme=spaceship-red-dark`);
  await expectEnhancedDropdowns(page, "spaceship-red-dark");
  const form = page.locator("form");
  const before = await form.screenshot({ animations: "disabled" });
  const hideNative = await page.addStyleTag({ content: ".themed-select-native { visibility: hidden !important }" });
  const after = await form.screenshot({ animations: "disabled" });
  await hideNative.evaluate(el => (el as Element).remove());
  expect(before.equals(after), "native select must not paint above or over the custom trigger").toBe(true);
});

test("stylesheet loss and restoration preserve focus and never leave duplicate controls", async ({ page }) => {
  await page.goto(`${origin}/configuration?enhanced`);
  const trigger = page.getByRole("combobox", { name: "Default view", exact: true });
  const native = page.locator("#default-view");
  await trigger.focus();
  await expect(trigger).toBeFocused();
  await page.locator('link[href^="/assets/css/chroma-compat.css"]').evaluate(el => el.remove());
  await expectNativeFallback(page);
  await expect(native).toBeFocused();

  await page.evaluate(() => {
    const link = document.createElement("link");
    link.rel = "stylesheet";
    link.href = "/assets/css/chroma-compat.css";
    document.head.append(link);
  });
  await expectEnhancedDropdowns(page, "restored stylesheet");
  await expect(trigger).toBeFocused();

  // CSSOM changes do not generate DOM mutations; removing this rule also
  // changes layout, which must trigger the size-based safety check.
  const removed = await page.evaluate(() => {
    const link = document.querySelector<HTMLLinkElement>('link[href^="/assets/css/chroma-compat.css"]')!;
    const sheet = link.sheet as CSSStyleSheet;
    for (const rule of Array.from(sheet.cssRules)) {
      if (!("cssRules" in rule)) continue;
      const group = rule as CSSGroupingRule;
      for (let index = 0; index < group.cssRules.length; index++) {
        const inner = group.cssRules[index] as CSSStyleRule;
        if (inner.selectorText === ".form-grid .themed-select-native") {
          group.deleteRule(index);
          return true;
        }
      }
    }
    return false;
  });
  expect(removed).toBe(true);
  await expectNativeFallback(page);
  await expect(native).toBeFocused();
});

test("enhancement preserves native form behavior and repeated HTMX recovery", async ({ page }) => {
  await page.goto(`${origin}/configuration?enhanced&theme=spaceship-red-dark`);
  // Loading the script twice and refreshing repeatedly must not wrap a select
  // twice or add extra event handlers.
  await page.addScriptTag({ url: "/assets/js/themed-select.js" });
  await page.evaluate(() => {
    document.dispatchEvent(new Event("htmx:afterSwap"));
    document.dispatchEvent(new Event("htmx:afterSettle"));
    document.dispatchEvent(new Event("htmx:afterSettle"));
  });
  await expectEnhancedDropdowns(page, "repeated initialization");
  await page.evaluate(() => {
    const snapshots: unknown[] = [];
    (window as unknown as { pickerSwapSnapshots: unknown[] }).pickerSwapSnapshots = snapshots;
    for (const phase of ["htmx:afterSwap", "htmx:afterSettle"]) {
      document.addEventListener(phase, () => {
        const selects = Array.from(document.querySelectorAll<HTMLSelectElement>("form select"));
        snapshots.push({
          phase,
          triggers: document.querySelectorAll(".themed-select-trigger").length,
          controls: selects.map(select => {
            const style = getComputedStyle(select);
            const target = phase === "htmx:afterSwap" ? select : select.parentElement!.querySelector("button");
            const label = document.querySelector<HTMLLabelElement>(`label[for="${target?.id}"]`);
            return {
              position: style.position, opacity: Number(style.opacity),
              hidden: select.getAttribute("aria-hidden"), labelTargetsControl: label?.control === target,
            };
          }),
        });
      });
    }
  });

  const nativeView = page.locator("#default-view");
  const view = page.getByRole("combobox", { name: "Default view", exact: true });
  await nativeView.evaluate(el => {
    const counts = { input: 0, change: 0 };
    el.addEventListener("input", () => counts.input++);
    el.addEventListener("change", () => counts.change++);
    (el as HTMLSelectElement & { counts: typeof counts }).counts = counts;
  });
  await view.click();
  await page.getByRole("option", { name: "Add", exact: true }).click();
  await expect(nativeView).toHaveValue("add");
  await view.click();
  await page.getByRole("option", { name: "Add", exact: true }).click();
  expect(await nativeView.evaluate(el => (el as HTMLSelectElement & { counts: { input: number; change: number } }).counts))
    .toEqual({ input: 1, change: 1 });
  expect(await page.locator("form").evaluate((form: HTMLFormElement) => Object.fromEntries(new FormData(form))))
    .toMatchObject({ csrf_token: "fixture", theme: "system", default_view: "add", database_tls_mode: "disable" });
  await page.getByRole("button", { name: "Reset" }).click();
  await expect(nativeView).toHaveValue("list");
  await expect(view).toHaveText("List");
  expect(await nativeView.evaluate(el => (el as HTMLSelectElement & { counts: { input: number; change: number } }).counts))
    .toEqual({ input: 1, change: 1 });

  const nativeTheme = page.locator("#theme");
  const theme = page.getByRole("combobox", { name: "Theme", exact: true });
  await nativeTheme.evaluate((el: HTMLSelectElement) => {
    el.selectedIndex = -1;
    el.dispatchEvent(new Event("change", { bubbles: true }));
  });
  await page.getByRole("button", { name: "Save configuration" }).click();
  await expect(theme).toBeFocused();
  await expect(theme).toHaveAttribute("aria-invalid", "true");
  await expect(theme).toHaveAttribute("aria-describedby", /theme-native-error/);
  await expect(page.locator("#theme-native-error")).toBeVisible();
  await theme.click();
  await page.getByRole("option", { name: "System", exact: true }).click();
  await expect(nativeTheme).toHaveValue("system");
  await expect(page.locator("#theme-native-error")).toBeHidden();

  for (let attempt = 0; attempt < 2; attempt++) {
    const response = page.waitForResponse(r => r.url() === `${origin}/configuration` && r.request().method() === "POST");
    await page.getByRole("button", { name: "Save configuration" }).click();
    expect((await response).status()).toBe(422);
    await expect(page.locator("#configuration-error")).toHaveText("Choose an available theme");
    await expectEnhancedDropdowns(page, `422 replacement ${attempt + 1}`);
    await expect(theme).toHaveAttribute("aria-invalid", "true");
    await expect(theme).toHaveAttribute("aria-describedby", "configuration-error");
    await theme.click();
    await expect(page.getByRole("option", { name: /unavailable/ })).toHaveAttribute("aria-disabled", "true");
    await page.getByRole("option", { name: "System", exact: true }).click();
  }
  const snapshots = await page.evaluate(() =>
    (window as unknown as { pickerSwapSnapshots: { phase: string; triggers: number; controls: unknown[] }[] }).pickerSwapSnapshots);
  expect(snapshots).toHaveLength(4);
  for (let attempt = 0; attempt < 2; attempt++) {
    expect(snapshots[attempt * 2]).toEqual({
      phase: "htmx:afterSwap", triggers: 0,
      controls: Array(3).fill({ position: "static", opacity: 1, hidden: null, labelTargetsControl: true }),
    });
    expect(snapshots[attempt * 2 + 1]).toEqual({
      phase: "htmx:afterSettle", triggers: 3,
      controls: Array(3).fill({ position: "absolute", opacity: 0, hidden: "true", labelTargetsControl: true }),
    });
  }
});

test("a rejected HTMX response does not tear down the existing controls", async ({ page }) => {
  await page.route("**/configuration", route => route.request().method() === "POST"
    ? route.fulfill({ status: 403, contentType: "text/html", body: "Forbidden" })
    : route.fallback());
  await page.goto(`${origin}/configuration?enhanced`);
  const response = page.waitForResponse(r => r.url() === `${origin}/configuration` && r.request().method() === "POST");
  await page.getByRole("button", { name: "Save configuration" }).click();
  expect((await response).status()).toBe(403);
  await expectEnhancedDropdowns(page, "rejected request");
  await page.getByRole("combobox", { name: "Default view", exact: true }).click();
  await expect(page.getByRole("option", { name: "List", exact: true })).toBeVisible();
});

test("overlapping HTMX swaps stay native until both have settled", async ({ page }) => {
  await page.goto(`${origin}/configuration?enhanced`);
  await page.evaluate(() => {
    const target = document.querySelector("#content")!;
    const detail = { target, shouldSwap: true };
    document.dispatchEvent(new CustomEvent("htmx:beforeSwap", { detail }));
    document.dispatchEvent(new CustomEvent("htmx:beforeSwap", { detail }));
    document.dispatchEvent(new CustomEvent("htmx:afterSettle", { detail }));
  });
  await expectNativeFallback(page);
  await page.evaluate(() => {
    const target = document.querySelector("#content")!;
    document.dispatchEvent(new CustomEvent("htmx:afterSettle", { detail: { target } }));
  });
  await expectEnhancedDropdowns(page, "overlapping swaps settled");
});

test("forced-colors teardown restores original labels, attributes, and focus", async ({ page }) => {
  await page.emulateMedia({ forcedColors: "active" });
  await page.goto(`${origin}/configuration?enhanced`);
  await expectNativeFallback(page);
  const native = page.locator("#default-view");
  const label = page.locator('label[for="default-view"]');
  await native.evaluate(el => {
    el.setAttribute("tabindex", "2");
    el.setAttribute("aria-hidden", "false");
    el.setAttribute("class", "existing-native");
  });
  await page.locator("#database-tls-mode").evaluate(el => el.setAttribute("class", ""));
  await label.evaluate(el => el.setAttribute("id", ""));
  await native.focus();
  await expect(native).toBeFocused();
  await page.emulateMedia({ forcedColors: "none" });
  const trigger = page.getByRole("combobox", { name: "Default view", exact: true });
  await expect(trigger).toBeFocused();
  await expect(native).toHaveAttribute("tabindex", "-1");
  await expect(native).toHaveAttribute("aria-hidden", "true");
  await page.emulateMedia({ forcedColors: "active" });
  await expectNativeFallback(page);
  await expect(native).toBeFocused();
  await expect(native).toHaveAttribute("tabindex", "2");
  await expect(native).toHaveAttribute("aria-hidden", "false");
  await expect(native).toHaveAttribute("class", "existing-native");
  await expect(page.locator("#database-tls-mode")).toHaveAttribute("class", "");
  await expect(label).toHaveAttribute("id", "");
  await expect(label).toHaveAttribute("for", "default-view");
});

test("Spaceship 2 native selects do not paint over enhanced triggers", async ({ page }) => {
  await page.goto(`${origin}/configuration?enhanced&theme=spaceship-2`);
  const triggers = page.locator(".themed-select-trigger");
  await expect(triggers).toHaveCount(3);
  for (let index = 0; index < 3; index++) {
    const trigger = triggers.nth(index);
    await trigger.scrollIntoViewIfNeeded();
    for (const state of ["closed", "open"]) {
      if (state === "open") await trigger.click();
      const before = await trigger.screenshot({ animations: "disabled" });
      const hideNative = await page.addStyleTag({ content: ".themed-select-native { visibility: hidden !important }" });
      const after = await trigger.screenshot({ animations: "disabled" });
      await hideNative.evaluate(el => el.parentNode?.removeChild(el));
      expect(before.equals(after), `native select paints on trigger ${index} (${state})`).toBe(true);
    }
    await trigger.press("Escape");
  }
});

test("Spaceship 2 uses the theme frame and a black popup with cyan selection", async ({ page }, testInfo) => {
  await page.goto(`${origin}/configuration?enhanced&theme=spaceship-2`);
  const native = page.locator("#theme");
  await native.selectOption("spaceship-2");
  const trigger = page.getByRole("combobox", { name: "Theme", exact: true });
  const nativeFrame = await native.evaluate(el => getComputedStyle(el).borderImageSource);
  expect(nativeFrame).not.toBe("none");
  await expect(trigger).toHaveCSS("border-image-source", nativeFrame);
  await expect(trigger).toHaveCSS("border-top-width", "10px");
  const image = (await trigger.screenshot({ animations: "disabled" })).toString("base64");
  const frame = await page.evaluate(async data => {
    const image = new Image();
    image.src = `data:image/png;base64,${data}`;
    await image.decode();
    const canvas = document.createElement("canvas");
    canvas.width = image.width;
    canvas.height = image.height;
    const context = canvas.getContext("2d")!;
    context.drawImage(image, 0, 0);
    const pixels = context.getImageData(0, 0, image.width, image.height).data;
    const counts = [0, 0];
    for (let y = 0; y < image.height; y++) {
      if (y >= 10 && y < image.height - 10) continue;
      for (let x = 10; x < image.width - 10; x++) {
        const index = (y * image.width + x) * 4;
        if (pixels[index] < 60 && pixels[index + 1] > 170 && pixels[index + 2] > 170) {
          counts[y < 10 ? 0 : 1]++;
        }
      }
    }
    return { counts, width: image.width };
  }, image);
  expect(frame.counts[0]).toBeGreaterThan(frame.width / 2);
  expect(frame.counts[1]).toBeGreaterThan(frame.width / 2);
  await trigger.click();
  const selected = page.getByRole("option", { name: "spaceship-2", exact: true });
  await expect(selected).toHaveAttribute("aria-selected", "true");
  await expect(selected).toHaveCSS("background-color", "rgb(0, 243, 255)");
  await expect(selected).toHaveCSS("color", "rgb(5, 6, 8)");
  await page.screenshot({ path: testInfo.outputPath("spaceship-2-open.png"), animations: "disabled" });
  await trigger.press("Home");
  const normal = page.getByRole("option", { name: "spaceship-2", exact: true });
  await expect(normal).toHaveCSS("background-color", "rgb(0, 243, 255)");
  const unselected = page.getByRole("option", { name: "dark", exact: true });
  await expect(unselected).toHaveCSS("background-color", "rgb(0, 0, 0)");
  await expect(unselected).toHaveCSS("color", "rgb(208, 240, 245)");
});

test("enhanced desktop popup has readable opaque option colors across themes", async ({ page }) => {
  test.setTimeout(180_000);
  const failures: string[] = [];
  for (const [id, scheme] of [["system", "light"], ["system", "dark"], ...themes.map(id => [id, "light"])] as [string, "light" | "dark"][]) {
    await page.emulateMedia({ colorScheme: scheme });
    await page.goto(`${origin}/configuration?enhanced&theme=${id}`);
    const trigger = page.getByRole("combobox", { name: "Default view" });
    await expect(trigger).toHaveClass(/themed-select-trigger/);
    await trigger.click();
    const ratios = await page.locator("#default-view-listbox [role=option]").evaluateAll(items => {
      const canvas = document.createElement("canvas");
      canvas.width = canvas.height = 1;
      const ctx = canvas.getContext("2d")!;
      function rgba(color: string) {
        ctx.clearRect(0, 0, 1, 1);
        ctx.fillStyle = color;
        ctx.fillRect(0, 0, 1, 1);
        return Array.from(ctx.getImageData(0, 0, 1, 1).data);
      }
      function luminance(rgb: number[]) {
        return rgb.slice(0, 3).map(c => {
          c /= 255;
          return c <= .04045 ? c / 12.92 : ((c + .055) / 1.055) ** 2.4;
        }).reduce((sum, c, i) => sum + c * [0.2126, 0.7152, 0.0722][i], 0);
      }
      return items.map(item => {
        const style = getComputedStyle(item);
        const ink = rgba(style.color), surface = rgba(style.backgroundColor);
        const a = luminance(ink), b = luminance(surface);
        return { ratio: (Math.max(a, b) + .05) / (Math.min(a, b) + .05), opacity: surface[3] };
      });
    });
    if (ratios.some(result => result.ratio < 4.5 || result.opacity !== 255)) {
      failures.push(`${id}/${scheme}: ${JSON.stringify(ratios)}`);
    }
  }
  expect(failures).toEqual([]);
});

test("enhanced popup text is readable in rendered representative themes", async ({ page }) => {
  test.setTimeout(120_000);
  const examples = ["dark", "crt-starfield-1", "aurora-glass-1"];
  expect(examples.every(id => themes.includes(id))).toBe(true);
  const cases: [string, "light" | "dark"][] = [["system", "light"], ["system", "dark"], ...examples.map(id => [id, "light"] as [string, "light"])];
  for (const [id, scheme] of cases) {
    await page.emulateMedia({ colorScheme: scheme });
    await page.goto(`${origin}/configuration?enhanced&theme=${id}`);
    const trigger = page.getByRole("combobox", { name: "Default view" });
    await trigger.click();
    for (const selector of ["#default-view-listbox [data-active=true]", "#default-view-listbox [role=option]:nth-child(2)"]) {
      const sample = await contrast(page, selector, ".themed-select-option");
      expect(sample.count, `${id}/${scheme} ${selector} glyph samples`).toBeGreaterThan(0);
      expect(sample.minimum, `${id}/${scheme} ${selector} contrast`).toBeGreaterThanOrEqual(4.5);
    }
  }
});

test("enhanced dropdown synchronizes dynamic options, reset and capability changes", async ({ page }) => {
  await page.goto(`${origin}/configuration?enhanced`);
  const native = page.locator("#default-view");
  const trigger = page.getByRole("combobox", { name: "Default view" });
  await trigger.click();
  await trigger.press("End");
  await expect(page.getByRole("option", { name: "Configuration" })).toHaveAttribute("data-active", "true");
  await trigger.press("Escape");
  await expect(native).toHaveValue("list");
  await native.evaluate((el: HTMLSelectElement) => {
    el.add(new Option("Dynamic option", "dynamic"));
    el.options[1].disabled = true;
  });
  await trigger.click();
  await expect(page.getByRole("option", { name: "Add", exact: true })).toHaveAttribute("aria-disabled", "true");
  await trigger.press("ArrowDown");
  await expect(page.getByRole("option", { name: "Search", exact: true })).toHaveAttribute("data-active", "true");
  await page.getByRole("option", { name: "Dynamic option" }).click();
  await expect(native).toHaveValue("dynamic");
  await page.getByRole("button", { name: "Reset" }).click();
  await expect(trigger).toHaveText("List");
  await trigger.click();
  await page.locator("h1").click();
  await expect(trigger).toHaveAttribute("aria-expanded", "false");
  await page.emulateMedia({ forcedColors: "active" });
  await expect(page.locator(".themed-select-trigger")).toHaveCount(0);
  await expect(page.getByLabel("Default view")).toBeVisible();
  await page.emulateMedia({ forcedColors: "none" });
  await expect(page.locator(".themed-select-trigger")).toHaveCount(3);
});

test("enhanced dropdown handles validation, keyboard navigation and long lists", async ({ page }) => {
  await page.setViewportSize({ width: 900, height: 500 });
  await page.goto(`${origin}/configuration?enhanced`);
  const native = page.locator("#theme");
  const trigger = page.getByRole("combobox", { name: "Theme", exact: true });
  await native.evaluate((el: HTMLSelectElement) => { el.selectedIndex = -1; el.dispatchEvent(new Event("change")); });
  await page.getByRole("button", { name: "Save configuration" }).click();
  await expect(trigger).toBeFocused();
  await expect(trigger).toHaveAttribute("aria-invalid", "true");
  await expect(page.locator("#theme-native-error")).toBeVisible();
  await trigger.press("ArrowDown");
  const list = page.locator("#theme-listbox");
  await trigger.press("End");
  const last = list.getByRole("option").last();
  await expect(last).toHaveAttribute("data-active", "true");
  expect(await list.evaluate(el => el.scrollTop)).toBeGreaterThan(0);
  const boxes = await Promise.all([list.boundingBox(), last.boundingBox()]);
  expect(boxes[1]!.y + boxes[1]!.height).toBeLessThanOrEqual(boxes[0]!.y + boxes[0]!.height + 1);
  await trigger.press("Home");
  await trigger.press("s");
  await trigger.press("Enter");
  await expect(native).not.toHaveValue("");
  await expect(page.locator("#theme-native-error")).toBeHidden();
  await trigger.click();
  await trigger.press("Tab");
  await expect(trigger).toHaveAttribute("aria-expanded", "false");
  const tls = page.getByRole("combobox", { name: "Database TLS mode" });
  await tls.evaluate(el => window.scrollBy(0, el.getBoundingClientRect().bottom - innerHeight + 20));
  await tls.click();
  const popup = await page.locator("#database-tls-mode-listbox").boundingBox();
  const control = await tls.boundingBox();
  expect(popup!.y + popup!.height).toBeLessThanOrEqual(control!.y + 1);
  expect(popup!.y).toBeGreaterThanOrEqual(0);
});

test("touch-capable contexts retain native controls", async ({ browser }) => {
  const context = await browser.newContext({ hasTouch: true, viewport: { width: 390, height: 844 } });
  try {
    await fixture(context);
    const page = await context.newPage();
    await page.goto(`${origin}/configuration?enhanced`);
    expect(await page.evaluate(() => matchMedia("(pointer: coarse)").matches)).toBe(true);
    await expect(page.locator(".themed-select-trigger")).toHaveCount(0);
    await expect(page.getByLabel("Theme", { exact: true })).toBeVisible();
    await page.getByLabel("Default view").selectOption("search");
    await expect(page.locator("#default-view")).toHaveValue("search");
  } finally {
    await context.close();
  }
});

test("opened DOM pickers use base-select and readable rendered options for every theme", async ({ page, browserName }) => {
  test.setTimeout(900_000);
  await page.goto(`${origin}/configuration`);
  const supports = await page.evaluate(() => CSS.supports("appearance", "base-select") && CSS.supports("selector(::picker(select))"));
  test.skip(!supports, `${browserName} does not support customizable select; native popup is OS-owned`);
  expect(themes.length).toBeGreaterThan(0);
  const failures: string[] = [];
  let worst = Infinity, checked = 0;
  for (const [id, scheme] of [["system", "light"], ["system", "dark"], ...themes.map(id => [id, "light"])] as [string, "light" | "dark"][]) {
    await page.emulateMedia({ colorScheme: scheme, reducedMotion: "reduce" });
    await page.goto(`${origin}/configuration?theme=${id}`);
    for (const selector of ["#theme", "#default-view", "#database-tls-mode"]) {
      const label = `${id}/${scheme} ${selector}`;
      const select = page.locator(selector);
      const styles = await select.evaluate(el => ({
        trigger: getComputedStyle(el).appearance,
        picker: getComputedStyle(el, "::picker(select)").appearance,
      }));
      if (styles.trigger !== "base-select" || styles.picker !== "base-select") {
        failures.push(`${label}: trigger/picker appearance ${JSON.stringify(styles)}`);
        continue;
      }
      await open(page, selector);
      const options = select.locator("option");
      const selected = select.locator("option:checked");
      const first = options.nth(1);
      for (const [state, option] of [["selected", selected], ["normal", first]] as const) {
        const optionIndex = await option.evaluate(el => Array.from(el.parentElement!.children).indexOf(el));
        const result = await contrast(page, `${selector} option:nth-child(${optionIndex + 1})`);
        checked++;
        if (result.count < 3 || result.minimum < 4.5) failures.push(`${label} ${state}: ${JSON.stringify(result)}`);
        if (result.count) worst = Math.min(worst, result.minimum);
      }
      await first.hover();
      const hovered = await first.evaluate(el => el.matches(":hover"));
      if (!hovered) failures.push(`${label}: pointer did not reach opened option`);
      const hoverResult = await contrast(page, `${selector} option:nth-child(2)`);
      checked++;
      if (hoverResult.count < 3 || hoverResult.minimum < 4.5) failures.push(`${label} hover: ${JSON.stringify(hoverResult)}`);
      if (hoverResult.count) worst = Math.min(worst, hoverResult.minimum);
      // Geometry belongs to the currently opened picker, not the closed select.
      const geometry = await first.evaluate(el => {
        const r = el.getBoundingClientRect();
        return { x: r.x, y: r.y, right: r.right, bottom: r.bottom, width: r.width, height: r.height, viewportWidth: innerWidth, viewportHeight: innerHeight };
      });
      if (geometry.width <= 0 || geometry.height <= 0 || geometry.x < 0 || geometry.y < 0 || geometry.right > geometry.viewportWidth || geometry.bottom > geometry.viewportHeight) failures.push(`${label}: option geometry ${JSON.stringify(geometry)}`);
      await page.keyboard.press("Escape");
      await expect.poll(() => select.evaluate(el => el.matches(":open"))).toBe(false);
    }
  }
  console.log(`DOM picker: ${checked} rendered option states; minimum measured contrast ${Number.isFinite(worst) ? worst.toFixed(2) : "not measured"}:1`);
  expect(failures, failures.join("\n")).toEqual([]);
});

test("native names, keyboard, reset, disabled selected option and validation survive", async ({ page }) => {
  await page.goto(`${origin}/configuration?unavailable`);
  await expect(page.getByLabel("Theme")).toHaveAttribute("aria-describedby", "configuration-error");
  expect(await page.locator("form").evaluate((form: HTMLFormElement) => new FormData(form).has("theme"))).toBe(false);
  await page.getByLabel("Theme").selectOption("system");
  await expect(page.getByLabel("Theme")).toHaveValue("system");
  const select = page.getByLabel("Default view");
  await select.focus();
  if (await select.evaluate(el => getComputedStyle(el).appearance === "base-select")) {
    await page.keyboard.press("Space");
  }
  await page.keyboard.press("Home");
  await page.keyboard.press("ArrowDown");
  await page.keyboard.press("Enter");
  await expect(select).toHaveValue("add");
  await page.getByLabel("Database TLS mode").selectOption("verify-full");
  const values = await page.locator("form").evaluate((form: HTMLFormElement) => Object.fromEntries(new FormData(form)));
  expect(values).toMatchObject({ theme: "system", default_view: "add", database_tls_mode: "verify-full", csrf_token: "fixture" });
  await page.getByRole("button", { name: "Reset" }).click();
  await expect(select).toHaveValue("list");
  await expect(page.getByLabel("Theme")).toHaveValue("missing");
  await page.getByLabel("Theme").evaluate((el: HTMLSelectElement) => { el.value = ""; });
  expect(await page.getByLabel("Theme").evaluate((el: HTMLSelectElement) => el.validity.valueMissing)).toBe(true);
});

test("disabled selected and keyboard-active options remain legible across themes", async ({ page, browserName }) => {
  test.setTimeout(600_000);
  const support = await page.evaluate(() => CSS.supports("appearance", "base-select") && CSS.supports("selector(::picker(select))"));
  test.skip(!support, `${browserName} uses a native picker`);
  const failures: string[] = [];
  let worstDisabled = Infinity, worstActive = Infinity;
  for (const [id, scheme] of [["system", "light"], ["system", "dark"], ...themes.map(id => [id, "light"])] as [string, "light" | "dark"][]) {
    await page.emulateMedia({ colorScheme: scheme, reducedMotion: "reduce" });
    await page.goto(`${origin}/configuration?theme=${id}&unavailable`);
    await open(page, "#theme");
    const disabled = await contrast(page, "#theme option:nth-child(1)");
    if (disabled.count < 3 || disabled.minimum < 4.5) failures.push(`${id}/${scheme} disabled: ${JSON.stringify(disabled)}`);
    if (disabled.count) worstDisabled = Math.min(worstDisabled, disabled.minimum);
    await page.keyboard.press("Home");
    await page.keyboard.press("ArrowDown");
    const active = page.locator("#theme option:focus");
    if (await active.count() !== 1) {
      failures.push(`${id}/${scheme}: no keyboard-active option`);
    } else {
      const index = await active.evaluate(el => Array.from(el.parentElement!.children).indexOf(el));
      const measured = await contrast(page, `#theme option:nth-child(${index + 1})`);
      if (measured.count < 3 || measured.minimum < 4.5) failures.push(`${id}/${scheme} active: ${JSON.stringify(measured)}`);
      if (measured.count) worstActive = Math.min(worstActive, measured.minimum);
    }
    await page.keyboard.press("Escape");
  }
  console.log(`Disabled selected minimum ${worstDisabled.toFixed(2)}:1; keyboard-active minimum ${worstActive.toFixed(2)}:1`);
  expect(failures, failures.join("\n")).toEqual([]);
});

test("opened picker border contrasts with its interior across themes", async ({ page, browserName }) => {
  test.setTimeout(300_000);
  const support = await page.evaluate(() => CSS.supports("appearance", "base-select") && CSS.supports("selector(::picker(select))"));
  test.skip(!support, `${browserName} uses a native picker`);
  const failures: string[] = [];
  let worst = Infinity, worstOutside = Infinity;
  for (const [id, scheme] of [["system", "light"], ["system", "dark"], ...themes.map(id => [id, "light"])] as [string, "light" | "dark"][]) {
    await page.emulateMedia({ colorScheme: scheme, reducedMotion: "reduce" });
    await page.goto(`${origin}/configuration?theme=${id}`);
    await open(page, "#default-view");
    const box = await page.locator("#default-view option").nth(1).boundingBox();
    const image = (await page.screenshot({ animations: "disabled" })).toString("base64");
    const ratio = await page.evaluate(async ({ image, box }) => {
      const img = new Image(); img.src = `data:image/png;base64,${image}`; await img.decode();
      const canvas = document.createElement("canvas");
      canvas.width = img.width; canvas.height = img.height;
      const ctx = canvas.getContext("2d")!; ctx.drawImage(img, 0, 0);
      const scale = img.width / innerWidth;
      const pixel = (x: number, y: number) => Array.from(ctx.getImageData(Math.floor(x * scale), Math.floor(y * scale), 1, 1).data).slice(0, 3);
      const luminance = (rgb: number[]) => rgb.map(c => {
        c /= 255; return c <= .04045 ? c / 12.92 : ((c + .055) / 1.055) ** 2.4;
      }).reduce((sum, c, i) => sum + c * [0.2126, 0.7152, 0.0722][i], 0);
      const y = box!.y + box!.height / 2;
      const border = pixel(box!.x - 1, y), interior = pixel(box!.x + 3, y), outside = pixel(box!.x - 4, y);
      const a = luminance(border), b = luminance(interior), c = luminance(outside);
      return {
        ratio: (Math.max(a, b) + .05) / (Math.min(a, b) + .05),
        outsideRatio: (Math.max(a, c) + .05) / (Math.min(a, c) + .05),
        border, interior, outside,
      };
    }, { image, box });
    worst = Math.min(worst, ratio.ratio);
    worstOutside = Math.min(worstOutside, ratio.outsideRatio);
    if (ratio.ratio < 3 || ratio.outsideRatio < 3) failures.push(`${id}/${scheme}: ${JSON.stringify(ratio)}`);
    await page.keyboard.press("Escape");
  }
  console.log(`Minimum sampled border/interior contrast ${worst.toFixed(2)}:1; border/exterior ${worstOutside.toFixed(2)}:1`);
  expect(failures, failures.join("\n")).toEqual([]);
});

test("HTMX 422 replacement keeps three usable selects without duplicate widgets", async ({ page }) => {
  await page.goto(`${origin}/configuration`);
  await page.getByRole("button", { name: "Save configuration" }).click();
  await expect(page.getByRole("alert")).toHaveText("Choose an available theme");
  await expect(page.locator("form select")).toHaveCount(3);
  await expect(page.locator(".custom-select-container")).toHaveCount(0);
  await page.getByLabel("Theme").selectOption("system");
  await page.getByLabel("Default view").selectOption("search");
  await expect(page.getByLabel("Default view")).toHaveValue("search");
  await page.getByRole("button", { name: "Save configuration" }).click();
  await expect(page.getByRole("alert")).toBeVisible();
  await expect(page.locator("form select")).toHaveCount(3);
  await expect(page.locator(".custom-select-container")).toHaveCount(0);
});

test("opened DOM picker skips disabled options and commits exactly one change", async ({ page, browserName }) => {
  await page.goto(`${origin}/configuration?unavailable`);
  const support = await page.evaluate(() => CSS.supports("appearance", "base-select") && CSS.supports("selector(::picker(select))"));
  test.skip(!support, `${browserName} uses a native picker`);
  const theme = page.getByLabel("Theme");
  const appearance = await theme.evaluate(el => getComputedStyle(el).appearance);
  expect(appearance, "customizable picker must be enabled on the actual select").toBe("base-select");
  const events = await theme.evaluate(el => {
    const counts = { input: 0, change: 0 };
    el.addEventListener("input", () => counts.input++);
    el.addEventListener("change", () => counts.change++);
    (el as HTMLSelectElement & { counts: typeof counts }).counts = counts;
    return true;
  });
  expect(events).toBe(true);
  await open(page, "#theme");
  const disabled = theme.locator("option:disabled:checked");
  await expect(disabled).toHaveCount(1);
  const state = await disabled.evaluate(el => ({ disabled: el.matches(":disabled"), selected: el.matches(":checked"), rect: el.getBoundingClientRect().toJSON() }));
  expect(state.disabled && state.selected).toBe(true);
  expect(state.rect.width).toBeGreaterThan(0);
  await page.keyboard.press("Home");
  await page.keyboard.press("Enter");
  await expect(theme).toHaveValue("system");
  expect(await theme.evaluate(el => (el as HTMLSelectElement & { counts: { input: number; change: number } }).counts)).toEqual({ input: 1, change: 1 });
  expect(await page.locator("form").evaluate((form: HTMLFormElement) => new FormData(form).get("theme"))).toBe("system");
  await open(page, "#theme");
  await theme.locator("option").nth(2).click();
  await expect(theme).toHaveValue(themes[0]);
  expect(await theme.evaluate(el => (el as HTMLSelectElement & { counts: { input: number; change: number } }).counts)).toEqual({ input: 2, change: 2 });
});

test("native picker keyboard, dynamic state, scrolling and outside click", async ({ page, browserName }) => {
  await page.goto(`${origin}/configuration`);
  const support = await page.evaluate(() => CSS.supports("appearance", "base-select") && CSS.supports("selector(::picker(select))"));
  test.skip(!support, `${browserName} uses a native picker`);
  const select = page.getByLabel("Theme", { exact: true });
  await select.focus();
  await page.keyboard.press("ArrowDown");
  await expect.poll(() => select.evaluate(el => el.matches(":open"))).toBe(true);
  await page.keyboard.press("End");
  const last = select.locator("option").last();
  expect(await last.evaluate(el => {
    const r = el.getBoundingClientRect();
    return r.top >= 0 && r.bottom <= innerHeight;
  })).toBe(true);
  await page.keyboard.press("Home");
  await page.keyboard.type("cartoon");
  await page.keyboard.press("Enter");
  await expect(select).toHaveValue("cartoon-1");
  await select.click();
  await page.keyboard.press("Escape");
  await expect.poll(() => select.evaluate(el => el.matches(":open"))).toBe(false);
  await expect(select).toBeFocused();
  await select.click();
  await page.locator("h1").click();
  await expect.poll(() => select.evaluate(el => el.matches(":open"))).toBe(false);
  await select.evaluate((el: HTMLSelectElement) => { el.disabled = true; });
  await expect(select).toBeDisabled();
  expect(await select.evaluate((el: HTMLSelectElement) => new FormData(el.form!).has(el.name))).toBe(false);
  await select.evaluate((el: HTMLSelectElement) => {
    el.disabled = false;
    el.options[1].disabled = true;
    el.selectedOptions[0].disabled = true;
  });
  expect(await select.evaluate((el: HTMLSelectElement) => new FormData(el.form!).has(el.name))).toBe(false);
  await select.click();
  await expect(select.locator("option").nth(1)).toBeDisabled();
  await expect(select.locator("option:checked")).toBeDisabled();
  await page.keyboard.press("Home");
  await page.keyboard.press("Enter");
  await expect(select).toHaveValue("system");
  await select.click();
  await page.keyboard.press("Tab");
  await expect.poll(() => select.evaluate(el => el.matches(":open"))).toBe(false);
});

test("opened native picker remains usable at a touch-sized viewport", async ({ browser, browserName }) => {
  test.skip(browserName !== "chromium", "DOM picker is only tested where supported");
  const context = await browser.newContext({ viewport: { width: 375, height: 812 }, hasTouch: true, isMobile: true });
  try {
    await fixture(context);
    const page = await context.newPage();
    await page.goto(`${origin}/configuration?theme=cartoon-1`);
    const view = page.getByLabel("Default view");
    await view.tap();
    await expect.poll(() => view.evaluate(el => el.matches(":open"))).toBe(true);
    const option = view.locator("option").nth(1);
    const box = await option.boundingBox();
    expect(box!.x).toBeGreaterThanOrEqual(0);
    expect(box!.x + box!.width).toBeLessThanOrEqual(375);
    await option.tap();
    await expect(view).toHaveValue("add");
  } finally { await context.close(); }
});

test("native option colors respond to System preference and theme changes", async ({ page }) => {
  await page.emulateMedia({ colorScheme: "light" });
  await page.goto(`${origin}/configuration`);
  const select = page.getByRole("combobox", { name: "Theme" });
  await expect(select).toHaveAttribute("required", "");
  const option = select.locator("option").nth(1);
  const color = () => option.evaluate(el => getComputedStyle(el).backgroundColor);
  const light = await color();
  await page.emulateMedia({ colorScheme: "dark" });
  await expect.poll(color).not.toBe(light);
  const dark = await color();
  await page.evaluate(() => {
    document.documentElement.dataset.theme = "cartoon-1";
    const link = document.createElement("link");
    link.rel = "stylesheet";
    link.href = "/ui/themes/cartoon-1/theme.css";
    document.head.insertBefore(link, document.querySelector('link[href^="/assets/css/chroma-compat.css"]'));
  });
  await expect.poll(color).not.toBe(dark);
});

test("no-JavaScript form submits native values", async ({ browser }) => {
  const context = await browser.newContext({ javaScriptEnabled: false });
  try {
    await fixture(context);
    const page = await context.newPage();
    // Include the enhancement script in the document; the browser must not
    // execute it when JavaScript is disabled.
    await page.goto(`${origin}/configuration?enhanced`);
    await expect(page.locator("form select")).toHaveCount(3);
    await expect(page.locator(".themed-select")).toHaveCount(0);
    await expect(page.getByRole("combobox")).toHaveCount(3);
    for (const [id] of pickerFields) await expect(page.locator(`#${id}`)).toBeVisible();
    await page.getByLabel("Default view").selectOption("search");
    const post = page.waitForRequest(request => request.url() === `${origin}/configuration` && request.method() === "POST");
    await page.getByRole("button", { name: "Save configuration" }).click();
    const body = new URLSearchParams((await post).postData() || "");
    expect(body.get("theme")).toBe("system");
    expect(body.get("default_view")).toBe("search");
    expect(body.get("database_tls_mode")).toBe("disable");
    await expect(page.getByRole("alert")).toBeVisible();
  } finally { await context.close(); }
});

test("forced colors retains native keyboard operation and a visible focus outline", async ({ page, browserName }, testInfo) => {
  test.skip(browserName !== "chromium", "forced-colors emulation is supported by Chromium");
  await page.emulateMedia({ forcedColors: "active" });
  await page.goto(`${origin}/configuration?theme=dark`);
  const select = page.getByLabel("Default view");
  await select.focus();
  const focus = await select.evaluate(el => {
    const s = getComputedStyle(el);
    return { style: s.outlineStyle, width: parseFloat(s.outlineWidth), color: s.outlineColor };
  });
  expect(focus.style).not.toBe("none");
  expect(focus.width).toBeGreaterThanOrEqual(2);
  if (await select.evaluate(el => getComputedStyle(el).appearance === "base-select")) {
    await open(page, "#default-view");
    const selected = await page.locator("#default-view option:checked").boundingBox();
    expect(selected?.width).toBeGreaterThan(0);
    expect(selected?.height).toBeGreaterThan(0);
    // Forced-color adjustment can override ink-hiding CSS, so preserve the
    // actual opened screenshot rather than claim a reliable pixel ratio here.
    await testInfo.attach("forced-colors-open-picker", { body: await page.screenshot(), contentType: "image/png" });
    await page.keyboard.press("Escape");
    await open(page, "#default-view");
    await page.keyboard.press("Home");
    await page.keyboard.press("ArrowDown");
  } else {
    await select.focus();
    await page.keyboard.press("ArrowDown");
  }
  await page.keyboard.press("Enter");
  await expect(select).toHaveValue("add");
});
