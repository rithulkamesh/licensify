# TypeScript SDK (Node.js)

The TypeScript SDK wraps the native client via `ffi-napi`. Use it for Node.js runtimes where you can ship or install the Licensify native library alongside your app.

## Install

```bash
npm i @licensify/sdk
```

## Prerequisites (native library)

The SDK loads the Licensify shared library using the system loader rules. Make sure it is discoverable via one of:

- your app’s packaged native dependencies
- `DYLD_LIBRARY_PATH` (macOS) / `LD_LIBRARY_PATH` (Linux)
- platform-specific loader configuration

If you’re developing inside this repo, build the client core first (see the root `README.md`).

## API shape

- `LicensifyClient.create(config)` async factory
- `activate(licenseKey)` and `check()`
- `dispose()` lifecycle
- Typed errors
- Optional dependency injection (native bindings + clock) for tests

## Example

```ts
import { LicensifyClient } from "@licensify/sdk";

const client = await LicensifyClient.create({
  serverUrl: "http://localhost:8080",
  cachePath: "/tmp/licensify.token",
  logger: console,
});

try {
  await client.activate("LICENSE-KEY");
  const status = await client.check();
  console.log(status);
} finally {
  client.dispose();
}
```

## Errors

- `InitializationError`
- `ActivationError` (has `code` + `metadata`)
- `CheckError` (has `code` + `metadata`)

## Tips

- Prefer an app-specific cache directory over `/tmp` for real products.
- If your deployment uses a locked-down runtime, validate native library loading early (startup) so failures are obvious.
