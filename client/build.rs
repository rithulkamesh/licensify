// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Rithul Kamesh
// Author: Rithul Kamesh <hi@rithul.dev>
// Description: client/build.rs — Build script for generating/validating FFI artifacts during Rust builds.

fn main() {
    println!("cargo:rerun-if-changed=src/ffi.rs");
}
