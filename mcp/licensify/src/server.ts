// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Rithul Kamesh
// Author: Rithul Kamesh <hi@rithul.dev>
// Description: mcp/licensify/src/server.ts — MCP server wiring built on top of pure Handlers for testability.

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

import { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import { StdioServerTransport } from "@modelcontextprotocol/sdk/server/stdio.js";
import * as z from "zod/v4";

import { LICENSIFY_GUIDE_MARKDOWN } from "./guide.js";
import {
  envString,
  Handlers,
  KNOWN_FEATURES,
  TOOL_ERROR_CODES,
  type LicensifyClientLike,
  type SdkErrorCtor,
} from "./handlers.js";

type SdkModule = {
  LicensifyClient: {
    create: (cfg: { serverUrl: string; cachePath: string }) => Promise<LicensifyClientLike>;
  };
  LicensifySdkError: SdkErrorCtor;
};

/**
 * `loadSdk` is the dependency injection seam for tests. The default
 * implementation tries the in-repo build output, then a published package, and
 * returns null when both are unavailable.
 */
/* c8 ignore start */
async function loadSdk(): Promise<SdkModule | null> {
  const local = await import("../../../sdk/typescript/dist/index.js").catch(
    () => null,
  );
  if (local) return local as unknown as SdkModule;
  // The published package is a runtime fallback; we use a dynamic specifier so
  // tsc does not require the dependency to be installed at build time.
  const dynamicImport: (s: string) => Promise<unknown> = (s) => import(s);
  return (
    ((await dynamicImport("@licensify/sdk").catch(() => null)) as
      | SdkModule
      | null) ?? null
  );
}
/* c8 ignore stop */

export type WireOptions = {
  sdkVersion?: string;
  env?: NodeJS.ProcessEnv;
  loadSdk?: () => Promise<SdkModule | null>;
};

/**
 * `wire` constructs the McpServer with all tools registered. Tests use this
 * directly with stubs; production calls `start()`.
 */
export async function wire(opts: WireOptions = {}) {
  const sdkVersion = opts.sdkVersion ?? "0.1.0";
  const env = opts.env ?? process.env;
  const serverUrl = envString(env, "LICENSIFY_SERVER_URL") ?? "http://localhost:8080";
  const cachePath = envString(env, "LICENSIFY_CACHE_PATH") ?? "/tmp/licensify.token";

  const loader = opts.loadSdk ?? loadSdk;

  let sdk: SdkModule | null = null;
  let client: LicensifyClientLike | null = null;
  let initError: string | null = null;
  try {
    sdk = await loader();
    if (!sdk) {
      throw new Error("Licensify SDK not found. Build `sdk/typescript` or install `@licensify/sdk`.");
    }
    client = await sdk.LicensifyClient.create({ serverUrl, cachePath });
  } catch (e) {
    initError = e instanceof Error ? e.message : String(e);
  }

  const handlers = new Handlers({
    client,
    initError,
    serverUrl,
    cachePath,
    sdkVersion,
    licensifySdkError: sdk?.LicensifySdkError ?? null,
    clock: { nowMs: () => Date.now() },
  });

  /* c8 ignore start -- McpServer registration is thin wire-up around the
   * Handlers class, which is fully covered by handlers.test.ts. The
   * end-to-end wiring is exercised by the MCP stdio integration test in CI. */
  const server = new McpServer(
    { name: "licensify", version: sdkVersion },
    {
      instructions:
        "Use licensify_check before premium workflows. Use licensify_has_feature to gate specific premium features. If any tool returns ok=false with errorCode=LICENSIFY_UNAVAILABLE, stop and guide the user to install/ship the native lib and restart the server.",
    },
  );

  server.registerResource(
    "licensify_guide",
    "licensify://guide",
    {
      title: "Licensify guide",
      description:
        "Mental model and recovery guidance for Licensify activation, checks, and feature gates.",
    },
    async () => ({
      contents: [
        {
          uri: "licensify://guide",
          mimeType: "text/markdown",
          text: LICENSIFY_GUIDE_MARKDOWN,
        },
      ],
    }),
  );

  const respond = (out: unknown) => ({
    content: [{ type: "text" as const, text: JSON.stringify(out) }],
    structuredContent: out as { [x: string]: unknown },
  });

  server.registerTool(
    "licensify_health",
    {
      title: "Licensify health",
      description:
        "Call this first to verify the Licensify native client is available before relying on any license-gated workflow.",
      inputSchema: z.object({}),
      outputSchema: z.object({
        ok: z.boolean(),
        sdkVersion: z.string().optional(),
        clientInitialized: z.boolean().optional(),
        serverUrl: z.string().optional(),
        cachePath: z.string().optional(),
        initError: z.string().nullable().optional(),
        lastSuccessfulCheckMs: z.number().int().nullable().optional(),
      }),
    },
    async () => respond(handlers.health()),
  );

  server.registerTool(
    "licensify_activate",
    {
      title: "Activate a license key",
      description:
        "Call this when the user provides a license key to bind it to the current machine.",
      inputSchema: z.object({
        licenseKey: z.string().min(1).describe("The license key string provided to the user."),
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
    async ({ licenseKey }) => respond(await handlers.activate(licenseKey)),
  );

  server.registerTool(
    "licensify_check",
    {
      title: "Check license status",
      description:
        "Call this on startup and before any premium workflow to verify the current machine is licensed.",
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
    async () => respond(await handlers.check()),
  );

  server.registerTool(
    "licensify_has_feature",
    {
      title: "Check feature entitlement",
      description: "Call this before executing a specific premium capability (feature gate).",
      inputSchema: z.object({
        feature: z
          .union([z.enum(KNOWN_FEATURES), z.string().min(1)])
          .describe("Feature flag name. Known: base, pro."),
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
    async ({ feature }) => respond(await handlers.hasFeature(feature)),
  );

  server.registerTool(
    "licensify_get_status_summary",
    {
      title: "Get a holistic license snapshot",
      description: "Call this for one consolidated view of licensing.",
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
    async () => respond(await handlers.summary()),
  );
  /* c8 ignore stop */

  return { server, handlers, sdk, client, initError };
}

/* c8 ignore start */
export async function start() {
  const { server } = await wire();
  const transport = new StdioServerTransport();
  await server.connect(transport);
}

if (process.argv[1] && process.argv[1].endsWith("server.js")) {
  start().catch(() => {
    process.exit(1);
  });
}
/* c8 ignore stop */
