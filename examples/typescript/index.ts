// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Rithul Kamesh
// Author: Rithul Kamesh <hi@rithul.dev>
// Description: examples/typescript/index.ts — Minimal TypeScript SDK example exercising activate/check/hasFeature.

import { LicensifyClient } from "@licensify/sdk";

async function main() {
  const serverUrl = process.env.LICENSIFY_BASE_URL ?? "http://localhost:8080";
  const cachePath = process.env.LICENSIFY_CACHE_PATH ?? "/tmp/licensify.token";
  const licenseKey = process.env.LICENSIFY_LICENSE_KEY ?? "LICENSE-KEY-DEV";

  const client = await LicensifyClient.create({ serverUrl, cachePath });
  try {
    await client.activate(licenseKey);
    const status = await client.check();
    const hasBase = await client.hasFeature("base");
    const hasPro = await client.hasFeature("pro");

    console.log(
      JSON.stringify(
        {
          ok: true,
          status,
          features: { base: hasBase, pro: hasPro },
        },
        null,
        2,
      ),
    );
    console.log("ts-example: success");
  } finally {
    client.dispose();
  }
}

main().catch((e: unknown) => {
  console.error(e);
  process.exit(1);
});
