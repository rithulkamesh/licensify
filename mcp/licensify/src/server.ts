// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Rithul Kamesh
// Author: Rithul Kamesh <hi@rithul.dev>
// Description: mcp/licensify/src/server.ts — MCP server implementation for integrating Licensify into agent workflows.

/**
 * Licensify MCP server
 *
 * We initialize a single Licensify client once at startup (not per tool call) to:
 * - avoid repeated native library loads
 * - keep a stable, single token cache context per server process
 *
 * If native initialization fails (missing shared library, missing ffi deps, etc),
 * we still start the MCP server and return `LICENSIFY_UNAVAILABLE` from every tool.
 */

import { McpServer, StdioServerTransport } from "@modelcontextprotocol/server";
import * as z from "zod/v4";

import { LICENSIFY_GUIDE_MARKDOWN } from "./guide.js";

type KnownFeature = "base" | "pro";
const KNOWN_FEATURES: readonly KnownFeature[] = ["base", "pro"] as const;
const TOOL_ERROR_CODES = [
  "LICENSIFY_UNAVAILABLE",
  "INVALID_ARGUMENT",
  "INITIALIZATION",
  "ACTIVATION",
  "CHECK",
  "UNKNOWN",
] as const;

type ToolOk<T extends Record<string, unknown>> = { ok: true } & T;
type ToolErr = {
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

type ToolResult<T extends Record<string, unknown>> = ToolOk<T> | ToolErr;

type StatusCode = 0 | 1;
type ErrorCode = 0 | 1 | 2 | 3 | 4;

type LicensifyStatus = { code: StatusCode; source: "native"; timingMs: number };

type LicensifyClientLike = {
  activate: (key: string) => Promise<void>;
  check: () => Promise<LicensifyStatus>;
  hasFeature: (feature: string) => Promise<boolean>;
};

type SdkModule = {
  LicensifyClient: { create: (cfg: { serverUrl: string; cachePath: string }) => Promise<LicensifyClientLike> };
  LicensifySdkError: new (...args: any[]) => Error;
};

function nowMs(): number {
  return Date.now();
}

function envString(name: string): string | undefined {
  const v = process.env[name];
  return typeof v === "string" && v.length > 0 ? v : undefined;
}

function toHintFromMessage(msg: string): string | undefined {
  const m = msg.toLowerCase();
  if (m.includes("disposed") || m.includes("closed")) return "Recreate the Licensify client (server restart) and retry.";
  if (m.includes("ffi-napi") || m.includes("ref-napi")) {
    return "Install the Node FFI deps (ffi-napi, ref-napi) and ensure the native lib is available, then restart the MCP server.";
  }
  if (m.includes("liblicensify")) {
    return "Ensure the Licensify native shared library (liblicensify) is on your system loader path (e.g. DYLD_LIBRARY_PATH/LD_LIBRARY_PATH) and restart.";
  }
  return undefined;
}

function mapSdkError(e: unknown, LicensifySdkError: SdkModule["LicensifySdkError"] | null): ToolErr {
  if (LicensifySdkError && e instanceof LicensifySdkError) {
    const code = (e as any).code as ErrorCode;
    const base: Omit<ToolErr, "errorCode"> = {
      ok: false,
      errorMessage: (e as Error).message,
      hint: toHintFromMessage((e as Error).message),
      retryable: code === 2 || code === 3 || code === 4,
    };
    if ((e as Error).name === "InitializationError" || code === 2) return { ...base, errorCode: "INITIALIZATION" };
    if ((e as Error).name === "ActivationError" || code === 3) return { ...base, errorCode: "ACTIVATION" };
    if ((e as Error).name === "CheckError" || code === 4) return { ...base, errorCode: "CHECK" };
    if (code === 1) return { ...base, errorCode: "INVALID_ARGUMENT", retryable: false };
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

function unavailable(reason: string): ToolErr {
  return {
    ok: false,
    errorCode: "LICENSIFY_UNAVAILABLE",
    errorMessage: reason,
    hint:
      "Install/ship the Licensify native library (liblicensify) and Node FFI deps for the TypeScript SDK, then restart this MCP server.",
    retryable: true,
  };
}

function statusToState(statusCode: StatusCode): { state: "ACTIVE" | "INACTIVE"; description: string } {
  // Source-derived behavior:
  // - Rust FFI sets out_status_code=0 only for LicenseStatus::Valid
  // - all other statuses collapse to out_status_code=1
  if (statusCode === 0) return { state: "ACTIVE", description: "License is valid for this machine (offline-first check passed)." };
  return {
    state: "INACTIVE",
    description:
      "License is not valid for this machine. (Current SDK returns a coarse inactive code for any non-valid state: never activated, invalid, expired, trial-expired, etc.)",
  };
}

async function main() {
  const sdkVersion = "0.1.0";

  const server = new McpServer(
    { name: "licensify", version: sdkVersion },
    {
      instructions:
        "Use licensify_check before premium workflows. Use licensify_has_feature to gate specific premium features. If any tool returns ok=false with errorCode=LICENSIFY_UNAVAILABLE, stop and guide the user to install/ship the native lib and restart the server.",
    }
  );

  const serverUrl = envString("LICENSIFY_SERVER_URL") ?? "http://localhost:8080";
  const cachePath = envString("LICENSIFY_CACHE_PATH") ?? "/tmp/licensify.token";

  let sdk: SdkModule | null = null;
  let client: LicensifyClientLike | null = null;
  let initError: string | null = null;
  let lastSuccessfulCheckMs: number | null = null;

  try {
    // Prefer in-repo build output (deterministic), then fall back to a published package if installed.
    // This keeps the MCP server process alive even when native deps are missing.
    sdk =
      (await import("../../../sdk/typescript/dist/index.js").catch(() => null)) ??
      (await import("@licensify/sdk").catch(() => null));
    if (!sdk) throw new Error("Licensify SDK not found. Build `sdk/typescript` or install `@licensify/sdk`.");
    client = await sdk.LicensifyClient.create({ serverUrl, cachePath });
  } catch (e) {
    initError = e instanceof Error ? e.message : String(e);
  }

  server.registerResource(
    "licensify_guide",
    "licensify://guide",
    { title: "Licensify guide", description: "Mental model and recovery guidance for Licensify activation, checks, and feature gates." },
    async () => ({
      contents: [
        {
          uri: "licensify://guide",
          mimeType: "text/markdown",
          text: LICENSIFY_GUIDE_MARKDOWN,
        },
      ],
    })
  );

  server.registerTool(
    "licensify_health",
    {
      title: "Licensify health",
      description:
        "Call this first to verify the Licensify native client is available before relying on any license-gated workflow. If clientInitialized is false, all Licensify tools will return LICENSIFY_UNAVAILABLE until the environment is fixed.",
      inputSchema: z.object({}),
      outputSchema: z.object({
        ok: z.boolean(),
        sdkVersion: z.string().optional(),
        clientInitialized: z.boolean().optional(),
        serverUrl: z.string().optional(),
        cachePath: z.string().optional(),
        initError: z.string().optional(),
        lastSuccessfulCheckMs: z.number().int().nullable().optional(),
        errorCode: z.enum(TOOL_ERROR_CODES).optional(),
        errorMessage: z.string().optional(),
        hint: z.string().optional(),
        retryable: z.boolean().optional(),
      }),
    },
    async (): Promise<any> => {
      const out: ToolResult<{
        sdkVersion: string;
        clientInitialized: boolean;
        serverUrl: string;
        cachePath: string;
        initError: string | null;
        lastSuccessfulCheckMs: number | null;
      }> =
        client !== null
          ? {
              ok: true,
              sdkVersion,
              clientInitialized: true,
              serverUrl,
              cachePath,
              initError: null,
              lastSuccessfulCheckMs,
            }
          : {
              ok: true,
              sdkVersion,
              clientInitialized: false,
              serverUrl,
              cachePath,
              initError: initError ?? "unknown initialization error",
              lastSuccessfulCheckMs,
            };

      return { content: [{ type: "text", text: JSON.stringify(out) }], structuredContent: out };
    }
  );

  server.registerTool(
    "licensify_activate",
    {
      title: "Activate a license key",
      description:
        "Call this when the user provides a license key (e.g. after purchase or onboarding) to bind it to the current machine. Do not call repeatedly in a loop; if it fails, surface the hint and ask the user to fix the environment/key.",
      inputSchema: z.object({
        licenseKey: z.string().min(1).describe("The license key string provided to the user (must be non-empty)."),
      }),
      outputSchema: z.object({
        ok: z.boolean(),
        activated: z.boolean().optional(),
        activationTimingMs: z.number().int().optional(),
        errorCode: z.enum(TOOL_ERROR_CODES).optional(),
        errorMessage: z.string().optional(),
        hint: z.string().optional(),
        retryable: z.boolean().optional(),
      }),
    },
    async ({ licenseKey }): Promise<any> => {
      if (!client) {
        const out = unavailable(initError ?? "Licensify client failed to initialize");
        return { content: [{ type: "text", text: JSON.stringify(out) }], structuredContent: out };
      }
      if (typeof licenseKey !== "string" || licenseKey.length === 0) {
        const out: ToolErr = {
          ok: false,
          errorCode: "INVALID_ARGUMENT",
          errorMessage: "licenseKey is required",
          hint: "Ask the user for a non-empty license key (often shown in their account or purchase email).",
          retryable: false,
        };
        return { content: [{ type: "text", text: JSON.stringify(out) }], structuredContent: out };
      }

      const start = nowMs();
      try {
        await client.activate(licenseKey);
        const out: ToolOk<{ activated: true; activationTimingMs: number }> = {
          ok: true,
          activated: true,
          activationTimingMs: nowMs() - start,
        };
        return { content: [{ type: "text", text: JSON.stringify(out) }], structuredContent: out };
      } catch (e) {
        const mapped = mapSdkError(e, sdk?.LicensifySdkError ?? null);
        const out: ToolErr = { ...mapped, hint: mapped.hint ?? "Verify the key is correct, then retry activation." };
        return { content: [{ type: "text", text: JSON.stringify(out) }], structuredContent: out };
      }
    }
  );

  server.registerTool(
    "licensify_check",
    {
      title: "Check license status",
      description:
        "Call this on startup and before any premium workflow to verify the current machine is licensed. If state is not ACTIVE, do not proceed with premium actions; instead follow the hint to recover (usually activate a key or fix environment/cache).",
      inputSchema: z.object({}),
      outputSchema: z.object({
        ok: z.boolean(),
        statusCode: z.union([z.literal(0), z.literal(1)]).optional(),
        state: z.enum(["ACTIVE", "INACTIVE"]).optional(),
        description: z.string().optional(),
        source: z.literal("native").optional(),
        timingMs: z.number().int().optional(),
        lastSuccessfulCheckMs: z.number().int().nullable().optional(),
        errorCode: z.enum(TOOL_ERROR_CODES).optional(),
        errorMessage: z.string().optional(),
        hint: z.string().optional(),
        retryable: z.boolean().optional(),
      }),
    },
    async (): Promise<any> => {
      if (!client) {
        const out = unavailable(initError ?? "Licensify client failed to initialize");
        return { content: [{ type: "text", text: JSON.stringify(out) }], structuredContent: out };
      }
      try {
        const st = await client.check();
        const mapped = statusToState(st.code);
        lastSuccessfulCheckMs = nowMs();
        const out: ToolOk<{
          statusCode: StatusCode;
          state: "ACTIVE" | "INACTIVE";
          description: string;
          source: "native";
          timingMs: number;
          lastSuccessfulCheckMs: number;
        }> = {
          ok: true,
          statusCode: st.code,
          state: mapped.state,
          description: mapped.description,
          source: st.source,
          timingMs: st.timingMs,
          lastSuccessfulCheckMs,
        };
        return { content: [{ type: "text", text: JSON.stringify(out) }], structuredContent: out };
      } catch (e) {
        const mapped = mapSdkError(e, sdk?.LicensifySdkError ?? null);
        const out: ToolErr = {
          ...mapped,
          hint:
            mapped.hint ??
            "If this is the first run, activate a license key. Otherwise, ensure the cache path is writable and not corrupted; then retry.",
        };
        return { content: [{ type: "text", text: JSON.stringify(out) }], structuredContent: out };
      }
    }
  );

  server.registerTool(
    "licensify_has_feature",
    {
      title: "Check feature entitlement",
      description:
        "Call this before executing a specific premium capability (feature gate). Do not proceed if available is false; instead offer an upgrade path or a free alternative. Prefer calling licensify_check first so you can distinguish INACTIVE vs missing feature.",
      inputSchema: z.object({
        feature: z
          .union([z.enum(KNOWN_FEATURES), z.string().min(1)])
          .describe("Feature flag/entitlement name. Known values in this repo: base, pro."),
      }),
      outputSchema: z.object({
        ok: z.boolean(),
        feature: z.string().optional(),
        available: z.boolean().optional(),
        reasoning: z.string().optional(),
        errorCode: z.enum(TOOL_ERROR_CODES).optional(),
        errorMessage: z.string().optional(),
        hint: z.string().optional(),
        retryable: z.boolean().optional(),
      }),
    },
    async ({ feature }): Promise<any> => {
      if (!client) {
        const out = unavailable(initError ?? "Licensify client failed to initialize");
        return { content: [{ type: "text", text: JSON.stringify(out) }], structuredContent: out };
      }
      if (typeof feature !== "string" || feature.length === 0) {
        const out: ToolErr = {
          ok: false,
          errorCode: "INVALID_ARGUMENT",
          errorMessage: "feature is required",
          hint: "Pick a non-empty feature name (known examples: base, pro).",
          retryable: false,
        };
        return { content: [{ type: "text", text: JSON.stringify(out) }], structuredContent: out };
      }
      try {
        const available = await client.hasFeature(feature);
        const out: ToolOk<{ feature: string; available: boolean; reasoning: string }> = {
          ok: true,
          feature,
          available,
          reasoning: available
            ? `Entitlement '${feature}' is present; you may proceed with the gated workflow.`
            : `Entitlement '${feature}' is not present; do not proceed with the gated workflow.`,
        };
        return { content: [{ type: "text", text: JSON.stringify(out) }], structuredContent: out };
      } catch (e) {
        const out = mapSdkError(e, sdk?.LicensifySdkError ?? null);
        return { content: [{ type: "text", text: JSON.stringify(out) }], structuredContent: out };
      }
    }
  );

  server.registerTool(
    "licensify_get_status_summary",
    {
      title: "Get a holistic license snapshot",
      description:
        "Call this when you need one consolidated view of licensing: active/inactive state plus all known features. This is especially useful to decide which tools or UI paths to expose next without making multiple separate tool calls.",
      inputSchema: z.object({}),
      outputSchema: z.object({
        ok: z.boolean(),
        state: z.enum(["ACTIVE", "INACTIVE"]).optional(),
        statusCode: z.union([z.literal(0), z.literal(1)]).optional(),
        description: z.string().optional(),
        feature_base: z.boolean().optional(),
        feature_pro: z.boolean().optional(),
        availableFeatures: z.array(z.string()).optional(),
        missingFeatures: z.array(z.string()).optional(),
        lastSuccessfulCheckMs: z.number().int().nullable().optional(),
        errorCode: z.enum(TOOL_ERROR_CODES).optional(),
        errorMessage: z.string().optional(),
        hint: z.string().optional(),
        retryable: z.boolean().optional(),
      }),
    },
    async (): Promise<any> => {
      if (!client) {
        const out = unavailable(initError ?? "Licensify client failed to initialize");
        return { content: [{ type: "text", text: JSON.stringify(out) }], structuredContent: out };
      }

      try {
        const st = await client.check();
        const mapped = statusToState(st.code);
        lastSuccessfulCheckMs = nowMs();

        const feature_base = await client.hasFeature("base");
        const feature_pro = await client.hasFeature("pro");
        const availableFeatures = ([
          feature_base ? "base" : null,
          feature_pro ? "pro" : null,
        ] as Array<KnownFeature | null>).filter((v): v is KnownFeature => v !== null);
        const missingFeatures = ([
          feature_base ? null : "base",
          feature_pro ? null : "pro",
        ] as Array<KnownFeature | null>).filter((v): v is KnownFeature => v !== null);

        const out: ToolOk<{
          state: "ACTIVE" | "INACTIVE";
          statusCode: StatusCode;
          description: string;
          feature_base: boolean;
          feature_pro: boolean;
          availableFeatures: KnownFeature[];
          missingFeatures: KnownFeature[];
          lastSuccessfulCheckMs: number;
        }> = {
          ok: true,
          state: mapped.state,
          statusCode: st.code,
          description: mapped.description,
          feature_base,
          feature_pro,
          availableFeatures,
          missingFeatures,
          lastSuccessfulCheckMs,
        };
        return { content: [{ type: "text", text: JSON.stringify(out) }], structuredContent: out };
      } catch (e) {
        const mapped = mapSdkError(e, sdk?.LicensifySdkError ?? null);
        const out: ToolErr = {
          ...mapped,
          hint:
            mapped.hint ??
            "If this is the first run, activate a license key. Otherwise, fix the environment/cache path and retry.",
        };
        return { content: [{ type: "text", text: JSON.stringify(out) }], structuredContent: out };
      }
    }
  );

  const transport = new StdioServerTransport();
  await server.connect(transport);
}

main().catch(() => {
  // Never crash hard from an unhandled rejection: MCP hosts expect a long-lived process.
  // If startup fails, the process will exit; callers should inspect stderr.
  process.exit(1);
});

