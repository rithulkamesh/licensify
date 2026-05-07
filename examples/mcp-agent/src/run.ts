// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Rithul Kamesh
// Author: Rithul Kamesh <hi@rithul.dev>
// Description: examples/mcp-agent/src/run.ts — Example agent runner using the Licensify MCP integration.

/**
 * End-to-end example: spawn the Licensify MCP server and call its tools.
 *
 * Usage:
 *   cd examples/mcp-agent
 *   npm install
 *   npm run build
 *   LICENSIFY_LICENSE_KEY="LICENSE-KEY-DEV" node dist/run.js
 */

import { Client } from "@modelcontextprotocol/client";
import { StdioClientTransport } from "@modelcontextprotocol/client/stdio";

type AnyJson = null | boolean | number | string | AnyJson[] | { [k: string]: AnyJson };

function asObj(v: unknown): Record<string, AnyJson> | null {
  if (v && typeof v === "object" && !Array.isArray(v)) return v as any;
  return null;
}

async function main() {
  const repoRoot = new URL("../../..", import.meta.url).pathname;
  const serverPath = `${repoRoot}/mcp/licensify/dist/server.js`;

  const transport = new StdioClientTransport({
    command: "node",
    args: [serverPath],
    env: {
      ...process.env,
      LICENSIFY_SERVER_URL: process.env.LICENSIFY_SERVER_URL ?? "http://localhost:8080",
      LICENSIFY_CACHE_PATH: process.env.LICENSIFY_CACHE_PATH ?? "/tmp/licensify.token",
    },
  });

  const client = new Client({ name: "licensify-mcp-agent-example", version: "0.1.0" });
  await client.connect(transport);

  const health = await client.callTool({ name: "licensify_health", arguments: {} });
  console.log("health:", health.structuredContent);

  const licenseKey = process.env.LICENSIFY_LICENSE_KEY;
  if (licenseKey) {
    const act = await client.callTool({ name: "licensify_activate", arguments: { licenseKey } });
    console.log("activate:", act.structuredContent);
  } else {
    console.log("activate: skipped (set LICENSIFY_LICENSE_KEY to run activation)");
  }

  const check = await client.callTool({ name: "licensify_check", arguments: {} });
  console.log("check:", check.structuredContent);

  // Fake premium action gated on `pro`.
  const hasPro = await client.callTool({ name: "licensify_has_feature", arguments: { feature: "pro" } });
  console.log("has_feature(pro):", hasPro.structuredContent);

  const proObj = asObj(hasPro.structuredContent);
  const available = proObj?.available === true;
  if (available) {
    console.log("premium action: proceeding (pro available)");
  } else {
    console.log("premium action: blocked (pro not available). Ask user to upgrade or use free alternative.");
  }

  const summary = await client.callTool({ name: "licensify_get_status_summary", arguments: {} });
  console.log("summary:", summary.structuredContent);

  await client.close();
}

main().catch((e) => {
  console.error(e);
  process.exit(1);
});

