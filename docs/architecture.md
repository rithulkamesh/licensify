# SPDX-License-Identifier: MIT

# Licensify Architecture

Licensify is split into a Go server, Rust client core, and multi-language SDK wrappers over a C FFI boundary.

- Server uses Echo + pgx and owns license issuance, certificate chain generation, token issuance, and audit events.
- Client computes a machine-bound fingerprint, performs online activation, and supports offline checks via an encrypted local token cache.
- SDKs for Go/C++/TS/Rust/C/Zig wrap the same C ABI (`licensify_*`) for consistent behavior.

## Key invariants

- **No JWT**: tokens are a binary format with an Ed25519 signature over every field; offline validation never depends on opaque JSON structures.
- **Stable token-signing key**: the server signs license tokens with one long-lived Ed25519 key (`LICENSIFY_TOKEN_SIGNING_KEY`, a 32-byte seed; random per-process if unset). Its public half is published at `GET /v1/.well-known/token-key`. Clients verify cached tokens against this key — configured explicitly, learned during `activate`, or via `LICENSIFY_SERVER_PUBLIC_KEY`.
- **Machine binding is enforced**: a token embeds `SHA-256(machine_id)`. `check()` recomputes the local fingerprint and rejects any token not bound to this machine — not just at cache-encryption time but on every validation.
- **Machine-bound cache**: the cached token is sealed with AES-256-GCM under a key derived (HKDF-SHA256, salted) from the hardware fingerprint, with a fresh random 96-bit nonce per write. Blob layout: `[version | nonce(12) | ciphertext+tag]`.
- **Rollback protection**: `check()` records the newest `issued_at` it has honoured (a `<cache>.seen` sidecar) and rejects a strictly older token — defeating clock-rollback and stale-token re-seed.
- **Real anti-tamper (opt-in)**: when a host-binary SHA-256 is pinned (`ClientConfig.expected_digest`, `licensify_set_expected_digest`, or `LICENSIFY_EXPECTED_DIGEST`), `check()` recomputes the running executable's digest and fails closed on mismatch.
- **Real certificate-chain verification**: `cert::verify_chain` validates a `leaf → intermediate → root` Ed25519 chain — issuer signatures, `notBefore/notAfter` windows, and `basicConstraints cA=TRUE` on the CAs. `activate` runs it against the server's published root before trusting an issued leaf.
- **FFI-first client**: the Rust client is the reference implementation; other SDKs delegate to it. The C ABI (`licensify_config_t` layout and the original exported functions) is stable; hardening APIs are additive.

## Auth model

- **Admin surface** (`POST /v1/license`, `GET/PUT /v1/license/:id`, `/v1/seats/*`) is guarded by `X-API-Key`.
- **Client surface** (`/v1/activate`, `/v1/validate`, `/v1/deactivate`, `/v1/heartbeat`) authenticates by license-key possession — a client SDK never needs the admin key.
