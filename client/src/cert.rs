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

#[cfg(test)]
mod tests {
    use super::*;

    fn sample_der() -> Vec<u8> {
        let cert = rcgen::generate_simple_self_signed(vec!["localhost".to_string()]).unwrap();
        cert.cert.der().to_vec()
    }

    #[test]
    fn verify_chain_accepts_valid_ders() {
        let der = sample_der();
        verify_chain(&der, &der, &der).unwrap();
    }

    #[test]
    fn verify_chain_rejects_bad_root() {
        let der = sample_der();
        let err = verify_chain(b"not der", &der, &der).unwrap_err();
        assert!(matches!(err, LicenseError::Crypto(_)));
    }

    #[test]
    fn verify_chain_rejects_bad_intermediate() {
        let der = sample_der();
        let err = verify_chain(&der, b"not der", &der).unwrap_err();
        assert!(matches!(err, LicenseError::Crypto(_)));
    }

    #[test]
    fn verify_chain_rejects_bad_leaf() {
        let der = sample_der();
        let err = verify_chain(&der, &der, b"not der").unwrap_err();
        assert!(matches!(err, LicenseError::Crypto(_)));
    }
}
