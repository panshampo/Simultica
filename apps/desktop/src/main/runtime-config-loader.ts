import { app } from "electron";
import { mkdir, readFile, rename, rm, writeFile } from "fs/promises";
import { dirname, join } from "path";
import {
  DEFAULT_RUNTIME_CONFIG,
  parseRuntimeConfig,
  runtimeConfigFromInput,
  runtimeConfigFromDevEnv,
  type RuntimeConfig,
  type RuntimeConfigEnv,
  type RuntimeConfigInput,
  type RuntimeConfigResult,
  type RuntimeConfigSaveResult,
} from "../shared/runtime-config";

export async function loadRuntimeConfig(options: {
  isDev: boolean;
  env: RuntimeConfigEnv;
  configPath?: string;
}): Promise<RuntimeConfigResult> {
  if (options.isDev) {
    try {
      return { ok: true, config: runtimeConfigFromDevEnv(options.env) };
    } catch (err) {
      return { ok: false, error: { message: errorMessage(err) } };
    }
  }

  const configPath = options.configPath ?? desktopConfigPath();
  try {
    const raw = await readFile(configPath, "utf-8");
    return { ok: true, config: parseRuntimeConfig(raw) };
  } catch (err) {
    if (isMissingFileError(err)) {
      return { ok: true, config: { ...DEFAULT_RUNTIME_CONFIG } };
    }
    return {
      ok: false,
      error: {
        message: `Invalid ${configPath}: ${errorMessage(err)}`,
      },
    };
  }
}

export function desktopConfigPath(): string {
  return join(app.getPath("home"), ".multicaa", "desktop.json");
}

export async function saveRuntimeConfig(
  input: RuntimeConfigInput,
  configPath = desktopConfigPath(),
): Promise<RuntimeConfigSaveResult> {
  let config: RuntimeConfig;
  try {
    config = runtimeConfigFromInput(input);
  } catch (err) {
    return { ok: false, error: errorMessage(err) };
  }

  try {
    await writeConfigFile(configPath, config);
    return { ok: true, config, path: configPath };
  } catch (err) {
    return {
      ok: false,
      error: `Failed to write ${configPath}: ${errorMessage(err)}`,
    };
  }
}

async function writeConfigFile(
  configPath: string,
  config: RuntimeConfig,
): Promise<void> {
  const dir = dirname(configPath);
  await mkdir(dir, { recursive: true });
  const tmp = join(
    dir,
    `.desktop.json.${process.pid}.${Date.now()}.tmp`,
  );
  try {
    await writeFile(tmp, `${JSON.stringify(config, null, 2)}\n`, "utf-8");
    await rename(tmp, configPath);
  } catch (err) {
    await rm(tmp, { force: true }).catch(() => {});
    throw err;
  }
}

function isMissingFileError(err: unknown): boolean {
  return Boolean(
    err &&
      typeof err === "object" &&
      "code" in err &&
      (err as NodeJS.ErrnoException).code === "ENOENT",
  );
}

function errorMessage(err: unknown): string {
  return err instanceof Error ? err.message : String(err);
}

export type { RuntimeConfig, RuntimeConfigInput, RuntimeConfigResult };
