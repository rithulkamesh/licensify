// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Rithul Kamesh
// Author: Rithul Kamesh <hi@rithul.dev>
// Description: client/src/hardware/fingerprint.rs — Cross-platform hardware fingerprint derivation helpers.

use sha2::{Digest, Sha256};

use super::{tpm, userspace};

#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct HardwareComponents {
    pub cpuid_brand: String,
    pub cpuid_topology: String,
    pub disk_serial: String,
    pub motherboard_uuid: String,
    pub mac_addr: String,
    pub os_machine_id: String,
    pub gpu_device_id: String,
    pub tpm_ek_fingerprint: Option<String>,
}

pub fn collect_components() -> HardwareComponents {
    let u = userspace::collect();
    let tpm = tpm::ek_fingerprint().map(hex::encode);
    HardwareComponents {
        cpuid_brand: u.cpuid_brand,
        cpuid_topology: u.cpuid_topology,
        disk_serial: u.disk_serial,
        motherboard_uuid: u.motherboard_uuid,
        mac_addr: u.mac_addr,
        os_machine_id: u.os_machine_id,
        gpu_device_id: u.gpu_device_id,
        tpm_ek_fingerprint: tpm,
    }
}

pub fn machine_id_bytes() -> [u8; 32] {
    let c = collect_components();
    let encoded = serde_json::to_vec(&c).unwrap_or_default();
    let mut hasher = Sha256::new();
    hasher.update(encoded);
    let out = hasher.finalize();
    let mut arr = [0_u8; 32];
    arr.copy_from_slice(&out);
    arr
}

pub fn machine_id_hex() -> String {
    hex::encode(machine_id_bytes())
}
