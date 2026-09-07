// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Rithul Kamesh
// Author: Rithul Kamesh <hi@rithul.dev>
// Description: sdk/rust/src/lib.rs — Rust SDK surface for consuming Licensify from Rust without FFI.

//! Licensify Rust SDK
//!
//! This crate is an idiomatic Rust re-export of the core `licensify-client` API.
//! Prefer using this crate in Rust applications; other languages should use the stable C ABI.
//!
//! ```no_run
//! use licensify_client_sdk_rust::{ClientConfig, LicensifyClient, LicenseError};
//! # fn main() -> Result<(), LicenseError> {
//! let cfg = ClientConfig {
//!   server_url: "http://localhost:8080".to_string(),
//!   cache_path: std::env::temp_dir().join("licensify.token"),
//!   server_public_key: [0u8; 32],
//!   expected_digest: None,
//! };
//! let mut c = LicensifyClient::new(cfg)?;
//! c.activate("LICENSE-KEY")?;
//! let _ = c.check()?;
//! # Ok(())
//! # }
//! ```
pub use licensify::{
    noop_integrity_check, ActivationResult, ClientConfig, Entitlements, LicenseError, LicenseStatus,
    LicenseType, LicensifyClient, ValidationSource,
};

#[cfg(test)]
mod tests {
    use super::*;

    fn cfg() -> ClientConfig {
        ClientConfig {
            server_url: "http://localhost:0".into(),
            cache_path: std::env::temp_dir().join("licensify-rust-sdk-test.token"),
            server_public_key: [0u8; 32],
            expected_digest: None,
        }
    }

    #[test]
    fn re_export_lifecycle() {
        let mut c = LicensifyClient::with_integrity_check(cfg(), noop_integrity_check()).unwrap();
        let act = c.activate("KEY").unwrap();
        assert!(matches!(act.entitlements.license_type, LicenseType::Perpetual));
        assert!(c.has_feature("base"));
        let _: ActivationResult = act;
        let _: Entitlements = c.entitlements().unwrap();
        let st = c.check().unwrap();
        match st {
            LicenseStatus::Valid { source, .. } => {
                let _ = source == ValidationSource::OfflineCache;
            }
            LicenseStatus::Invalid => {}
            _ => panic!("unexpected status"),
        }
        c.deactivate().unwrap();
        assert!(matches!(c.entitlements(), Err(LicenseError::Inactive)));
    }
}
