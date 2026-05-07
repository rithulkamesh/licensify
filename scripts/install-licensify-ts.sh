#!/usr/bin/env bash
set -euo pipefail

# One-command installer for using Licensify TypeScript SDK in any Node/TS repo.
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/rithulkamesh/licensify/main/scripts/install-licensify-ts.sh | bash
#
# Optional env vars:
#   LICENSIFY_REF=main            # branch/tag/commit to fetch from
#   LICENSIFY_REPO=rithulkamesh/licensify
#   LICENSIFY_VENDOR_DIR=.licensify/typescript-sdk

LICENSIFY_REF="${LICENSIFY_REF:-main}"
LICENSIFY_REPO="${LICENSIFY_REPO:-rithulkamesh/licensify}"
LICENSIFY_VENDOR_DIR="${LICENSIFY_VENDOR_DIR:-.licensify/typescript-sdk}"

ROOT_DIR="$(pwd)"
PACKAGE_JSON="${ROOT_DIR}/package.json"

if [[ ! -f "${PACKAGE_JSON}" ]]; then
  echo "error: package.json not found in current directory."
  echo "run this command from your Node/TypeScript project root."
  exit 1
fi

if ! command -v npm >/dev/null 2>&1; then
  echo "error: npm is required for this installer."
  exit 1
fi

TMP_DIR="$(mktemp -d)"
cleanup() {
  rm -rf "${TMP_DIR}"
}
trap cleanup EXIT

echo "==> downloading Licensify (${LICENSIFY_REF})..."
curl -fsSL "https://codeload.github.com/${LICENSIFY_REPO}/tar.gz/${LICENSIFY_REF}" -o "${TMP_DIR}/licensify.tar.gz"
tar -xzf "${TMP_DIR}/licensify.tar.gz" -C "${TMP_DIR}"

EXTRACT_DIR="$(find "${TMP_DIR}" -maxdepth 1 -type d -name 'licensify-*' | head -n 1)"
if [[ -z "${EXTRACT_DIR}" ]]; then
  echo "error: failed to extract Licensify archive."
  exit 1
fi

SRC_SDK_DIR="${EXTRACT_DIR}/sdk/typescript"
if [[ ! -d "${SRC_SDK_DIR}" ]]; then
  echo "error: TypeScript SDK directory not found in archive."
  exit 1
fi

echo "==> preparing vendored SDK at ${LICENSIFY_VENDOR_DIR}..."
rm -rf "${ROOT_DIR}/${LICENSIFY_VENDOR_DIR}"
mkdir -p "${ROOT_DIR}/.licensify"
cp -R "${SRC_SDK_DIR}" "${ROOT_DIR}/${LICENSIFY_VENDOR_DIR}"

echo "==> building vendored SDK..."
(
  cd "${ROOT_DIR}/${LICENSIFY_VENDOR_DIR}"
  npm ci
  npm run build
  # Keep runtime payload small; dist is what consumers need.
  rm -rf node_modules
)

echo "==> wiring dependency into your project..."
(
  cd "${ROOT_DIR}"
  npm install "@licensify/sdk@file:${LICENSIFY_VENDOR_DIR}"
)

echo "==> writing .gitignore entries..."
if [[ -f "${ROOT_DIR}/.gitignore" ]]; then
  if ! grep -q '^\.licensify/typescript-sdk/node_modules$' "${ROOT_DIR}/.gitignore"; then
    printf '\n.licensify/typescript-sdk/node_modules\n' >> "${ROOT_DIR}/.gitignore"
  fi
else
  printf '.licensify/typescript-sdk/node_modules\n' > "${ROOT_DIR}/.gitignore"
fi

cat <<'EOF'

Licensify TypeScript SDK installed.

You can now import it normally:

  import { LicensifyClient } from "@licensify/sdk";

Re-run this same installer anytime to update/refresh.
EOF
