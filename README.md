# Licensify

[![CI](https://github.com/rithulkamesh/licensify/actions/workflows/ci.yml/badge.svg)](https://github.com/rithulkamesh/licensify/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.23-00ADD8?logo=go)](https://go.dev)
[![Rust](https://img.shields.io/badge/Rust-stable-000000?logo=rust)](https://www.rust-lang.org)
[![TypeScript](https://img.shields.io/badge/TypeScript-5.x-3178C6?logo=typescript)](https://www.typescriptlang.org)
[![Zig](https://img.shields.io/badge/Zig-0.13-F7A41D?logo=zig)](https://ziglang.org)

**Offline-capable, machine-bound license validation** with a **language-agnostic client** exposed via a stable C ABI.

> One hardened Rust core — consumed from **Go, C, C++, TypeScript, Rust, Zig**.

## ⭐ Star History

[![Star History Chart](https://api.star-history.com/svg?repos=rithulkamesh/licensify&type=Date)](https://star-history.com/#rithulkamesh/licensify&Date)

## Why Licensify

- **No JWT** — custom binary token with an Ed25519 signature over every field, not self-describing JSON payloads.
- **Machine-bound and enforced** — a token embeds `SHA-256(machine_id)`; `check()` recomputes the local fingerprint on every validation and rejects tokens bound to another host.
- **Offline-first enforcement** — the cached token is sealed with AES-256-GCM (per-write random nonce) under a key derived from the hardware fingerprint, with clock-rollback protection.
- **FFI-first architecture** — one hardened Rust core, consumed consistently from Go/C++/TypeScript/Rust/C/Zig over a stable C ABI.
- **Server-issued cert chain** — `activate` verifies the issued `leaf → intermediate → root` Ed25519 chain (signatures, validity windows, CA constraints) against the server's published root.
- **Optional anti-tamper** — pin the host binary's SHA-256 (`LICENSIFY_EXPECTED_DIGEST` / `licensify_set_expected_digest`) and `check()` fails closed if the running executable is modified.

### Key discovery & hardening knobs

| Setting | Purpose |
|---------|---------|
| `LICENSIFY_TOKEN_SIGNING_KEY` (server) | 32-byte seed for the stable token-signing key; keep it stable so old offline tokens keep verifying. |
| `GET /v1/.well-known/token-key` | Server's token-signing public key (hex); the client fetches this during `activate`. |
| `LICENSIFY_SERVER_PUBLIC_KEY` / `licensify_set_server_key` (client) | Set the verification key explicitly for offline-only deployments. |
| `LICENSIFY_EXPECTED_DIGEST` / `licensify_set_expected_digest` (client) | Pin the host-binary SHA-256 for anti-tamper. |

## Quick start

### Docker (recommended)

```bash
git clone https://github.com/rithulkamesh/licensify.git
cd licensify
docker compose up -d --build
curl -sS http://localhost:8080/v1/health
```

### Native server

```bash
git clone https://github.com/rithulkamesh/licensify.git
cd licensify/server
export DATABASE_URL='postgres://licensify:licensify@localhost:5432/licensify?sslmode=disable'
export LICENSIFY_API_KEY='dev'
go run ./cmd/licensify-server
```

## SDK install (pick your language)

### Go

```bash
go get github.com/rithulkamesh/licensify/sdk/go@main
```

```go
import "github.com/rithulkamesh/licensify/sdk/go/licensify"
```

### Rust

```toml
[dependencies]
licensify-client-sdk-rust = { git = "https://github.com/rithulkamesh/licensify", package = "licensify-client-sdk-rust", branch = "main" }
```

### TypeScript / Node.js

```bash
curl -fsSL https://raw.githubusercontent.com/rithulkamesh/licensify/main/scripts/install-licensify-ts.sh | bash
```

```ts
import { LicensifyClient } from "@licensify/sdk";
```

### C / C++ / Zig

```bash
cargo build -p licensify-client
```

- C header: `client/include/licensify.h`
- C shim: `sdk/c/include/licensify_c.h`
- C++ wrapper: `sdk/cpp/include/licensify.hpp`
- Zig wrapper: `sdk/zig/src/licensify.zig`

Link your app against the generated `liblicensify` from the Rust build output.

## Repository layout

| Directory | Description |
|-----------|-------------|
| `server/` | Go licensing API (Echo + pgx) |
| `client/` | Rust client core + C FFI (`licensify_*`) |
| `sdk/` | Language SDKs wrapping the C ABI |
| `proto/` | Shared protocol contracts |
| `docs/` | Architecture, protocol, integration |
| `examples/` | Smoke tests and usage snippets |

## Development & testing

100% line coverage is enforced on every component. See [`docs/testing.md`](docs/testing.md) and [`docs/coverage-exclusions.md`](docs/coverage-exclusions.md).

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

## Contributing

Contributions are welcome! Please read [`CONTRIBUTING.md`](CONTRIBUTING.md) for guidelines.

## Security

Report vulnerabilities per [`SECURITY.md`](SECURITY.md).

## License

[MIT](LICENSE) &copy; Rithul Kamesh
