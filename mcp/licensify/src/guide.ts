// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Rithul Kamesh
// Author: Rithul Kamesh <hi@rithul.dev>
// Description: mcp/licensify/src/guide.ts — Helper content and prompts for the Licensify MCP server.

export const LICENSIFY_GUIDE_MARKDOWN = `# Licensify guide (for agents)

Licensify is an **offline-capable, machine-bound** licensing system. Your client is a thin wrapper over a native library (\`liblicensify\`) and exposes:

- **Activation**: binds a license key to the current machine and caches entitlements locally.
- **Check**: validates the cached token **offline-first**. A successful check means the current machine is licensed.
- **Feature gates**: a feature is available if the local entitlements contain that feature string.

## Lifecycle you should follow

1) **On startup**, call \`licensify_health\`.
   - If it reports \`clientInitialized=false\`, Licensify tools will return \`LICENSIFY_UNAVAILABLE\`. Tell the user how to install/ship the native library.

2) If you need licensing, call \`licensify_activate\` once (typically with a user-provided key).

3) Before any premium workflow, call \`licensify_check\`.
   - If \`state !== "ACTIVE"\`, do not proceed. Explain what the user can do next using the returned \`hint\`.

4) For specific premium capabilities, call \`licensify_has_feature\` for the gate you care about.
   - If \`available === false\`, do not proceed. Offer an upgrade path, or a degraded/free alternative.

## States and what to do

- **ACTIVE**: proceed with premium actions.
- **INACTIVE**: do not proceed. Common causes are: never activated, cache missing, token invalid/expired, or corrupted cache. Ask the user to re-activate with a valid key, and/or ensure the token cache path is writable and stable.

## Important implementation detail (current SDK)

The current Node-facing FFI only returns a coarse status code:
- \`statusCode=0\` => active/valid
- \`statusCode=1\` => not valid for any reason (expired/invalid/trial-expired/etc are not distinguishable)

Therefore: treat \`INACTIVE\` as a generic “not licensed” state and guide the user to re-activate or fix environment issues.
`;

