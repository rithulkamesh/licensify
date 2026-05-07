// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Rithul Kamesh
// Author: Rithul Kamesh <hi@rithul.dev>
// Description: sdk/typescript/src/index.test.ts — Unit tests for the TypeScript SDK.

import test from "node:test";
import assert from "node:assert/strict";

import { LicensifyClient } from "./index.js";

test("module loads without ffi-napi installed", () => {
  assert.ok(LicensifyClient);
});

