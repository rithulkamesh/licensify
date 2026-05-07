// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Rithul Kamesh
// Author: Rithul Kamesh <hi@rithul.dev>
// Description: mcp/licensify/src/handlers.ts — Pure tool handlers extracted from server.ts so they can be unit-tested in isolation.

export const KNOWN_FEATURES = ["base", "pro"] as const;
export type KnownFeature = (typeof KNOWN_FEATURES)[number];

export const TOOL_ERROR_CODES = [
  "LICENSIFY_UNAVAILABLE",
  "INVALID_ARGUMENT",
  "INITIALIZATION",
  "ACTIVATION",
  "CHECK",
  "UNKNOWN",
] as const;

export type ToolOk<T extends Record<string, unknown>> = { ok: true } & T;
export type ToolErr = {
  ok: false;
  errorCode:
    | "LICENSIFY_UNAVAILABLE"
    | "INVALID_ARGUMENT"
    | "INITIALIZATION"
    | "ACTIVATION"
    | "CHECK"
    | "UNKNOWN";
  errorMessage: string;
  hint?: string;
  retryable: boolean;
};

export type ToolResult<T extends Record<string, unknown>> = ToolOk<T> | ToolErr;

export type StatusCode = 0 | 1;
export type ErrorCode = 0 | 1 | 2 | 3 | 4;

export type LicensifyStatus = { code: StatusCode; source: "native"; timingMs: number };

export type LicensifyClientLike = {
  activate: (key: string) => Promise<void>;
  check: () => Promise<LicensifyStatus>;
  hasFeature: (feature: string) => Promise<boolean>;
};

export type SdkErrorCtor = new (...args: any[]) => Error;

export function nowMs(): number {
  return Date.now();
}

export function envString(env: NodeJS.ProcessEnv, name: string): string | undefined {
  const v = env[name];
  return typeof v === "string" && v.length > 0 ? v : undefined;
}

export function toHintFromMessage(msg: string): string | undefined {
  const m = msg.toLowerCase();
  if (m.includes("disposed") || m.includes("closed"))
    return "Recreate the Licensify client (server restart) and retry.";
  if (m.includes("ffi-napi") || m.includes("ref-napi")) {
    return "Install the Node FFI deps (ffi-napi, ref-napi) and ensure the native lib is available, then restart the MCP server.";
  }
  if (m.includes("liblicensify")) {
    return "Ensure the Licensify native shared library (liblicensify) is on your system loader path (e.g. DYLD_LIBRARY_PATH/LD_LIBRARY_PATH) and restart.";
  }
  return undefined;
}

export function statusToState(statusCode: StatusCode): {
  state: "ACTIVE" | "INACTIVE";
  description: string;
} {
  if (statusCode === 0) {
    return {
      state: "ACTIVE",
      description: "License is valid for this machine (offline-first check passed).",
    };
  }
  return {
    state: "INACTIVE",
    description:
      "License is not valid for this machine. (Current SDK returns a coarse inactive code for any non-valid state: never activated, invalid, expired, trial-expired, etc.)",
  };
}

export function unavailable(reason: string): ToolErr {
  return {
    ok: false,
    errorCode: "LICENSIFY_UNAVAILABLE",
    errorMessage: reason,
    hint:
      "Install/ship the Licensify native library (liblicensify) and Node FFI deps for the TypeScript SDK, then restart this MCP server.",
    retryable: true,
  };
}

export function mapSdkError(e: unknown, LicensifySdkError: SdkErrorCtor | null): ToolErr {
  if (LicensifySdkError && e instanceof LicensifySdkError) {
    const code = (e as any).code as ErrorCode;
    const base: Omit<ToolErr, "errorCode"> = {
      ok: false,
      errorMessage: (e as Error).message,
      hint: toHintFromMessage((e as Error).message),
      retryable: code === 2 || code === 3 || code === 4,
    };
    if ((e as Error).name === "InitializationError" || code === 2) {
      return { ...base, errorCode: "INITIALIZATION" };
    }
    if ((e as Error).name === "ActivationError" || code === 3) {
      return { ...base, errorCode: "ACTIVATION" };
    }
    if ((e as Error).name === "CheckError" || code === 4) {
      return { ...base, errorCode: "CHECK" };
    }
    if (code === 1) {
      return { ...base, errorCode: "INVALID_ARGUMENT", retryable: false };
    }
    return { ...base, errorCode: "UNKNOWN" };
  }

  const msg = e instanceof Error ? e.message : typeof e === "string" ? e : "unknown error";
  return {
    ok: false,
    errorCode: "UNKNOWN",
    errorMessage: msg,
    hint: toHintFromMessage(msg),
    retryable: true,
  };
}

export type HandlerDeps = {
  client: LicensifyClientLike | null;
  initError: string | null;
  serverUrl: string;
  cachePath: string;
  sdkVersion: string;
  licensifySdkError: SdkErrorCtor | null;
  clock: { nowMs: () => number };
};

export type HealthOk = {
  ok: true;
  sdkVersion: string;
  clientInitialized: boolean;
  serverUrl: string;
  cachePath: string;
  initError: string | null;
  lastSuccessfulCheckMs: number | null;
};

export type ActivateOk = { ok: true; activated: true; activationTimingMs: number };

export type CheckOk = {
  ok: true;
  statusCode: StatusCode;
  state: "ACTIVE" | "INACTIVE";
  description: string;
  source: "native";
  timingMs: number;
  lastSuccessfulCheckMs: number;
};

