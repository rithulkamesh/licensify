# SPDX-License-Identifier: MIT

# Licensify Integration

## Rust

Instantiate `LicensifyClient`, activate with license key, then call `check()` on startup.

## Go

Use `github.com/rithulkamesh/licensify/sdk/go/licensify`, create `Config`, call `New`, then `Activate`/`Check`.

## TypeScript

Use `@licensify/sdk` for Node.js environments where the shared library is available on the system loader path.

## C / C++ / Zig

Include `client/include/licensify.h` (or SDK wrappers) and call `licensify_new`, `licensify_activate`, `licensify_check`.
