import { defineConfig, devices } from "@playwright/test";
import { randomUUID } from "node:crypto";
import { join } from "node:path";

// No shared global setup or shared test-results: each run owns its outputs.
export default defineConfig({
  testDir: ".",
  testMatch: "configuration-theme.spec.ts",
  workers: 1,
  reporter: "list",
  outputDir: join(process.cwd(), "test-results", `configuration-theme-${randomUUID()}`),
  preserveOutput: "always",
  use: { headless: true, trace: "retain-on-failure" },
  projects: [
    { name: "chromium", use: { ...devices["Desktop Chrome"] } },
    { name: "firefox", use: { ...devices["Desktop Firefox"] } },
    { name: "webkit", use: { ...devices["Desktop Safari"] } },
  ],
});
