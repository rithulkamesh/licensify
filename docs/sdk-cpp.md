# C++ SDK

Header-only RAII wrapper over the Licensify C ABI.

Use this when you want:

- exceptions + typed error classes
- RAII lifecycle management
- a minimal dependency footprint (no extra runtime beyond the native library)

## Prerequisites

- The Licensify shared library must be discoverable by the dynamic loader.
- Include paths must include the SDK header(s).

## Example

```cpp
#include <licensify.hpp>

int main() {
  auto client = Licensify::Client::create({.server_url="http://localhost:8080", .cache_path="/tmp/licensify.token"});
  client.activate("LICENSE-KEY");
  const int st = client.check();
  (void)st;
  client.close();
}
```

## Errors

Throws typed exceptions:
- `Licensify::InitializationError`
- `Licensify::ActivationError` (includes native error code)
- `Licensify::CheckError` (includes native error code)

## Tips

- Use a stable, per-user cache path in production (avoid `/tmp`).
- Prefer `close()` in a scope guard / destructor path if you integrate into larger apps.
