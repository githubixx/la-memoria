import { defineConfig, devices } from "@playwright/test";
import { randomUUID } from "node:crypto";
import { join } from "node:path";

// This suite owns its isolated server/database; do not run the shared setup.
export default defineConfig({
  testDir: ".",
  testMatch: "theme-menu.spec.ts",
  workers: 1,
  reporter: "list",
  // Playwright clears its output directory on startup; never reuse an
  // existing run's results, and retain the overlap screenshots on success.
  outputDir: join(process.cwd(), "test-results", `theme-menu-${randomUUID()}`),
  preserveOutput: "always",
  use: { headless: true, trace: "retain-on-failure" },
  projects: [
    { name: "chromium", use: { ...devices["Desktop Chrome"] } },
    { name: "firefox", use: { ...devices["Desktop Firefox"] } },
  ],
});
