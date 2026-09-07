# Coverage exclusions

This file is the authoritative ledger of every coverage-excluded region in the
repository. Any new exclusion **must** be added here with a justification and
the test that would otherwise be required.

Reviewer policy: a PR that introduces an `// LCOV_EXCL_LINE`, `c8 ignore`,
`grcov-excl-line`, `#[cfg(not(coverage))]`, or equivalent without an entry in
this file will be rejected.

## Active exclusions

### `client/src/ffi.rs` — `licensify_new` `Err` arm

- **Reason**: `LicensifyClient::new` is currently infallible. The `Err` arm of
  the match in `licensify_new` exists for ABI stability — if the constructor
  becomes fallible later, callers should observe a NULL return rather than UB.
  Triggering it from a test would require artificially making `new` return
  `Err`, which would itself defeat the test's purpose.
- **Marker**: `// grcov-excl-line: defensive path retained for ABI stability.`
- **Test that would otherwise be required**: a test that injects a panicking
  `ClientConfig` and asserts `licensify_new` returns NULL.

### `client/src/{cache,ffi,lib,token}.rs` — defensive `.map_err` crypto arms

- **Reason**: Each `.map_err(|e| LicenseError::Crypto(e.to_string()))` after an
  `aes-gcm`, `hkdf`, `ed25519-dalek`, or `x509-cert` call is reachable only
  when the underlying primitive returns an error — which requires a malformed
  fixed-size key/nonce that the type system already prevents. We measure them
  in CI but allow the workspace gate at `--fail-under-lines 90` rather than
  `100` to acknowledge those branches; every reachable line is covered.
- **Marker**: none in source; documented here.
- **Test that would otherwise be required**: replacing the stdlib RNG with a
  failing one, which is out of scope.

### `sdk/rust/src/lib.rs` — `re_export_lifecycle` non-default branches

- **Reason**: The `re_export_lifecycle` test exercises `Valid`/`Invalid` arms
  of `LicenseStatus`. The `Expired`, `OfflineGrace`, and `TrialExpired`
  variants would require driving the harness through expiration and offline
  drift, which is covered by integration tests in `client/src/token.rs` and
  the e2e harness rather than this re-export sanity check.
- **Marker**: none in source; documented here.

### `sdk/typescript/src/index.ts` — `loadNative()`

- **Reason**: This function loads `koffi` and binds `liblicensify`. Both
  are runtime-loaded native dependencies that are already exercised end-to-end
  by the native e2e test (`LICENSIFY_NATIVE=1`) which runs in CI. Including it
  in the c8 measurement would double-count and require the test runner to load
  native deps it doesn't need.
- **Marker**: `/* c8 ignore start ... c8 ignore stop */`
- **Test that would otherwise be required**: a unit test that mocks `koffi`
  via the Node loader, which would not exercise the actual library binding.

### `mcp/licensify/src/server.ts` — `loadSdk()`, `start()`, McpServer wiring

- **Reason**: `loadSdk` resolves the in-repo SDK path via dynamic import,
  `start()` connects an `StdioServerTransport`, and the inner `registerTool`
  callbacks are thin wire-up around the `Handlers` class which is fully tested
  in `mcp/licensify/src/handlers.test.ts`. The composition is exercised by the
  example mcp-agent in `examples/_e2e/run.sh`.
- **Marker**: `/* c8 ignore start ... c8 ignore stop */`
- **Test that would otherwise be required**: a test that drives the McpServer
  through its in-memory transport (the SDK does not currently expose one) — we
  rely on the stdio integration smoke test instead.

### Platform-specific hardware probes

- **Files**: future `client/src/hardware/{linux,macos,windows}.rs`
  implementations.
- **Reason**: CI is Linux-only; macOS/Windows-specific branches are gated with
  `#[cfg(target_os = "...")]` and only the active OS path is measured.
- **Marker**: `#[cfg(target_os = "...")]` (no extra exclusion needed; the dead
  branches don't compile).

### Server: `cmd/licensify-server/main.go:35` — `main()`

- **Reason**: `main` is a one-line wrapper that calls `entrypoint(os.Getenv)`.
  All branching logic lives in `entrypoint`, which is exhaustively tested via
  the `runFn` / `fatalFn` seams in `cmd/licensify-server/main_test.go`.
- **Marker**: `scripts/coverage-allowlist.txt` entry.

### Server: defensive `crypto/rand` and `x509.CreateCertificate` error returns

- **Files / functions**:
  - `internal/auth/service.go:36` — `StartLogin` (`rand.Read` failure)
  - `internal/ca/ca.go:36` — `NewAuthority` (`ed25519.GenerateKey`,
    `x509.CreateCertificate`, `x509.ParseCertificate` failures)
  - `internal/ca/ca.go:98` — `IssueLeaf` (same crypto failures)
  - `internal/token/token.go:71` — `Build` (`rand.Read` failure)
  - `internal/api/server.go:165` — `activate` (cascading `IssueLeaf` failure)
- **Reason**: These return error paths only fire when the OS-level RNG or the
  Go `crypto/x509` package returns an error — situations that cannot be
  reproduced from a Go test without monkey-patching the standard library. The
  happy paths and structural assertions are 100% covered.
- **Marker**: `scripts/coverage-allowlist.txt` entries.

### Server: `internal/server/run.go:39` — `Run()` default `Listener`

- **Reason**: The default `Listener` calls `srv.Echo.Start(addr)` which blocks
  until error or graceful shutdown. Exercising it without a real bind would
  spawn goroutines and race the test runner. Instead, the default path is
  exercised by `server/cmd/harness/main.go` (the in-process server harness used by
  every SDK e2e test).
- **Marker**: `scripts/coverage-allowlist.txt` entry.

### Server: `internal/db/memstore.go:106` — `MemStore.Close`

- **Reason**: `Close()` has no body (the in-memory store has no resources to
  release). `go test` reports 0/0 statements as 0.0% even though it is called
  from `TestMemStoreHealthAndClose`.
- **Marker**: `scripts/coverage-allowlist.txt` entry.

### Server: `internal/db/pgstore.go` — entire file

- **Reason**: `PgStore` requires a running Postgres instance. The CI matrix
  spins one up via the `postgres:16` service container and runs
  `pgstore_test.go` against it; locally the tests skip when
  `LICENSIFY_TEST_DATABASE_URL` is unset, so coverage reports 0%. The
  allowlist treats this whole file as defensive locally; CI re-runs the gate
  with the env var set so it must reach 100% there.
- **Marker**: `scripts/coverage-allowlist.txt` entry.

## Inactive exclusions

None.
