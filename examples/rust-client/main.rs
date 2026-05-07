// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Rithul Kamesh
// Author: Rithul Kamesh <hi@rithul.dev>
// Description: examples/rust-client/main.rs — Minimal Rust client usage example.

use licensify_client::{ClientConfig, LicensifyClient};

fn main() -> Result<(), Box<dyn std::error::Error>> {
    let cfg = ClientConfig {
        server_url: "http://localhost:8080".to_string(),
        cache_path: std::env::temp_dir().join("licensify.token"),
        server_public_key: [0u8; 32],
    };
    let mut client = LicensifyClient::new(cfg)?;
    client.activate("LICENSE-KEY-DEV")?;
    let status = client.check()?;
    println!("{status:?}");
    Ok(())
}
