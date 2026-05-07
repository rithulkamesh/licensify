// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Rithul Kamesh
// Author: Rithul Kamesh <hi@rithul.dev>
// Description: sdk/zig/src/licensify.zig — Zig SDK wrapper around the Licensify C ABI.

const std = @import("std");

pub const Config = struct {
    server_url: []const u8,
    cache_path: []const u8,
};

pub const ErrorCode = enum(i32) {
    ok = 0,
    invalid_argument = 1,
    initialization = 2,
    activation = 3,
    check = 4,
};

pub const LicensifyError = error{
    InvalidArgument,
    InitializationFailed,
    ActivationFailed,
    CheckFailed,
    Disposed,
};

const c = @cImport({
    @cInclude("licensify.h");
});

pub const Client = struct {
    allocator: std.mem.Allocator,
    config: Config,
    native: ?*c.licensify_client_t = null,

    pub fn init(allocator: std.mem.Allocator, config: Config) !Client {
        if (config.server_url.len == 0 or config.cache_path.len == 0) return LicensifyError.InvalidArgument;
        var self = Client{ .allocator = allocator, .config = config, .native = null };
        const server_z = try allocator.dupeZ(u8, config.server_url);
        defer allocator.free(server_z);
        const cache_z = try allocator.dupeZ(u8, config.cache_path);
        defer allocator.free(cache_z);
        var cfg = c.licensify_config_t{ .server_url = server_z.ptr, .cache_path = cache_z.ptr };
        const ptr = c.licensify_new(&cfg);
        if (ptr == null) return LicensifyError.InitializationFailed;
        self.native = ptr;
        return self;
    }

    pub fn deinit(self: *Client) void {
        if (self.native) |p| {
            c.licensify_free(p);
            self.native = null;
        }
    }

    pub fn check(self: *Client) !i32 {
        const p = self.native orelse return LicensifyError.Disposed;
        var out_status: i32 = 0;
        const code: i32 = @intCast(c.licensify_check_code(p, &out_status));
        if (code != 0) return LicensifyError.CheckFailed;
        return out_status;
    }

    pub fn activate(self: *Client, key: []const u8) !void {
        const p = self.native orelse return LicensifyError.Disposed;
        if (key.len == 0) return LicensifyError.InvalidArgument;
        const key_z = try self.allocator.dupeZ(u8, key);
        defer self.allocator.free(key_z);
        const code: i32 = @intCast(c.licensify_activate_code(p, key_z.ptr));
        if (code != 0) return LicensifyError.ActivationFailed;
    }
};
