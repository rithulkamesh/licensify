# Licensify

Licensify is an open-source licensing system designed for **offline-capable, machine-bound license validation** with a **language-agnostic client** exposed via a stable C ABI.

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

## Development checks

```bash
# Rust
cargo build
cargo test

# Go (server)
cd server
go test ./...
go build ./...

# TypeScript SDK
cd ../sdk/typescript
npm ci
npx tsc --noEmit
```

## OSS

- Contributing: `CONTRIBUTING.md`
- Security: `SECURITY.md`
