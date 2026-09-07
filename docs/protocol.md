# SPDX-License-Identifier: MIT

# Licensify Protocol

Protocol contracts are defined in `proto/licensify.proto` (`package licensify.v1`).

Core flows:
- Activation: `POST /v1/activate` with `{license_key, machine_id, opaque_registration_upload, hardware_components}` → `{leaf_certificate, intermediate_certificate}`.
- Validation: `POST /v1/validate` with `{license_key, machine_id, opaque_login_request, client_nonce}` → `{license_token}` (base64 binary token).
- Deactivation: `POST /v1/deactivate` with `{license_key}`.
- Admin: create/get/update license and manage floating seats (guarded by `X-API-Key`).

Discovery:
- `GET /v1/.well-known/ca` — root CA certificate (DER).
- `GET /v1/.well-known/token-key` — hex-encoded Ed25519 public key that signs license tokens.

The client surface authenticates by license-key possession, not the admin API key.

Tokens use a custom binary format: `version(1) | license_id(16) | machine_id(32) | issued_at(8, BE) | expires_at(8, BE) | ent_len(4, BE) | entitlements | nonce(32) | signature(64)`. The Ed25519 signature covers every field before it. `machine_id` is `SHA-256` of the client's hardware fingerprint. JWT is intentionally not used.
