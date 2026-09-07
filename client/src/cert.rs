// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Rithul Kamesh
// Author: Rithul Kamesh <hi@rithul.dev>
// Description: client/src/cert.rs — Certificate parsing and Ed25519 chain verification for Licensify.

use std::time::{SystemTime, UNIX_EPOCH};

use ed25519_dalek::{Signature, Verifier, VerifyingKey};
use x509_cert::der::{oid::ObjectIdentifier, Decode, Encode};
use x509_cert::ext::pkix::BasicConstraints;
use x509_cert::Certificate;

use crate::error::LicenseError;

/// OID for Ed25519 (RFC 8410). Used for both the SPKI algorithm and the
/// certificate signature algorithm — Licensify's CA hierarchy is Ed25519 only.
const OID_ED25519: ObjectIdentifier = ObjectIdentifier::new_unwrap("1.3.101.112");

/// Verifies a `leaf → intermediate → root` Ed25519 certificate chain against the
/// current system clock. This performs real cryptographic validation:
///
/// * every certificate is inside its `notBefore..notAfter` window,
/// * `root` and `intermediate` assert `basicConstraints: cA=TRUE`,
/// * `root` carries a valid self-signature,
/// * `intermediate` is signed by `root`'s key,
/// * `leaf` is signed by `intermediate`'s key.
///
/// It does **not** anchor `root` to an external trust store — the caller is
/// responsible for pinning the expected root (Licensify does this via the
/// server's published CA material).
pub fn verify_chain(root_der: &[u8], intermediate_der: &[u8], leaf_der: &[u8]) -> Result<(), LicenseError> {
    let now = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .map_err(|e| LicenseError::Crypto(e.to_string()))?
        .as_secs() as i64;
    verify_chain_at(root_der, intermediate_der, leaf_der, now)
}

/// [`verify_chain`] with an explicit `now` (seconds since the Unix epoch), so the
/// validity-window checks are deterministically testable.
pub fn verify_chain_at(
    root_der: &[u8],
    intermediate_der: &[u8],
    leaf_der: &[u8],
    now_unix: i64,
) -> Result<(), LicenseError> {
    let root = parse(root_der)?;
    let intermediate = parse(intermediate_der)?;
    let leaf = parse(leaf_der)?;

    for cert in [&root, &intermediate, &leaf] {
        check_validity(cert, now_unix)?;
    }

    require_ca(&root)?;
    require_ca(&intermediate)?;

    verify_signed_by(&root, &root)?;
    verify_signed_by(&intermediate, &root)?;
    verify_signed_by(&leaf, &intermediate)?;

    Ok(())
}

fn parse(der: &[u8]) -> Result<Certificate, LicenseError> {
    Certificate::from_der(der).map_err(|e| LicenseError::Crypto(e.to_string()))
}

fn check_validity(cert: &Certificate, now_unix: i64) -> Result<(), LicenseError> {
    let validity = &cert.tbs_certificate.validity;
    let not_before = validity.not_before.to_unix_duration().as_secs() as i64;
    let not_after = validity.not_after.to_unix_duration().as_secs() as i64;
    if now_unix < not_before || now_unix > not_after {
        return Err(LicenseError::InvalidCertificate);
    }
    Ok(())
}

fn require_ca(cert: &Certificate) -> Result<(), LicenseError> {
    match cert.tbs_certificate.get::<BasicConstraints>() {
        Ok(Some((_critical, bc))) if bc.ca => Ok(()),
        _ => Err(LicenseError::InvalidCertificate),
    }
}

/// Verifies that `child`'s signature was produced by `issuer`'s Ed25519 key over
/// `child`'s TBSCertificate.
fn verify_signed_by(child: &Certificate, issuer: &Certificate) -> Result<(), LicenseError> {
    let spki = &issuer.tbs_certificate.subject_public_key_info;
    if spki.algorithm.oid != OID_ED25519 || child.signature_algorithm.oid != OID_ED25519 {
        return Err(LicenseError::InvalidCertificate);
    }

    let pk_bytes = spki
        .subject_public_key
        .as_bytes()
        .ok_or(LicenseError::InvalidCertificate)?;
    let pk: [u8; 32] = pk_bytes.try_into().map_err(|_| LicenseError::InvalidCertificate)?;
    let verifying_key = VerifyingKey::from_bytes(&pk).map_err(|_| LicenseError::InvalidCertificate)?;

    let sig_bytes = child
        .signature
        .as_bytes()
        .ok_or(LicenseError::InvalidCertificate)?;
    let sig: [u8; 64] = sig_bytes.try_into().map_err(|_| LicenseError::InvalidCertificate)?;
    let signature = Signature::from_bytes(&sig);

    let tbs = child
        .tbs_certificate
        .to_der()
        .map_err(|e| LicenseError::Crypto(e.to_string()))?;

    verifying_key
        .verify(&tbs, &signature)
        .map_err(|_| LicenseError::InvalidCertificate)
}

