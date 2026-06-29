import { mkdtemp, readFile, writeFile } from "fs/promises";
import { join } from "path";
import { tmpdir } from "os";
import { describe, expect, it } from "vitest";
import { loadRuntimeConfig, saveRuntimeConfig } from "./runtime-config-loader";

describe("loadRuntimeConfig", () => {
  it("uses dev env and ignores desktop.json during electron-vite dev", async () => {
    const dir = await mkdtemp(join(tmpdir(), "multica-desktop-config-"));
    const configPath = join(dir, "desktop.json");
    await writeFile(
      configPath,
      JSON.stringify({ schemaVersion: 1, apiUrl: "https://prod.example.com" }),
    );

    await expect(
      loadRuntimeConfig({
        isDev: true,
        configPath,
        env: {
          apiUrl: "http://localhost:8080",
          wsUrl: "ws://localhost:8080/ws",
          appUrl: "http://localhost:3000",
        },
      }),
    ).resolves.toEqual({
      ok: true,
      config: {
        schemaVersion: 1,
        apiUrl: "http://localhost:8080",
        wsUrl: "ws://localhost:8080/ws",
        appUrl: "http://localhost:3000",
      },
    });
  });

  it("uses cloud defaults when packaged config is absent", async () => {
    const dir = await mkdtemp(join(tmpdir(), "multica-desktop-config-"));
    await expect(
      loadRuntimeConfig({
        isDev: false,
        configPath: join(dir, "missing.json"),
        env: {},
      }),
    ).resolves.toEqual({
      ok: true,
      config: {
        schemaVersion: 1,
        apiUrl: "https://api.multica.ai",
        wsUrl: "wss://api.multica.ai/ws",
        appUrl: "https://multica.ai",
      },
    });
  });

  it("parses a valid packaged desktop.json", async () => {
    const dir = await mkdtemp(join(tmpdir(), "multica-desktop-config-"));
    const configPath = join(dir, "desktop.json");
    await writeFile(
      configPath,
      JSON.stringify({ schemaVersion: 1, apiUrl: "https://api.example.com" }),
    );

    await expect(
      loadRuntimeConfig({ isDev: false, configPath, env: {} }),
    ).resolves.toEqual({
      ok: true,
      config: {
        schemaVersion: 1,
        apiUrl: "https://api.example.com",
        wsUrl: "wss://api.example.com/ws",
        appUrl: "https://example.com",
      },
    });
  });

  it("fails closed when packaged desktop.json is invalid", async () => {
    const dir = await mkdtemp(join(tmpdir(), "multica-desktop-config-"));
    const configPath = join(dir, "desktop.json");
    await writeFile(configPath, "{");

    const result = await loadRuntimeConfig({ isDev: false, configPath, env: {} });

    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.error.message).toContain(configPath);
      expect(result.error.message).toContain("Invalid desktop runtime config JSON");
    }
  });

  it("saves a normalized packaged desktop.json", async () => {
    const dir = await mkdtemp(join(tmpdir(), "multica-desktop-config-"));
    const configPath = join(dir, "nested", "desktop.json");

    const result = await saveRuntimeConfig(
      { apiUrl: " http://192.0.2.10/ " },
      configPath,
    );

    expect(result).toEqual({
      ok: true,
      path: configPath,
      config: {
        schemaVersion: 1,
        apiUrl: "http://192.0.2.10",
        wsUrl: "ws://192.0.2.10/ws",
        appUrl: "http://192.0.2.10",
      },
    });
    await expect(readFile(configPath, "utf-8")).resolves.toBe(
      `${JSON.stringify(
        {
          schemaVersion: 1,
          apiUrl: "http://192.0.2.10",
          wsUrl: "ws://192.0.2.10/ws",
          appUrl: "http://192.0.2.10",
        },
        null,
        2,
      )}\n`,
    );
  });

  it("saves split self-hosted API and web app URLs", async () => {
    const dir = await mkdtemp(join(tmpdir(), "multica-desktop-config-"));
    const configPath = join(dir, "desktop.json");

    const result = await saveRuntimeConfig(
      {
        apiUrl: " http://192.0.2.10:8080/ ",
        appUrl: " http://192.0.2.10:3000/ ",
      },
      configPath,
    );

    expect(result).toEqual({
      ok: true,
      path: configPath,
      config: {
        schemaVersion: 1,
        apiUrl: "http://192.0.2.10:8080",
        wsUrl: "ws://192.0.2.10:8080/ws",
        appUrl: "http://192.0.2.10:3000",
      },
    });
    await expect(readFile(configPath, "utf-8")).resolves.toBe(
      `${JSON.stringify(
        {
          schemaVersion: 1,
          apiUrl: "http://192.0.2.10:8080",
          wsUrl: "ws://192.0.2.10:8080/ws",
          appUrl: "http://192.0.2.10:3000",
        },
        null,
        2,
      )}\n`,
    );
  });

  it("returns a save error without writing invalid desktop config", async () => {
    const dir = await mkdtemp(join(tmpdir(), "multica-desktop-config-"));
    const configPath = join(dir, "desktop.json");

    const result = await saveRuntimeConfig(
      { apiUrl: "file:///tmp/multica" },
      configPath,
    );

    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.error).toContain("apiUrl must use http or https");
    }
    await expect(readFile(configPath, "utf-8")).rejects.toMatchObject({
      code: "ENOENT",
    });
  });
});
