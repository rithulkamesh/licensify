# SPDX-License-Identifier: MIT

# Licensify Protocol

Protocol contracts are defined in `proto/licensify.proto` (`package licensify.v1`).

Core flows:
- Activation: registration request/upload plus certificate issuance.
- Validation: login request plus token refresh.
- Deactivation: revoke machine binding.
- Admin: create/get/update license and manage floating seats.

Tokens use a custom binary format with an Ed25519 signature and per-token nonce. JWT is intentionally not used.
