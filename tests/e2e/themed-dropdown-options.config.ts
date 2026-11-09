import { defineConfig, devices } from "@playwright/test";
import { randomUUID } from "node:crypto";
import { join } from "node:path";
import { tmpdir } from "node:os";

// Playwright removes outputDir at startup: never point it at another run.
export default defineConfig({
  testDir: ".",
  testMatch: "themed-dropdown-options.spec.ts",
  outputDir: join(tmpdir(), `themed-dropdown-options-${randomUUID()}`),
  preserveOutput: "always",
  reporter: "list",
  workers: 1,
  use: { headless: true, trace: "retain-on-failure" },
  projects: [
    { name: "chromium", use: { ...devices["Desktop Chrome"] } },
    { name: "firefox", use: { ...devices["Desktop Firefox"] } },
    { name: "webkit", use: { ...devices["Desktop Safari"] } },
  ],
});
