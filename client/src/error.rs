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

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn from_io_error_uses_io_variant() {
        let io = std::io::Error::new(std::io::ErrorKind::Other, "boom");
        let e: LicenseError = io.into();
        assert!(matches!(e, LicenseError::Io(_)));
    }

    #[test]
    fn display_messages_for_every_variant() {
        let cases: &[(LicenseError, &str)] = &[
            (LicenseError::Io("io".into()), "io error: io"),
            (LicenseError::Network("nw".into()), "network error: nw"),
            (LicenseError::Crypto("c".into()), "crypto error: c"),
            (LicenseError::InvalidToken, "invalid token"),
            (LicenseError::Expired, "expired token"),
            (LicenseError::InvalidCertificate, "invalid certificate"),
            (LicenseError::Inactive, "license inactive"),
        ];
        for (err, want) in cases {
            assert_eq!(err.to_string(), *want);
        }
    }
}
