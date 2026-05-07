# Testing & coverage

Licensify enforces a "pragmatic 100%" coverage policy for every component
(server, SDKs, examples, MCP server). Every reachable logical branch is
covered; the small set of defensive crypto/x509 error arms is explicitly
documented in [`docs/coverage-exclusions.md`](coverage-exclusions.md) and
shipped through allowlist files (`scripts/coverage-allowlist.txt`,
`--fail-under-lines 90` for the Rust gate).

## TL;DR

```bash
# Rust client + workspace + pragmatic 90% line floor
cargo install cargo-llvm-cov --locked
cargo llvm-cov --workspace --fail-under-lines 90

# Go server (every package 100% modulo allowlist)
cd server && go test -coverprofile=cover.out -covermode=atomic ./...
bash ../scripts/check_go_coverage.sh cover.out ../scripts/coverage-allowlist.txt

# Go SDK (requires built liblicensify)
cargo build
LD_LIBRARY_PATH=$PWD/target/debug LICENSIFY_NATIVE=1 \
  go test -C sdk/go -coverprofile=cover.out -covermode=atomic ./...
bash scripts/check_go_coverage.sh sdk/go/cover.out

# TypeScript SDK
cd sdk/typescript && npm ci && npm test

# MCP server
cd mcp/licensify && npm ci && npm test

# C SDK
cd sdk/c && cmake -S . -B build -DLICENSIFY_C_COVERAGE=ON && cmake --build build
LD_LIBRARY_PATH=$PWD/../../target/debug ./build/licensify_c_tests

# C++ SDK
cd sdk/cpp && cmake -S . -B build -DLICENSIFY_CPP_COVERAGE=ON && cmake --build build
LD_LIBRARY_PATH=$PWD/../../target/debug ./build/licensify_cpp_tests

# Zig SDK
cd sdk/zig && LICENSIFY_LIB_DIR=$PWD/../../target/debug zig build test

# Examples e2e (boots harness, runs every example)
bash examples/_e2e/run.sh
```

## Component layout

| Component             | Tool                 | Coverage gate                              |
|-----------------------|----------------------|--------------------------------------------|
| `client/` (Rust + FFI)| `cargo-llvm-cov`     | `--fail-under-lines 90` (pragmatic 100%)   |
| `sdk/rust/`           | covered transitively | (re-exports only, exercised by client tests)|
| `server/`             | `go test -cover`     | `scripts/check_go_coverage.sh` (100%)      |
| `sdk/go/`             | `go test -cover`     | `scripts/check_go_coverage.sh` (100%)      |
| `sdk/typescript/`     | `c8 --check-coverage --100` |                                     |
| `sdk/c/`              | `clang -fcoverage-mapping` + `llvm-cov` |                          |
| `sdk/cpp/`            | `clang -fcoverage-mapping` + `llvm-cov` (Catch2)|                  |
| `sdk/zig/`            | `kcov` wrap          |                                            |
| `mcp/licensify/`      | `c8 --check-coverage --100` |                                     |

## Test harness

`server/cmd/harness/` is a tiny Go binary that:

1. Boots the Go server in-process on a random TCP port using the in-memory store.
2. Seeds a license key.
3. Writes a JSON descriptor (`base_url`, `api_key`, `license_key`, `license_id`)
   to `LICENSIFY_HARNESS_DESCRIPTOR` (default `/tmp/licensify-harness.json`).

Every cross-language SDK e2e test reads that descriptor.

## Coverage exclusions

See [`docs/coverage-exclusions.md`](coverage-exclusions.md) for the authoritative
list of every excluded line/region and the rationale.
