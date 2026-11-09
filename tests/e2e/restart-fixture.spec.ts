import { expect, test } from "@playwright/test";
import { existsSync, mkdtempSync, readFileSync, readdirSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { createRestartFixture } from "./restart-fixture";

test.setTimeout(120_000);

test("isolated server restarts with exactly the saved configuration", async () => {
  const fixture = await createRestartFixture();
  const directory = fixture.directory;
  try {
    const saved = readFileSync(fixture.configPath, "utf8") + "\ntheme: system\n";
    writeFileSync(fixture.configPath, saved, { mode: 0o600 });
    expect((await fetch(`${fixture.baseURL}/bookmarks`)).ok).toBeTruthy();
    await fixture.stop();
    await expect(fetch(`${fixture.baseURL}/bookmarks`)).rejects.toThrow();
    await fixture.start();
    expect((await fetch(`${fixture.baseURL}/bookmarks`)).ok).toBeTruthy();
    expect(readFileSync(fixture.configPath, "utf8")).toBe(saved);
  } finally {
    await fixture.close();
  }
  expect(existsSync(directory)).toBeFalsy();
});

test("startup errors are surfaced and fixture-owned files are removed", async () => {
  const root = mkdtempSync(join(tmpdir(), "bookmarker-fixture-check-"));
  try {
    await expect(createRestartFixture({ temporaryRoot: root, executable: join(root, "does-not-exist") })).rejects.toThrow();
    expect(readdirSync(root)).toEqual([]);
  } finally {
    rmSync(root, { recursive: true, force: true });
  }
});

test("unexpected process termination is reported and cleanup still works", async () => {
  const fixture = await createRestartFixture();
  const directory = fixture.directory;
  try {
    fixture.process!.kill("SIGKILL");
    await expect(fixture.stop()).rejects.toThrow();
  } finally {
    await fixture.close();
  }
  expect(existsSync(directory)).toBeFalsy();
});
