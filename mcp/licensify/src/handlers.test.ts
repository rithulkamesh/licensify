// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Rithul Kamesh
// Author: Rithul Kamesh <hi@rithul.dev>
// Description: mcp/licensify/src/handlers.test.ts — Unit tests for the Licensify MCP server tool handlers.

import test from "node:test";
import assert from "node:assert/strict";

import {
  envString,
  Handlers,
  mapSdkError,
  nowMs,
  statusToState,
  toHintFromMessage,
  unavailable,
  type LicensifyClientLike,
} from "./handlers.js";

class FakeSdkError extends Error {
  public readonly code: number;
  constructor(name: string, code: number, message: string) {
    super(message);
    this.name = name;
    this.code = code;
  }
}

function makeClient(overrides: Partial<LicensifyClientLike> = {}): LicensifyClientLike {
  return {
    activate: async () => {},
    check: async () => ({ code: 0, source: "native", timingMs: 1 }),
    hasFeature: async () => true,
    ...overrides,
  };
}

function makeHandlers(client: LicensifyClientLike | null, initError: string | null = null) {
  let now = 1000;
  return new Handlers({
    client,
    initError,
    serverUrl: "u",
    cachePath: "c",
    sdkVersion: "0.1.0",
    licensifySdkError: FakeSdkError as any,
    clock: { nowMs: () => now++ },
  });
}

test("nowMs returns a finite number", () => {
  const a = nowMs();
  assert.equal(typeof a, "number");
  assert.ok(a > 0);
});

test("envString handles empty/missing/non-string values", () => {
  assert.equal(envString({ A: "x" }, "A"), "x");
  assert.equal(envString({ A: "" }, "A"), undefined);
  assert.equal(envString({}, "A"), undefined);
});

test("toHintFromMessage maps known phrases", () => {
  assert.match(toHintFromMessage("client is disposed") ?? "", /Recreate/);
  assert.match(toHintFromMessage("client is closed") ?? "", /Recreate/);
  assert.match(toHintFromMessage("Cannot find module 'ffi-napi'") ?? "", /ffi-napi/);
  assert.match(toHintFromMessage("missing ref-napi binding") ?? "", /ffi-napi/);
  assert.match(toHintFromMessage("error loading liblicensify.so") ?? "", /liblicensify/);
  assert.equal(toHintFromMessage("totally unrelated"), undefined);
});

test("statusToState maps codes 0/1", () => {
  assert.equal(statusToState(0).state, "ACTIVE");
  assert.equal(statusToState(1).state, "INACTIVE");
});

test("unavailable returns LICENSIFY_UNAVAILABLE with hint", () => {
  const out = unavailable("nope");
  assert.equal(out.ok, false);
  assert.equal(out.errorCode, "LICENSIFY_UNAVAILABLE");
  assert.equal(out.retryable, true);
  assert.match(out.hint ?? "", /Install/);
});

test("mapSdkError handles every code path", () => {
  // Plain Error
  let m = mapSdkError(new Error("plain"), null);
  assert.equal(m.errorCode, "UNKNOWN");
  assert.equal(m.errorMessage, "plain");

  // String input
  m = mapSdkError("a string", null);
  assert.equal(m.errorMessage, "a string");

  // Non-Error, non-string
  m = mapSdkError({ foo: 1 }, null);
  assert.equal(m.errorMessage, "unknown error");

  // Initialization
  m = mapSdkError(new FakeSdkError("InitializationError", 2, "init failed"), FakeSdkError as any);
  assert.equal(m.errorCode, "INITIALIZATION");

  // Activation
  m = mapSdkError(new FakeSdkError("ActivationError", 3, "act failed"), FakeSdkError as any);
  assert.equal(m.errorCode, "ACTIVATION");

  // Check
  m = mapSdkError(new FakeSdkError("CheckError", 4, "check failed"), FakeSdkError as any);
  assert.equal(m.errorCode, "CHECK");

  // Invalid argument (code 1)
  m = mapSdkError(new FakeSdkError("Other", 1, "bad arg"), FakeSdkError as any);
  assert.equal(m.errorCode, "INVALID_ARGUMENT");
  assert.equal(m.retryable, false);

  // Code 0 fallthrough -> UNKNOWN
  m = mapSdkError(new FakeSdkError("Other", 0, "weird"), FakeSdkError as any);
  assert.equal(m.errorCode, "UNKNOWN");

  // Name takes precedence over code
  m = mapSdkError(
    new FakeSdkError("InitializationError", 99 as any, "named init"),
    FakeSdkError as any,
  );
  assert.equal(m.errorCode, "INITIALIZATION");
});

test("Handlers.health reports both initialized and uninitialized states", () => {
  const ok = makeHandlers(makeClient()).health();
  assert.equal(ok.clientInitialized, true);
  assert.equal(ok.initError, null);

  const bad = makeHandlers(null, "boom").health();
  assert.equal(bad.clientInitialized, false);
  assert.equal(bad.initError, "boom");

  const fallback = makeHandlers(null, null).health();
  assert.equal(fallback.initError, "unknown initialization error");
});