#[cfg(test)]
mod tests {
    use super::*;
    use rcgen::{BasicConstraints as RcBasicConstraints, CertificateParams, IsCa, KeyPair, PKCS_ED25519};

    struct Chain {
        root: Vec<u8>,
        intermediate: Vec<u8>,
        leaf: Vec<u8>,
    }

    fn build_chain() -> Chain {
        let root_key = KeyPair::generate_for(&PKCS_ED25519).unwrap();
        let mut root_params = CertificateParams::new(Vec::<String>::new()).unwrap();
        root_params.is_ca = IsCa::Ca(RcBasicConstraints::Unconstrained);
        let root = root_params.self_signed(&root_key).unwrap();

        let int_key = KeyPair::generate_for(&PKCS_ED25519).unwrap();
        let mut int_params = CertificateParams::new(Vec::<String>::new()).unwrap();
        int_params.is_ca = IsCa::Ca(RcBasicConstraints::Constrained(0));
        let intermediate = int_params.signed_by(&int_key, &root, &root_key).unwrap();

        let leaf_key = KeyPair::generate_for(&PKCS_ED25519).unwrap();
        let mut leaf_params = CertificateParams::new(vec!["licensify-leaf".to_string()]).unwrap();
        leaf_params.is_ca = IsCa::NoCa;
        let leaf = leaf_params.signed_by(&leaf_key, &intermediate, &int_key).unwrap();

        Chain {
            root: root.der().to_vec(),
            intermediate: intermediate.der().to_vec(),
            leaf: leaf.der().to_vec(),
        }
    }

    // A representative "now" inside the rcgen default validity window (1975..4096).
    const NOW: i64 = 1_700_000_000;

    #[test]
    fn accepts_a_valid_ed25519_chain() {
        let c = build_chain();
        verify_chain_at(&c.root, &c.intermediate, &c.leaf, NOW).unwrap();
    }

    #[test]
    fn verify_chain_uses_system_clock() {
        let c = build_chain();
        // The wall clock is well within 1975..4096, so this must succeed.
        verify_chain(&c.root, &c.intermediate, &c.leaf).unwrap();
    }

    #[test]
    fn rejects_non_der_inputs() {
        let c = build_chain();
        assert!(matches!(
            verify_chain_at(b"not der", &c.intermediate, &c.leaf, NOW).unwrap_err(),
            LicenseError::Crypto(_)
        ));
        assert!(matches!(
            verify_chain_at(&c.root, b"not der", &c.leaf, NOW).unwrap_err(),
            LicenseError::Crypto(_)
        ));
        assert!(matches!(
            verify_chain_at(&c.root, &c.intermediate, b"not der", NOW).unwrap_err(),
            LicenseError::Crypto(_)
        ));
    }

    #[test]
    fn rejects_swapped_intermediate_and_leaf() {
        let c = build_chain();
        // leaf is not a CA and does not sign the intermediate.
        let err = verify_chain_at(&c.root, &c.leaf, &c.intermediate, NOW).unwrap_err();
        assert!(matches!(err, LicenseError::InvalidCertificate));
    }

    #[test]
    fn rejects_leaf_signed_by_a_foreign_intermediate() {
        let a = build_chain();
        let b = build_chain();
        // b.leaf was signed by b.intermediate, not a.intermediate.
        let err = verify_chain_at(&a.root, &a.intermediate, &b.leaf, NOW).unwrap_err();
        assert!(matches!(err, LicenseError::InvalidCertificate));
    }

    #[test]
    fn rejects_wrong_root_for_intermediate() {
        let a = build_chain();
        let b = build_chain();
        let err = verify_chain_at(&b.root, &a.intermediate, &a.leaf, NOW).unwrap_err();
        assert!(matches!(err, LicenseError::InvalidCertificate));
    }

    #[test]
    fn rejects_expired_chain() {
        let c = build_chain();
        // Year ~9999: past the rcgen default notAfter (4096-01-01).
        let err = verify_chain_at(&c.root, &c.intermediate, &c.leaf, 253_402_300_799).unwrap_err();
        assert!(matches!(err, LicenseError::InvalidCertificate));
    }

    #[test]
    fn rejects_not_yet_valid_chain() {
        let c = build_chain();
        // 1970: before the rcgen default notBefore (1975-01-01).
        let err = verify_chain_at(&c.root, &c.intermediate, &c.leaf, 0).unwrap_err();
        assert!(matches!(err, LicenseError::InvalidCertificate));
    }

    #[test]
    fn rejects_leaf_in_root_position() {
        let c = build_chain();
        // leaf has no basicConstraints cA=TRUE, so it cannot stand in as the root.
        let err = verify_chain_at(&c.leaf, &c.intermediate, &c.leaf, NOW).unwrap_err();
        assert!(matches!(err, LicenseError::InvalidCertificate));
    }
}
