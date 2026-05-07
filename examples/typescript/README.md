# SPDX-License-Identifier: MIT

# TypeScript SDK example

A minimal Node.js script that activates a license key, checks status, and tests
two feature gates against a running Licensify server.

## Prerequisites

- The Licensify TypeScript SDK is built (`cd sdk/typescript && npm ci && npm run build`).
- The Licensify Rust client is built (`cargo build`) so `liblicensify` is on the loader path.
- `ffi-napi` and `ref-napi` are installed in the example: `npm ci`.

## Run

```bash
cd examples/typescript
npm ci
npm run build
LICENSIFY_BASE_URL=http://localhost:8080 \
  LICENSIFY_LICENSE_KEY=LICENSE-KEY-DEV \
  LICENSIFY_CACHE_PATH=/tmp/licensify.token \
  npm start
```

Expected stdout includes the line `ts-example: success`.
