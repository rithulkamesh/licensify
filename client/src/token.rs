// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Rithul Kamesh
// Author: Rithul Kamesh <hi@rithul.dev>
// Description: client/src/token.rs — License token parsing, validation, and entitlement checks.

use ed25519_dalek::{Signature, Verifier, VerifyingKey};

use crate::error::LicenseError;

#[derive(Debug, Clone)]
pub struct LicenseToken {
    pub version: u8,
    pub license_id: [u8; 16],
    pub machine_id: [u8; 32],
    pub issued_at: i64,
    pub expires_at: i64,
    pub entitlements: Vec<u8>,
    pub nonce: [u8; 32],
    pub signature: [u8; 64],
}

pub fn parse(data: &[u8]) -> Result<LicenseToken, LicenseError> {
    if data.len() < 1 + 16 + 32 + 8 + 8 + 4 + 32 + 64 {
        return Err(LicenseError::InvalidToken);
    }
    let mut at = 0_usize;
    let version = data[at];
    at += 1;
    let mut license_id = [0_u8; 16];
    license_id.copy_from_slice(&data[at..at + 16]);
    at += 16;
    let mut machine_id = [0_u8; 32];
    machine_id.copy_from_slice(&data[at..at + 32]);
    at += 32;
    let issued_at = i64::from_be_bytes(data[at..at + 8].try_into().map_err(|_| LicenseError::InvalidToken)?);
    at += 8;
    let expires_at = i64::from_be_bytes(data[at..at + 8].try_into().map_err(|_| LicenseError::InvalidToken)?);
    at += 8;
    let ent_len = u32::from_be_bytes(data[at..at + 4].try_into().map_err(|_| LicenseError::InvalidToken)?) as usize;
    at += 4;
    if at + ent_len + 32 + 64 > data.len() {
        return Err(LicenseError::InvalidToken);
    }
    let entitlements = data[at..at + ent_len].to_vec();
    at += ent_len;
    let mut nonce = [0_u8; 32];
    nonce.copy_from_slice(&data[at..at + 32]);
    at += 32;
    let mut signature = [0_u8; 64];
    signature.copy_from_slice(&data[at..at + 64]);
    Ok(LicenseToken { version, license_id, machine_id, issued_at, expires_at, entitlements, nonce, signature })
}

pub fn verify(token_bytes: &[u8], pubkey_bytes: &[u8; 32], now: i64) -> Result<LicenseToken, LicenseError> {
    let token = parse(token_bytes)?;
    let body_len = token_bytes.len().saturating_sub(64);
    let verifying_key = VerifyingKey::from_bytes(pubkey_bytes).map_err(|e| LicenseError::Crypto(e.to_string()))?;
    let sig = Signature::from_bytes(&token.signature);
    verifying_key
        .verify(&token_bytes[..body_len], &sig)
        .map_err(|_| LicenseError::InvalidToken)?;
    if now > token.expires_at {
        return Err(LicenseError::Expired);
    }
    Ok(token)
}

#[cfg(test)]
mod tests {
    use super::*;
    use ed25519_dalek::{Signer, SigningKey};

    fn build(signing: &SigningKey, ent: &[u8], now: i64, ttl: i64) -> Vec<u8> {
        let mut out = Vec::new();
        out.push(1u8);
        out.extend_from_slice(&[0xAAu8; 16]);
        out.extend_from_slice(&[0xBBu8; 32]);
        out.extend_from_slice(&now.to_be_bytes());
        out.extend_from_slice(&(now + ttl).to_be_bytes());
        out.extend_from_slice(&(ent.len() as u32).to_be_bytes());
        out.extend_from_slice(ent);
        out.extend_from_slice(&[0xCCu8; 32]);
        let sig = signing.sign(&out);
        out.extend_from_slice(&sig.to_bytes());
        out
    }

    #[test]
    fn parse_rejects_too_short() {
        let err = parse(&[0u8; 10]).unwrap_err();
        assert!(matches!(err, LicenseError::InvalidToken));
    }

    #[test]
    fn parse_rejects_bad_entlen() {
        // Construct a buffer with the minimum prefix but a giant ent_len.
        let mut buf = vec![1u8; 1 + 16 + 32 + 8 + 8];
        buf.extend_from_slice(&u32::MAX.to_be_bytes()); // ent_len that overflows
        buf.extend_from_slice(&[0u8; 32 + 64]);
        let err = parse(&buf).unwrap_err();
        assert!(matches!(err, LicenseError::InvalidToken));
    }

    #[test]
    fn verify_happy_path() {
        let key = SigningKey::from_bytes(&[7u8; 32]);
        let pk = key.verifying_key();
        let tok = build(&key, b"{\"f\":[\"a\"]}", 1_700_000_000, 60);
        let parsed = verify(&tok, &pk.to_bytes(), 1_700_000_000).unwrap();
        assert_eq!(parsed.version, 1);
        assert_eq!(parsed.entitlements, b"{\"f\":[\"a\"]}");
    }

    #[test]
    fn verify_expired_token() {
        let key = SigningKey::from_bytes(&[7u8; 32]);
        let pk = key.verifying_key();
        let tok = build(&key, b"x", 1_700_000_000, 10);
        let err = verify(&tok, &pk.to_bytes(), 1_700_000_100).unwrap_err();
        assert!(matches!(err, LicenseError::Expired));
    }

    #[test]
    fn verify_bad_signature() {
        let key = SigningKey::from_bytes(&[7u8; 32]);
        let other = SigningKey::from_bytes(&[8u8; 32]).verifying_key();
        let tok = build(&key, b"x", 1_700_000_000, 60);
        let err = verify(&tok, &other.to_bytes(), 1_700_000_000).unwrap_err();
        assert!(matches!(err, LicenseError::InvalidToken));
    }

    #[test]
    fn verify_with_unrelated_pubkey_returns_invalid_token() {
        // ed25519-dalek's `VerifyingKey::from_bytes` only does basic encoding
        // sanity, so an unrelated-but-valid pubkey reaches `verify()` and
        // surfaces as `LicenseError::InvalidToken` rather than `Crypto`.
        let key = SigningKey::from_bytes(&[7u8; 32]);
        let other_pub = SigningKey::from_bytes(&[42u8; 32]).verifying_key();
        let tok = build(&key, b"x", 1_700_000_000, 60);
        let err = verify(&tok, &other_pub.to_bytes(), 1_700_000_000).unwrap_err();
        assert!(matches!(err, LicenseError::InvalidToken));
    }
}
