// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Rithul Kamesh
// Author: Rithul Kamesh <hi@rithul.dev>
// Description: client/tests/token_cache_tests.rs — Integration tests for offline token caching behavior.

use ed25519_dalek::{SigningKey, Signer};
use sha2::Digest;

fn build_token(signing_key: &SigningKey, entitlements: &[u8], machine_id: [u8; 32], now: i64, ttl_secs: i64) -> Vec<u8> {
    let mut out = Vec::new();
    out.push(1u8); // version
    out.extend_from_slice(&[0x11u8; 16]); // license_id bytes
    out.extend_from_slice(&machine_id);
    out.extend_from_slice(&(now.to_be_bytes()));
    out.extend_from_slice(&((now + ttl_secs).to_be_bytes()));
    out.extend_from_slice(&((entitlements.len() as u32).to_be_bytes()));
    out.extend_from_slice(entitlements);
    out.extend_from_slice(&[0x22u8; 32]); // nonce
    let sig = signing_key.sign(&out);
    out.extend_from_slice(&sig.to_bytes());
    out
}

#[test]
fn token_roundtrip_verify() {
    let signing_key = SigningKey::from_bytes(&[7u8; 32]);
    let verifying_key = signing_key.verifying_key();
    let mut machine_id = [0u8; 32];
    machine_id.copy_from_slice(&sha2::Sha256::digest(b"mid")[..]);
    let now = 1_700_000_000i64;
    let tok = build_token(&signing_key, b"{\"features\":[\"pro\"]}", machine_id, now, 60);
    let parsed = licensify_client::token::verify(&tok, &verifying_key.to_bytes(), now).expect("verify ok");
    assert_eq!(parsed.version, 1);
    assert_eq!(parsed.machine_id, machine_id);
}

#[test]
fn cache_encrypt_decrypt_is_lossless_and_machine_bound() {
    std::env::set_var("LICENSIFY_MACHINE_ID", "test-machine");
    let token = b"hello-token-bytes";
    let enc = licensify_client::cache::encrypt_token(token).expect("encrypt");
    let dec = licensify_client::cache::decrypt_token(&enc).expect("decrypt");
    assert_eq!(dec, token);

    std::env::set_var("LICENSIFY_MACHINE_ID", "different-machine");
    let wrong = licensify_client::cache::decrypt_token(&enc);
    assert!(wrong.is_err());
}

