# Licensify MCP Skill

This repo ships a production-grade **MCP server** that exposes the Licensify SDK as callable tools so an agent can **activate**, **check**, and **feature-gate** premium workflows using Licensify’s offline-capable, machine-bound licensing model.

## What this skill does

It runs a local MCP server (stdio transport) that initializes a single `LicensifyClient` once at startup and exposes tools for activation, status checks, and feature entitlements. If the native client cannot be initialized (missing `liblicensify`, missing Node FFI deps, etc.), the server **still starts** and tools return a structured `LICENSIFY_UNAVAILABLE` error instead of crashing.

## Installation (Claude Desktop / any MCP host)

### 1) Build the MCP server

```bash
cd mcp/licensify
npm install
npm run build
```

Note: the build script also builds the in-repo TypeScript SDK (`sdk/typescript`) so the MCP server can import `dist/` output at runtime.

### 2) Configure your MCP host (stdio)

Set environment variables:

- `LICENSIFY_SERVER_URL` (default: `http://localhost:8080`)
- `LICENSIFY_CACHE_PATH` (default: `/tmp/licensify.token`)

Claude Desktop example config snippet:

```json
{
  "mcpServers": {
    "licensify": {
      "command": "node",
      "args": ["<REPO_ROOT>/mcp/licensify/dist/server.js"],
      "env": {
        "LICENSIFY_SERVER_URL": "http://localhost:8080",
        "LICENSIFY_CACHE_PATH": "/tmp/licensify.token"
      }
    }
  }
}
```

## Tools

All tools return **structured output** with:

- `ok: true` on success
- `ok: false` on failure with:
  - `errorCode` (typed)
  - `errorMessage`
  - optional `hint` (user-facing recovery guidance)
  - `retryable` (whether it makes sense to retry after fixing conditions)

### `licensify_health`

**When to call**: Call this first to confirm the server’s Licensify client is initialized.

Input:

```json
{}
```

Output (example):

```json
{
  "ok": true,
  "sdkVersion": "0.1.0",
  "clientInitialized": true,
  "serverUrl": "http://localhost:8080",
  "cachePath": "/tmp/licensify.token",
  "initError": null,
  "lastSuccessfulCheckMs": null
}
```

### `licensify_activate`

**When to call**: Call when a user provides a license key and you need to bind it to this machine.

Input:

```json
{ "licenseKey": "LICENSE-KEY-DEV" }
```

Output (example):

```json
{ "ok": true, "activated": true, "activationTimingMs": 12 }
```

Failure example:

```json
{
  "ok": false,
  "errorCode": "LICENSIFY_UNAVAILABLE",
  "errorMessage": "Cannot find module 'ffi-napi'",
  "hint": "Install the Node FFI deps (ffi-napi, ref-napi) and ensure the native lib is available, then restart the MCP server.",
  "retryable": true
}
```

### `licensify_check`

**When to call**: Call on startup and before any premium workflow.

Input:

```json
{}
```

Output (example):

```json
{
  "ok": true,
  "statusCode": 0,
  "state": "ACTIVE",
  "description": "License is valid for this machine (offline-first check passed).",
  "source": "native",
  "timingMs": 3,
  "lastSuccessfulCheckMs": 1715070000000
}
```

### `licensify_has_feature`

**When to call**: Call before executing a feature-gated premium capability.

Input:

```json
{ "feature": "pro" }
```

Output (example):

```json
{
  "ok": true,
  "feature": "pro",
  "available": false,
  "reasoning": "Entitlement 'pro' is not present; do not proceed with the gated workflow."
}
```

### `licensify_get_status_summary`

**When to call**: Call when you need one consolidated snapshot for tool-chaining decisions (state + known features).

Input:

```json
{}
```

Output (example):

```json
{
  "ok": true,
  "state": "ACTIVE",
  "statusCode": 0,
  "description": "License is valid for this machine (offline-first check passed).",
  "feature_base": true,
  "feature_pro": false,
  "availableFeatures": ["base"],
  "missingFeatures": ["pro"],
  "lastSuccessfulCheckMs": 1715070000000
}
```

## Common agent patterns

- **Gate a premium workflow on license state**:
  - call `licensify_check`
  - proceed only if `state === "ACTIVE"`

- **Gate a premium workflow on a feature flag**:
  - call `licensify_check` (stop early if inactive)
  - call `licensify_has_feature`
  - proceed only if `available === true`

## Troubleshooting by errorCode

- `LICENSIFY_UNAVAILABLE`
  - Install Node deps required by the TypeScript SDK (`ffi-napi`, `ref-napi`)
  - Ensure `liblicensify` is present on the loader path
  - Restart the MCP server process

- `INVALID_ARGUMENT`
  - Fix the tool input (e.g., provide non-empty `licenseKey` or `feature`)

- `ACTIVATION`
  - Verify the license key is correct
  - Retry after fixing environment issues; activation is safe to retry after failure

- `CHECK`
  - If first run, activate a key first
  - Ensure `LICENSIFY_CACHE_PATH` points to a stable, writable file location

