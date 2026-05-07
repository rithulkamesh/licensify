// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Rithul Kamesh
// Author: Rithul Kamesh <hi@rithul.dev>
// Description: sdk/zig/build.zig — Zig build script for compiling/linking the Licensify Zig SDK.

const std = @import("std");

pub fn build(b: *std.Build) void {
    const mod = b.addModule("licensify", .{
        .root_source_file = b.path("src/licensify.zig"),
    });
    _ = mod;
}
