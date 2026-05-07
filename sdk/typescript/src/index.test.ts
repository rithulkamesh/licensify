// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Rithul Kamesh
// Author: Rithul Kamesh <hi@rithul.dev>
// Description: sdk/typescript/src/index.test.ts — Unit tests for the TypeScript SDK using injected native bindings.

import test from "node:test";
import assert from "node:assert/strict";

import {
  ActivationError,
  CheckError,
  InitializationError,
  LicensifyClient,
  LicensifySdkError,
} from "./index.js";
import type { ErrorCode, NativeBindings, StatusCode } from "./types.js";

type CallLog = {
  newClient: number;
  free: number;
  activateCode: number;
  checkCode: number;
  hasFeature: number;
  lastError: number;
};

function makeStubNative(overrides: Partial<NativeBindings> = {}): { native: NativeBindings; calls: CallLog } {
  const calls: CallLog = { newClient: 0, free: 0, activateCode: 0, checkCode: 0, hasFeature: 0, lastError: 0 };
  let lastErr: string | null = null;
  const base: NativeBindings = {
    newClient: (_u, _c) => {
      calls.newClient++;
      return Buffer.alloc(8);
    },
    free: (_p) => {
      calls.free++;
    },
    activateCode: (_p, key) => {
      calls.activateCode++;
      if (key === "FAIL") {
        lastErr = "stub activation failure";
        return 3 as ErrorCode;
      }
      lastErr = null;
      return 0 as ErrorCode;
    },
    checkCode: (_p) => {
      calls.checkCode++;
      return { code: 0 as ErrorCode, status: 0 as StatusCode };
    },
    hasFeature: (_p, feature) => {
      calls.hasFeature++;
      return feature === "base";
    },
    lastError: (_p) => {
      calls.lastError++;
      return lastErr;
    },
    ...overrides,
  };
  return { native: base, calls };
}

const baseConfig = (native: NativeBindings) => ({
  serverUrl: "http://example.com",
  cachePath: "/tmp/licensify.token",
  native,
});

test("LicensifySdkError exposes name and code", () => {
  const err = new LicensifySdkError("X", 1, "boom", { foo: 1 });
  assert.equal(err.name, "X");
  assert.equal(err.code, 1);
  assert.equal(err.message, "boom");
  assert.deepEqual(err.metadata, { foo: 1 });
});

test("create rejects missing config", async () => {
  // @ts-expect-error: intentional bad call
  await assert.rejects(() => LicensifyClient.create(undefined), InitializationError);
  await assert.rejects(
    // @ts-expect-error: intentional bad call
    () => LicensifyClient.create({}),
    InitializationError,
  );
});

test("create rejects empty serverUrl/cachePath", async () => {
  const native: NativeBindings = makeStubNative({}).native;
  await assert.rejects(
    () => LicensifyClient.create({ serverUrl: "", cachePath: "/x", native }),
    InitializationError,
  );
  await assert.rejects(
    () => LicensifyClient.create({ serverUrl: "x", cachePath: "", native }),
    InitializationError,
  );
});

test("create rejects null pointer", async () => {
  const native: NativeBindings = makeStubNative({
    newClient: () => null,
  }).native;
  await assert.rejects(() => LicensifyClient.create(baseConfig(native)), InitializationError);
});

test("activate happy path with logger and clock", async () => {
  const events: string[] = [];
  const logger = {
    debug: (m: string) => events.push("debug:" + m),
    info: (m: string) => events.push("info:" + m),
    warn: (m: string) => events.push("warn:" + m),
    error: (m: string) => events.push("error:" + m),
  };
  const { native, calls } = makeStubNative();
  const c = await LicensifyClient.create({
    serverUrl: "u",
    cachePath: "c",
    logger,
    native,
    clock: { nowMs: () => 5 },
  });
  await c.activate("KEY");
  assert.equal(calls.activateCode, 1);
  assert.ok(events.some((e) => e.startsWith("info:")));
  c.dispose();
  c.dispose();
});

test("activate rejects empty key", async () => {
  const { native } = makeStubNative();
  const c = await LicensifyClient.create(baseConfig(native));
  await assert.rejects(() => c.activate(""), ActivationError);
  // @ts-expect-error
  await assert.rejects(() => c.activate(123), ActivationError);
});

test("activate maps native error code", async () => {
  const errors: string[] = [];
  const { native } = makeStubNative();
  const c = await LicensifyClient.create({
    ...baseConfig(native),
    logger: {
      debug: () => {},
      info: () => {},
      warn: () => {},
      error: (m: string) => errors.push(m),
    },
  });
  await assert.rejects(() => c.activate("FAIL"), ActivationError);
  assert.ok(errors.length > 0);
});

test("activate falls back when lastError returns null", async () => {
  const { native } = makeStubNative({
    activateCode: () => 3 as ErrorCode,
    lastError: () => null,
  });
  const c = await LicensifyClient.create(baseConfig(native));
  await assert.rejects(() => c.activate("X"), (err) => {
    assert.ok(err instanceof ActivationError);
    assert.match(err.message, /activation failed/);
    return true;
  });
});

test("check happy path returns status with timing", async () => {
  const { native } = makeStubNative();
  const c = await LicensifyClient.create({
    ...baseConfig(native),
    clock: { nowMs: () => 100 },
    logger: { debug: () => {}, info: () => {}, warn: () => {}, error: () => {} },
  });
  const status = await c.check();
  assert.equal(status.source, "native");
  assert.equal(status.code, 0);
});

test("check maps native error and logs through provided logger", async () => {
  const { native } = makeStubNative({
    checkCode: () => ({ code: 4 as ErrorCode, status: 1 as StatusCode }),
  });
  const logs: Array<[string, unknown]> = [];
  const logger = {
    debug: (msg: string, ctx?: unknown) => logs.push(["debug", { msg, ctx }]),
    info: (msg: string, ctx?: unknown) => logs.push(["info", { msg, ctx }]),
    warn: (msg: string, ctx?: unknown) => logs.push(["warn", { msg, ctx }]),
    error: (msg: string, ctx?: unknown) => logs.push(["error", { msg, ctx }]),
  };
  const c = await LicensifyClient.create({ ...baseConfig(native), logger });
  await assert.rejects(() => c.check(), CheckError);
  assert.ok(logs.some(([level]) => level === "error"));
});

test("check fallback message when lastError null", async () => {
  const { native } = makeStubNative({
    checkCode: () => ({ code: 4 as ErrorCode, status: 1 as StatusCode }),
    lastError: () => null,
  });
  const c = await LicensifyClient.create(baseConfig(native));
  await assert.rejects(() => c.check(), (err) => {
    assert.ok(err instanceof CheckError);
    assert.match(err.message, /check failed/);
    return true;
  });
});

test("hasFeature short-circuits empty input", async () => {
  const { native, calls } = makeStubNative();
  const c = await LicensifyClient.create(baseConfig(native));
  assert.equal(await c.hasFeature(""), false);
  // @ts-expect-error
  assert.equal(await c.hasFeature(123), false);
  assert.equal(calls.hasFeature, 0);
  assert.equal(await c.hasFeature("base"), true);
  assert.equal(await c.hasFeature("other"), false);
  assert.ok(calls.hasFeature >= 1);
});

test("operations on disposed client throw InitializationError", async () => {
  const { native } = makeStubNative();
  const c = await LicensifyClient.create(baseConfig(native));
  c.dispose();
  await assert.rejects(() => c.activate("k"), InitializationError);
  await assert.rejects(() => c.check(), InitializationError);
  await assert.rejects(() => c.hasFeature("base"), InitializationError);
});
