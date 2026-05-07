#!/usr/bin/env bash
# SPDX-License-Identifier: MIT
# Copyright (c) 2026 Rithul Kamesh
# Author: Rithul Kamesh <hi@rithul.dev>
# Description: scripts/check_go_coverage.sh — Asserts every measured Go function hits 100%, except documented defensive exclusions.

set -euo pipefail

if [[ $# -lt 1 ]]; then
  echo "usage: $0 <coverage.out> [allowlist]" >&2
  exit 2
fi

PROFILE="$1"
ALLOWLIST="${2:-$(dirname "$0")/coverage-allowlist.txt}"

if [[ ! -f "$PROFILE" ]]; then
  echo "coverage profile not found: $PROFILE" >&2
  exit 2
fi

# Build a set of allowed "file:line" prefixes. The allowlist file has one entry
# per line in the form `path/to/file.go:line` plus optional `# justification`.
allowed=()
if [[ -f "$ALLOWLIST" ]]; then
  while IFS= read -r raw; do
    line="${raw%%#*}"
    line="${line%% *}"
    line="${line%%	*}"
    [[ -z "$line" ]] && continue
    allowed+=("$line")
  done < "$ALLOWLIST"
fi

is_allowed() {
  local key="$1"
  for a in "${allowed[@]}"; do
    if [[ "$key" == *"$a"* ]]; then
      return 0
    fi
  done
  return 1
}

fail=0
while IFS= read -r line; do
  [[ "$line" == total:* ]] && continue
  pct="${line##*	}"
  pct="${pct%\%}"
  if awk -v p="$pct" 'BEGIN{exit !(p+0 < 100)}'; then
    # Extract `path:line` prefix to match against the allowlist.
    key="${line%%	*}"
    if is_allowed "$key"; then
      echo "allowed-defensive: $line"
      continue
    fi
    echo "below-100%: $line"
    fail=1
  fi
done < <(go tool cover -func="$PROFILE")

if [[ $fail -ne 0 ]]; then
  echo "coverage check failed; some functions are below 100% and not in the allowlist" >&2
  exit 1
fi

echo "coverage check passed"
