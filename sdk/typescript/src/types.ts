// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Rithul Kamesh
// Author: Rithul Kamesh <hi@rithul.dev>
// Description: sdk/typescript/src/types.ts — Public TypeScript types for the Licensify SDK.

/**
 * Structured logger interface for observability hooks.
 */
export type Logger = {
  debug: (msg: string, fields?: Record<string, unknown>) => void;
  info: (msg: string, fields?: Record<string, unknown>) => void;
  warn: (msg: string, fields?: Record<string, unknown>) => void;
  error: (msg: string, fields?: Record<string, unknown>) => void;
};

export type Config = {
  serverUrl: string;
  cachePath: string;
  logger?: Logger;
  /**
   * Dependency injection hook for unit tests: override the native loader.
   */
  native?: NativeBindings;
  /**
   * Dependency injection hook for unit tests.
   */
  clock?: { nowMs: () => number };
};

export type StatusCode = 0 | 1;

export type Status = {
  code: StatusCode;
  source: "native";
  timingMs: number;
};

export type ErrorCode = 0 | 1 | 2 | 3 | 4;

export type NativeBindings = {
  activateCode: (ptr: Buffer, key: string) => ErrorCode;
  checkCode: (ptr: Buffer) => { code: ErrorCode; status: StatusCode };
  hasFeature: (ptr: Buffer, feature: string) => boolean;
  free: (ptr: Buffer) => void;
  lastError: (ptr: Buffer) => string | null;
  newClient: (serverUrl: string, cachePath: string) => Buffer;
};
