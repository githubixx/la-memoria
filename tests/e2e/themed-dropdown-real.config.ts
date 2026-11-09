import { defineConfig, devices } from "@playwright/test";
import { randomUUID } from "node:crypto";
import { join } from "node:path";
import { tmpdir } from "node:os";

export default defineConfig({
  testDir: ".",
  testMatch: "themed-dropdown-real.spec.ts",
  outputDir: join(tmpdir(), `themed-dropdown-real-${randomUUID()}`),
  preserveOutput: "always",
  reporter: "list",
  workers: 1,
  use: { headless: true, trace: "retain-on-failure" },
  projects: [{ name: "chromium", use: { ...devices["Desktop Chrome"] } }],
});
