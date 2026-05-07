// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Rithul Kamesh
// Author: Rithul Kamesh <hi@rithul.dev>
// Description: client/src/error.rs — Error types and error-code mapping for the Rust client and FFI.

use thiserror::Error;

#[derive(Debug, Error)]
pub enum LicenseError {
    #[error("io error: {0}")]
    Io(String),
    #[error("network error: {0}")]
    Network(String),
    #[error("crypto error: {0}")]
    Crypto(String),
    #[error("invalid token")]
    InvalidToken,
    #[error("expired token")]
    Expired,
    #[error("invalid certificate")]
    InvalidCertificate,
    #[error("license inactive")]
    Inactive,
}

impl From<std::io::Error> for LicenseError {
    fn from(value: std::io::Error) -> Self {
        Self::Io(value.to_string())
    }
}
