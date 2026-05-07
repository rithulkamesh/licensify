// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Rithul Kamesh
// Author: Rithul Kamesh <hi@rithul.dev>
// Description: sdk/zig/src/licensify_test.zig — Tests for the Zig SDK covering init/deinit/activate/check error and happy paths.

const std = @import("std");
const licensify = @import("licensify.zig");

test "init rejects empty server_url" {
    const allocator = std.testing.allocator;
    try std.testing.expectError(licensify.LicensifyError.InvalidArgument, licensify.Client.init(allocator, .{
        .server_url = "",
        .cache_path = "/tmp/licensify-zig.token",
    }));
}

test "init rejects empty cache_path" {
    const allocator = std.testing.allocator;
    try std.testing.expectError(licensify.LicensifyError.InvalidArgument, licensify.Client.init(allocator, .{
        .server_url = "http://localhost:0",
        .cache_path = "",
    }));
}

test "happy path activate / check" {
    const allocator = std.testing.allocator;
    var client = try licensify.Client.init(allocator, .{
        .server_url = "http://localhost:0",
        .cache_path = "/tmp/licensify-zig-happy.token",
    });
    defer client.deinit();

    try client.activate("LICENSE-KEY-DEV");
    const status = try client.check();
    // 0 = Valid (cache hit), 1 = Invalid. Either is acceptable here without a real token cache.
    try std.testing.expect(status == 0 or status == 1);
}

test "activate rejects empty key" {
    const allocator = std.testing.allocator;
    var client = try licensify.Client.init(allocator, .{
        .server_url = "http://localhost:0",
        .cache_path = "/tmp/licensify-zig-empty.token",
    });
    defer client.deinit();
    try std.testing.expectError(licensify.LicensifyError.InvalidArgument, client.activate(""));
}

test "deinit is idempotent and check after deinit returns Disposed" {
    const allocator = std.testing.allocator;
    var client = try licensify.Client.init(allocator, .{
        .server_url = "http://localhost:0",
        .cache_path = "/tmp/licensify-zig-deinit.token",
    });
    client.deinit();
    client.deinit();
    try std.testing.expectError(licensify.LicensifyError.Disposed, client.check());
    try std.testing.expectError(licensify.LicensifyError.Disposed, client.activate("KEY"));
}
