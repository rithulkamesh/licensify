// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Rithul Kamesh
// Author: Rithul Kamesh <hi@rithul.dev>
// Description: sdk/typescript/src/index.ts — TypeScript SDK implementation wrapping the native client.

/**
 * Licensify TypeScript SDK (Node.js)
 *
 * Wraps the Licensify native client via `ffi-napi`/`ref-napi`.
 * - Typed errors with error codes + context
 * - Strict input validation at public boundaries
 * - Deterministic native resource cleanup via `dispose()`
 * - Optional structured logger hooks
 * - Injectable native bindings + clock for unit tests
 *
 * Minimal usage:
 *
 * ```ts
 * const client = await LicensifyClient.create({ serverUrl, cachePath });
 * await client.activate("LICENSE-KEY");
 * const status = await client.check();
 * client.dispose();
 * ```
 */

import { createRequire } from "node:module";
import type { Config, NativeBindings, Status, ErrorCode } from "./types.js";

const require = createRequire(import.meta.url);
export class LicensifySdkError extends Error {
  public readonly code: ErrorCode;
  public readonly metadata?: Record<string, unknown>;
  constructor(name: string, code: ErrorCode, message: string, metadata?: Record<string, unknown>) {
    super(message);
    this.name = name;
    this.code = code;
    this.metadata = metadata;
  }
}

export class InitializationError extends LicensifySdkError {
  constructor(message: string, metadata?: Record<string, unknown>) {
    super("InitializationError", 2, message, metadata);
  }
}

export class ActivationError extends LicensifySdkError {
  constructor(code: ErrorCode, message: string, metadata?: Record<string, unknown>) {
    super("ActivationError", code, message, metadata);
  }
}

export class CheckError extends LicensifySdkError {
  constructor(code: ErrorCode, message: string, metadata?: Record<string, unknown>) {
    super("CheckError", code, message, metadata);
  }
}

function defaultClock() {
  return { nowMs: () => Date.now() };
}

/* c8 ignore start -- loadNative() requires `koffi` + the native shared lib;
 * we exercise it from a separate native-only e2e test gated on LICENSIFY_NATIVE=1.
 */
function loadNative(): NativeBindings {
  const koffi = require("koffi");
  const nodePath = require("path");
  const nodeUrl = require("url");
  const fs = require("fs");

  const libFile = process.platform === "darwin" ? "liblicensify.dylib" : "liblicensify.so";
  const __dirname = nodePath.dirname(nodeUrl.fileURLToPath(import.meta.url));
  const targetDir = nodePath.resolve(__dirname, "..", "..", "..", "target");

  let libPath = "liblicensify";
  for (const subdir of ["debug", "release"]) {
    const candidate = nodePath.join(targetDir, subdir, libFile);
    if (fs.existsSync(candidate)) {
      libPath = candidate;
      break;
    }
  }

  const lib = koffi.load(libPath);

  koffi.struct("licensify_config_t", {
    server_url: "const char *",
    cache_path: "const char *",
  });

  const licensify_new = lib.func("licensify_new", "void *", ["licensify_config_t *"]);
  const licensify_free = lib.func("licensify_free", "void", ["void *"]);
  const licensify_activate_code = lib.func("licensify_activate_code", "int", ["void *", "const char *"]);
  const licensify_check_code = lib.func("licensify_check_code", "int", ["void *", "int *"]);
  const licensify_has_feature = lib.func("licensify_has_feature", "bool", ["void *", "const char *"]);
  const licensify_last_error = lib.func("licensify_last_error", "const char *", ["void *"]);

  return {
    newClient(serverUrl: string, cachePath: string) {
      const cfg = { server_url: serverUrl, cache_path: cachePath };
      return licensify_new(cfg);
    },
    free(ptr: unknown) {
      licensify_free(ptr);
    },
    activateCode(ptr: unknown, key: string) {
      return licensify_activate_code(ptr, key) as ErrorCode;
    },
    checkCode(ptr: unknown) {
      const out = new Int32Array(1);
      const code = licensify_check_code(ptr, out) as ErrorCode;
      return { code, status: out[0] as 0 | 1 };
    },
    hasFeature(ptr: unknown, feature: string) {
      return licensify_has_feature(ptr, feature) as boolean;
    },
    lastError(ptr: unknown) {
      try {
        const s = licensify_last_error(ptr) as string;
        return s && s.length > 0 ? s : null;
      } catch {
        return null;
      }
    },
  };
}
/* c8 ignore stop */

export class LicensifyClient {
  private readonly native: NativeBindings;
  private readonly clock: { nowMs: () => number };
  private readonly logger: Config["logger"];
  private ptr: unknown;

  private constructor(private readonly config: Config, native: NativeBindings, ptr: unknown) {
    this.native = native;
    this.clock = config.clock ?? defaultClock();
    this.logger = config.logger;
    this.ptr = ptr;
  }

  /**
   * Create a client instance. Use this instead of `new` because native initialization can fail.
   */
  static async create(config: Config): Promise<LicensifyClient> {
    if (!config || typeof config !== "object") throw new InitializationError("config is required");
    if (typeof config.serverUrl !== "string" || config.serverUrl.length === 0) {
      throw new InitializationError("serverUrl is required");
    }
    if (typeof config.cachePath !== "string" || config.cachePath.length === 0) {
      throw new InitializationError("cachePath is required");
    }
    const native = config.native ?? /* c8 ignore next */ loadNative();
    const ptr = native.newClient(config.serverUrl, config.cachePath);
    if (!ptr) throw new InitializationError("native returned a null client pointer");
    return new LicensifyClient(config, native, ptr);
  }

  /**
   * Dispose native resources. Safe to call multiple times.
   */
  dispose(): void {
    if (this.ptr) {
      this.native.free(this.ptr);
      this.ptr = null;
    }
  }

  private requireOpen(): unknown {
    if (!this.ptr) throw new InitializationError("client is disposed");
    return this.ptr;
  }

  /**
   * Activate a license key on this machine.
   *
   * Throws `ActivationError` on failure.
   */
  async activate(key: string): Promise<void> {
    if (typeof key !== "string" || key.length === 0) throw new ActivationError(1, "license key is required");
    const ptr = this.requireOpen();
    const start = this.clock.nowMs();
    const code = this.native.activateCode(ptr, key);
    const timingMs = this.clock.nowMs() - start;
    if (code !== 0) {
      const msg = this.native.lastError(ptr) ?? "activation failed";
      this.logger?.error("licensify.activate failed", { code, timingMs });
      throw new ActivationError(code, msg, { timingMs });
    }
    this.logger?.info("licensify.activate ok", { timingMs });
  }

  /**
   * Check current license status (offline-first in the native client).
   *
   * Throws `CheckError` on failure.
   */
  async check(): Promise<Status> {
    const ptr = this.requireOpen();
    const start = this.clock.nowMs();
    const { code, status } = this.native.checkCode(ptr);
    const timingMs = this.clock.nowMs() - start;
    if (code !== 0) {
      const msg = this.native.lastError(ptr) ?? "check failed";
      this.logger?.error("licensify.check failed", { code, timingMs });
      throw new CheckError(code, msg, { timingMs });
    }
    this.logger?.debug("licensify.check ok", { timingMs, status });
    return { code: status, source: "native", timingMs };
  }

  /**
   * Returns true if a named entitlement/feature flag is present.
   */
  async hasFeature(feature: string): Promise<boolean> {
    if (typeof feature !== "string" || feature.length === 0) return false;
    const ptr = this.requireOpen();
    return this.native.hasFeature(ptr, feature);
  }
}
