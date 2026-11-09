import { execFile as execFileCallback, spawn, type ChildProcess } from "node:child_process";
import { randomUUID } from "node:crypto";
import { mkdtempSync, mkdirSync, rmSync, writeFileSync } from "node:fs";
import { createServer } from "node:net";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { promisify } from "node:util";

const execFile = promisify(execFileCallback);
const projectRoot = process.cwd();
const password = "bookmarker-test";

async function unusedPort(): Promise<number> {
  const server = createServer();
  await new Promise<void>((resolve, reject) => {
    server.once("error", reject);
    server.listen(0, "127.0.0.1", resolve);
  });
  const address = server.address();
  if (!address || typeof address === "string") throw new Error("could not allocate fixture port");
  await new Promise<void>((resolve, reject) => server.close(error => error ? reject(error) : resolve()));
  return address.port;
}

const sleep = (ms: number) => new Promise(resolve => setTimeout(resolve, ms));

export class RestartFixture {
  readonly directory: string;
  readonly configPath: string;
  readonly baseURL: string;
  private readonly postgresPort: number;
  private readonly container: string;
  private readonly executable: string;
  private child?: ChildProcess;
  private containerStarted = false;
  private closed = false;

  constructor(directory: string, webPort: number, postgresPort: number, executable: string) {
    this.directory = directory;
    this.configPath = join(directory, "config.yaml");
    this.baseURL = `http://127.0.0.1:${webPort}`;
    this.postgresPort = postgresPort;
    this.container = `bookmarker-restart-${randomUUID()}`;
    this.executable = executable;
  }

  get process(): ChildProcess | undefined { return this.child; }

  async initialize(): Promise<void> {
    const screenshotRoot = join(this.directory, "screenshots");
    mkdirSync(screenshotRoot);
    const webPort = new URL(this.baseURL).port;
    const configuration = `administrator:
  username: admin
  password_hash: $argon2id$v=19$m=65536,t=3,p=1$C9O9Mk4c2Zn/ZfyqxooQvA$HPd9+zneNgxejjew94cvrd8E8+MwuZcPhibe4x1i0vM
branding:
  page_title: Restart fixture
default_view: list
screenshots:
  root: ${screenshotRoot}
database:
  host: 127.0.0.1
  port: ${this.postgresPort}
  name: bookmarker
  user: bookmarker
  password_env: BOOKMARKER_DB_PASSWORD
  tls_mode: disable
search:
  page_size: 10
  maximum_results: 10000
server:
  listen_address: 127.0.0.1:${webPort}
`;
    writeFileSync(this.configPath, configuration, { mode: 0o600 });
    await execFile("docker", ["run", "--detach", "--rm", "--name", this.container,
      "--publish", `127.0.0.1:${this.postgresPort}:5432`,
      "--env", "POSTGRES_DB=bookmarker", "--env", "POSTGRES_USER=bookmarker",
      "--env", `POSTGRES_PASSWORD=${password}`, "postgres:18.6"]);
    this.containerStarted = true;
    let ready = false;
    for (let i = 0; i < 80; i++) {
      try {
        await execFile("pg_isready", ["--host", "127.0.0.1", "--port", String(this.postgresPort), "--username", "bookmarker", "--dbname", "bookmarker"]);
        ready = true;
        break;
      } catch { await sleep(250); }
    }
    if (!ready) throw new Error("fixture database did not become ready");
    await execFile("psql", ["--host", "127.0.0.1", "--port", String(this.postgresPort), "--username", "bookmarker", "--dbname", "bookmarker", "-v", "ON_ERROR_STOP=1", "-f", join(projectRoot, "migrations/0001_bookmarks.up.sql")],
      { env: { ...process.env, PGPASSWORD: password } });
    await this.start();
  }

  async start(): Promise<void> {
    if (this.closed || this.child) throw new Error("fixture is closed or already running");
    const child = spawn(this.executable, [], {
      cwd: projectRoot,
      env: { ...process.env, BOOKMARKER_CONFIG: this.configPath, BOOKMARKER_DB_PASSWORD: password },
      stdio: "ignore",
    });
    this.child = child;
    let spawnError: Error | undefined;
    child.on("error", error => { spawnError = error; });
    for (let i = 0; i < 100; i++) {
      if (spawnError) throw new Error("fixture server failed to start");
      if (child.exitCode !== null || child.signalCode !== null) throw new Error("fixture server exited before readiness");
      try {
        const response = await fetch(`${this.baseURL}/bookmarks`, { signal: AbortSignal.timeout(1000) });
        if (response.ok) return;
      } catch { /* wait for startup */ }
      await sleep(100);
    }
    throw new Error("fixture server did not become ready");
  }

  async stop(): Promise<void> {
    const child = this.child;
    if (!child) return;
    if (child.killed && child.exitCode === null && child.signalCode === null) {
      await new Promise<void>(resolve => child.once("exit", () => resolve()));
    }
    if (child.exitCode !== null || child.signalCode !== null || child.pid === undefined) {
      this.child = undefined;
      throw new Error("fixture server terminated unexpectedly");
    }
    const exit = new Promise<void>(resolve => child.once("exit", () => resolve()));
    child.kill("SIGTERM");
    let timedOut = false;
    let timer: NodeJS.Timeout | undefined;
    await Promise.race([exit, new Promise<void>(resolve => {
      timer = setTimeout(() => { timedOut = true; resolve(); }, 15_000);
    })]);
    if (timer) clearTimeout(timer);
    if (timedOut) {
      child.kill("SIGKILL");
      await exit;
      this.child = undefined;
      throw new Error("fixture server did not terminate");
    }
    this.child = undefined;
  }

  async close(): Promise<void> {
    if (this.closed) return;
    this.closed = true;
    try { await this.stop(); } catch { /* cleanup continues after failed termination */ }
    try {
      if (this.containerStarted) await execFile("docker", ["rm", "--force", this.container]);
    } finally {
      rmSync(this.directory, { recursive: true, force: true });
    }
  }
}

export async function createRestartFixture(options: { temporaryRoot?: string; executable?: string } = {}): Promise<RestartFixture> {
  const directory = mkdtempSync(join(options.temporaryRoot ?? tmpdir(), "bookmarker-restart-"));
  const executable = options.executable ?? join(directory, "bookmarker-web");
  let fixture: RestartFixture | undefined;
  try {
    const webPort = await unusedPort();
    let postgresPort = await unusedPort();
    while (postgresPort === webPort) postgresPort = await unusedPort();
    fixture = new RestartFixture(directory, webPort, postgresPort, executable);
    if (!options.executable) await execFile("go", ["build", "-o", executable, "./cmd/bookmarker-web"], { cwd: projectRoot });
    await fixture.initialize();
    return fixture;
  } catch (error) {
    if (fixture) await fixture.close();
    else rmSync(directory, { recursive: true, force: true });
    throw error;
  }
}
