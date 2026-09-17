import { defineConfig, devices } from "@playwright/test";

const projects = [
  { name: "chromium", use: { ...devices["Desktop Chrome"] } },
  { name: "firefox", use: { ...devices["Desktop Firefox"] } },
  {
    name: "mobile",
    use: {
      ...devices["iPhone 13"],
      browserName: "chromium" as const,
    },
  },
];

export default defineConfig({
  testDir: "./tests/e2e",
  globalSetup: "./tests/e2e/global-setup.ts",
  fullyParallel: false,
  workers: 1,
  forbidOnly: Boolean(process.env.CI),
  retries: process.env.CI ? 2 : 0,
  reporter: process.env.CI ? [["html", { open: "never" }], ["list"]] : "list",
  use: {
    baseURL: process.env.BOOKMARKER_BASE_URL ?? "http://127.0.0.1:18080",
    trace: "retain-on-failure",
  },
  projects,
});