import { defineConfig, devices } from "@playwright/test";
import { randomUUID } from "node:crypto";
import { tmpdir } from "node:os";
import { join } from "node:path";

// No shared global setup, database, or Playwright output directory.
export default defineConfig({
  testDir: ".",
  testMatch: "page-content-width.spec.ts",
  workers: 1,
  reporter: "list",
  outputDir: join(tmpdir(), `bookmarker-width-results-${randomUUID()}`),
  preserveOutput: "always",
  use: { headless: true, trace: "retain-on-failure" },
  projects: [
    { name: "chromium", use: { ...devices["Desktop Chrome"] } },
    { name: "firefox", use: { ...devices["Desktop Firefox"] } },
    { name: "webkit", use: { ...devices["Desktop Safari"] } },
    { name: "mobile", use: { ...devices["iPhone 13"], browserName: "chromium" } },
  ],
});