test("Handlers.activate happy path and validation errors", async () => {
  const h = makeHandlers(makeClient());
  const ok = await h.activate("KEY");
  assert.equal(ok.ok, true);

  const empty = await h.activate("");
  assert.equal(empty.ok, false);
  if (!empty.ok) assert.equal(empty.errorCode, "INVALID_ARGUMENT");

  const wrongType = await h.activate(123);
  assert.equal(wrongType.ok, false);

  const noClient = await makeHandlers(null, "bad").activate("KEY");
  assert.equal(noClient.ok, false);
  if (!noClient.ok) assert.equal(noClient.errorCode, "LICENSIFY_UNAVAILABLE");

  // Null client + null initError covers the "Licensify client failed to initialize" fallback.
  const noClientNoErr = await makeHandlers(null, null).activate("KEY");
  assert.equal(noClientNoErr.ok, false);
  if (!noClientNoErr.ok) assert.match(noClientNoErr.errorMessage ?? "", /failed to initialize/);
});

test("Handlers.activate maps thrown SDK error", async () => {
  const h = makeHandlers(
    makeClient({
      activate: async () => {
        throw new FakeSdkError("ActivationError", 3, "bad key");
      },
    }),
  );
  const out = await h.activate("KEY");
  assert.equal(out.ok, false);
  if (!out.ok) {
    assert.equal(out.errorCode, "ACTIVATION");
    assert.match(out.hint ?? "", /Verify the key|retry/);
  }
});

test("Handlers.check happy path and error mapping", async () => {
  const h = makeHandlers(makeClient());
  const out = await h.check();
  assert.equal(out.ok, true);
  if (out.ok) {
    assert.equal(out.state, "ACTIVE");
    assert.equal(out.statusCode, 0);
  }

  const inactive = await makeHandlers(
    makeClient({
      check: async () => ({ code: 1, source: "native", timingMs: 0 }),
    }),
  ).check();
  assert.equal(inactive.ok, true);
  if (inactive.ok) assert.equal(inactive.state, "INACTIVE");

  const errH = makeHandlers(
    makeClient({
      check: async () => {
        throw new FakeSdkError("CheckError", 4, "boom");
      },
    }),
  );
  const errOut = await errH.check();
  assert.equal(errOut.ok, false);
  if (!errOut.ok) assert.equal(errOut.errorCode, "CHECK");

  const noClient = await makeHandlers(null, "down").check();
  assert.equal(noClient.ok, false);

  // Null client + null initError covers the "Licensify client failed to initialize" fallback.
  const noClientNoErr = await makeHandlers(null, null).check();
  assert.equal(noClientNoErr.ok, false);
  if (!noClientNoErr.ok) assert.match(noClientNoErr.errorMessage ?? "", /failed to initialize/);
});

test("Handlers.hasFeature covers all branches", async () => {
  const h = makeHandlers(makeClient());
  const ok = await h.hasFeature("base");
  assert.equal(ok.ok, true);
  if (ok.ok) assert.equal(ok.available, true);

  const empty = await h.hasFeature("");
  assert.equal(empty.ok, false);
  const wrongType = await h.hasFeature(7 as unknown as string);
  assert.equal(wrongType.ok, false);

  const noClient = await makeHandlers(null).hasFeature("base");
  assert.equal(noClient.ok, false);

  const errH = makeHandlers(
    makeClient({
      hasFeature: async () => {
        throw new Error("native panic");
      },
    }),
  );
  const errOut = await errH.hasFeature("pro");
  assert.equal(errOut.ok, false);
  if (!errOut.ok) assert.equal(errOut.errorCode, "UNKNOWN");

  const falseH = makeHandlers(
    makeClient({
      hasFeature: async () => false,
    }),
  );
  const denied = await falseH.hasFeature("pro");
  assert.equal(denied.ok, true);
  if (denied.ok) assert.equal(denied.available, false);
});

test("Handlers.summary aggregates check + features and handles errors", async () => {
  const h = makeHandlers(
    makeClient({
      hasFeature: async (f) => f === "base",
    }),
  );
  const out = await h.summary();
  assert.equal(out.ok, true);
  if (out.ok) {
    assert.deepEqual(out.availableFeatures, ["base"]);
    assert.deepEqual(out.missingFeatures, ["pro"]);
    assert.equal(out.feature_base, true);
    assert.equal(out.feature_pro, false);
  }

  // Both features missing exercises the `else missing.push("base")` branch.
  const noFeatures = await makeHandlers(
    makeClient({
      hasFeature: async () => false,
    }),
  ).summary();
  assert.equal(noFeatures.ok, true);
  if (noFeatures.ok) {
    assert.deepEqual(noFeatures.availableFeatures, []);
    assert.deepEqual(noFeatures.missingFeatures, ["base", "pro"]);
  }

  // All features present.
  const allFeatures = await makeHandlers(
    makeClient({
      hasFeature: async () => true,
    }),
  ).summary();
  assert.equal(allFeatures.ok, true);
  if (allFeatures.ok) {
    assert.deepEqual(allFeatures.availableFeatures, ["base", "pro"]);
    assert.deepEqual(allFeatures.missingFeatures, []);
  }

  const noClient = await makeHandlers(null).summary();
  assert.equal(noClient.ok, false);

  const errH = makeHandlers(
    makeClient({
      check: async () => {
        throw new Error("err");
      },
    }),
  );
  const errOut = await errH.summary();
  assert.equal(errOut.ok, false);
  if (!errOut.ok) assert.match(errOut.hint ?? "", /first run|fix the environment/);
});
