# C SDK

The C SDK is the stable ABI surface exposed by the Rust client.

It is intended for:

- embedding in native apps that want the reference client behavior
- building higher-level wrappers (e.g. the C++/Zig/Go SDKs in this repo)

## Key functions

- `licensify_new`
- `licensify_activate_code` + `licensify_last_error`
- `licensify_check_code` + `licensify_last_error`
- `licensify_free`

## Memory management

- If you call `licensify_activate` (struct-return), you must free `result.message` with `licensify_string_free`.
- If you use `*_code` APIs, use `licensify_last_error` (no allocation; owned by the client).

## Recommended usage

For most embedders, the `*_code` APIs are easiest:

- they return a numeric status code
- you can fetch the last error string without managing allocations

If you need richer data (messages, metadata), use the struct-return APIs and follow the ownership rules above.

## Related docs

- `sdk-cpp.md` — header-only RAII wrapper
- `sdk-zig.md` — Zig wrapper and allocator integration