export type HasFeatureOk = { ok: true; feature: string; available: boolean; reasoning: string };

export type SummaryOk = {
  ok: true;
  state: "ACTIVE" | "INACTIVE";
  statusCode: StatusCode;
  description: string;
  feature_base: boolean;
  feature_pro: boolean;
  availableFeatures: KnownFeature[];
  missingFeatures: KnownFeature[];
  lastSuccessfulCheckMs: number;
};

export class Handlers {
  private lastSuccessfulCheckMs: number | null = null;

  constructor(private readonly deps: HandlerDeps) {}

  health(): HealthOk {
    if (this.deps.client !== null) {
      return {
        ok: true,
        sdkVersion: this.deps.sdkVersion,
        clientInitialized: true,
        serverUrl: this.deps.serverUrl,
        cachePath: this.deps.cachePath,
        initError: null,
        lastSuccessfulCheckMs: this.lastSuccessfulCheckMs,
      };
    }
    return {
      ok: true,
      sdkVersion: this.deps.sdkVersion,
      clientInitialized: false,
      serverUrl: this.deps.serverUrl,
      cachePath: this.deps.cachePath,
      initError: this.deps.initError ?? "unknown initialization error",
      lastSuccessfulCheckMs: this.lastSuccessfulCheckMs,
    };
  }

  async activate(licenseKey: unknown): Promise<ToolResult<ActivateOk>> {
    if (!this.deps.client) {
      return unavailable(this.deps.initError ?? "Licensify client failed to initialize");
    }
    if (typeof licenseKey !== "string" || licenseKey.length === 0) {
      return {
        ok: false,
        errorCode: "INVALID_ARGUMENT",
        errorMessage: "licenseKey is required",
        hint:
          "Ask the user for a non-empty license key (often shown in their account or purchase email).",
        retryable: false,
      };
    }
    const start = this.deps.clock.nowMs();
    try {
      await this.deps.client.activate(licenseKey);
      return { ok: true, activated: true, activationTimingMs: this.deps.clock.nowMs() - start };
    } catch (e) {
      const mapped = mapSdkError(e, this.deps.licensifySdkError);
      return { ...mapped, hint: mapped.hint ?? "Verify the key is correct, then retry activation." };
    }
  }

  async check(): Promise<ToolResult<CheckOk>> {
    if (!this.deps.client) {
      return unavailable(this.deps.initError ?? "Licensify client failed to initialize");
    }
    try {
      const st = await this.deps.client.check();
      const mapped = statusToState(st.code);
      this.lastSuccessfulCheckMs = this.deps.clock.nowMs();
      return {
        ok: true,
        statusCode: st.code,
        state: mapped.state,
        description: mapped.description,
        source: st.source,
        timingMs: st.timingMs,
        lastSuccessfulCheckMs: this.lastSuccessfulCheckMs,
      };
    } catch (e) {
      const mapped = mapSdkError(e, this.deps.licensifySdkError);
      return {
        ...mapped,
        hint:
          mapped.hint ??
          "If this is the first run, activate a license key. Otherwise, ensure the cache path is writable and not corrupted; then retry.",
      };
    }
  }

  async hasFeature(feature: unknown): Promise<ToolResult<HasFeatureOk>> {
    if (!this.deps.client) {
      return unavailable(this.deps.initError ?? "Licensify client failed to initialize");
    }
    if (typeof feature !== "string" || feature.length === 0) {
      return {
        ok: false,
        errorCode: "INVALID_ARGUMENT",
        errorMessage: "feature is required",
        hint: "Pick a non-empty feature name (known examples: base, pro).",
        retryable: false,
      };
    }
    try {
      const available = await this.deps.client.hasFeature(feature);
      return {
        ok: true,
        feature,
        available,
        reasoning: available
          ? `Entitlement '${feature}' is present; you may proceed with the gated workflow.`
          : `Entitlement '${feature}' is not present; do not proceed with the gated workflow.`,
      };
    } catch (e) {
      return mapSdkError(e, this.deps.licensifySdkError);
    }
  }

  async summary(): Promise<ToolResult<SummaryOk>> {
    if (!this.deps.client) {
      return unavailable(this.deps.initError ?? "Licensify client failed to initialize");
    }
    try {
      const st = await this.deps.client.check();
      const mapped = statusToState(st.code);
      this.lastSuccessfulCheckMs = this.deps.clock.nowMs();
      const feature_base = await this.deps.client.hasFeature("base");
      const feature_pro = await this.deps.client.hasFeature("pro");
      const available: KnownFeature[] = [];
      const missing: KnownFeature[] = [];
      if (feature_base) available.push("base");
      else missing.push("base");
      if (feature_pro) available.push("pro");
      else missing.push("pro");
      return {
        ok: true,
        state: mapped.state,
        statusCode: st.code,
        description: mapped.description,
        feature_base,
        feature_pro,
        availableFeatures: available,
        missingFeatures: missing,
        lastSuccessfulCheckMs: this.lastSuccessfulCheckMs,
      };
    } catch (e) {
      const mapped = mapSdkError(e, this.deps.licensifySdkError);
      return {
        ...mapped,
        hint:
          mapped.hint ??
          "If this is the first run, activate a license key. Otherwise, fix the environment/cache path and retry.",
      };
    }
  }
}
