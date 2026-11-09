import { defineConfig, devices } from "@playwright/test";
import base from "../../playwright.config";

// This suite owns its server and database and must not run the shared setup,
// which uses fixed ports and removes shared .e2e-* files.
export default defineConfig(base, {
  testDir: ".",
  testMatch: "chroma-background.spec.ts",
  globalSetup: undefined,
  projects: [
    ...(base.projects ?? []),
    { name: "webkit-mobile", use: { ...devices["iPhone 13"] } },
  ],
});
