# SPDX-License-Identifier: MIT

# Server example

## Run

```bash
export LICENSIFY_API_KEY=dev
cd server
go run ./cmd/licensify-server
```

## Smoke test

```bash
curl -sS http://localhost:8080/v1/health
curl -sS http://localhost:8080/v1/.well-known/ca --output /tmp/licensify-root.der
```
