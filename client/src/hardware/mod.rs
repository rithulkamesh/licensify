// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Rithul Kamesh
// Author: Rithul Kamesh <hi@rithul.dev>
// Description: client/src/hardware/mod.rs — Hardware fingerprint interfaces for machine identity binding.

pub mod fingerprint;
pub mod tpm;
pub mod userspace;

pub use fingerprint::{machine_id_bytes, machine_id_hex, HardwareComponents};
