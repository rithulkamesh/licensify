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

import type { Config, NativeBindings, Status, ErrorCode } from "./types.js";

declare const require: (name: string) => any;

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

function loadNative(): NativeBindings {
  const ffi = require("ffi-napi");
  const ref = require("ref-napi");
  const voidPtr = ref.refType(ref.types.void);

  // We avoid returning structs across FFI boundaries for robustness.
  const lib = ffi.Library("liblicensify", {
    licensify_new: [voidPtr, [voidPtr]],
    licensify_free: ["void", [voidPtr]],
    licensify_activate_code: ["int", [voidPtr, "string"]],
    licensify_check_code: ["int", [voidPtr, voidPtr]],
    licensify_has_feature: ["bool", [voidPtr, "string"]],
    licensify_last_error: ["string", [voidPtr]],
  });

  return {
    newClient(serverUrl: string, cachePath: string) {
      // The current native ABI expects a `licensify_config_t*`, but we don't have a portable
      // struct builder without extra deps. We keep the existing behavior (zeroed config) and
      // rely on serverUrl/cachePath being wired in native in future.
      // For now, create a fixed-size buffer to satisfy the call shape.
      // eslint-disable-next-line @typescript-eslint/no-unused-vars
      const _ = { serverUrl, cachePath };
      return lib.licensify_new(Buffer.alloc(16)) as Buffer;
    },
    free(ptr: Buffer) {
      lib.licensify_free(ptr);
    },
    activateCode(ptr: Buffer, key: string) {
      return lib.licensify_activate_code(ptr, key) as ErrorCode;
    },
    checkCode(ptr: Buffer) {
      const out = Buffer.alloc(4);
      const code = lib.licensify_check_code(ptr, out) as ErrorCode;
      const status = out.readInt32LE(0) as 0 | 1;
      return { code, status };
    },
    hasFeature(ptr: Buffer, feature: string) {
      return lib.licensify_has_feature(ptr, feature) as boolean;
    },
    lastError(ptr: Buffer) {
      try {
        const s = lib.licensify_last_error(ptr) as string;
        return s && s.length > 0 ? s : null;
      } catch {
        return null;
      }
    },
  };
}

export class LicensifyClient {
  private readonly native: NativeBindings;
  private readonly clock: { nowMs: () => number };
  private readonly logger: Config["logger"];
  private ptr: Buffer | null;

  private constructor(private readonly config: Config, native: NativeBindings, ptr: Buffer) {
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
    const native = config.native ?? loadNative();
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

  private requireOpen(): Buffer {
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
