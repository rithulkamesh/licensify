// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Rithul Kamesh
// Author: Rithul Kamesh <hi@rithul.dev>
// Description: examples/rust-client/main.rs — Minimal Rust client usage example.

use licensify::{ClientConfig, LicenseStatus, LicensifyClient};

fn main() -> Result<(), Box<dyn std::error::Error>> {
    let cfg = ClientConfig {
        server_url: std::env::var("LICENSIFY_BASE_URL").unwrap_or_else(|_| "http://localhost:8080".to_string()),
        cache_path: std::env::temp_dir().join("licensify.token"),
        server_public_key: [0u8; 32],
        expected_digest: None,
    };
    let mut client = LicensifyClient::new(cfg)?;
    let key = std::env::var("LICENSIFY_LICENSE_KEY").unwrap_or_else(|_| "LICENSE-KEY-DEV".to_string());
    client.activate(&key)?;
    let status = client.check()?;
    let label = match status {
        LicenseStatus::Valid { .. } => "Valid",
        LicenseStatus::Invalid => "Invalid",
        LicenseStatus::Expired => "Expired",
        LicenseStatus::OfflineGrace { .. } => "OfflineGrace",
        LicenseStatus::TrialExpired => "TrialExpired",
    };
    println!("rust-client-example status={label}");
    println!("rust-client-example: success");
    Ok(())
}
