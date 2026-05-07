#!/usr/bin/env bash
# SPDX-License-Identifier: MIT
# Copyright (c) 2026 Rithul Kamesh
# Author: Rithul Kamesh <hi@rithul.dev>
# Description: examples/_e2e/run.sh — End-to-end smoke runner that boots the harness and asserts every example exits zero with expected output.

set -euo pipefail

REPO_ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." &>/dev/null && pwd)"
cd "$REPO_ROOT"

# Build the native client lib.
cargo build

LIBLICENSIFY_DIR="$REPO_ROOT/target/debug"
case "$(uname -s)" in
  Darwin) export DYLD_LIBRARY_PATH="$LIBLICENSIFY_DIR:${DYLD_LIBRARY_PATH:-}" ;;
  Linux)  export LD_LIBRARY_PATH="$LIBLICENSIFY_DIR:${LD_LIBRARY_PATH:-}" ;;
esac
export LICENSIFY_NATIVE=1

# Build the harness and start it.
DESCRIPTOR="$(mktemp -t licensify-harness-XXXXXX.json)"
export LICENSIFY_HARNESS_DESCRIPTOR="$DESCRIPTOR"
export LICENSIFY_HARNESS_API_KEY="dev"

(cd server && go build -o "$REPO_ROOT/target/harness" ./cmd/harness)
"$REPO_ROOT/target/harness" &
HARNESS_PID=$!
trap 'kill "$HARNESS_PID" 2>/dev/null || true; rm -f "$DESCRIPTOR"' EXIT

# Wait for descriptor file to appear.
for _ in $(seq 1 50); do
  if [[ -s "$DESCRIPTOR" ]]; then break; fi
  sleep 0.1
done
if [[ ! -s "$DESCRIPTOR" ]]; then
  echo "harness failed to write descriptor at $DESCRIPTOR" >&2
  exit 1
fi

BASE_URL=$(python3 -c "import json,sys;print(json.load(open(sys.argv[1]))['base_url'])" "$DESCRIPTOR")
API_KEY=$(python3 -c "import json,sys;print(json.load(open(sys.argv[1]))['api_key'])" "$DESCRIPTOR")
LICENSE_KEY=$(python3 -c "import json,sys;print(json.load(open(sys.argv[1]))['license_key'])" "$DESCRIPTOR")
export LICENSIFY_BASE_URL="$BASE_URL"
export LICENSIFY_API_KEY="$API_KEY"
export LICENSIFY_LICENSE_KEY="$LICENSE_KEY"

assert_contains() {
  local out="$1" needle="$2"
  if ! grep -qF -- "$needle" <<<"$out"; then
    echo "expected stdout to contain: $needle"
    echo "got: $out"
    exit 1
  fi
}

# 1) demo-app
echo "::: examples/demo-app"
out=$(go run ./examples/demo-app)
echo "$out"
assert_contains "$out" "demo: success"

# 2) go-sdk
echo "::: examples/go-sdk"
out=$(go run ./examples/go-sdk)
echo "$out"
assert_contains "$out" "status="

# 3) rust-client (uses cargo run --example via a tiny crate setup)
echo "::: examples/rust-client"
out=$(cargo run -p rust-client-example 2>&1)
echo "$out"
assert_contains "$out" "rust-client-example: success"

# 4) typescript example
echo "::: examples/typescript"
(cd examples/typescript && npm install && npm run build)
out=$(node examples/typescript/dist/index.js)
echo "$out"
assert_contains "$out" "ts-example: success"

# 5) mcp-agent example
echo "::: examples/mcp-agent"
(cd mcp/licensify && npm ci && npm run build)
(cd examples/mcp-agent && npm install && npm run build)
out=$(LICENSIFY_LICENSE_KEY="$LICENSE_KEY" node examples/mcp-agent/dist/run.js)
echo "$out"
assert_contains "$out" "summary:"

echo "examples e2e: all green"
