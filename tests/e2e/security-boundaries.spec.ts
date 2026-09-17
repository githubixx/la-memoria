import { expect, test } from "./fixtures";

const ADMIN_USERNAME = "admin";
const ADMIN_PASSWORD = "bookmarker-fixture-password";

async function signIn(page: import("@playwright/test").Page) {
  await page.goto("/login");
  await page.getByLabel("Username").fill(ADMIN_USERNAME);
  await page.getByLabel("Password").fill(ADMIN_PASSWORD);
  await Promise.all([
    page.waitForURL(/\/bookmarks$/),
    page.getByRole("button", { name: "Sign in" }).click(),
  ]);
}

test.describe("Security boundaries", () => {
  test("unauthenticated configuration writes are denied", async ({ request, baseURL }) => {
    const response = await request.post("/configuration", {
      form: { page_title: "Unauthorised" },
      headers: { Origin: new URL(baseURL!).origin },
      maxRedirects: 0,
    });
    expect(response.status()).toBe(303);
    expect(response.headers()["location"]).toContain("/login?return_to=%2Fconfiguration");
  });

  test("cross-origin configuration writes are rejected", async ({ request }) => {
    const response = await request.post("/configuration", {
      form: { page_title: "Unauthorised" },
      headers: { Origin: "https://attacker.example.test" },
      maxRedirects: 0,
    });
    expect(response.status()).toBe(403);
  });

  test("every unauthenticated bookmark write route redirects to sign-in", async ({ request, baseURL }) => {
    for (const path of ["/bookmarks", "/bookmarks/not-a-bookmark/delete", "/captures"]) {
      const response = await request.post(path, {
        headers: { Origin: new URL(baseURL!).origin },
        maxRedirects: 0,
      });
      expect(response.status(), path).toBe(303);
      expect(response.headers()["location"], path).toContain("/login?return_to=");
    }
  });

  test("revoked sessions and pre-authentication CSRF values cannot write", async ({ page, context }) => {
    await page.goto("/login");
    const oldToken = await page.locator("input[name=csrf_token]").inputValue();
    await signIn(page);
    const response = await page.evaluate(async (csrfToken) => {
      const body = new URLSearchParams({ csrf_token: csrfToken, page_title: "Rejected old token" });
      return fetch("/configuration", { method: "POST", body, redirect: "manual" }).then((reply) => reply.status);
    }, oldToken);
    expect(response).toBe(403);
    await context.clearCookies();
    await page.goto("/configuration");
    await expect(page).toHaveURL(/\/login/);
  });
});