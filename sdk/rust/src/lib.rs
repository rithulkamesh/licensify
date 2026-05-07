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
//! use licensify_client_sdk_rust::{ClientConfig, LicensifyClient};
//! # fn main() -> Result<(), licensify_client_sdk_rust::LicenseError> {
//! let cfg = ClientConfig {
//!   server_url: "http://localhost:8080".to_string(),
//!   cache_path: std::env::temp_dir().join("licensify.token"),
//!   server_public_key: [0u8; 32],
//! };
//! let mut c = LicensifyClient::new(cfg)?;
//! c.activate("LICENSE-KEY")?;
//! let _ = c.check()?;
//! # Ok(())
//! # }
//! ```
pub use licensify_client::{
    ActivationResult, ClientConfig, Entitlements, LicenseError, LicenseStatus, LicensifyClient, ValidationSource,
};
