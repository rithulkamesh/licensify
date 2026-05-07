// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Rithul Kamesh
// Author: Rithul Kamesh <hi@rithul.dev>
// Description: sdk/zig/build.zig — Zig build script for compiling/linking the Licensify Zig SDK and its tests.

const std = @import("std");

pub fn build(b: *std.Build) void {
    const target = b.standardTargetOptions(.{});
    const optimize = b.standardOptimizeOption(.{});

    const mod = b.addModule("licensify", .{
        .root_source_file = b.path("src/licensify.zig"),
    });
    _ = mod;

    // Tests link against the prebuilt `liblicensify` produced by `cargo build`
    // in the workspace's target/{debug,release} directory. Set the
    // LICENSIFY_LIB_DIR environment variable to point at a custom location.
    const lib_dir = std.process.getEnvVarOwned(b.allocator, "LICENSIFY_LIB_DIR") catch |err| switch (err) {
        error.EnvironmentVariableNotFound => b.pathFromRoot("../../target/debug"),
        else => @panic("cannot read LICENSIFY_LIB_DIR"),
    };

    const tests = b.addTest(.{
        .root_source_file = b.path("src/licensify_test.zig"),
        .target = target,
        .optimize = optimize,
    });
    tests.addIncludePath(b.path("../../client/include"));
    tests.addLibraryPath(.{ .cwd_relative = lib_dir });
    tests.linkSystemLibrary("licensify");
    tests.linkLibC();

    const run_tests = b.addRunArtifact(tests);
    const test_step = b.step("test", "Run Zig SDK tests");
    test_step.dependOn(&run_tests.step);
}
