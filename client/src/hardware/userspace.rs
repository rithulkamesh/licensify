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

#[cfg(test)]
mod tests {
    use super::*;
    use serial_test::serial;

    #[test]
    #[serial]
    fn read_or_returns_env_when_set() {
        std::env::set_var("LICENSIFY_TEST_USERSPACE_KEY", "value");
        assert_eq!(read_or("LICENSIFY_TEST_USERSPACE_KEY", "fallback"), "value");
        std::env::remove_var("LICENSIFY_TEST_USERSPACE_KEY");
    }

    #[test]
    #[serial]
    fn read_or_returns_fallback_when_unset() {
        std::env::remove_var("LICENSIFY_TEST_USERSPACE_KEY_2");
        assert_eq!(read_or("LICENSIFY_TEST_USERSPACE_KEY_2", "fb"), "fb");
    }

    #[test]
    #[serial]
    fn collect_uses_envs() {
        std::env::set_var("LICENSIFY_DISK_SERIAL", "disk-X");
        std::env::set_var("LICENSIFY_BOARD_UUID", "board-Y");
        std::env::set_var("LICENSIFY_MAC", "AA:BB:CC:DD:EE:FF");
        std::env::set_var("LICENSIFY_MACHINE_ID", "mid-Z");
        std::env::set_var("LICENSIFY_GPU_ID", "gpu-W");
        let info = collect();
        assert_eq!(info.disk_serial, "disk-X");
        assert_eq!(info.motherboard_uuid, "board-Y");
        assert_eq!(info.mac_addr, "AA:BB:CC:DD:EE:FF");
        assert_eq!(info.os_machine_id, "mid-Z");
        assert_eq!(info.gpu_device_id, "gpu-W");
        assert!(!info.cpuid_brand.is_empty());
        assert!(info.cpuid_topology.contains("-cores"));
        std::env::remove_var("LICENSIFY_DISK_SERIAL");
        std::env::remove_var("LICENSIFY_BOARD_UUID");
        std::env::remove_var("LICENSIFY_MAC");
        std::env::remove_var("LICENSIFY_MACHINE_ID");
        std::env::remove_var("LICENSIFY_GPU_ID");
    }

    #[test]
    #[serial]
    fn collect_uses_fallbacks_when_envs_absent() {
        std::env::remove_var("LICENSIFY_DISK_SERIAL");
        std::env::remove_var("LICENSIFY_BOARD_UUID");
        std::env::remove_var("LICENSIFY_MAC");
        std::env::remove_var("LICENSIFY_MACHINE_ID");
        std::env::remove_var("LICENSIFY_GPU_ID");
        let info = collect();
        assert_eq!(info.disk_serial, "unknown-disk");
        assert_eq!(info.motherboard_uuid, "unknown-board");
        assert_eq!(info.mac_addr, "00:00:00:00:00:00");
        assert_eq!(info.os_machine_id, std::env::consts::OS);
        assert_eq!(info.gpu_device_id, "unknown-gpu");
    }
}
