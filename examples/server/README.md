# SPDX-License-Identifier: MIT

# Server example

Three ways to run the Licensify server depending on your environment.

## 1) In-memory (no Postgres)

Quickest dev loop, no persistence.

```bash
export LICENSIFY_API_KEY=dev
go run ./server/cmd/licensify-server
```

In another terminal:

```bash
curl -sS http://localhost:8080/v1/health
curl -sS http://localhost:8080/v1/.well-known/ca --output /tmp/licensify-root.der
```

## 2) Docker Compose with Postgres

```bash
docker compose -f docker-compose.yml up --build
```

## 3) Hermetic harness (used by SDK e2e tests)

The repo ships a small harness binary at `server/cmd/harness/` that boots an
in-memory server and writes a descriptor JSON file every SDK example reads:

```bash
go run ./server/cmd/harness
# -> writes /tmp/licensify-harness.json with base_url + api_key + license_id
```
