# Zig SDK

Zig wrapper over the Licensify C ABI (`licensify.h`).

## Prerequisites

- The Licensify shared library must be discoverable by the dynamic loader.
- Your build must link against the native library (or load it dynamically, depending on your wrapper setup).

## Example

```zig
const std = @import("std");
const licensify = @import("licensify");

pub fn main() !void {
    var gpa = std.heap.GeneralPurposeAllocator(.{}){};
    defer _ = gpa.deinit();
    const a = gpa.allocator();

    var client = try licensify.Client.init(a, .{ .server_url = "http://localhost:8080", .cache_path = "/tmp/licensify.token" });
    defer client.deinit();
    try client.activate("LICENSE-KEY");
    _ = try client.check();
}
```

## Tips

- Pick a stable cache path; `/tmp` is for demos.
- If you’re building your own wrapper, model your ownership rules after the C SDK docs (`sdk-c.md`).
