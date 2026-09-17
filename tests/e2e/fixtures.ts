import { execFileSync } from "node:child_process";
import { chmodSync, copyFileSync, mkdirSync, rmSync } from "node:fs";
import { join } from "node:path";
import { test as base, expect } from "@playwright/test";

const root = process.cwd();
const databaseURL = "postgresql://bookmarker:bookmarker-test@127.0.0.1:54329/bookmarker?sslmode=disable";
const runtimeConfig = join(root, ".e2e-config.yaml");
const templateConfig = join(root, ".e2e-config.template.yaml");
const screenshots = join(root, ".e2e-screenshots");

export const test = base.extend<{ resetE2E: void }>({
  resetE2E: [async ({}, use) => {
    copyFileSync(templateConfig, runtimeConfig);
    chmodSync(runtimeConfig, 0o600);
    rmSync(screenshots, { recursive: true, force: true });
    mkdirSync(join(screenshots, "staging"), { recursive: true });
    execFileSync("psql", [databaseURL, "-v", "ON_ERROR_STOP=1", "-c", "TRUNCATE bookmarks, tags, web_sessions, screenshot_cleanup, login_throttle_pairs CASCADE"], { stdio: "inherit" });
    execFileSync("psql", [databaseURL, "-v", "ON_ERROR_STOP=1", "-f", join(root, "tests/fixtures/bookmarks.sql")], { stdio: "inherit" });
    await use();
  }, { auto: true }],
});

export { expect };