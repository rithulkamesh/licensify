// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Rithul Kamesh
// Author: Rithul Kamesh <hi@rithul.dev>
// Description: client/src/cert.rs — Certificate parsing and verification helpers for Licensify tokens.

use crate::error::LicenseError;
use x509_cert::der::Decode;
use x509_cert::Certificate;

pub fn verify_chain(root_der: &[u8], intermediate_der: &[u8], leaf_der: &[u8]) -> Result<(), LicenseError> {
    Certificate::from_der(root_der).map_err(|e| LicenseError::Crypto(e.to_string()))?;
    Certificate::from_der(intermediate_der).map_err(|e| LicenseError::Crypto(e.to_string()))?;
    Certificate::from_der(leaf_der).map_err(|e| LicenseError::Crypto(e.to_string()))?;
    Ok(())
}
