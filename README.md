# Licensify

Licensify is an open-source licensing system designed for **offline-capable, machine-bound license validation** with a **language-agnostic client** exposed via a stable C ABI.

## Why Licensify is different

- **No JWT**: token format is custom binary + signed, not self-describing JSON payloads.
- **Machine-bound by design**: hardware-derived machine identity is part of activation and validation.
- **Offline-first enforcement**: cached encrypted token supports offline checks with local policy enforcement.
- **FFI-first architecture**: one hardened Rust core, consumed consistently from Go/C++/TypeScript/Rust/C/Zig.
- **Server-issued cert chain model**: activation and validation are cryptographically anchored from server CA material.

## Repository layout

- `server/`: Go licensing API (Echo + pgx)
- `client/`: Rust client core + C FFI (`licensify_*`)
- `sdk/`: Language SDKs wrapping the C ABI (Go/C++/TypeScript/Rust/C/Zig)
- `proto/`: Shared protocol contracts
- `docs/`: Architecture, protocol, integration
- `examples/`: Smoke tests and usage snippets

## Install & run

### Option A: Docker (recommended)

```bash
git clone git@github.com:rithulkamesh/licensify.git
cd licensify
docker compose up -d --build
curl -sS http://localhost:8080/v1/health
```

### Option B: Native server

```bash
git clone git@github.com:rithulkamesh/licensify.git
cd licensify/server
export DATABASE_URL='postgres://licensify:licensify@localhost:5432/licensify?sslmode=disable'
export LICENSIFY_API_KEY='dev'
go run ./cmd/licensify-server
```

### End-to-end smoke test app

```bash
cd examples/demo-app
go run .
```

## SDK install (pick your language)

All SDKs live in this repo. Start by cloning once:

```bash
git clone git@github.com:rithulkamesh/licensify.git
cd licensify
```

### Go (easiest)

Install:

```bash
go get github.com/rithulkamesh/licensify/sdk/go@main
```

Use:

```go
import "github.com/rithulkamesh/licensify/sdk/go/licensify"
```

### Rust

Add to your `Cargo.toml`:

```toml
[dependencies]
licensify-client-sdk-rust = { git = "https://github.com/rithulkamesh/licensify", package = "licensify-client-sdk-rust", branch = "main" }
```

### TypeScript / Node.js

One-command install from any Node/TS repo:

```bash
curl -fsSL https://raw.githubusercontent.com/rithulkamesh/licensify/main/scripts/install-licensify-ts.sh | bash
```

What it does:

- vendors the SDK into `.licensify/typescript-sdk`
- builds it
- wires `@licensify/sdk` into your `package.json` as a local file dependency
- keeps reruns idempotent (safe update path)

Use:

```ts
import { LicensifyClient } from "@licensify/sdk";
```

### C / C++ / Zig

Build the native core once:

```bash
cargo build -p licensify-client
```

Then use:

- C header: `client/include/licensify.h`
- C shim header: `sdk/c/include/licensify_c.h`
- C++ wrapper: `sdk/cpp/include/licensify.hpp`
- Zig wrapper: `sdk/zig/src/licensify.zig`

Link your app against the generated `liblicensify` from the Rust client build output.

## Development & testing

Licensify enforces 100% line coverage on every component (server, all SDKs,
MCP server, examples). See [`docs/testing.md`](docs/testing.md) for the
authoritative how-to-run-everything-locally guide and
[`docs/coverage-exclusions.md`](docs/coverage-exclusions.md) for the ledger of
all justified exclusions.

Quick smoke per component:

```bash
# Rust workspace + 100% gate
cargo build && cargo test

# Go server
cd server && go test ./...

# TypeScript SDK
cd sdk/typescript && npm ci && npm test

# MCP server
cd mcp/licensify && npm ci && npm test

# Cross-language e2e (boots harness, runs every example)
bash examples/_e2e/run.sh
```

## OSS

- Contributing: `CONTRIBUTING.md`
- Security: `SECURITY.md`
