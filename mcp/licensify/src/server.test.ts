// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Rithul Kamesh
// Author: Rithul Kamesh <hi@rithul.dev>
// Description: mcp/licensify/src/server.test.ts — Tests for the wire() function exposed by server.ts.

import test from "node:test";
import assert from "node:assert/strict";

import { wire } from "./server.js";

test("wire returns initError when SDK load returns null", async () => {
  const out = await wire({
    env: { LICENSIFY_SERVER_URL: "u", LICENSIFY_CACHE_PATH: "c" },
    loadSdk: async () => null,
  });
  assert.equal(out.client, null);
  assert.match(out.initError ?? "", /SDK not found/);
  const health = out.handlers.health();
  assert.equal(health.clientInitialized, false);
});

test("wire creates client when SDK load succeeds", async () => {
  const out = await wire({
    env: {},
    loadSdk: async () => ({
      LicensifyClient: {
        create: async () => ({
          activate: async () => {},
          check: async () => ({ code: 0, source: "native", timingMs: 1 }),
          hasFeature: async () => true,
        }),
      },
      LicensifySdkError: Error,
    }),
  });
  assert.notEqual(out.client, null);
  assert.equal(out.initError, null);
  const health = out.handlers.health();
  assert.equal(health.clientInitialized, true);
});

test("wire surfaces SDK creation errors", async () => {
  const out = await wire({
    env: {},
    loadSdk: async () => ({
      LicensifyClient: {
        create: async () => {
          throw new Error("create failed");
        },
      },
      LicensifySdkError: Error,
    }),
  });
  assert.equal(out.client, null);
  assert.equal(out.initError, "create failed");
});

test("wire stringifies non-Error rejects", async () => {
  const out = await wire({
    env: {},
    loadSdk: async () => {
      throw "string error";
    },
  });
  assert.equal(out.initError, "string error");
});
