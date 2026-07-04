import { spawn, type ChildProcess } from "child_process";
import { existsSync, mkdirSync, openSync } from "fs";
import { homedir } from "os";
import { dirname, join, resolve } from "path";
import * as net from "net";

const DEFAULT_DOCS_PORT = 4000;
const DOCS_START_TIMEOUT_MS = 30_000;

let docsProcess: ChildProcess | null = null;

export async function ensureDocsDevServer(): Promise<void> {
  const port = docsPort();
  if (await isPortListening(port)) {
    console.log(`[docs] dev server already listening on ${port}`);
    return;
  }

  const root = repoRoot();
  const logPath = join(homedir(), ".multicaa", "logs", "docs-dev.log");
  mkdirSync(dirname(logPath), { recursive: true });
  const out = openSync(logPath, "a");

  docsProcess = spawn("pnpm", ["dev:docs"], {
    cwd: root,
    detached: false,
    env: {
      ...process.env,
      DOCS_PORT: String(port),
    },
    stdio: ["ignore", out, out],
  });

  docsProcess.on("exit", (code, signal) => {
    console.log(`[docs] dev server exited code=${code ?? "null"} signal=${signal ?? "null"}`);
    docsProcess = null;
  });

  console.log(`[docs] starting dev server on ${port}; logs: ${logPath}`);
  await waitForPort(port, DOCS_START_TIMEOUT_MS);
  console.log(`[docs] dev server ready on ${port}`);
}

export function stopDocsDevServer(): void {
  if (!docsProcess || docsProcess.killed) return;
  docsProcess.kill();
  docsProcess = null;
}

function docsPort(): number {
  const raw = process.env.DOCS_PORT;
  if (!raw) return DEFAULT_DOCS_PORT;
  const parsed = Number.parseInt(raw, 10);
  return Number.isFinite(parsed) && parsed > 0 ? parsed : DEFAULT_DOCS_PORT;
}

function repoRoot(): string {
  const explicit = process.env.SIMULTICA_REPO_ROOT;
  if (explicit && isRepoRoot(explicit)) return explicit;

  let dir = process.cwd();
  for (let i = 0; i < 8; i += 1) {
    if (isRepoRoot(dir)) return dir;
    const parent = resolve(dir, "..");
    if (parent === dir) break;
    dir = parent;
  }

  const sourceRelativeRoot = resolve(__dirname, "../../../..");
  if (isRepoRoot(sourceRelativeRoot)) return sourceRelativeRoot;

  const localWorkspaceRoot = "/Users/bytedance/agent_workspace/simultica";
  if (isRepoRoot(localWorkspaceRoot)) return localWorkspaceRoot;

  throw new Error(
    "could not find Simultica repo root for docs server; set SIMULTICA_REPO_ROOT",
  );
}

function isRepoRoot(dir: string): boolean {
  return (
    existsSync(join(dir, "pnpm-workspace.yaml")) &&
    existsSync(join(dir, "apps", "docs", "package.json"))
  );
}

function isPortListening(port: number): Promise<boolean> {
  return new Promise((resolve) => {
    const socket = net.createConnection({ host: "127.0.0.1", port });
    socket.once("connect", () => {
      socket.destroy();
      resolve(true);
    });
    socket.once("error", () => {
      socket.destroy();
      resolve(false);
    });
    socket.setTimeout(1_000, () => {
      socket.destroy();
      resolve(false);
    });
  });
}

async function waitForPort(port: number, timeoutMs: number): Promise<void> {
  const startedAt = Date.now();
  while (Date.now() - startedAt < timeoutMs) {
    if (await isPortListening(port)) return;
    await new Promise((resolve) => setTimeout(resolve, 500));
  }
  throw new Error(`docs dev server did not listen on ${port} within ${timeoutMs}ms`);
}
