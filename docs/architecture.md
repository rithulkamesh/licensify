# SPDX-License-Identifier: MIT

# Licensify Architecture

Licensify is split into a Go server, Rust client core, and multi-language SDK wrappers over a C FFI boundary.

- Server uses Echo + pgx and owns license issuance, certificate chain generation, token issuance, and audit events.
- Client computes a machine-bound fingerprint, performs online validation, and supports offline checks via encrypted local token cache.
- SDKs for Go/C++/TS/Rust/C/Zig wrap the same C ABI (`licensify_*`) for consistent behavior.

## Key invariants

- **No JWT**: tokens are binary and signed; offline validation never depends on opaque JSON structures.
- **Machine-bound cache**: cached tokens are encrypted with a key derived from the hardware fingerprint.
- **FFI-first client**: Rust client is the reference implementation; other SDKs delegate to it.
