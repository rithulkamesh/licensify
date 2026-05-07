# Contributing

Thanks for taking the time to contribute to **Licensify**.

This repository is a public OSS project, so we try to keep changes:
- Small and reviewable
- Well-tested (at least at the package/module level)
- Backwards-compatible for published client APIs

## Ways to contribute

- Bug reports and reproducible test cases
- Documentation fixes (typos, clarity, missing details)
- SDK improvements (Go/TypeScript/C/C++/Zig wrappers)
- Hardening: input validation, error reporting, edge-case coverage

## Quick start (local dev)

### Prereqs

- Rust toolchain (stable)
- Go toolchain (stable)
- Node.js + npm (for `sdk/typescript`)
- Docker (optional, for Postgres + server via Compose)

### Run the server + Postgres (optional)

```bash
docker compose up -d --build
curl -sS http://localhost:8080/v1/health
```

## Development

- **Rust**: `cargo build && cargo test`
- **Go (server)**: `go build ./... && go test ./...` (from `server/`)
- **TypeScript SDK**: `npm ci && npx tsc --noEmit` (from `sdk/typescript/`)

## Pull requests

- Keep PRs focused; avoid unrelated refactors.
- Prefer adding/adjusting tests when fixing bugs.
- Update docs when public-facing behavior changes.

## Code style

- Keep public APIs stable (Rust `LicensifyClient`, C ABI `licensify_*`, Go/TS wrappers).
- No JWT anywhere; tokens are binary + Ed25519.
- No stubs / TODO placeholders in shipped code.

## Licensing

By contributing, you agree that your contributions will be licensed under the MIT License (see `LICENSE`).
