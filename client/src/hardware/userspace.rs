// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Rithul Kamesh
// Author: Rithul Kamesh <hi@rithul.dev>
// Description: client/src/hardware/userspace.rs — Userspace hardware probes used to compute machine identifiers.

use std::env;

#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct UserspaceInfo {
    pub cpuid_brand: String,
    pub cpuid_topology: String,
    pub disk_serial: String,
    pub motherboard_uuid: String,
    pub mac_addr: String,
    pub os_machine_id: String,
    pub gpu_device_id: String,
}

pub fn collect() -> UserspaceInfo {
    UserspaceInfo {
        cpuid_brand: env::consts::ARCH.to_string(),
        cpuid_topology: format!("{}-cores", std::thread::available_parallelism().map(|v| v.get()).unwrap_or(1)),
        disk_serial: read_or("LICENSIFY_DISK_SERIAL", "unknown-disk"),
        motherboard_uuid: read_or("LICENSIFY_BOARD_UUID", "unknown-board"),
        mac_addr: read_or("LICENSIFY_MAC", "00:00:00:00:00:00"),
        os_machine_id: read_or("LICENSIFY_MACHINE_ID", env::consts::OS),
        gpu_device_id: read_or("LICENSIFY_GPU_ID", "unknown-gpu"),
    }
}

fn read_or(key: &str, fallback: &str) -> String {
    std::env::var(key).unwrap_or_else(|_| fallback.to_string())
}
